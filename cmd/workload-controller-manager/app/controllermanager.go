/*
Copyright 2020 Authors of Arktos.

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

package app

import (
	"context"
	"fmt"
	"k8s.io/client-go/datapartition"
	controller "k8s.io/kubernetes/pkg/cloudfabric-controller"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/deployment"
	"net/http"
	"time"

	"github.com/grafov/bcast"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/apiserver/pkg/server/mux"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog"
	"k8s.io/kubernetes/cmd/workload-controller-manager/app/config"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/controllerframework"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/replicaset"
)

// ControllerContext defines the context object for controller
type ControllerContext struct {
	// ClientBuilder will provide a client for this controller to use
	ClientBuilder controller.ControllerClientBuilder

	// InformerFactory gives access to informers for the controller.
	InformerFactory informers.SharedInformerFactory

	// GenericInformerFactory gives access to informers for typed resources
	// and dynamic resources.
	GenericInformerFactory controller.InformerFactory

	// DeferredDiscoveryRESTMapper is a RESTMapper that will defer
	// initialization of the RESTMapper until the first mapping is
	// requested.
	RESTMapper *restmapper.DeferredDiscoveryRESTMapper

	// AvailableResources is a map listing currently available resources
	AvailableResources map[schema.GroupVersionResource]bool

	// Stop is the stop channel
	Stop <-chan struct{}

	// ResetChGroups are the reset channels for informer filters
	ResetChGroups []*bcast.Group

	// InformersStarted is closed after all of the controllers have been initialized and are running.  After this point it is safe,
	// for an individual controller to start the shared informers. Before it is closed, they should not.
	InformersStarted chan struct{}

	// ControllerInstanceUpdateByControllerType is the controller type that has controller instance updates. This is to notify controller instance
	// that it has a peer update and trigger hash ring updates if necessary.
	ControllerInstanceUpdateChGrp *bcast.Group

	// Rest client dedicated to controller heartbeat
	HeartBeatClient clientset.Interface
}

const (
	source_DeploymentController = "Deployment_Controller"
	source_ReplicaSetController = "ReplicaSet_Controller"

	ownerKind_Empty      = ""
	ownerKind_Deployment = "Deployment"
	ownerKind_ReplicaSet = "ReplicaSet"
)

func StartControllerManager(c *config.CompletedConfig, stopCh <-chan struct{}) error {
	// Setup any healthz checks we will want to use.
	var checks []healthz.HealthChecker

	// Start the controller manager HTTP server
	// unsecuredMux is the handler for these controller *after* authn/authz filters have been applied
	var unsecuredMux *mux.PathRecorderMux
	debugConfig := &componentbaseconfig.DebuggingConfiguration{}
	if c.SecureServing != nil {
		unsecuredMux = NewBaseHandler(debugConfig, checks...)
		handler := BuildHandlerChain(unsecuredMux, &c.Authorization, &c.Authentication)
		// TODO: handle stoppedCh returned by c.SecureServing.Serve
		if _, err := c.SecureServing.Serve(handler, 0, stopCh); err != nil {
			return err
		}
	}
	if c.InsecureServing != nil {
		unsecuredMux = NewBaseHandler(debugConfig, checks...)
		insecureSuperuserAuthn := server.AuthenticationInfo{Authenticator: &server.InsecureSuperuser{}}
		handler := BuildHandlerChain(unsecuredMux, nil, &insecureSuperuserAuthn)
		if err := c.InsecureServing.Serve(handler, 0, stopCh); err != nil {
			return err
		}
	}

	rootClientBuilder := controller.SimpleControllerClientBuilder{
		ClientConfig: c.ControllerManagerConfig,
	}

	clientBuilder := rootClientBuilder
	heartBeatClientBuilder := controller.SimpleControllerClientBuilder{ClientConfig: c.ControllerManagerConfig}

	ctx := context.TODO()

	controllerContext, err := CreateControllerContext(rootClientBuilder, clientBuilder, heartBeatClientBuilder,
		c.Config.ControllerTypeConfig.GetDeafultResyncPeriod(), ctx.Done())

	if err != nil {
		klog.Fatalf("error building controller context: %v", err)
	}

	reportHealthIntervalInSecond := c.ControllerTypeConfig.GetReportHealthIntervalInSecond()
	startAPIServerConfigManager(controllerContext)
	startControllerInstanceManager(controllerContext)
	replicatSetWorkerNumber, isOK := c.ControllerTypeConfig.GetWorkerNumber("replicaset")
	if isOK {
		startReplicaSetController(controllerContext, reportHealthIntervalInSecond, replicatSetWorkerNumber)
	}
	deploymentWorkerNumber, isOK := c.ControllerTypeConfig.GetWorkerNumber("deployment")
	if isOK {
		startDeploymentController(controllerContext, reportHealthIntervalInSecond, deploymentWorkerNumber)
	}

	controllerContext.InformerFactory.Start(controllerContext.Stop)
	controllerContext.GenericInformerFactory.Start(controllerContext.Stop)
	close(controllerContext.InformersStarted)

	select {}
	panic("unreachable")
}

//func CreateControllerContext(s *config.CompletedConfig, rootClientBuilder, clientBuilder controller.ControllerClientBuilder, stop <-chan struct{}) (ControllerContext, error) {
func CreateControllerContext(rootClientBuilder, clientBuilder, heatBeatClientBuilder controller.ControllerClientBuilder, resyncPeriod time.Duration, stop <-chan struct{}) (ControllerContext, error) {
	versionedClient := rootClientBuilder.ClientOrDie("shared-informers")
	sharedInformers := informers.NewSharedInformerFactory(versionedClient, resyncPeriod)

	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	if err := WaitForAPIServer(versionedClient, 10*time.Second); err != nil {
		return ControllerContext{}, fmt.Errorf("failed to wait for apiserver being healthy: %v", err)
	}

	// Use a discovery client capable of being refreshed.
	discoveryClient := rootClientBuilder.ClientOrDie("controller-discovery")
	cachedClient := cacheddiscovery.NewMemCacheClient(discoveryClient.Discovery())
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)
	go wait.Until(func() {
		restMapper.Reset()
	}, 30*time.Second, stop)

	heartBeatClient := heatBeatClientBuilder.ClientOrDie("controller-heart-beat")

	availableResources, err := GetAvailableResources(rootClientBuilder)
	if err != nil {
		return ControllerContext{}, err
	}

	ctx := ControllerContext{
		ClientBuilder:                 clientBuilder,
		InformerFactory:               sharedInformers,
		GenericInformerFactory:        controller.NewInformerFactory(sharedInformers),
		RESTMapper:                    restMapper,
		HeartBeatClient:               heartBeatClient,
		AvailableResources:            availableResources,
		Stop:                          stop,
		InformersStarted:              make(chan struct{}),
		ControllerInstanceUpdateChGrp: bcast.NewGroup(),
		ResetChGroups:                 make([]*bcast.Group, 0),
	}

	return ctx, nil
}

// GetAvailableResources gets the map which contains all available resources of the apiserver
// TODO: In general, any controller checking this needs to be dynamic so
// users don't have to restart their controller manager if they change the apiserver.
// Until we get there, the structure here needs to be exposed for the construction of a proper ControllerContext.
func GetAvailableResources(clientBuilder controller.ControllerClientBuilder) (map[schema.GroupVersionResource]bool, error) {
	client := clientBuilder.ClientOrDie("controller-discovery")
	discoveryClient := client.Discovery()
	resourceMap, err := discoveryClient.ServerResources()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get all supported resources from server: %v", err))
	}
	if len(resourceMap) == 0 {
		return nil, fmt.Errorf("unable to get any supported resources from server")
	}

	allResources := map[schema.GroupVersionResource]bool{}
	for _, apiResourceList := range resourceMap {
		version, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			return nil, err
		}

		for _, apiResource := range apiResourceList.APIResources {
			allResources[version.WithResource(apiResource.Name)] = true
		}
	}

	return allResources, nil
}

func startReplicaSetController(ctx ControllerContext, reportHealthIntervalInSecond int, workerNum int) (http.Handler, bool, error) {
	if !ctx.AvailableResources[schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}] {
		return nil, false, nil
	}

	rsResetChGrp := bcast.NewGroup()
	ctx.ResetChGroups = append(ctx.ResetChGroups, rsResetChGrp)
	go rsResetChGrp.Broadcast(0)

	rsInformer := ctx.InformerFactory.Apps().V1().ReplicaSets()
	rsResetCh := rsResetChGrp.Join()
	rsInformer.Informer().AddResetCh(rsResetCh, source_ReplicaSetController, ownerKind_Empty)

	podInformer := ctx.InformerFactory.Core().V1().Pods()
	podResetCh := rsResetChGrp.Join() // for owner reference
	podInformer.Informer().AddResetCh(podResetCh, source_ReplicaSetController, ownerKind_ReplicaSet)
	podResetCh2 := rsResetChGrp.Join() // for adoption
	podInformer.Informer().AddResetCh(podResetCh2, source_ReplicaSetController, ownerKind_Empty)

	cimChangeCh := ctx.ControllerInstanceUpdateChGrp.Join()

	controller := replicaset.NewReplicaSetController(
		rsInformer,
		podInformer,
		ctx.ClientBuilder.ClientOrDie("replicaset-controller"),
		replicaset.BurstReplicas,
		cimChangeCh,
		rsResetChGrp,
	)
	go controller.Run(workerNum, ctx.Stop)
	go wait.Until(func() {
		controller.ReportHealth(ctx.HeartBeatClient)
	}, time.Second*time.Duration(reportHealthIntervalInSecond), ctx.Stop)

	return nil, true, nil
}

func startDeploymentController(ctx ControllerContext, reportHealthIntervalInSecond int, workerNum int) (http.Handler, bool, error) {
	if !ctx.AvailableResources[schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}] {
		return nil, false, nil
	}

	dResetChGrp := bcast.NewGroup()
	ctx.ResetChGroups = append(ctx.ResetChGroups, dResetChGrp)
	go dResetChGrp.Broadcast(0)

	deploymentInformer := ctx.InformerFactory.Apps().V1().Deployments()
	deploymentResetCh := dResetChGrp.Join()
	deploymentInformer.Informer().AddResetCh(deploymentResetCh, source_DeploymentController, ownerKind_Empty)

	rsInformer := ctx.InformerFactory.Apps().V1().ReplicaSets()
	rsResetCh := dResetChGrp.Join()

	rsInformer.Informer().AddResetCh(rsResetCh, source_DeploymentController, ownerKind_Deployment)

	podInformer := ctx.InformerFactory.Core().V1().Pods()
	podResetCh := dResetChGrp.Join() // for owner reference
	podInformer.Informer().AddResetCh(podResetCh, source_DeploymentController, ownerKind_Deployment)
	podResetCh2 := dResetChGrp.Join() // for adoption
	podInformer.Informer().AddResetCh(podResetCh2, source_DeploymentController, ownerKind_Empty)

	cimChangeCh := ctx.ControllerInstanceUpdateChGrp.Join()

	controller, err := deployment.NewDeploymentController(
		deploymentInformer,
		rsInformer,
		podInformer,
		ctx.ClientBuilder.ClientOrDie("deployment-controller"),
		cimChangeCh,
		dResetChGrp,
	)
	if err != nil {
		return nil, true, fmt.Errorf("error creating Deployment controller: %v", err)
	}
	go controller.Run(workerNum, ctx.Stop)
	go wait.Until(func() {
		controller.ReportHealth(ctx.HeartBeatClient)
	}, time.Second*time.Duration(reportHealthIntervalInSecond), ctx.Stop)
	return nil, true, nil
}

func startControllerInstanceManager(ctx ControllerContext) (bool, error) {
	if !ctx.AvailableResources[schema.GroupVersionResource{Group: "", Version: "v1", Resource: "controllerinstances"}] {
		return false, nil
	}

	go ctx.ControllerInstanceUpdateChGrp.Broadcast(0)

	go controllerframework.NewControllerInstanceManager(
		ctx.InformerFactory.Core().V1().ControllerInstances(),
		ctx.ClientBuilder.ClientOrDie("controller-instance-manager"),
		ctx.ControllerInstanceUpdateChGrp).Run(ctx.Stop)

	return true, nil
}

func startAPIServerConfigManager(ctx ControllerContext) (bool, error) {
	if !ctx.AvailableResources[schema.GroupVersionResource{Group: "", Version: "v1", Resource: "endpoints"}] {
		return false, nil
	}

	datapartition.StartAPIServerConfigManager(ctx.InformerFactory.Core().V1().Endpoints(),
		ctx.ClientBuilder.ClientOrDie("apiserver-configuration-manager"), ctx.Stop)

	return true, nil
}
