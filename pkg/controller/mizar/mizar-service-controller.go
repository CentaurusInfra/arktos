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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	arktosapisv1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	arktosinformer "k8s.io/arktos-ext/pkg/generated/informers/externalversions/arktosextensions/v1"
	arktosextv1 "k8s.io/arktos-ext/pkg/generated/listers/arktosextensions/v1"
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
	dnsServiceDefaultName    = "kube-dns"
	kubernetesSvcDefaultName = "kubernetes"
)

// MizarServiceController manages service on mizar side and update cluster IP on arktos side
type MizarServiceController struct {
	netClient           arktos.Interface
	kubeClientset       *kubernetes.Clientset
	serviceLister       corelisters.ServiceLister
	serviceListerSynced cache.InformerSynced
	networkLister       arktosextv1.NetworkLister
	networkListerSynced cache.InformerSynced
	syncHandler         func(eventKeyWithType KeyWithEventType) error
	queue               workqueue.RateLimitingInterface
	recorder            record.EventRecorder
	grpcHost            string
	grpcAdaptor         IGrpcAdaptor
}

// NewMizarServiceController starts mizar service controller
func NewMizarServiceController(kubeClientset *kubernetes.Clientset, netClient arktos.Interface, serviceInformer coreinformers.ServiceInformer, arktosNetworkInformer arktosinformer.NetworkInformer, grpcHost string, grpcAdaptor IGrpcAdaptor) *MizarServiceController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClientset.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	c := &MizarServiceController{
		kubeClientset:       kubeClientset,
		netClient:           netClient,
		serviceLister:       serviceInformer.Lister(),
		serviceListerSynced: serviceInformer.Informer().HasSynced,
		networkLister:       arktosNetworkInformer.Lister(),
		networkListerSynced: arktosNetworkInformer.Informer().HasSynced,
		queue:               workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		recorder:            eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "mizar-service-controller"}),
		grpcHost:            grpcHost,
		grpcAdaptor:         grpcAdaptor,
	}

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.createService,
		UpdateFunc: c.updateService,
		DeleteFunc: c.deleteService,
	})
	c.serviceLister = serviceInformer.Lister()
	c.serviceListerSynced = serviceInformer.Informer().HasSynced

	c.syncHandler = c.syncService

	return c
}

// Run create, update, delete service on mizar side
func (c *MizarServiceController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	defer klog.Info("shutting down mizar service controller")

	klog.Infoln("Starting mizar service controller")
	klog.Infoln("Waiting cache to be synced.")
	if ok := cache.WaitForCacheSync(stopCh, c.serviceListerSynced, c.networkListerSynced); !ok {
		klog.Fatalln("Timeout expired during waiting for caches to sync.")
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
}

func (c *MizarServiceController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *MizarServiceController) processNextWorkItem() bool {
	workItem, quit := c.queue.Get()
	if quit {
		return false
	}

	eventKeyWithType := workItem.(KeyWithEventType)
	key := eventKeyWithType.Key
	defer c.queue.Done(workItem)

	err := c.syncHandler(eventKeyWithType)
	if err == nil {
		c.queue.Forget(workItem)
		return true
	}
	utilruntime.HandleError(fmt.Errorf("Handle service of key %v failed with %v", key, err))
	c.queue.AddRateLimited(eventKeyWithType)

	return true
}

func (c *MizarServiceController) createService(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for service %#v: %v", obj, err))
		return
	}
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Create})
}

func (c *MizarServiceController) updateService(old, cur interface{}) {
	new := cur.(*v1.Service)
	pre := old.(*v1.Service)

	if new.ResourceVersion == pre.ResourceVersion {
		return
	}

	key, err := controller.KeyFunc(cur)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for service %#v: %v", new, err))
		return
	}
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Update, ResourceVersion: new.ResourceVersion})
}

func (c *MizarServiceController) deleteService(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for service %#v: %v", obj, err))
		return
	}
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Delete})
}

func (c *MizarServiceController) syncService(eventKeyWithType KeyWithEventType) error {
	key := eventKeyWithType.Key
	event := eventKeyWithType.EventType

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing service %q (%v)", key, time.Since(startTime))
	}()

	tenant, namespace, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		return err
	}

	svc, err := c.serviceLister.ServicesWithMultiTenancy(namespace, tenant).Get(name)
	if err != nil {
		if event != EventType_Delete || !apierrors.IsNotFound(err) {
			return err
		}
	}

	klog.V(4).Infof("Mizar-Service-controller - get service: %#v.", svc)

	switch event {
	case EventType_Create:
		err = c.processServiceCreateOrUpdate(svc, eventKeyWithType)
	case EventType_Update:
		err = c.processServiceCreateOrUpdate(svc, eventKeyWithType)
	case EventType_Delete:
		err = c.processServiceDeletion(svc, eventKeyWithType)
	default:
		utilruntime.HandleError(fmt.Errorf("Unable to process service %v %v", event, key))
	}
	if err != nil {
		return err
	}

	return nil
}

func (c *MizarServiceController) processServiceCreateOrUpdate(service *v1.Service, eventKeyWithType KeyWithEventType) error {
	key := eventKeyWithType.Key
	tenant, _, _, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		return err
	}

	// Get tenant default network
	tenantDefaultNetwork, err := c.networkLister.NetworksWithMultiTenancy(tenant).Get(defaultNetworkName)
	if err != nil {
		klog.Warningf("Failed to retrieve network in local cache by tenant %s, name %s: %v", tenant, defaultNetworkName, err)
		return err
	}
	if tenantDefaultNetwork.Spec.Type != mizarNetworkType {
		return nil
	}

	if tenantDefaultNetwork.Status.Phase != arktosapisv1.NetworkReady {
		return errors.New(fmt.Sprintf("arktos network %s is not Ready.", tenantDefaultNetwork.Name))
	}

	vpc, vpcNameOk := service.Annotations[mizarAnnotationsVpcKey]
	subnet, subnetNameOk := service.Annotations[mizarAnnotationsSubnetKey]

	if vpcNameOk && subnetNameOk && eventKeyWithType.EventType == EventType_Update {
		// don't update mizar service if this is update event and service is already annotated
		return nil
	}
	if !vpcNameOk && !subnetNameOk {
		// assign default network when only there is no mizar annotation
		// otherwise, this pod annotation needs to be fixed manually
		if service.Annotations == nil {
			service.Annotations = make(map[string]string)
		}
		service.Annotations[mizarAnnotationsVpcKey] = getVPC(tenantDefaultNetwork)
		service.Annotations[mizarAnnotationsSubnetKey] = getSubnetNameFromVPC(tenantDefaultNetwork.Spec.VPCID)
		_, err := c.kubeClientset.CoreV1().ServicesWithMultiTenancy(service.Namespace, service.Tenant).Update(service)
		klog.V(4).Infof("Add mizar annotation for service %s. error %v", key, err)
		if err != nil {
			return errors.New(fmt.Sprintf("update service %s mizar annotation got error (%v)", key, err))
		}
	} else if !vpcNameOk || !subnetNameOk {
		// Not supported case in 2022-01-30 release
		// avoid getting infinite loop due to client error
		klog.Warningf("Invalid VPC %s or subnet %s. Skip processing service %s", vpc, subnet, key)
		return nil
	}

	klog.V(4).Infof("Starting processServiceCreation service %v: annotation [%v]. event type %v", key, service.Annotations, eventKeyWithType.EventType)

	// create service in mizar
	msg := &BuiltinsServiceMessage{
		Name:      service.Name,
		Namespace: service.Namespace,
		Tenant:    service.Tenant,
		Ip:        service.Spec.ClusterIP,
		Vpc:       service.Annotations[mizarAnnotationsVpcKey],
		Subnet:    service.Annotations[mizarAnnotationsSubnetKey],
	}

	var response *ReturnCode
	if eventKeyWithType.EventType == EventType_Create {
		response = c.grpcAdaptor.CreateService(c.grpcHost, msg)
	} else {
		response = c.grpcAdaptor.UpdateService(c.grpcHost, msg)
	}
	code := response.Code
	ip := response.Message
	klog.V(4).Infof("Assigned ip by mizar is %v. Service %v", ip, key)

	// Handle special service kubernetes-default introduced by multi-tenancy support.
	// The endpoint is not available in endpoint list but can be retrieved via get.
	// Need to create mizar endpoint for kubernetes-default
	switch code {
	case CodeType_OK:
		if beginsWithKubernetes(service.Name) && eventKeyWithType.EventType == EventType_Create {
			kubernetesEndpoint, err := c.kubeClientset.CoreV1().EndpointsWithMultiTenancy(metav1.NamespaceDefault, metav1.TenantSystem).Get(kubernetesSvcDefaultName, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("Failed to get kubernetes endpoint: %v. Error: %v", kubernetesEndpoint, err)
				return err
			}
			message := convertToServiceEndpointMessage(service.Name, service.Namespace, service.Tenant, kubernetesEndpoint, service)
			resp := c.grpcAdaptor.CreateServiceEndpoint(c.grpcHost, message)
			returnCode := resp.Code
			context := resp.Message
			switch returnCode {
			case CodeType_OK:
				klog.Info("Mizar handled kubernetes network service endpoint successfully.")
			case CodeType_TEMP_ERROR:
				klog.Warningf("Mizar hit temporary error for kubernetes network service endpoint: %s.", context)
			case CodeType_PERM_ERROR:
				klog.Errorf("Mizar hit permanent error for kubernetes network service endpoint %s.", context)
			}
		}
	case CodeType_TEMP_ERROR:
		klog.Warningf("Mizar hit temporary error for service creation for service: %s.", key)
		return errors.New("Service creation failed on mizar side, will try again.....")
	case CodeType_PERM_ERROR:
		klog.Errorf("Mizar hit permanent error for service creation for service: %s.", key)
		return errors.New("Service creation failed permanently on mizar side")
	}

	// Update service cluster ip with ip from mizar
	// Haven't found condition == true for services created by system upon start up
	// Leave the logic here and add some log for further checking
	if len(service.Spec.ClusterIP) == 0 {
		klog.Infof("Set service %s cluster ip to %v", key, ip)
		svcToUpdate := service.DeepCopy()
		svcToUpdate.Spec.ClusterIP = ip
		_, err := c.kubeClientset.CoreV1().ServicesWithMultiTenancy(service.Namespace, service.Tenant).Update(svcToUpdate)
		if err != nil {
			klog.Errorf("Failed to update service %s cluster ip to %v. Error: %v", key, ip, err)
			return err
		}
	} else if service.Spec.ClusterIP != ip {
		klog.Warningf("Service %s cluster ip %s is different from mizar assigned ip %s", key, service.Spec.ClusterIP, ip)
	}

	return nil
}

func (c *MizarServiceController) processServiceDeletion(service *v1.Service, eventKeyWithType KeyWithEventType) error {
	msg := &BuiltinsServiceMessage{
		Name:      service.Name,
		Namespace: service.Namespace,
		Tenant:    service.Tenant,
		Ip:        "",
		Vpc:       "",
		Subnet:    "",
	}

	response := c.grpcAdaptor.DeleteService(c.grpcHost, msg)
	code := response.Code
	switch code {
	case CodeType_OK:
		klog.V(4).Infof("Mizar handled service deletion successfully: %s", eventKeyWithType.Key)
	case CodeType_TEMP_ERROR:
		klog.Warningf("Mizar hit temporary error for service deletion for service: %s", eventKeyWithType.Key)
		return errors.New("Service deletion failed on mizar side, will try again.....")
	case CodeType_PERM_ERROR:
		klog.Errorf("Mizar hit permanent error for service deletion for service: %s", eventKeyWithType.Key)
		return errors.New("Service deletion failed permanently on mizar side")
	}

	return nil
}

func beginsWithKubernetes(svcName string) bool {
	name := svcName
	prefix := kubernetesSvcDefaultName + "-"
	index := strings.Index(name, prefix)
	if index == 0 {
		return true
	}
	klog.V(4).Infof("process kubernetes network services")
	return false
}

func convertToServiceEndpointMessage(name string, namespace string, tenant string, endpoints *v1.Endpoints, service *v1.Service) *BuiltinsServiceEndpointMessage {
	backendIps := []string{}
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			backendIps = append(backendIps, address.IP)
		}
	}
	backendIpsJson, _ := json.Marshal(backendIps)

	ports := []*PortsMessage{}
	for _, port := range service.Spec.Ports {
		portsMessage := &PortsMessage{
			FrontendPort: strconv.Itoa(int(port.Port)),
			BackendPort:  strconv.Itoa(int(port.TargetPort.IntVal)),
			Protocol:     string(port.Protocol),
		}
		ports = append(ports, portsMessage)
	}
	portsJson, _ := json.Marshal(ports)

	return &BuiltinsServiceEndpointMessage{
		Name:           name,
		Namespace:      namespace,
		Tenant:         tenant,
		BackendIps:     []string{},
		Ports:          []*PortsMessage{},
		BackendIpsJson: string(backendIpsJson),
		PortsJson:      string(portsJson),
	}
}
