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

package mizar

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	controllerForMizarStarter = "mizar_starter"
)

// MizarStarterController points to current controller
type MizarStarterController struct {
	// A store of objects, populated by the shared informer passed to MizarStarterController
	lister corelisters.ConfigMapLister
	// listerSynced returns true if the store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	listerSynced cache.InformerSynced

	// To allow injection for testing.
	handler func(key string) error

	// Queue that used to hold thing to be handled.
	queue workqueue.RateLimitingInterface

	// Controller context which is required to start new controller
	controllerContext interface{}

	// Will invoke the handler to start mizar controllers
	startHandler StartHandler
}

// NewMizarStarterController creates and configures a new controller instance
func NewMizarStarterController(configMapInformer coreinformers.ConfigMapInformer, kubeClient clientset.Interface, controllerContext interface{}, startHandler StartHandler) *MizarStarterController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	c := &MizarStarterController{
		lister:            configMapInformer.Lister(),
		listerSynced:      configMapInformer.Informer().HasSynced,
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerForMizarStarter),
		controllerContext: controllerContext,
		startHandler:      startHandler,
	}

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.createObj,
	})
	c.lister = configMapInformer.Lister()
	c.listerSynced = configMapInformer.Informer().HasSynced

	c.handler = c.handle

	return c
}

// Run begins watching and handling.
func (c *MizarStarterController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %v controller", controllerForMizarStarter)
	defer klog.Infof("Shutting down %v controller", controllerForMizarStarter)

	if !controller.WaitForCacheSync(controllerForMizarStarter, stopCh, c.listerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *MizarStarterController) createObj(obj interface{}) {
	key, _ := controller.KeyFunc(obj)
	c.queue.Add(key)
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the handler is never invoked concurrently with the same key.
func (c *MizarStarterController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *MizarStarterController) processNextWorkItem() bool {
	workItem, quit := c.queue.Get()

	if quit {
		return false
	}

	key := workItem.(string)
	defer c.queue.Done(key)

	err := c.handler(key)
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("Handle %v of key %v failed with %v", controllerForMizarStarter, key, err))
	c.queue.AddRateLimited(key)

	return true
}

func (c *MizarStarterController) handle(key string) error {
	klog.Infof("Entering handling for %v. key %s", controllerForMizarStarter, key)

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished handling %v %q (%v)", controllerForMizarStarter, key, time.Since(startTime))
	}()

	tenant, namespace, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		return err
	}

	obj, err := c.lister.ConfigMapsWithMultiTenancy(namespace, tenant).Get(name)
	if err != nil {
		return err
	}

	klog.V(4).Infof("Handling %v %s/%s/%s hashkey %v", controllerForMizarStarter, tenant, namespace, obj.Name, obj.HashKey)

	if namespace == "default" && name == "mizar-grpc-service" {
		grpcHost := obj.Data["host"]
		c.startHandler(c.controllerContext, grpcHost)
	}
	return nil
}
