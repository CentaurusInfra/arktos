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
	NODE_KIND          string = "Node"
	NODE_READY         string = "True"
	NODE_CREATE        string = "Create"
	NODE_UPDATE        string = "Update"
	NODE_DELETE        string = "Delete"
	NODE_RESUME        string = "Resume"
	NODE_statusMessage string = "HANDLED"
)

type NodeController struct {
	kubeclientset  *kubernetes.Clientset
	informer       coreinformers.NodeInformer
	informerSynced cache.InformerSynced
	lister         corelisters.NodeLister
	recorder       record.EventRecorder
	queue          workqueue.RateLimitingInterface
}

func NewNodeController(kubeclientset *kubernetes.Clientset, nodeInformer coreinformers.NodeInformer) (*NodeController, error) {
	informer := nodeInformer
	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "mizar-node-controller"})
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&v1core.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c := &NodeController{
		kubeclientset:  kubeclientset,
		informer:       informer,
		informerSynced: informer.Informer().HasSynced,
		lister:         informer.Lister(),
		recorder:       recorder,
		queue:          queue,
	}
	klog.Infof(c.logInfoMessage("Sending events to api server"))
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			resource := object.(*v1.Node)
			key := c.genKey(resource)
			c.Enqueue(NODE_CREATE, key)
			fmt.Printf("NODE_CREATE: %s", key)
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			oldResource := oldObject.(*v1.Node)
			newResource := newObject.(*v1.Node)
			oldKey := c.genKey(oldResource)
			newKey := c.genKey(newResource)
			if oldKey == newKey {
				klog.Infof(c.logEventMessage("No actual change in nodes, discarding: ", newKey))
			} else {
				command, err := c.compareKey(oldKey, newKey)
				if err != nil {
					klog.Errorf(c.logInfoMessage("Unexpected string in queue; discarding: " + newKey))
				} else {
					c.Enqueue(command, newKey)
				}
				fmt.Printf("NODE_UPDATE: %s, %s", command, newKey)
			}
		},
		DeleteFunc: func(object interface{}) {
			resource := object.(*v1.Node)
			key := c.genKey(resource)
			c.Enqueue(NODE_DELETE, key)
			fmt.Printf("NODE_DELETE: %s", key)
		},
	})
	return c, nil
}

// Run starts an asynchronous loop that detects events of cluster nodes.
func (c *NodeController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Infof(c.logInfoMessage("Starting node controller"))
	klog.Infoln(c.logInfoMessage("Waiting cache to be synced"))
	if ok := cache.WaitForCacheSync(stopCh, c.informerSynced); !ok {
		klog.Fatalln(c.logInfoMessage("Timeout expired during waiting for caches to sync."))
	}
	klog.Infoln(c.logInfoMessage("Starting workers..."))
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	<-stopCh
	klog.Info(c.logInfoMessage("Shutting down node controller"))
}

// Enqueue puts key of the node object in the work queue
func (c *NodeController) Enqueue(command string, key string) {
	commandKey := c.genCommandKey(command, key)
	c.queue.Add(commandKey)
}

// Dequeue an item and process it
func (c *NodeController) runWorker() {
	for {
		item, queueIsEmpty := c.queue.Get()
		if queueIsEmpty {
			break
		}
		c.process(item)
	}
}

// Parsing a item key and call gRPC request
func (c *NodeController) process(item interface{}) {
	defer c.queue.Done(item)
	commandKey, ok := item.(string)
	if !ok {
		klog.Errorf(c.logInfoMessage("unexpected item in queue: " + commandKey))
		c.queue.Forget(item)
		return
	}
	command, key, err := c.parseCommandKey(commandKey)
	if err != nil {
		klog.Errorf(c.logInfoMessage("Unexpected item in queue: " + commandKey))
		c.queue.Forget(item)
		return
	}
	nodeKind, nodeTenant, nodeName, nodeStatusType, nodeStatus, nodeAddressType, nodeAddress, err := c.parseKey(key)
	if err != nil {
		klog.Errorf(c.logInfoMessage("Unexpected string in queue; discarding: ") + key)
		c.queue.Forget(item)
		return
	}
	if nodeKind != NODE_KIND {
		klog.Errorf(c.logInfoMessage("Unexpected object in queue; discarding: ") + key)
		c.queue.Forget(item)
		return
	}
	klog.V(5).Infof(c.logInfoMessage("Processing a node: ")+"%s/%s/%s/%s/%s/%s/%s", nodeKind, nodeTenant, nodeName, nodeStatusType, nodeStatus, nodeAddressType, nodeAddress)
	c.gRPCRequest(command, nodeAddress)
	c.queue.Forget(item)
}

// Generate a node key
func (c *NodeController) genKey(resource *v1.Node) string {
	nodeKind := resource.GetObjectKind().GroupVersionKind().Kind
	if nodeKind == "" {
		nodeKind = NODE_KIND
	}
	nodeTenant := resource.GetTenant()
	nodeName := resource.GetName()
	if nodeName == "" {
		klog.Fatalln(c.logInfoMessage("Node name should not be null"))
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
func (c *NodeController) parseKey(key string) (nodeKind, nodeTenant, nodeName, nodeStatusType, nodeStatus, nodeAddressType, nodeAddress string, err error) {
	segs := strings.Split(key, "/")
	nodeKind = segs[0]
	nodeTenant = segs[1]
	nodeName = segs[2]
	nodeStatusType = segs[3]
	nodeStatus = segs[4]
	nodeAddressType = segs[5]
	nodeAddress = segs[6]
	if len(segs) < 7 || nodeName == "" {
		err = fmt.Errorf("Invalid key format - %s", key)
		return
	}
	return
}

func (c *NodeController) compareKey(key1, key2 string) (command string, err error) {
	segs1 := strings.Split(key1, "/")
	segs2 := strings.Split(key2, "/")
	if len(segs1) < 7 || len(segs2) < 7 {
		err = fmt.Errorf("Invalid key format - key1=%s, key2=%s", key1, key2)
		return
	}
	status1 := segs1[4]
	status2 := segs2[4]
	command = NODE_UPDATE
	if status1 != status2 && status2 == NODE_READY {
		command = NODE_RESUME
	}
	return
}

func (c *NodeController) genCommandKey(command, key string) string {
	var commandKey string
	commandKey = fmt.Sprintf("%s/%s", command, key)
	return commandKey
}

func (c *NodeController) parseCommandKey(commandKey string) (command string, key string, err error) {
	segs := strings.SplitN(commandKey, "/", 2)
	command = segs[0]
	key = segs[1]
	if len(segs) < 2 || command == "" || key == "" {
		err = fmt.Errorf("Invalid key format - %s", commandKey)
		return
	}
	return
}

func (c *NodeController) logInfoMessage(info string) string {
	message := fmt.Sprintf("[NodeController][%s]", info)
	return message
}

func (c *NodeController) logEventMessage(event string, nodeinfo string) string {
	message := fmt.Sprintf("[NodeController][%s][Node Info]:%s", event, nodeinfo)
	return message
}

//gRPC request message, Integration is needed
func (c *NodeController) gRPCRequest(command string, ip string) {
	/* 
	// Set up a connection to the server.
	fmt.Println("command=%s", command)
	conn, err := grpc.Dial(config.Server_addr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	clientcon := pb.NewBuiltinsServiceClient(conn)
	// Contact the server and request crd services.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	switch command {
	case NODE_CREATE:
		var resource pb.BuiltinsNodeMessage
		resource = pb.BuiltinsNodeMessage{ip: ip}
		returnCode, err = clientcon.CreateNode(ctx, &resource)
	case NODE_UPDATE:
		var resource pb.BuiltinsNodeMessage
		resource = pb.BuiltinsNodeMessage{ip: ip}
		returnCode, err = clientcon.UpdateNode(ctx, &resource)
	case NODE_DELETE:
		var resource pb.BuiltinsNodeMessage
		resource = pb.BuiltinsNodeMessage{ip: ip}
		returnCode, err = clientcon.DeleteeNode(ctx, &resource)
	case NODE_RESUME:
		var resource pb.BuiltinsNodeMessage
		resource = pb.BuiltinsNodeMessage{ip: ip}
		returnCode, err = clientcon.ResumeNode(ctx, &resource)
	default:
		klog.Fatalln(logInfoMessage("gRPC command is not correct"))
	}
	if err != nil {
		klog.Fatalf(logInfoMessage("gRPC error ")+"%v", err)
	}*/
	klog.Infoln(c.logInfoMessage("gRPC request is sent"))
}

