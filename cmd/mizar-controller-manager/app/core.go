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
	"net/http"

	"k8s.io/klog"

	endpointscontroller "k8s.io/kubernetes/cmd/mizar-controller-manager/app/endpoints"
	nodecontroller "k8s.io/kubernetes/cmd/mizar-controller-manager/app/node"
	podcontroller "k8s.io/kubernetes/cmd/mizar-controller-manager/app/pod"
	servicecontroller "k8s.io/kubernetes/cmd/mizar-controller-manager/app/service"
)

func startEndpointsController(ctx ControllerContext) (http.Handler, bool, error) {
	controllerName := "mizar-endpoints-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go endpointscontroller.NewObjectController(
		ctx.InformerFactory.Core().V1().Endpoints(),
		ctx.ClientBuilder.ClientOrDie(controllerName),
	).Run(1, ctx.Stop)
	return nil, true, nil
}

func startNodeController(ctx ControllerContext) (http.Handler, bool, error) {
	controllerName := "mizar-node-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go nodecontroller.NewObjectController(
		ctx.InformerFactory.Core().V1().Nodes(),
		ctx.ClientBuilder.ClientOrDie(controllerName),
	).Run(1, ctx.Stop)
	return nil, true, nil
}

func startPodController(ctx ControllerContext) (http.Handler, bool, error) {
	controllerName := "mizar-pod-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go podcontroller.NewObjectController(
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.ClientBuilder.ClientOrDie(controllerName),
	).Run(1, ctx.Stop)
	return nil, true, nil
}

func startServiceController(ctx ControllerContext) (http.Handler, bool, error) {
	controllerName := "mizar-service-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go servicecontroller.NewObjectController(
		ctx.InformerFactory.Core().V1().Services(),
		ctx.ClientBuilder.ClientOrDie(controllerName),
	).Run(1, ctx.Stop)
	return nil, true, nil
}
