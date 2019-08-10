/*
Copyright 2019 The Kubernetes Authors.

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

package main

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/api/types/v1alpha1"
	client_v1alpha1 "k8s.io/kubernetes/pkg/cloudfabric-controller/clientset/v1alpha1"
)

// WatchResources watch controller managers
func WatchResources(clientSet client_v1alpha1.DefaultV1Alpha1Interface) cache.Store {
	controllerManagerStore, controllerManagerController := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo metav1.ListOptions) (result runtime.Object, err error) {
				return clientSet.ControllerManagers("default").List(lo)
			},
			WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
				return clientSet.ControllerManagers("default").Watch(lo)
			},
		},
		&v1alpha1.ControllerManager{},
		1*time.Minute,
		cache.ResourceEventHandlerFuncs{},
	)

	go controllerManagerController.Run(wait.NeverStop)
	return controllerManagerStore
}
