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

	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/networking/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	controllerForMizarNetworkPolicy = "mizar_policies"
)

// MizarNetworkPolicyController points to current controller
type MizarNetworkPolicyController struct {
	// A store of objects, populated by the shared informer passed to MizarNetworkPolicyController
	lister corelisters.NetworkPolicyLister
	// listerSynced returns true if the store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	listerSynced cache.InformerSynced

	// To allow injection for testing.
	handler func(keyWithEventType KeyWithEventType) error

	// Queue that used to hold thing to be handled.
	queue workqueue.RateLimitingInterface

	grpcHost string

	grpcAdaptor IGrpcAdaptor
}

// NewMizarNetworkPolicyController creates and configures a new controller instance
func NewMizarNetworkPolicyController(networkPolicyInformer coreinformers.NetworkPolicyInformer, kubeClient clientset.Interface, grpcHost string, grpcAdaptor IGrpcAdaptor) *MizarNetworkPolicyController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	c := &MizarNetworkPolicyController{
		lister:       networkPolicyInformer.Lister(),
		listerSynced: networkPolicyInformer.Informer().HasSynced,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerForMizarNetworkPolicy),
		grpcHost:     grpcHost,
		grpcAdaptor:  grpcAdaptor,
	}

	networkPolicyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.createObj,
		UpdateFunc: c.updateObj,
		DeleteFunc: c.deleteObj,
	})
	c.lister = networkPolicyInformer.Lister()
	c.listerSynced = networkPolicyInformer.Informer().HasSynced

	c.handler = c.handle

	return c
}

// Run begins watching and handling.
func (c *MizarNetworkPolicyController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %v controller", controllerForMizarNetworkPolicy)
	defer klog.Infof("Shutting down %v controller", controllerForMizarNetworkPolicy)

	if !controller.WaitForCacheSync(controllerForMizarNetworkPolicy, stopCh, c.listerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *MizarNetworkPolicyController) createObj(obj interface{}) {
	key, _ := controller.KeyFunc(obj)
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Create})
}

// When an object is updated.
func (c *MizarNetworkPolicyController) updateObj(old, cur interface{}) {
	curObj := cur.(*v1.NetworkPolicy)
	oldObj := old.(*v1.NetworkPolicy)
	klog.Infof("curObj resource version %s", curObj.ResourceVersion)
	klog.Infof("oldObj resource version %s", oldObj.ResourceVersion)
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
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Update, ResourceVersion: curObj.ResourceVersion})
}

func (c *MizarNetworkPolicyController) deleteObj(obj interface{}) {
	key, _ := controller.KeyFunc(obj)
	klog.Infof("%v deleted. key %s.", controllerForMizarNetworkPolicy, key)
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Delete})
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the handler is never invoked concurrently with the same key.
func (c *MizarNetworkPolicyController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *MizarNetworkPolicyController) processNextWorkItem() bool {
	workItem, quit := c.queue.Get()

	if quit {
		return false
	}

	keyWithEventType := workItem.(KeyWithEventType)
	key := keyWithEventType.Key
	defer c.queue.Done(workItem)

	err := c.handler(keyWithEventType)
	if err == nil {
		c.queue.Forget(workItem)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("Handle %v of key %v failed with %v", controllerForMizarNetworkPolicy, key, err))
	c.queue.AddRateLimited(keyWithEventType)

	return true
}

func (c *MizarNetworkPolicyController) handle(keyWithEventType KeyWithEventType) error {
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType
	klog.Infof("Entering handling for %v. key %s, eventType %s", controllerForMizarNetworkPolicy, key, eventType)

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished handling %v %q (%v)", controllerForMizarNetworkPolicy, key, time.Since(startTime))
	}()

	tenant, namespace, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		return err
	}

	obj, err := c.lister.NetworkPoliciesWithMultiTenancy(namespace, tenant).Get(name)
	if err != nil {
		if eventType == EventType_Delete && errors.IsNotFound(err) {
			obj = &v1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Tenant:    tenant,
				},
			}
		} else {
			return err
		}
	}

	klog.V(4).Infof("Handling %v %s/%s/%s for event %v", controllerForMizarNetworkPolicy, tenant, namespace, name, eventType)

	switch eventType {
	case EventType_Create:
		processNetworkPolicyGrpcReturnCode(c, c.grpcAdaptor.CreateNetworkPolicy(c.grpcHost, obj), keyWithEventType)
	case EventType_Update:
		processNetworkPolicyGrpcReturnCode(c, c.grpcAdaptor.UpdateNetworkPolicy(c.grpcHost, obj), keyWithEventType)
	case EventType_Delete:
		processNetworkPolicyGrpcReturnCode(c, c.grpcAdaptor.DeleteNetworkPolicy(c.grpcHost, obj), keyWithEventType)
	default:
		panic(fmt.Sprintf("unimplemented for eventType %v", eventType))
	}

	return nil
}

func processNetworkPolicyGrpcReturnCode(c *MizarNetworkPolicyController, returnCode *ReturnCode, keyWithEventType KeyWithEventType) {
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType
	switch returnCode.Code {
	case CodeType_OK:
		klog.Infof("Mizar handled request successfully for %v. key %s, eventType %v", controllerForMizarNetworkPolicy, key, eventType)
	case CodeType_TEMP_ERROR:
		klog.Warningf("Mizar hit temporary error for %v. key %s. %s, eventType %v", controllerForMizarNetworkPolicy, key, returnCode.Message, eventType)
		c.queue.AddRateLimited(keyWithEventType)
	case CodeType_PERM_ERROR:
		klog.Errorf("Mizar hit permanent error for %v. key %s. %s, eventType %v", controllerForMizarNetworkPolicy, key, returnCode.Message, eventType)
	default:
		klog.Errorf("unimplemented for CodeType %v", returnCode.Code)
	}
}
