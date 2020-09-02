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

Reference: https://github.com/h-w-chen/arktos.git, arktos/cmd/arktos-network-controller
*/

package app

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	arktoscheme "k8s.io/arktos-ext/pkg/generated/clientset/versioned/scheme"
	arktosinformer "k8s.io/arktos-ext/pkg/generated/informers/externalversions/arktosextensions/v1"
	arktosv1 "k8s.io/arktos-ext/pkg/generated/listers/arktosextensions/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type Controller struct {
	domainName   string
	cacheSynced  cache.InformerSynced
	store        arktosv1.NetworkLister
	queue        workqueue.RateLimitingInterface
	netClientset *arktos.Clientset
	svcClientset *kubernetes.Clientset
	recorder     record.EventRecorder
}

func NewArktosNetworkController(domainName string, netClientset *arktos.Clientset, svcClientset *kubernetes.Clientset, informer arktosinformer.NetworkInformer) *Controller {
	utilruntime.Must(arktoscheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: svcClientset.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	return &Controller{
		domainName:   domainName,
		store:        informer.Lister(),
		queue:        workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		cacheSynced:  informer.Informer().HasSynced,
		netClientset: netClientset,
		svcClientset: svcClientset,
		recorder:     eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "flat-network-controller"}),
	}
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			fmt.Println("Arktos Network Added")
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			fmt.Println("Arktos Network Updated")
		},
	})
	return c, nil
}

func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Info("starting flat network controller")
	klog.V(5).Info("waiting for informer caches to sync")
	if !cache.WaitForCacheSync(stopCh, c.cacheSynced) {
		klog.Error("failed to wait for cache to sync")
		return
	}
	klog.V(5).Info("staring workers of flat network controller")
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	klog.V(5).Infof("%d workers started", workers)
	<-stopCh
	klog.Info("shutting down flat network controller")
}

func (c *Controller) runWorker() {
	for {
		item, queueIsEmpty := c.queue.Get()
		if queueIsEmpty {
			break
		}
		c.process(item)
	}
}

func (c *Controller) process(item interface{}) {
	defer c.workqueue.Done(item)
	c.workqueue.Forget(item)
}
