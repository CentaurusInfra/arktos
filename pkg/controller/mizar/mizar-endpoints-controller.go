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

package mizar

import (
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
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
	EndpointsKind          string = "Endpoints"
	Endpoints_Ready        string = "True"
	EndpointsStatusMessage string = "HANDLED"
)

type ServiceEndpoint struct {
	name      string
	addresses []string
	ports     []Ports
}

//frontPort: service' port, backendPort: endpoint' port
//protocol: network protocol TCP by default
type Ports struct {
	frontPort   string
	backendPort string
	protocol    string
}

type MizarEndpointsController struct {
	kubeclientset       *kubernetes.Clientset
	informer            coreinformers.EndpointsInformer
	informerSynced      cache.InformerSynced
	lister              corelisters.EndpointsLister
	serviceListerSynced cache.InformerSynced
	serviceLister       corelisters.ServiceLister
	recorder            record.EventRecorder
	queue               workqueue.RateLimitingInterface
	grpcHost            string
}

func NewMizarEndpointsController(kubeclientset *kubernetes.Clientset, endpointInformer coreinformers.EndpointsInformer, serviceInformer coreinformers.ServiceInformer, grpcHost string) (*MizarEndpointsController, error) {
	informer := endpointInformer
	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "mizar-endpoints-controller"})
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&v1core.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c := &MizarEndpointsController{
		kubeclientset:       kubeclientset,
		informer:            informer,
		informerSynced:      informer.Informer().HasSynced,
		lister:              informer.Lister(),
		serviceListerSynced: serviceInformer.Informer().HasSynced,
		serviceLister:       serviceInformer.Lister(),
		recorder:            recorder,
		queue:               queue,
		grpcHost:            grpcHost,
	}
	klog.Infof(c.logInfoMessage("Sending events to api server"))
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			resource := object.(*v1.Endpoints)
			key := c.genKey(resource)
			c.Enqueue(key, EventType_Create)
			klog.Infof(c.logMessage("Create Endpoint: ", key))
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
				klog.Infof(c.logMessage("No actual change in endpoints, discarding: ", newKey))
			} else {
				eventType, err := c.determineEventType(oldResource, newResource)
				if err != nil {
					klog.Errorf(c.logMessage("Unexpected string in queue; discarding: ", newKey))
				} else {
					c.Enqueue(newKey, eventType)
				}
				klog.Infof(c.logMessage("Update Endpoints: ", newKey))
			}
		},
		DeleteFunc: func(object interface{}) {
			resource := object.(*v1.Endpoints)
			key := c.genKey(resource)
			c.Enqueue(key, EventType_Delete)
			klog.Infof(c.logMessage("Delete Endpoints: ", key))
		},
	})
	return c, nil
}

// Run starts an asynchronous loop that detects events of cluster nodes.
func (c *MizarEndpointsController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Infof(c.logInfoMessage("Starting endpoint controller"))
	klog.Infof(c.logInfoMessage("Waiting cache to be synced"))
	if ok := cache.WaitForCacheSync(stopCh, c.informerSynced); !ok {
		klog.Infof(c.logInfoMessage("Timeout expired during waiting for caches to sync."))
	}
	klog.Infof(c.logInfoMessage("Starting workers..."))
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	<-stopCh
	klog.Info(c.logInfoMessage("Shutting down endpoint controller"))
}

// Enqueue puts key of the endpoints object in the work queue
// EventType: Create=0, Update=1, Delete=2, Resume=3
func (c *MizarEndpointsController) Enqueue(key string, eventType EventType) {
	c.queue.Add(KeyWithEventType{Key: key, EventType: eventType})
}

// Dequeue an item and process it
func (c *MizarEndpointsController) runWorker() {
	for {
		item, queueIsEmpty := c.queue.Get()
		if queueIsEmpty {
			break
		}
		c.process(item)
	}
}

// Parsing a item key and call gRPC request
func (c *MizarEndpointsController) process(item interface{}) {
	defer c.queue.Done(item)
	keyWithEventType, ok := item.(KeyWithEventType)
	if !ok {
		klog.Errorf(c.logMessage("Unexpected item in queue: ", keyWithEventType))
		c.queue.Forget(item)
		return
	}
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType
	epKind, epNamespace, epTenant, epName, err := c.parseKey(key)
	if err != nil {
		klog.Errorf(c.logMessage("Unexpected string in queue; discarding: ", key))
		c.queue.Forget(item)
		return
	}
	ep, err := c.lister.EndpointsWithMultiTenancy(epNamespace, epTenant).Get(epName)
	if err != nil {
		klog.Errorf(c.logMessage("Failed to retrieve endpoint in local cache by tenant, name: ", epTenant+", "+epName))
		c.queue.Forget(item)
		return
	}
	subsets := ep.Subsets
	if subsets == nil {
		klog.Warningf(c.logMessage("Failed to retrieve endpoints subsets in local cache by tenant, name: ", epTenant+", "+epName))
		c.queue.Forget(item)
		return
	}
	var serviceEndPoint ServiceEndpoint
	serviceEndPoint.name = epName
	for i := 0; i < len(subsets); i++ {
		subset := subsets[i]
		addresses := subset.Addresses
		ports := subset.Ports
		if addresses != nil && ports != nil {
			for j := 0; j < len(addresses); j++ {
				serviceEndPoint.addresses[j] = addresses[j].IP
			}
			for j := 0; j < len(ports); j++ {
				epPort := ports[j].Port
				serviceEndPoint.ports[j] = c.getPorts(epNamespace, epTenant, epName, epPort)
			}
		}
	}
	klog.V(5).Infof(c.logInfoMessage("Processing an endpoint: ")+"%s/%s/%s/%s/%s/%s", epKind, epTenant, epNamespace, epName)
	result, err := c.gRPCRequest(eventType, serviceEndPoint)
	if !result {
		klog.Errorf(c.logMessage("Failed endpoints processing: ", key))
		c.queue.Add(item)
	} else {
		klog.Infof(c.logMessage(" Processed endpoints: ", key))
		c.queue.Forget(item)
	}
}

func (c *MizarEndpointsController) genKey(resource *v1.Endpoints) string {
	epKind := resource.GetObjectKind().GroupVersionKind().Kind
	if epKind == "" {
		epKind = EndpointsKind
	}
	epNamespace := resource.GetNamespace()
	epTenant := resource.GetTenant()
	epName := resource.GetName()
	if epName == "" {
		klog.Errorf(c.logInfoMessage("Endpoint name should not be null"))
	}
	key := fmt.Sprintf("%s/%s/%s/%s/%s/%s", epKind, epNamespace, epTenant, epName)
	return key
}

// Parse a key and get endpoint information
func (c *MizarEndpointsController) parseKey(key string) (epKind, epNamespace, epTenant, epName string, err error) {
	segs := strings.Split(key, "/")
	epKind = segs[0]
	epNamespace = segs[1]
	epTenant = segs[2]
	epName = segs[3]
	if len(segs) < 4 || epName == "" {
		err = fmt.Errorf(c.logMessage("Invalid key format: ", key))
		return
	}
	return
}

//Determine an event is Update or Resume
func (c *MizarEndpointsController) determineEventType(resource1 *v1.Endpoints, resource2 *v1.Endpoints) (eventType EventType, err error) {
	epSubsets1 := resource1.Subsets
	epSubsets2 := resource2.Subsets
	var notReadyAddressSet sets.String
	var readyAddressSet sets.String
	for i := 0; i < len(epSubsets1); i++ {
		notReadyAddresses := epSubsets1[i].NotReadyAddresses
		for j := 0; j < len(notReadyAddresses); j++ {
			notReadyAddress := notReadyAddresses[j].IP
			notReadyAddressSet.Insert(notReadyAddress)
		}
	}
	for i := 0; i < len(epSubsets2); i++ {
		readyAddresses := epSubsets2[i].Addresses
		for j := 0; j < len(readyAddresses); j++ {
			readyAddress := readyAddresses[j].IP
			readyAddressSet.Insert(readyAddress)
		}
	}
	newAddresses := readyAddressSet.Intersection(notReadyAddressSet)
	eventType = EventType_Update
	if newAddresses != nil {
		eventType = EventType_Resume
	}
	return
}

func (c *MizarEndpointsController) logInfoMessage(info string) string {
	message := fmt.Sprintf("[MizarEndpointController][%s]", info)
	return message
}

func (c *MizarEndpointsController) logMessage(msg string, detail interface{}) string {
	message := fmt.Sprintf("[MizarEndpointController][%s][Endpoint Info]:%v", msg, detail)
	return message
}

//This function returns front port, backend port, and protocol
//ServicePort: protocol, port (=service port = front port), targetPort (endpoint port = backend port)
//(e.g) ports: {protocol: TCP, port: 80,  targetPort: 9376 }
func (c *MizarEndpointsController) getPorts(epNamespace, epTenant, epName string, epPort int32) Ports {
	var ports Ports
	service, err := c.serviceLister.ServicesWithMultiTenancy(epNamespace, epTenant).Get(epName)
	if err != nil {
		klog.Errorf(c.logInfoMessage("Service not found: ") + epName)
		return ports
	}
	serviceports := service.Spec.Ports
	if serviceports == nil {
		klog.Errorf(c.logInfoMessage("Service ports are not found: ") + epName)
		return ports
	}
	for i := 0; i < len(serviceports); i++ {
		serviceport := serviceports[i]
		targetPort := serviceport.TargetPort.IntVal
		if targetPort == epPort {
			ports.frontPort = fmt.Sprintf("%s", serviceport.Port)
			ports.backendPort = fmt.Sprintf("%s", serviceport.TargetPort)
			ports.protocol = fmt.Sprintf("%s", serviceport.Protocol)
			return ports
		}
	}
	return ports
}

//gRPC request message, Integration is needed
func (c *MizarEndpointsController) gRPCRequest(event EventType, ep ServiceEndpoint) (response bool, err error) {
	client, ctx, conn, cancel, err := getGrpcClient(c.grpcHost)
	if err != nil {
		klog.Errorf(c.logMessage("gRPC connection failed ", err))
		return false, err
	}
	defer conn.Close()
	defer cancel()
	var ports []*PortsMessage
	var resource BuiltinsServiceEndpointMessage
	if (ep.ports) != nil {
		for i := 0; i < len(ep.ports); i++ {
			ports[i].FrontendPort = ep.ports[i].frontPort
			ports[i].BackendPort = ep.ports[i].backendPort
			ports[i].Protocol = ep.ports[i].protocol
		}
	}
	resource = BuiltinsServiceEndpointMessage{
		Name:       ep.name,
		BackendIps: ep.addresses,
		Ports:      ports,
	}
	switch event {
	case EventType_Create:
		returnCode, err := client.CreateServiceEndpoint(ctx, &resource)
		if returnCode.Code != CodeType_OK {
			klog.Errorf(c.logMessage("Endpoint creation failed on Mizar side ", err))
			return false, err
		}
	case EventType_Update:
		returnCode, err := client.UpdateServiceEndpoint(ctx, &resource)
		if returnCode.Code != CodeType_OK {
			klog.Errorf(c.logMessage("Endpoint update failed on Mizar side ", err))
			return false, err
		}
	case EventType_Resume:
		returnCode, err := client.ResumeServiceEndpoint(ctx, &resource)
		if returnCode.Code != CodeType_OK {
			klog.Errorf(c.logMessage("Endpoint resume failed on Mizar side ", err))
			return false, err
		}
	default:
		klog.Errorf(c.logMessage("gRPC event is not correct", event))
		err = fmt.Errorf(c.logMessage("gRPC event is not correct", event))
		return false, err
	}
	klog.Infof(c.logInfoMessage("gRPC request is sent"))
	return true, nil
}
