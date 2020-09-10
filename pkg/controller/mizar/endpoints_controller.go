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
(2) https://github.com/futurewei-cloud/arktos.git, arktos/pkg/controller/endpoints_controller
*/

package app

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
	EP_KIND          string = "Endpoints"
	EP_READY         string = "True"
	EP_CREATE        string = "Create"
	EP_UPDATE        string = "Update"
	EP_DELETE        string = "Delete"
	EP_RESUME        string = "Resume"
	EP_statusMessage string = "HANDLED"
)

type ServiceEndpoint struct {
	name      string
	addresses []string
	ports     []Port
}

type Port struct {
	port     string
	protocol string
}

type EndpointsController struct {
	kubeclientset  *kubernetes.Clientset
	informer       coreinformers.EndpointsInformer
	informerSynced cache.InformerSynced
	lister         corelisters.EndpointsLister
	recorder       record.EventRecorder
	queue          workqueue.RateLimitingInterface
}

func NewEndpointsController(kubeclientset *kubernetes.Clientset, endpointInformer coreinformers.EndpointsInformer) (*EndpointsController, error) {
	informer := endpointInformer
	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "mizar-endpoint-controller"})
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&v1core.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c := &EndpointsController{
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
			resource := object.(*v1.Endpoints)
			key := c.genKey(resource)
			c.Enqueue(EP_CREATE, key)
			fmt.Printf("EP_CREATE: %s", key)
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			oldResource := oldObject.(*v1.Endpoints)
			newResource := newObject.(*v1.Endpoints)
			oldKey := c.genKey(oldResource)
			newKey := c.genKey(newResource)
			oldEpSubsets := oldResource.Subsets
			newEpSubsets := newResource.Subsets
			oldSubsets := fmt.Sprintf("%v", oldEpSubsets)
			newSubsets := fmt.Sprintf("%v", newEpSubsets)
			if oldKey == newKey && oldSubsets == newSubsets {
				klog.Infof(c.logEventMessage("No actual change in endpoints, discarding: ", newKey))
			} else {
				command, err := c.compareEndpoints(oldResource, newResource)
				if err != nil {
					klog.Errorf(c.logInfoMessage("unexpected string in queue; discarding: " + newKey))
				} else {
					c.Enqueue(command, newKey)
				}
				fmt.Printf("EP_UPDATE: %s, %s", command, newKey)
			}
		},
		DeleteFunc: func(object interface{}) {
			resource := object.(*v1.Endpoints)
			key := c.genKey(resource)
			c.Enqueue(EP_DELETE, key)
			fmt.Printf("EP_DELETE: %s", key)
		},
	})
	return c, nil
}

// Run starts an asynchronous loop that detects events of cluster nodes.
func (c *EndpointsController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Infof(c.logInfoMessage("Starting endpoint controller"))
	klog.Infoln(c.logInfoMessage("Waiting cache to be synced"))
	if ok := cache.WaitForCacheSync(stopCh, c.informerSynced); !ok {
		klog.Fatalln(c.logInfoMessage("Timeout expired during waiting for caches to sync."))
	}
	klog.Infoln(c.logInfoMessage("Starting workers..."))
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	<-stopCh
	klog.Info(c.logInfoMessage("Shutting down endpoint controller"))
}

// Enqueue puts key of the node object in the work queue
func (c *EndpointsController) Enqueue(command string, key string) {
	commandKey := c.genCommandKey(command, key)
	c.queue.Add(commandKey)
}

// Dequeue an item and process it
func (c *EndpointsController) runWorker() {
	for {
		item, queueIsEmpty := c.queue.Get()
		if queueIsEmpty {
			break
		}
		c.process(item)
	}
}

// Parsing a item key and call gRPC request
func (c *EndpointsController) process(item interface{}) {
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
	epKind, epNamespace, epTenant, epName, err := c.parseKey(key)
	if err != nil {
		klog.Errorf(c.logInfoMessage("Unexpected string in queue; discarding: ") + key)
		c.queue.Forget(item)
		return
	}
	ep, err := c.lister.EndpointsWithMultiTenancy(epNamespace, epTenant).Get(epName)
	if err != nil {
		klog.Warningf(c.logInfoMessage("Failed to retrieve endpoint in local cache by tenant, name: ") + epTenant + ", " + epName)
		c.queue.Forget(item)
		return
	}
	subsets := ep.Subsets
	if subsets != nil {
		klog.Warningf(c.logInfoMessage("Failed to retrieve endpoint in local cache by tenant, name: ") + epTenant + ", " + epName)
		c.queue.Forget(item)
		return
	}
	var serviceEndPoint ServiceEndpoint
	serviceEndPoint.name = epName
	for i := 0; i < len(subsets); i++ {
		subset := subsets[i]
		addresses := subset.Addresses
		ports := subset.Ports
		if addresses && nil && ports && nil {
			for j := 0; j < len(addresses); j++ {
				serviceEndPoint.addresses[j] = addresses[j].ip
			}
			for j := 0; j < len(ports); j++ {
				serviceEndPoint.ports[j].port = ports[i].port
				serviceEndPoint.ports[j].protocol = ports[j].protocol
				if serviceEndPoint.ports[j].protocol == "" {
					serviceEndPoint.ports[j].protocol = "TCP"
				}
			}
		}
	}
	epAddresses := fmt.Sprintf("%v", fmt.Sprintf("%v", serviceEndPoint.addresses))
	epPorts := fmt.Sprintf("%v", fmt.Sprintf("%v", serviceEndPoint.ports))
	klog.V(5).Infof(c.logInfoMessage("Processing an endpoint: ")+"%s/%s/%s/%s/%s/%s", epKind, epTenant, epNamespace, epName, epAddresses, epPorts)
	c.gRPCRequest(command, serviceEndPoint)
	c.queue.Forget(item)
}

func (c *EndpointsController) genKey(resource *v1.Endpoints) string {
	epKind := resource.GetObjectKind().GroupVersionKind().Kind
	if epKind == "" {
		epKind = EP_KIND
	}
	epNamespace := resource.GetNamespace()
	epTenant := resource.GetTenant()
	epName := resource.GetName()
	if epName == "" {
		klog.Fatalln(c.logInfoMessage("Endpoint name should not be null"))
	}
	key := fmt.Sprintf("%s/%s/%s/%s/%s/%s", epKind, epNamespace, epTenant, epName)
	return key
}

// Parse a key and get endpoint information
func (c *EndpointsController) parseKey(key string) (epKind, epNamespace, epTenant, epName string, err error) {
	segs := strings.Split(key, "/")
	epKind = segs[0]
	epNamespace = segs[1]
	epTenant = segs[2]
	epName = segs[3]
	if len(segs) < 4 || epName == "" {
		err = fmt.Errorf("Invalid key format - %s", key)
		return
	}
	return
}

func (c *EndpointsController) compareKey(key1, key2 string) (command string, err error) {
	segs1 := strings.Split(key1, "/")
	segs2 := strings.Split(key2, "/")
	if len(segs1) < 4 || len(segs2) < 4 {
		err = fmt.Errorf("Invalid key format - key1=%s, key2=%s", key1, key2)
		return
	}
	status1 := segs1[3]
	status2 := segs2[3]
	command = EP_UPDATE
	if status1 != status2 && status2 == EP_READY {
		command = EP_RESUME
	}
	return
}

func (c *EndpointsController) compareEndpoints(resource1 *v1.Endpoints, resource2 *v1.Endpoints) (command string, err error) {
	epSubsets1 := resource1.Subsets
	notReadyAddresses1 := epSubsets1.NotReadyAddresses
	epSubsets2 := resource2.Subsets
	addresses2 := epSubsets2.Addresses

	command = EP_UPDATE
	for i := 0; i < len(notReadyAddresses1); i++ {
		address1 := notReadyAddresses1[i].IP
		for j := 0; j < len(addresses2); j++ {
			address2 := addresses2.IP
			if address1 == address2 {
				command = EP_RESUME
			}
		}
	}
	return
}

func (c *EndpointsController) genCommandKey(command, key string) string {
	var commandKey string
	commandKey = fmt.Sprintf("%s/%s", command, key)
	return commandKey
}

func (c *EndpointsController) parseCommandKey(commandKey string) (command string, key string, err error) {
	segs := strings.SplitN(commandKey, "/", 2)
	command = segs[0]
	key = segs[1]
	if len(segs) < 2 || command == "" || key == "" {
		err = fmt.Errorf("Invalid key format - %s", commandKey)
		return
	}
	return
}

func (c *EndpointsController) logInfoMessage(info string) string {
	message := fmt.Sprintf("[EndpointController][%s]", info)
	return message
}

func (c *EndpointsController) logEventMessage(event string, epinfo string) string {
	message := fmt.Sprintf("[EndpointController][%s][Endpoint Info]:%s", event, epinfo)
	return message
}

func (c *EndpointsController) gRPCRequest(command string, ep ServiceEndpoint) {
	/* integration is needed
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
	case EP_CREATE:
		var resource pb.BuiltinsServiceEndpointMessage
		resource = pb.Marshal(ep)
		returnCode, err = clientcon.CreateServiceEndpoint(ctx, &resource)
	case EP_UPDATE:
		var resource pb.BuiltinsServiceEndpointMessage
		resource = pb.Marshal(ep)
		returnCode, err = clientcon.UpdateServiceEndpoint(ctx, &resource)
	case EP_DELETE:
		var resource pb.BuiltinsServiceEndpointMessage
		resource = pb.Marshal(ep)
		returnCode, err = clientcon.DeleteServiceEndpoint(ctx, &resource)
	case EP_RESUME:
		var resource pb.BuiltinsServiceEndpointMessage
		resource = pb.Marshal(ep)
		returnCode, err = clientcon.ResumeServiceEndpoint(ctx, &resource)
	default:
		klog.Fatalln(logInfoMessage("gRPC command is not correct"))
	}
	if err != nil {
		klog.Fatalf(logInfoMessage("gRPC error ")+"%v", err)
	}*/
	klog.Infoln(c.logInfoMessage("gRPC request is sent"))
}
