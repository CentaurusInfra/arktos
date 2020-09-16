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

Reference:
(1) https://github.com/futurewei-cloud/arktos.git, arktos/pkg/controller/nodelifecycle/node_lifecycle_controller
*/

package mizar

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"k8s.io/kubernetes/pkg/controller"
)

const (
	NodeKind          string = "Node"
	NodeStatusType    string = "NodeReady"
	NodeReadyTrue     string = "True"
	NodeReadyFalse    string = "False"
	NodeReadyUnknown  string = "Unknown"
	NodeInternalIP    string = "InternalIP"
	NodeStatusMessage string = "HANDLED"
	NodeNoChange      int    = 1
	NodeUpdate        int    = 2
	NodeResume        int    = 3
)

// Node Controller Struct
type MizarNodeController struct {
	kubeclientset  *kubernetes.Clientset
	informer       coreinformers.NodeInformer
	informerSynced cache.InformerSynced
	syncHandler    func(eventKey KeyWithEventType) error
	lister         corelisters.NodeLister
	recorder       record.EventRecorder
	queue          workqueue.RateLimitingInterface
	grpcHost       string
}

func NewMizarNodeController(kubeclientset *kubernetes.Clientset, nodeInformer coreinformers.NodeInformer, grpcHost string) (*MizarNodeController, error) {
	informer := nodeInformer
	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "mizar-node-controller"})
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&v1core.EventSinkImpl{Interface: kubeclientset.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c := &MizarNodeController{
		kubeclientset:  kubeclientset,
		informer:       informer,
		informerSynced: informer.Informer().HasSynced,
		lister:         informer.Lister(),
		recorder:       recorder,
		queue:          queue,
		grpcHost:       grpcHost,
	}
	klog.Infof("Sending events to api server")
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			key, err := controller.KeyFunc(object)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", object, err))
				return
			}
			c.Enqueue(key, EventType_Create)
			klog.Infof("Create Node -%v ", key)
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			key1, err1 := controller.KeyFunc(oldObject)
			key2, err2 := controller.KeyFunc(newObject)
			if key1 == "" || key2 == "" || err1 != nil || err2 != nil {
				klog.Errorf("Unexpected string in queue; discarding - %v", key2)
				return
			}
			oldResource := oldObject.(*v1.Node)
			newResource := newObject.(*v1.Node)
			eventType, err := c.determineEventType(oldResource, newResource)
			if err != nil {
				klog.Errorf("Unexpected string in queue; discarding - %v ", key2)
				return
			}
			switch eventType {
			case NodeNoChange:
				{
					klog.Infof("No actual change in nodes, discarding -%v ", newResource.Name)
					break
				}
			case NodeUpdate:
				{
					c.Enqueue(key2, EventType_Update)
					klog.Infof("Update Node - %v", key2)
					break
				}
			case NodeResume:
				{
					c.Enqueue(key2, EventType_Resume)
					klog.Infof("Resume Node - %v", key2)
				}
			default:
				{
					klog.Errorf("Unexpected node event; discarding - %v", key2)
					return
				}
			}
		},
		DeleteFunc: func(object interface{}) {
			key, err := controller.KeyFunc(object)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", object, err))
				return
			}
			c.Enqueue(key, EventType_Delete)
			klog.Infof("Delete Node - %v", key)
		},
	})

	c.syncHandler = c.syncNode
	return c, nil
}

// Run starts an asynchronous loop that detects events of cluster nodes.
func (c *MizarNodeController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Infof("Starting node controller")
	klog.Infof("Waiting cache to be synced")

	ok := cache.WaitForCacheSync(stopCh, c.informerSynced)
	klog.Infof("sync done")
	if !ok {
		klog.Infof("Timeout expired during waiting for caches to sync.")
	}
	klog.Infof("Starting workers...")
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
	klog.Infof("Shutting down node controller")
}

// Enqueue puts key of the node object in the work queue
// EventType: Create=0, Update=1, Delete=2, Resume=3
func (c *MizarNodeController) Enqueue(key string, eventType EventType) {
	c.queue.Add(KeyWithEventType{Key: key, EventType: eventType})
}

func (c *MizarNodeController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *MizarNodeController) processNextWorkItem() bool {
	workItem, quit := c.queue.Get()
	if quit {
		return false
	}
	eventKey := workItem.(KeyWithEventType)
	key := eventKey.Key
	defer c.queue.Done(key)

	err := c.syncHandler(eventKey)
	if err == nil {
		c.queue.Forget(key)
		return true
	}
	utilruntime.HandleError(fmt.Errorf("Handle %v of key %v failed with %v", "serivce", key, err))
	c.queue.AddRateLimited(eventKey)
	return true
}

func (c *MizarNodeController) syncNode(keyWithEventType KeyWithEventType) error {
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing  %q (%v)", key, time.Since(startTime))
	}()
	_, _, nodeName, err := cache.SplitMetaTenantNamespaceKey(key)
	node, err := c.lister.Get(nodeName)
	if err != nil || node == nil {
		klog.Errorf("Failed to retrieve node in local cache by node name - %s", nodeName)
		c.queue.AddRateLimited(keyWithEventType)
		return err
	}
	result, err := c.gRPCRequest(eventType, node)
	if !result {
		klog.Errorf("Failed a node processing - %v", key)
		c.queue.AddRateLimited(keyWithEventType)
	} else {
		klog.Infof(" Processed a node - %v", key)
		c.queue.Forget(key)
	}
	return nil
}

// Retrieve node info
func (c *MizarNodeController) getNodeInfo(node *v1.Node) (nodeTenant, nodeName, nodeStatus, nodeAddress string, err error) {
	if node == nil {
		err = fmt.Errorf("node is null")
		return
	}
	nodeTenant = node.GetTenant()
	nodeName = node.GetName()
	if nodeName == "" {
		err = fmt.Errorf("Node name is not valid - %s", nodeName)
		return
	}
	conditions := node.Status.Conditions
	if conditions == nil {
		err = fmt.Errorf("Node status information is not available - %s", nodeName)
		return
	}
	var nodeStatusType string
	for i := 0; i < len(conditions); i++ {
		nodeStatusType = fmt.Sprintf("%v", conditions[i].Type)
		nodeStatus = fmt.Sprintf("%v", conditions[i].Status)
		if nodeStatusType == NodeStatusType {
			break
		}
	}
	addresses := node.Status.Addresses
	if addresses == nil {
		err = fmt.Errorf("Node address information is not available - %v", nodeName)
		return
	}
	var nodeAddressType string
	for i := 0; i < len(addresses); i++ {
		nodeAddressType = fmt.Sprintf("%s", addresses[i].Type)
		nodeAddress = fmt.Sprintf("%s", addresses[i].Address)
		if nodeAddressType == NodeInternalIP {
			break
		}
	}
	return
}

func (c *MizarNodeController) determineEventType(node1, node2 *v1.Node) (event int, err error) {
	nodeTenant1, nodeName1, nodeStatus1, nodeAddress1, err1 := c.getNodeInfo(node1)
	nodeTenant2, nodeName2, nodeStatus2, nodeAddress2, err2 := c.getNodeInfo(node1)
	if node1 == nil || node2 == nil || err1 != nil || err2 != nil {
		err = fmt.Errorf("It cannot determine null nodes event type - node1: %v, node2:%v", node1, node2)
		return
	}
	event = NodeUpdate
	if nodeTenant1 == nodeTenant2 && nodeName1 == nodeName2 && nodeAddress1 == nodeAddress2 && nodeStatus1 == nodeStatus2 {
		event = NodeNoChange
	} else if nodeStatus1 != nodeStatus2 && nodeStatus2 == NodeReadyTrue {
		event = NodeResume
	}
	return
}

//gRPC request message, Integration is needed
func (c *MizarNodeController) gRPCRequest(event EventType, node *v1.Node) (response bool, err error) {
	switch event {
	case EventType_Create:
		response := GrpcCreateNode(c.grpcHost, node)
		if response.Code != CodeType_OK {
			klog.Errorf("Node creation failed on Mizar side")
			return false, err
		}
	case EventType_Update:
		response := GrpcUpdateNode(c.grpcHost, node)
		if response.Code != CodeType_OK {
			klog.Errorf("Node update failed on Mizar side")
			return false, err
		}
	case EventType_Delete:
		response := GrpcDeleteNode(c.grpcHost, node)
		if response.Code != CodeType_OK {
			klog.Errorf("Node deletion failed on Mizar side")
			return false, err
		}
	case EventType_Resume:
		response := GrpcResumeNode(c.grpcHost, node)
		if response.Code != CodeType_OK {
			klog.Errorf("Node resume failed on Mizar side")
			return false, err
		}
	default:
		klog.Errorf("gRPC event is not correct - %v", event)
		err = fmt.Errorf("gRPC event is not correct - %v", event)
		return false, err
	}
	klog.Infof("gRPC request is sent")
	return true, nil
}
