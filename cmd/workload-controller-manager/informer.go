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
