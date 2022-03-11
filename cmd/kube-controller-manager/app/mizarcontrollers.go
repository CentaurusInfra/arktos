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

// Mizar is a server-less platform for network functions.
// You can read more about it at: https://mizar.readthedocs.io/en/latest/
package app

import (
	"net/http"

	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	"k8s.io/arktos-ext/pkg/generated/informers/externalversions"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	controllers "k8s.io/kubernetes/pkg/controller/mizar"
)

const (
	mizarStarterControllerWorkerCount       = 2
	mizarArktosNetworkControllerWorkerCount = 4
	mizarEndpointsControllerWorkerCount     = 4
	mizarNodeControllerWorkerCount          = 2
	mizarPodControllerWorkerCount           = 4
	mizarServiceControllerWorkerCount       = 4
	mizarNetworkPolicyControllerWorkerCount = 4
	mizarNamespaceControllerWorkerCount     = 4
)

func startMizarStarterController(ctx ControllerContext) (http.Handler, bool, error) {
	controllerName := "mizar-starter-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go controllers.NewMizarStarterController(
		ctx.InformerFactory.Core().V1().ConfigMaps(),
		ctx.ClientBuilder.ClientOrDie(controllerName),
		ctx,
		startHandler,
	).Run(mizarStarterControllerWorkerCount, ctx.Stop)
	return nil, true, nil
}

func startHandler(controllerContext interface{}, grpcHost string) {
	grpcAdaptor := new(controllers.GrpcAdaptor)
	ctx := controllerContext.(ControllerContext)

	// get arktos informer
	kubeConfigs := ctx.ClientBuilder.ConfigOrDie("mizar-controller-shared-informers")
	networkClient := arktos.NewForConfigOrDie(kubeConfigs)
	networkInformerFactory := externalversions.NewSharedInformerFactory(networkClient, 0)

	startMizarEndpointsController(&ctx, grpcHost, grpcAdaptor)
	startMizarNodeController(&ctx, grpcHost, grpcAdaptor)
	startMizarPodController(&ctx, networkInformerFactory, grpcHost, grpcAdaptor)
	startMizarServiceController(&ctx, networkInformerFactory, grpcHost, grpcAdaptor)
	startArktosNetworkController(&ctx, networkInformerFactory, grpcHost, grpcAdaptor)
	startMizarNetworkPolicyController(&ctx, grpcHost, grpcAdaptor)
	startMizarNamespaceController(&ctx, grpcHost, grpcAdaptor)

	networkInformerFactory.Start(ctx.Stop)
}

func startMizarEndpointsController(ctx *ControllerContext, grpcHost string, grpcAdaptor controllers.IGrpcAdaptor) (http.Handler, bool, error) {
	controllerName := "mizar-endpoints-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go controllers.NewMizarEndpointsController(
		ctx.InformerFactory.Core().V1().Endpoints(),
		ctx.InformerFactory.Core().V1().Services(),
		ctx.ClientBuilder.ClientOrDie(controllerName),
		grpcHost,
		grpcAdaptor,
	).Run(mizarEndpointsControllerWorkerCount, ctx.Stop)
	return nil, true, nil
}

func startMizarNodeController(ctx *ControllerContext, grpcHost string, grpcAdaptor controllers.IGrpcAdaptor) (http.Handler, bool, error) {
	controllerName := "mizar-node-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go controllers.NewMizarNodeController(
		ctx.InformerFactory.Core().V1().Nodes(),
		ctx.ResourceProviderNodeInformers,
		ctx.ClientBuilder.ClientOrDie(controllerName),
		grpcHost,
		grpcAdaptor,
	).Run(mizarNodeControllerWorkerCount, ctx.Stop)
	return nil, true, nil
}

func startMizarPodController(ctx *ControllerContext, networkInformerFactory externalversions.SharedInformerFactory, grpcHost string, grpcAdaptor controllers.IGrpcAdaptor) (http.Handler, bool, error) {
	controllerName := "mizar-pod-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go func() {
		podController := controllers.NewMizarPodController(
			ctx.InformerFactory.Core().V1().Pods(),
			ctx.ClientBuilder.ClientOrDie(controllerName),
			networkInformerFactory.Arktos().V1().Networks(),
			grpcHost,
			grpcAdaptor,
		)

		podController.Run(mizarPodControllerWorkerCount, ctx.Stop)
	}()
	return nil, true, nil
}

func startMizarServiceController(ctx *ControllerContext, networkInformerFactory externalversions.SharedInformerFactory, grpcHost string, grpcAdaptor controllers.IGrpcAdaptor) (http.Handler, bool, error) {
	controllerName := "mizar-service-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	svcKubeconfigs := ctx.ClientBuilder.ConfigOrDie(controllerName)
	for _, svcKubeconfig := range svcKubeconfigs.GetAllConfigs() {
		svcKubeconfig.QPS *= 5
		svcKubeconfig.Burst *= 10
	}
	svcKubeClient := clientset.NewForConfigOrDie(svcKubeconfigs)

	crConfigs := *svcKubeconfigs
	for _, cfg := range crConfigs.GetAllConfigs() {
		cfg.ContentType = "application/json"
		cfg.AcceptContentTypes = "application/json"
	}
	networkClient := arktos.NewForConfigOrDie(&crConfigs)

	go controllers.NewMizarServiceController(
		svcKubeClient,
		networkClient,
		ctx.InformerFactory.Core().V1().Services(),
		networkInformerFactory.Arktos().V1().Networks(),
		grpcHost,
		grpcAdaptor,
	).Run(mizarServiceControllerWorkerCount, ctx.Stop)
	return nil, true, nil
}

func startArktosNetworkController(ctx *ControllerContext, networkInformerFactory externalversions.SharedInformerFactory, grpcHost string, grpcAdaptor controllers.IGrpcAdaptor) (http.Handler, bool, error) {
	controllerName := "mizar-arktos-network-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	netKubeconfigs := ctx.ClientBuilder.ConfigOrDie(controllerName)
	for _, netKubeconfig := range netKubeconfigs.GetAllConfigs() {
		netKubeconfig.QPS *= 5
		netKubeconfig.Burst *= 10
	}
	crConfigs := *netKubeconfigs
	for _, cfg := range crConfigs.GetAllConfigs() {
		cfg.ContentType = "application/json"
		cfg.AcceptContentTypes = "application/json"
	}
	networkClient := arktos.NewForConfigOrDie(&crConfigs)
	svcKubeClient := clientset.NewForConfigOrDie(netKubeconfigs)
	dynamicClient := dynamic.NewForConfigOrDie(netKubeconfigs)

	go func() {
		networkController := controllers.NewMizarArktosNetworkController(
			dynamicClient,
			networkClient,
			svcKubeClient,
			networkInformerFactory.Arktos().V1().Networks(),
			grpcHost,
			grpcAdaptor,
			int(ctx.ComponentConfig.MizarArktosNetworkController.VPCRangeStart),
			int(ctx.ComponentConfig.MizarArktosNetworkController.VPCRangeEnd),
		)

		networkController.Run(mizarArktosNetworkControllerWorkerCount, ctx.Stop)
	}()
	return nil, true, nil
}

func startMizarNetworkPolicyController(ctx *ControllerContext, grpcHost string, grpcAdaptor controllers.IGrpcAdaptor) (http.Handler, bool, error) {
	controllerName := "mizar-network-policy-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go controllers.NewMizarNetworkPolicyController(
		ctx.InformerFactory.Networking().V1().NetworkPolicies(),
		ctx.ClientBuilder.ClientOrDie(controllerName),
		grpcHost,
		grpcAdaptor,
	).Run(mizarNetworkPolicyControllerWorkerCount, ctx.Stop)
	return nil, true, nil
}

func startMizarNamespaceController(ctx *ControllerContext, grpcHost string, grpcAdaptor controllers.IGrpcAdaptor) (http.Handler, bool, error) {
	controllerName := "mizar-namespace-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go controllers.NewMizarNamespaceController(
		ctx.InformerFactory.Core().V1().Namespaces(),
		ctx.ClientBuilder.ClientOrDie(controllerName),
		grpcHost,
		grpcAdaptor,
	).Run(mizarNamespaceControllerWorkerCount, ctx.Stop)
	return nil, true, nil
}
