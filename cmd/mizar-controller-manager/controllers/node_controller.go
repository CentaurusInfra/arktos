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
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type Controller struct {
	kubeclientset  *kubernetes.Clientset
	informer       coreinformers.NodeInformer
	informerSynced cache.InformerSynced
	lister         corelisters.NodeLister
	recorder       record.EventRecorder
	workqueue      workqueue.RateLimitingInterface
}

func NewNodeController(kubeclientset *kubernetes.Clientset, nodeInformer coreinformers.NodeInformer) (*Controller, error) {
	informer := nodeInformer
	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "mizar-node-controller"})
	eventBroadcaster.StartLogging(klog.Infof)
	klog.Infof("Sending events to api server.")
	eventBroadcaster.StartRecordingToSink(
		&v1core.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	workqueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	c := &Controller{
		kubeclientset:  kubeclientset,
		informer:       informer,
		informerSynced: informer.Informer().HasSynced,
		lister:         informer.Lister(),
		recorder:       recorder,
		workqueue:      workqueue,
	}

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			fmt.Println("Node Added")
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			fmt.Println("Node Updated")
		},
	})
	return nc, nil
}

// Run starts an asynchronous loop that monitors the status of cluster nodes.
func (nc *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer nc.workqueue.ShutDown()
	klog.Infof("Starting node controller")
	klog.Infoln("Waiting cache to be synced.")
	if ok := cache.WaitForCacheSync(stopCh, nc.informerSynced); !ok {
		klog.Fatalln("Timeout expired during waiting for caches to sync.")
	}
	klog.Infoln("Starting custom controller.")
	for i := 0; i < workers; i++ {
		go wait.Until(nc.runWorker, time.Second, stopCh)
	}
	<-stopCh
	klog.Info("shutting down node controller")
}

func (nc *Controller) runWorker() {
	for {
		item, queueIsEmpty := nc.workqueue.Get()
		if queueIsEmpty {
			break
		}
		nc.process(item)
	}
}

func (nc *Controller) process(item interface{}) {
	defer nc.workqueue.Done(item)
	nc.workqueue.Forget(item)
}
