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
	"fmt"
	"net/http"
	"time"

	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	"k8s.io/arktos-ext/pkg/generated/informers/externalversions"
	informers "k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	controllers "k8s.io/kubernetes/pkg/controller/mizar"
)

const (
	mizarStarterControllerWorkerCount       = 2
	mizarPodControllerWorkerCount           = 4
	mizarServiceControllerWorkerCount       = 4
	mizarArktosNetworkControllerWorkerCount = 4
	mizarNodeControllerWorkerCount          = 4
	mizarEndpointsControllerWorkerCount     = 4
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
	ctx := controllerContext.(ControllerContext)
	go startMizarEndpointsController(&ctx, grpcHost)
	go startMizarNodeController(&ctx, grpcHost)
	startMizarPodController(&ctx, grpcHost)
	startMizarServiceController(&ctx, grpcHost)
	startArktosNetworkController(&ctx, grpcHost)

}

func startMizarPodController(ctx *ControllerContext, grpcHost string) (http.Handler, bool, error) {
	controllerName := "mizar-pod-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go controllers.NewMizarPodController(
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.ClientBuilder.ClientOrDie(controllerName),
		grpcHost,
	).Run(mizarPodControllerWorkerCount, ctx.Stop)
	return nil, true, nil
}

func startMizarServiceController(ctx *ControllerContext, grpcHost string) (http.Handler, bool, error) {
	controllerName := "mizar-service-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	svcKubeconfigs := ctx.ClientBuilder.ConfigOrDie(controllerName)
	svcKubeClient := clientset.NewForConfigOrDie(svcKubeconfigs)

	go controllers.NewMizarServiceController(
		svcKubeClient,
		ctx.InformerFactory.Core().V1().Services(),
		grpcHost,
	).Run(mizarServiceControllerWorkerCount, ctx.Stop)
	return nil, true, nil
}

func startArktosNetworkController(ctx *ControllerContext, grpcHost string) (http.Handler, bool, error) {
	controllerName := "mizar-arktos-network-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	netKubeConfigs := ctx.ClientBuilder.ConfigOrDie(controllerName)
	svcKubeClient := clientset.NewForConfigOrDie(netKubeConfigs)
	networkClient := arktos.NewForConfigOrDie(netKubeConfigs)
	informerFactory := externalversions.NewSharedInformerFactory(networkClient, 10*time.Minute)

	go controllers.NewMizarArktosNetworkController(
		networkClient,
		svcKubeClient,
		informerFactory.Arktos().V1().Networks(),
		grpcHost,
	).Run(mizarArktosNetworkControllerWorkerCount, ctx.Stop)
	return nil, true, nil
}

func startMizarNodeController(ctx *ControllerContext, grpcHost string) (http.Handler, bool, error) {
	controllerName := "mizar-node-controller"
	klog.V(2).Infof("Starting %v", controllerName)
	nodeKubeconfigs := ctx.ClientBuilder.ConfigOrDie(controllerName)
	nodeKubeClient, err := clientset.NewForConfig(nodeKubeconfigs)
	stopCh := make(chan struct{})
	defer close(stopCh)

	informerFactory := informers.NewSharedInformerFactory(nodeKubeClient, 3*time.Minute)
	nodeInformer := informerFactory.Core().V1().Nodes()
	nodeController, err := controllers.NewMizarNodeController(nodeKubeClient, nodeInformer, grpcHost)
	if err != nil {
		klog.Infof("Error in building mizar node controller: %v", err)
	}
	informerFactory.Start(stopCh)
	nodeController.Run(mizarNodeControllerWorkerCount, ctx.Stop)
	fmt.Scanln()
	klog.Infof("mizar node controller exited")
	return nil, true, nil
}

func startMizarEndpointsController(ctx *ControllerContext, grpcHost string) (http.Handler, bool, error) {
	controllerName := "mizar-endpoints-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	epKubeconfigs := ctx.ClientBuilder.ConfigOrDie(controllerName)
	epKubeClient := clientset.NewForConfigOrDie(epKubeconfigs)
	stopCh := make(chan struct{})
	defer close(stopCh)
	informerFactory := informers.NewSharedInformerFactory(epKubeClient, 3*time.Minute)
	epInformer := informerFactory.Core().V1().Endpoints()
	serviceInformer := informerFactory.Core().V1().Services()
	epController, err := controllers.NewMizarEndpointsController(epKubeClient, epInformer, serviceInformer, grpcHost)
	informerFactory.Start(stopCh)
	if err != nil {
		klog.Infof("Error in building mizar node controller: %v", err.Error())
	}
	epController.Run(mizarEndpointsControllerWorkerCount, ctx.Stop)
	fmt.Scanln()
	klog.Infof("mizar endpoints controller exited")
	return nil, true, nil
}
