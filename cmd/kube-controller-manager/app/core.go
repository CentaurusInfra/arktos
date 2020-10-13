/*
Copyright 2016 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package app implements a server that runs a set of active
// components.  This includes replication controllers, service endpoints and
// nodes.
//
package app

import (
	"fmt"
	"net/http"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	"k8s.io/client-go/discovery"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	endpointcontroller "k8s.io/kubernetes/pkg/controller/endpoint"
	"k8s.io/kubernetes/pkg/controller/garbagecollector"
	namespacecontroller "k8s.io/kubernetes/pkg/controller/namespace"
	"k8s.io/kubernetes/pkg/controller/podgc"
	serviceaccountcontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	tenantcontroller "k8s.io/kubernetes/pkg/controller/tenant"
	"k8s.io/kubernetes/pkg/controller/vmpod"
)

func startEndpointController(ctx ControllerContext) (http.Handler, bool, error) {
	go endpointcontroller.NewEndpointController(
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.InformerFactory.Core().V1().Services(),
		ctx.InformerFactory.Core().V1().Endpoints(),
		ctx.ClientBuilder.ClientOrDie("endpoint-controller"),
	).Run(int(ctx.ComponentConfig.EndpointController.ConcurrentEndpointSyncs), ctx.Stop)
	return nil, true, nil
}

func startPodGCController(ctx ControllerContext) (http.Handler, bool, error) {
	go podgc.NewPodGC(
		ctx.ClientBuilder.ClientOrDie("pod-garbage-collector"),
		ctx.InformerFactory.Core().V1().Pods(),
		int(ctx.ComponentConfig.PodGCController.TerminatedPodGCThreshold),
	).Run(ctx.Stop)
	return nil, true, nil
}

func startVMPodController(ctx ControllerContext) (http.Handler, bool, error) {
	go vmpod.NewVMPod(
		ctx.ClientBuilder.ClientOrDie("vm-pod-controller"),
		ctx.InformerFactory.Core().V1().Pods(),
	).Run(ctx.Stop)
	return nil, true, nil
}

func startNamespaceController(ctx ControllerContext) (http.Handler, bool, error) {
	// the namespace cleanup controller is very chatty.  It makes lots of discovery calls and then it makes lots of delete calls
	// the ratelimiter negatively affects its speed.  Deleting 100 total items in a namespace (that's only a few of each resource
	// including events), takes ~10 seconds by default.
	nsKubeconfigs := ctx.ClientBuilder.ConfigOrDie("namespace-controller")
	for _, nsKubeconfig := range nsKubeconfigs.GetAllConfigs() {
		nsKubeconfig.QPS *= 20
		nsKubeconfig.Burst *= 100
	}
	namespaceKubeClient := clientset.NewForConfigOrDie(nsKubeconfigs)
	return startModifiedNamespaceController(ctx, namespaceKubeClient, nsKubeconfigs)
}

func startTenantController(ctx ControllerContext) (http.Handler, bool, error) {
	// tenant controller will be even more chatty than namespace controller when doing cleanups.
	// but tenant deletion probably doesn't have to be very quick. For now we keep the same
	// QPS/Burst settings as namespace controller. After we implement tenant cleanup, We will
	// adjust these if there are feedback on the duration of tenant cleanup
	tnKubeConfigs := ctx.ClientBuilder.ConfigOrDie("tenant-controller")
	for _, tnKubeConfig := range tnKubeConfigs.GetAllConfigs() {
		tnKubeConfig.QPS *= 20
		tnKubeConfig.Burst *= 100
	}
	tenantKubeClient := clientset.NewForConfigOrDie(tnKubeConfigs)

	crConfigs := *tnKubeConfigs
	for _, cfg := range crConfigs.GetAllConfigs() {
		cfg.ContentType = "application/json"
		cfg.AcceptContentTypes = "application/json"
	}
	networkClient := arktos.NewForConfigOrDie(&crConfigs)

	dynamicClient, err := dynamic.NewForConfig(tnKubeConfigs)
	if err != nil {
		return nil, true, err
	}

	discoverTenantedResourcesFn := func() ([]*metav1.APIResourceList, error) {
		all, err := tenantKubeClient.Discovery().ServerPreferredResources()
		return discovery.FilteredBy(discovery.ResourcePredicateFunc(func(groupVersion string, r *metav1.APIResource) bool {
			return !r.Namespaced && r.Tenanted
		}), all), err
	}

	tenantController := tenantcontroller.NewTenantController(tenantKubeClient,
		ctx.InformerFactory.Core().V1().Tenants(),
		ctx.InformerFactory.Core().V1().Namespaces(),
		ctx.InformerFactory.Rbac().V1().ClusterRoles(),
		ctx.InformerFactory.Rbac().V1().ClusterRoleBindings(),
		ctx.ComponentConfig.TenantController.TenantSyncPeriod.Duration,
		networkClient,
		ctx.ComponentConfig.TenantController.DefaultNetworkTemplatePath,
		dynamicClient,
		discoverTenantedResourcesFn,
		v1.FinalizerArktos)
	go tenantController.Run(int(ctx.ComponentConfig.TenantController.ConcurrentTenantSyncs), ctx.Stop)

	return nil, true, nil
}

func startModifiedNamespaceController(ctx ControllerContext, namespaceKubeClient clientset.Interface, nsKubeconfig *restclient.Config) (http.Handler, bool, error) {

	dynamicClient, err := dynamic.NewForConfig(nsKubeconfig)
	if err != nil {
		return nil, true, err
	}

	discoverResourcesFn := namespaceKubeClient.Discovery().ServerPreferredNamespacedResources

	namespaceController := namespacecontroller.NewNamespaceController(
		namespaceKubeClient,
		dynamicClient,
		discoverResourcesFn,
		ctx.InformerFactory.Core().V1().Namespaces(),
		ctx.ComponentConfig.NamespaceController.NamespaceSyncPeriod.Duration,
		v1.FinalizerKubernetes,
	)
	go namespaceController.Run(int(ctx.ComponentConfig.NamespaceController.ConcurrentNamespaceSyncs), ctx.Stop)

	return nil, true, nil
}

func startServiceAccountController(ctx ControllerContext) (http.Handler, bool, error) {
	sac, err := serviceaccountcontroller.NewServiceAccountsController(
		ctx.InformerFactory.Core().V1().ServiceAccounts(),
		ctx.InformerFactory.Core().V1().Namespaces(),
		ctx.ClientBuilder.ClientOrDie("service-account-controller"),
		serviceaccountcontroller.DefaultServiceAccountsControllerOptions(),
	)
	if err != nil {
		return nil, true, fmt.Errorf("error creating ServiceAccount controller: %v", err)
	}
	go sac.Run(1, ctx.Stop)
	return nil, true, nil
}

func startGarbageCollectorController(ctx ControllerContext) (http.Handler, bool, error) {
	if !ctx.ComponentConfig.GarbageCollectorController.EnableGarbageCollector {
		return nil, false, nil
	}

	gcClientset := ctx.ClientBuilder.ClientOrDie("generic-garbage-collector")
	discoveryClient := cacheddiscovery.NewMemCacheClient(gcClientset.Discovery())

	config := ctx.ClientBuilder.ConfigOrDie("generic-garbage-collector")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, true, err
	}

	// Get an initial set of deletable resources to prime the garbage collector.
	deletableResources := garbagecollector.GetDeletableResources(discoveryClient)
	ignoredResources := make(map[schema.GroupResource]struct{})
	for _, r := range ctx.ComponentConfig.GarbageCollectorController.GCIgnoredResources {
		ignoredResources[schema.GroupResource{Group: r.Group, Resource: r.Resource}] = struct{}{}
	}
	garbageCollector, err := garbagecollector.NewGarbageCollector(
		dynamicClient,
		ctx.RESTMapper,
		deletableResources,
		ignoredResources,
		ctx.GenericInformerFactory,
		ctx.InformersStarted,
	)
	if err != nil {
		return nil, true, fmt.Errorf("failed to start the generic garbage collector: %v", err)
	}

	// Start the garbage collector.
	workers := int(ctx.ComponentConfig.GarbageCollectorController.ConcurrentGCSyncs)
	go garbageCollector.Run(workers, ctx.Stop)

	// Periodically refresh the RESTMapper with new discovery information and sync
	// the garbage collector.
	go garbageCollector.Sync(gcClientset.Discovery(), 30*time.Second, ctx.Stop)

	return garbagecollector.NewDebugHandler(garbageCollector), true, nil
}
