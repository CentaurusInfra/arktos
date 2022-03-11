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
	nodeutil "k8s.io/kubernetes/pkg/util/node"
)

const (
	controllerForMizarNode = "mizar_node"
)

// MizarNodeController points to current controller
type MizarNodeController struct {
	// A store of node objects, populated from Resource partition node informers
	nodeListers map[string]corelisters.NodeLister
	// listerSynced returns true if the store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	nodeListersSynced map[string]cache.InformerSynced

	// A store of node objects, populated from Tenant partition node informer
	tpNodeLister       corelisters.NodeLister
	tpNodeListerSynced cache.InformerSynced

	// To allow injection for testing.
	handler func(keyWithEventType KeyWithEventType) error

	// Queue that used to hold thing to be handled.
	queue workqueue.RateLimitingInterface

	grpcHost string

	grpcAdaptor IGrpcAdaptor
}

// NewMizarNodeController creates and configures a new controller instance
func NewMizarNodeController(tpNodeInformer coreinformers.NodeInformer, nodeInformers map[string]coreinformers.NodeInformer, kubeClient clientset.Interface, grpcHost string, grpcAdaptor IGrpcAdaptor) *MizarNodeController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	klog.V(2).Infof("Mizar NodeController initialized with %v nodeinformers", len(nodeInformers))
	nodeListers, nodeListersSynced := nodeutil.GetNodeListersAndSyncedFromNodeInformers(nodeInformers)

	c := &MizarNodeController{
		nodeListers:        nodeListers,
		nodeListersSynced:  nodeListersSynced,
		tpNodeLister:       tpNodeInformer.Lister(),
		tpNodeListerSynced: tpNodeInformer.Informer().HasSynced,
		queue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerForMizarNode),
		grpcHost:           grpcHost,
		grpcAdaptor:        grpcAdaptor,
	}

	for _, nodeInformer := range nodeInformers {
		nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    c.createObj,
			UpdateFunc: c.updateObj,
			DeleteFunc: c.deleteObj,
		})
	}

	tpNodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.createObj,
		UpdateFunc: c.updateObj,
		DeleteFunc: c.deleteObj,
	})

	c.handler = c.handle

	return c
}

// Run begins watching and handling.
func (c *MizarNodeController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %v controller", controllerForMizarNode)
	defer klog.Infof("Shutting down %v controller", controllerForMizarNode)

	if !nodeutil.WaitForNodeCacheSync(controllerForMizarNode, c.nodeListersSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *MizarNodeController) createObj(obj interface{}) {
	key, _ := controller.KeyFunc(obj)
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Create})
}

// When an object is updated.
func (c *MizarNodeController) updateObj(old, cur interface{}) {
	curObj := cur.(*v1.Node)
	oldObj := old.(*v1.Node)
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

func (c *MizarNodeController) deleteObj(obj interface{}) {
	key, _ := controller.KeyFunc(obj)
	klog.Infof("%v deleted. key %s.", controllerForMizarNode, key)
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Delete})
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the handler is never invoked concurrently with the same key.
func (c *MizarNodeController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *MizarNodeController) processNextWorkItem() bool {
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

	utilruntime.HandleError(fmt.Errorf("Handle %v of key %v failed with %v", controllerForMizarNode, key, err))
	c.queue.AddRateLimited(keyWithEventType)

	return true
}

func (c *MizarNodeController) handle(keyWithEventType KeyWithEventType) error {
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType
	klog.Infof("Entering handling for %v. key %s, eventType %s", controllerForMizarNode, key, eventType)

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished handling %v %q (%v)", controllerForMizarNode, key, time.Since(startTime))
	}()

	_, _, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		return err
	}

	var nodeObj *v1.Node
	// check tenant partition
	nodeObj, err = c.tpNodeLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			// check resource partitions
			nodeObj, _, err = nodeutil.GetNodeFromNodelisters(c.nodeListers, name)
			if err != nil {
				if eventType == EventType_Delete && errors.IsNotFound(err) {
					nodeObj = &v1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
						},
					}
				} else {
					return err
				}
			}
		} else {
			return err
		}
	}

	klog.V(4).Infof("Handling %v %s for event %v", controllerForMizarNode, name, eventType)

	switch eventType {
	case EventType_Create:
		processNodeGrpcReturnCode(c, c.grpcAdaptor.CreateNode(c.grpcHost, nodeObj), keyWithEventType)
	case EventType_Update:
		processNodeGrpcReturnCode(c, c.grpcAdaptor.UpdateNode(c.grpcHost, nodeObj), keyWithEventType)
	case EventType_Delete:
		processNodeGrpcReturnCode(c, c.grpcAdaptor.DeleteNode(c.grpcHost, nodeObj), keyWithEventType)
	default:
		panic(fmt.Sprintf("unimplemented for eventType %v", eventType))
	}

	return nil
}

func processNodeGrpcReturnCode(c *MizarNodeController, returnCode *ReturnCode, keyWithEventType KeyWithEventType) {
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType
	switch returnCode.Code {
	case CodeType_OK:
		klog.Infof("Mizar handled request successfully for %v. key %s, eventType %v", controllerForMizarNode, key, eventType)
	case CodeType_TEMP_ERROR:
		klog.Warningf("Mizar hit temporary error for %v. key %s. %s, eventType %v", controllerForMizarNode, key, returnCode.Message, eventType)
		c.queue.AddRateLimited(keyWithEventType)
	case CodeType_PERM_ERROR:
		klog.Errorf("Mizar hit permanent error for %v. key %s. %s, eventType %v", controllerForMizarNode, key, returnCode.Message, eventType)
	default:
		klog.Errorf("unimplemented for CodeType %v", returnCode.Code)
	}
}
