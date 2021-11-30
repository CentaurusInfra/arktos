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
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	controllerForMizarEndpoints = "mizar_endpoints"
)

// MizarEndpointsController points to current controller
type MizarEndpointsController struct {
	// A store of endpoints objects, populated by the shared informer passed to MizarEndpointsController
	endpointsLister corelisters.EndpointsLister
	// endpointsListerSynced returns true if the store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	endpointsListerSynced cache.InformerSynced

	// A store of service objects, populated by the shared informer passed to MizarEndpointsController
	serviceLister corelisters.ServiceLister
	// serviceListerSynced returns true if the store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	serviceListerSynced cache.InformerSynced

	// To allow injection for testing.
	handler func(keyWithEventType KeyWithEventType) error

	// Queue that used to hold thing to be handled.
	queue workqueue.RateLimitingInterface

	grpcHost string

	grpcAdaptor IGrpcAdaptor
}

// NewMizarEndpointsController creates and configures a new controller instance
func NewMizarEndpointsController(endpointsInformer coreinformers.EndpointsInformer, serviceInformer coreinformers.ServiceInformer, kubeClient clientset.Interface, grpcHost string, grpcAdaptor IGrpcAdaptor) *MizarEndpointsController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	c := &MizarEndpointsController{
		endpointsLister:       endpointsInformer.Lister(),
		endpointsListerSynced: endpointsInformer.Informer().HasSynced,
		serviceLister:         serviceInformer.Lister(),
		serviceListerSynced:   serviceInformer.Informer().HasSynced,
		queue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerForMizarEndpoints),
		grpcHost:              grpcHost,
		grpcAdaptor:           grpcAdaptor,
	}

	endpointsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.createObj,
		UpdateFunc: c.updateObj,
	})
	c.endpointsLister = endpointsInformer.Lister()
	c.endpointsListerSynced = endpointsInformer.Informer().HasSynced
	c.serviceLister = serviceInformer.Lister()
	c.serviceListerSynced = serviceInformer.Informer().HasSynced

	c.handler = c.handle

	return c
}

// Run begins watching and handling.
func (c *MizarEndpointsController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %v controller", controllerForMizarEndpoints)
	defer klog.Infof("Shutting down %v controller", controllerForMizarEndpoints)

	if !controller.WaitForCacheSync(controllerForMizarEndpoints, stopCh, c.endpointsListerSynced) {
		return
	}

	if !controller.WaitForCacheSync(controllerForMizarEndpoints, stopCh, c.serviceListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *MizarEndpointsController) createObj(obj interface{}) {
	key, _ := controller.KeyFunc(obj)
	if shouldIgnore(key) {
		return
	}
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Create})
}

// When an object is updated.
func (c *MizarEndpointsController) updateObj(old, cur interface{}) {
	curObj := cur.(*v1.Endpoints)
	oldObj := old.(*v1.Endpoints)
	if curObj.ResourceVersion == oldObj.ResourceVersion {
		// Periodic resync will send update events for all known objects.
		// Two different versions of the same object will always have different RVs.
		return
	}

	key, _ := controller.KeyFunc(curObj)
	if shouldIgnore(key) {
		return
	}
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Update, ResourceVersion: curObj.ResourceVersion})
}

func shouldIgnore(key string) bool {
	_, namespace, name, _ := cache.SplitMetaTenantNamespaceKey(key)
	if (namespace == "kube-system" && name == "kube-scheduler") ||
		(namespace == "kube-system" && name == "kube-controller-manager") {
		return true
	} else {
		return false
	}
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the handler is never invoked concurrently with the same key.
func (c *MizarEndpointsController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *MizarEndpointsController) processNextWorkItem() bool {
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

	utilruntime.HandleError(fmt.Errorf("Handle %v of key %v failed with %v", controllerForMizarEndpoints, key, err))
	c.queue.AddRateLimited(keyWithEventType)

	return true
}

func (c *MizarEndpointsController) handle(keyWithEventType KeyWithEventType) error {
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType
	klog.Infof("Entering handling for %v. key %s, eventType %s", controllerForMizarEndpoints, key, eventType)

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished handling %v %q (%v)", controllerForMizarEndpoints, key, time.Since(startTime))
	}()

	tenant, namespace, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		return err
	}

	endpoints, err := c.endpointsLister.EndpointsWithMultiTenancy(namespace, tenant).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("Endpoints %v %v has been deleted", namespace, name)
			return nil
		} else {
			return err
		}
	}

	service, err := c.serviceLister.ServicesWithMultiTenancy(namespace, tenant).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("Cannot find service %v %v", namespace, name)
			return nil
		} else {
			return err
		}
		return err
	}

	klog.V(4).Infof("Handling %v %s/%s/%s hashkey %v for event %v", controllerForMizarEndpoints, tenant, namespace, endpoints.Name, endpoints.HashKey, eventType)

	msg := ConvertToServiceEndpointContract(endpoints, service)

	switch eventType {
	case EventType_Create:
		processEndpointsGrpcReturnCode(c, c.grpcAdaptor.CreateServiceEndpoint(c.grpcHost, msg), keyWithEventType)
	case EventType_Update:
		processEndpointsGrpcReturnCode(c, c.grpcAdaptor.UpdateServiceEndpoint(c.grpcHost, msg), keyWithEventType)
	default:
		panic(fmt.Sprintf("unimplemented for eventType %v", eventType))
	}

	return nil
}

func processEndpointsGrpcReturnCode(c *MizarEndpointsController, returnCode *ReturnCode, keyWithEventType KeyWithEventType) {
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType
	switch returnCode.Code {
	case CodeType_OK:
		klog.Infof("Mizar handled request successfully for %v. key %s, eventType %v", controllerForMizarEndpoints, key, eventType)
	case CodeType_TEMP_ERROR:
		klog.Warningf("Mizar hit temporary error for %v. key %s. %s, eventType %v", controllerForMizarEndpoints, key, returnCode.Message, eventType)
		c.queue.AddRateLimited(keyWithEventType)
	case CodeType_PERM_ERROR:
		klog.Errorf("Mizar hit permanent error for %v. key %s. %s, eventType %v", controllerForMizarEndpoints, key, returnCode.Message, eventType)
	default:
		klog.Errorf("unimplemented for CodeType %v", returnCode.Code)
	}
}
