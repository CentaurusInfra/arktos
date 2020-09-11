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
(1) https://github.com/h-w-chen/arktos.git, arktos/cmd/arktos-network-controller
(2) https://github.com/futurewei-cloud/arktos.git, arktos/pkg/controller/nodelifecycle/node_lifecycle_controller
*/

package mizar

import (
	"fmt"
	"strings"
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

const (
	NodeKind          string = "Node"
	NodeReady         string = "True"
	NodeStatusMessage string = "HANDLED"
)

// Node Controller Struct
type MizarNodeController struct {
	kubeclientset  *kubernetes.Clientset
	informer       coreinformers.NodeInformer
	informerSynced cache.InformerSynced
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
		&v1core.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
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
	klog.Infof(c.logInfoMessage("Sending events to api server"))
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			resource := object.(*v1.Node)
			key := c.genKey(resource)
			c.Enqueue(key, EventType_Create)
			klog.Infof(c.logMessage("Create Node: ", key))
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			oldResource := oldObject.(*v1.Node)
			newResource := newObject.(*v1.Node)
			oldKey := c.genKey(oldResource)
			newKey := c.genKey(newResource)
			if oldKey == newKey {
				klog.Infof(c.logMessage("No actual change in nodes, discarding: ", newKey))
			} else {
				eventType, err := c.determineEventType(oldKey, newKey)
				if err != nil {
					klog.Errorf(c.logMessage("Unexpected string in queue; discarding: ", newKey))
				} else {
					c.Enqueue(newKey, eventType)
				}
				klog.Infof(c.logMessage("Update Node: ", newKey))
			}
		},
		DeleteFunc: func(object interface{}) {
			resource := object.(*v1.Node)
			key := c.genKey(resource)
			c.Enqueue(key, EventType_Delete)
			klog.Infof(c.logMessage("Delete Node: ", key))
		},
	})
	return c, nil
}

// Run starts an asynchronous loop that detects events of cluster nodes.
func (c *MizarNodeController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Infof(c.logInfoMessage("Starting node controller"))
	klog.Infof(c.logInfoMessage("Waiting cache to be synced"))
	if ok := cache.WaitForCacheSync(stopCh, c.informerSynced); !ok {
		klog.Fatalf(c.logInfoMessage("Timeout expired during waiting for caches to sync."))
	}
	klog.Infof(c.logInfoMessage("Starting workers..."))
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	<-stopCh
	klog.Infof(c.logInfoMessage("Shutting down node controller"))
}

// Enqueue puts key of the node object in the work queue
// EventType: Create=0, Update=1, Delete=2, Resume=3
func (c *MizarNodeController) Enqueue(key string, eventType EventType) {
	c.queue.Add(KeyWithEventType{Key: key, EventType: eventType})
}

// Dequeue an item and process it
func (c *MizarNodeController) runWorker() {
	for {
		item, queueIsEmpty := c.queue.Get()
		if queueIsEmpty {
			break
		}
		c.process(item)
	}
}

// Parsing a item key and call gRPC request
func (c *MizarNodeController) process(item interface{}) {
	defer c.queue.Done(item)
	keyWithEventType, ok := item.(KeyWithEventType)
	if !ok {
		klog.Errorf(c.logMessage("Unexpected item in queue: ", keyWithEventType))
		c.queue.Forget(item)
		return
	}
	//command, key, err := c.parseCommandKey(commandKey)
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType
	nodeKind, nodeTenant, nodeName, nodeStatusType, nodeStatus, nodeAddressType, nodeAddress, err := c.parseKey(key)
	if err != nil || nodeKind != NodeKind {
		klog.Errorf(c.logMessage("Unexpected string in queue; discarding: ", key))
		c.queue.Forget(item)
		return
	}
	klog.Infof(c.logInfoMessage("Processing a node: ")+"%s/%s/%s/%s/%s/%s/%s", nodeKind, nodeTenant, nodeName, nodeStatusType, nodeStatus, nodeAddressType, nodeAddress)
	c.gRPCRequest(eventType, nodeName, nodeAddress)
	c.queue.Forget(item)
}

// Generate a node key
func (c *MizarNodeController) genKey(resource *v1.Node) string {
	nodeKind := resource.GetObjectKind().GroupVersionKind().Kind
	if nodeKind == "" {
		nodeKind = NodeKind
	}
	nodeTenant := resource.GetTenant()
	nodeName := resource.GetName()
	if nodeName == "" {
		klog.Fatalln(c.logMessage("Node name should not be null", nodeName))
	}
	var nodeStatusType, nodeStatus string
	conditions := resource.Status.Conditions
	if len(conditions) < 6 {
		klog.Fatalln(c.logInfoMessage("Node status information is needed"))
	} else {
		nodeStatusType = fmt.Sprintf("%s", resource.Status.Conditions[5].Type)
		nodeStatus = fmt.Sprintf("%s", resource.Status.Conditions[5].Status)
	}
	var nodeAddressType, nodeAddress string
	addresses := resource.Status.Addresses
	if len(addresses) < 1 {
		klog.Fatalln(c.logInfoMessage("Node ip address is needed"))
	} else {
		nodeAddressType = fmt.Sprintf("%s", resource.Status.Addresses[0].Type)
		nodeAddress = fmt.Sprintf("%s", resource.Status.Addresses[0].Address)
	}
	key := fmt.Sprintf("%s/%s/%s/%s/%s/%s/%s", nodeKind, nodeTenant, nodeName, nodeStatusType, nodeStatus, nodeAddressType, nodeAddress)
	return key
}

// Parse a key and get node information
func (c *MizarNodeController) parseKey(key string) (nodeKind, nodeTenant, nodeName, nodeStatusType, nodeStatus, nodeAddressType, nodeAddress string, err error) {
	segs := strings.Split(key, "/")
	nodeKind = segs[0]
	nodeTenant = segs[1]
	nodeName = segs[2]
	nodeStatusType = segs[3]
	nodeStatus = segs[4]
	nodeAddressType = segs[5]
	nodeAddress = segs[6]
	if len(segs) < 7 || nodeName == "" {
		err = fmt.Errorf("Invalid key format - key=%s", key)
		return
	}
	return
}

func (c *MizarNodeController) determineEventType(key1, key2 string) (eventType EventType, err error) {
	segs1 := strings.Split(key1, "/")
	segs2 := strings.Split(key2, "/")
	if len(segs1) < 7 || len(segs2) < 7 {
		err = fmt.Errorf("Invalid key format - key1=%s, key2=%s", key1, key2)
		return
	}
	status1 := segs1[4]
	status2 := segs2[4]
	eventType = EventType_Update
	if status1 != status2 && status2 == NodeReady {
		eventType = EventType_Resume
	}
	return
}

func (c *MizarNodeController) logInfoMessage(info string) string {
	message := fmt.Sprintf("[NodeController][%s]", info)
	return message
}

func (c *MizarNodeController) logMessage(msg string, detail interface{}) string {
	message := fmt.Sprintf("[NodeController][%s][Node Info]:%v", msg, detail)
	return message
}

//gRPC request message, Integration is needed
func (c *MizarNodeController) gRPCRequest(event EventType, nodeName, nodeAddress string) {
	client, ctx, conn, cancel, err := getGrpcClient(c.grpcHost)
	if err != nil {
		klog.Errorf(c.logMessage("gRPC connection failed ", err))
		return
	}
	defer conn.Close()
	defer cancel()
	var resource BuiltinsNodeMessage
	resource = BuiltinsNodeMessage{
		Name: nodeName,
		Ip:   nodeAddress,
	}
	switch event {
	case EventType_Create:
		returnCode, err := client.CreateNode(ctx, &resource)
		if returnCode.Code != CodeType_OK {
			klog.Errorf(c.logMessage("Node creation failed on Mizar side ", err))
		}
	case EventType_Update:
		returnCode, err := client.UpdateNode(ctx, &resource)
		if returnCode.Code != CodeType_OK {
			klog.Errorf(c.logMessage("Node creation failed on Mizar side ", err))
		}
	case EventType_Delete:
		returnCode, err := client.DeleteNode(ctx, &resource)
		if returnCode.Code != CodeType_OK {
			klog.Errorf(c.logMessage("Node creation failed on Mizar side ", err))
		}
	case EventType_Resume:
		returnCode, err := client.ResumeNode(ctx, &resource)
		if returnCode.Code != CodeType_OK {
			klog.Errorf(c.logMessage("Node creation failed on Mizar side ", err))
		}
	default:
		klog.Errorf(c.logMessage("gRPC event is not correct", event))
	}
	klog.Infof(c.logInfoMessage("gRPC request is sent"))
}
