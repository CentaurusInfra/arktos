/*
Copyright 2016 The Kubernetes Authors.

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
	"net/http"

	"k8s.io/klog"
	controllers "k8s.io/kubernetes/pkg/controller/mizarcontrollers"
)

const (
	workers = 4
)

func startMizarPodController(ctx ControllerContext) (http.Handler, bool, error) {
	controllerName := "mizar-pod-controller"
	klog.V(2).Infof("Starting %v", controllerName)

	go controllers.NewPodController(
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.ClientBuilder.ClientOrDie(controllerName),
	).Run(workers, ctx.Stop)
	return nil, true, nil
}
