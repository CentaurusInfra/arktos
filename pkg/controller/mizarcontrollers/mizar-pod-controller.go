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

package mizarcontrollers

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	"k8s.io/kubernetes/pkg/controller"

	"k8s.io/klog"
)

const (
	controllerKind = "pod"
	controllerFor  = "mizar_pod"
)

// PodController points to current controller
type PodController struct {
	kubeClient clientset.Interface

	// A store of objects, populated by the shared informer passed to PodController
	lister corelisters.PodLister
	// listerSynced returns true if the store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	listerSynced cache.InformerSynced

	// To allow injection for testing.
	handler func(key string, eventType string) error

	// Queue that used to hold thing to be handled.
	queue workqueue.RateLimitingInterface
}

// NewPodController configures a new controller instance
func NewPodController(informer coreinformers.PodInformer, kubeClient clientset.Interface) *PodController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	oc := &PodController{
		kubeClient:   kubeClient,
		lister:       informer.Lister(),
		listerSynced: informer.Informer().HasSynced,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerFor),
	}

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    oc.createObj,
		UpdateFunc: oc.updateObj,
		DeleteFunc: oc.deleteObj,
	})
	oc.lister = informer.Lister()
	oc.listerSynced = informer.Informer().HasSynced

	oc.handler = oc.handle

	return oc
}

// Run begins watching and handling.
func (oc *PodController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer oc.queue.ShutDown()

	klog.Infof("Starting %v controller", controllerFor)
	defer klog.Infof("Shutting down %v controller", controllerFor)

	if !controller.WaitForCacheSync(controllerKind, stopCh, oc.listerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(oc.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (oc *PodController) createObj(obj interface{}) {
	key, _ := controller.KeyFunc(obj)
	oc.queue.Add(KeyWithEventType{Key: key, EventType: "create"})
}

// When an object is updated.
func (oc *PodController) updateObj(old, cur interface{}) {
	curObj := cur.(*v1.Pod)
	oldObj := old.(*v1.Pod)
	if curObj.ResourceVersion == oldObj.ResourceVersion {
		// Periodic resync will send update events for all known objects.
		// Two different versions of the same object will always have different RVs.
		return
	}
	if curObj.DeletionTimestamp != nil {
		// Object is being deleted. Don't handle update anymore.
		return
	}

	key, _ := controller.KeyFunc(curObj)
	oc.queue.Add(KeyWithEventType{Key: key, EventType: "update"})
}

func (oc *PodController) deleteObj(obj interface{}) {
	key, _ := controller.KeyFunc(obj)
	klog.Infof("%v deleted. key %s.", controllerFor, key)
	oc.queue.Add(KeyWithEventType{Key: key, EventType: "delete"})
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the handler is never invoked concurrently with the same key.
func (oc *PodController) worker() {
	for oc.processNextWorkItem() {
	}
}

func (oc *PodController) processNextWorkItem() bool {
	workItem, quit := oc.queue.Get()

	if quit {
		return false
	}

	keyWithEventType := workItem.(KeyWithEventType)
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType
	defer oc.queue.Done(key)

	err := oc.handler(key, eventType)
	if err == nil {
		oc.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("Handle %v of key %v failed with %v", controllerFor, key, err))
	oc.queue.AddRateLimited(keyWithEventType)

	return true
}

func (oc *PodController) handle(key string, eventType string) error {
	klog.Infof("Entering handling for %v. key %s", controllerFor, key)

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished handling %v %q (%v)", controllerFor, key, time.Since(startTime))
	}()

	tenant, namespace, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		return err
	}

	obj, err := oc.lister.PodsWithMultiTenancy(namespace, tenant).Get(name)
	if errors.IsNotFound(err) {
		klog.V(4).Infof("%v %v cannot be found", controllerFor, key)
		return nil
	} else {
		klog.V(4).Infof("Handling %v %s/%s/%s hashkey %v for event %v", controllerFor, tenant, namespace, obj.Name, obj.HashKey, eventType)

		if eventType == "create" {
			// TODO: invoke grpc create
			print("create ok")
		} else if eventType == "update" {
			// TODO: invoke grpc update
			print("update ok")
		}
	}

	return err
}
