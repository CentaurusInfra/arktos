package mizar

import (
	"errors"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
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
	labelDNSService       = "arktos.futurewei.com/network"
	dnsServiceDefaultName = "kube-dns"
)

// MizarServiceController manages service on mizar side and update cluster IP on arktos side
type MizarServiceController struct {
	netClient           arktos.Interface
	kubeClientset       *kubernetes.Clientset
	serviceLister       corelisters.ServiceLister
	netLister           arktosextv1.NetworkLister
	serviceListerSynced cache.InformerSynced
	syncHandler         func(eventKey KeyWithEventType) error
	queue               workqueue.RateLimitingInterface
	recorder            record.EventRecorder
	grpcHost            string
}

// NewMizarServiceController starts mizar service controller
func NewMizarServiceController(kubeClientset *kubernetes.Clientset, netClient arktos.Interface, serviceInformer coreinformers.ServiceInformer, arktosInformer arktosinformer.NetworkInformer, grpcHost string) *MizarServiceController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClientset.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	c := &MizarServiceController{
		kubeClientset:       kubeClientset,
		netClient:           netClient,
		serviceLister:       serviceInformer.Lister(),
		netLister:           arktosInformer.Lister(),
		serviceListerSynced: serviceInformer.Informer().HasSynced,
		queue:               workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		recorder:            eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "mizar-service-controller"}),
		grpcHost:            grpcHost,
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
	klog.Infof("Starting node controller")
	klog.Infoln("Waiting cache to be synced.")
	if ok := cache.WaitForCacheSync(stopCh, c.serviceListerSynced); !ok {
		klog.Fatalln("Timeout expired during waiting for caches to sync.")
	}
	klog.Infoln("Starting custom controller.")
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
	klog.Info("shutting down node controller")
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
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Update})
}

func (c *MizarServiceController) deleteService(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for service %#v: %v", obj, err))
		return
	}
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Delete})
}

func (c *MizarServiceController) syncService(eventKey KeyWithEventType) error {
	key := eventKey.Key
	event := eventKey.EventType

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
		return err
	}

	klog.Infof("Mizar-Service-controller - get service: %#v.", svc)

	switch event {
	case EventType_Create:
		err = c.processServiceCreation(svc, key)
	case EventType_Update:
		err = c.processServiceUpdate(svc, key)
	case EventType_Delete:
		err = c.processServiceDeletion(key)
	default:
		utilruntime.HandleError(fmt.Errorf("Unable to process service %v %v", event, key))
	}
	if err != nil {
		return err
	}

	return nil
}

func (c *MizarServiceController) processServiceCreation(service *v1.Service, key string) error {
	name := service.Name
	prefix := dnsServiceDefaultName + "-"
	netName := ""
	index := strings.Index(name, prefix)
	if index == 0 {
		pos := len(prefix)
		netName = name[pos:]
	}
	fmt.Println("processServiceCreation network name is %v", netName)
	fmt.Println("processServiceCreation service is %v", service)

	msg := &BuiltinsServiceMessage{
		Name:          service.Name,
		ArktosNetwork: netName,
		Namespace:     service.Namespace,
		Tenant:        service.Tenant,
		Ip:            service.Spec.ClusterIP,
	}

	response := GrpcCreateService(c.grpcHost, msg)
	code := response.Code
	ip := response.Message
	fmt.Println("processServiceCreation ip is %v", ip)

	if code != CodeType_OK {
		return errors.New("Service creation failed on Mizar side")
	}

	if _, hasDNSServiceLabel := service.Labels[labelDNSService]; hasDNSServiceLabel && len(netName) != 0 {
		fmt.Println("Network update starts ...")
		net, err := c.netClient.ArktosV1().NetworksWithMultiTenancy(service.Tenant).Get(netName, metav1.GetOptions{})
		if err != nil {
			fmt.Println("Network is %v, error message is: %v", err, net)
			return err
		}
		fmt.Println("Updating network: %v", net)
		if len(net.Status.DNSServiceIP) == 0 {
			netReady := net.DeepCopy()
			netReady.Status.DNSServiceIP = ip
			_, err = c.netClient.ArktosV1().NetworksWithMultiTenancy(net.Tenant).UpdateStatus(netReady)
			if err != nil {
				klog.Infof("The following network failed to update: %v", netReady)
				return err
			}
		}
	}

	if len(service.Spec.ClusterIP) == 0 {
		svcToUpdate := service.DeepCopy()
		svcToUpdate.Spec.ClusterIP = ip
		svcUpdated, err := c.kubeClientset.CoreV1().ServicesWithMultiTenancy(service.Namespace, service.Tenant).Update(svcToUpdate)
		fmt.Println("svcToUpdate is %v", svcToUpdate)
		if err != nil {
			klog.Infof("The following service failed to update: %v", svcUpdated)
			return err
		}
	}
	return nil
}

func (c *MizarServiceController) processServiceUpdate(service *v1.Service, key string) error {
	fmt.Println("processServiceUpdate network name is %v", service.Name)
	msg := &BuiltinsServiceMessage{
		Name:          service.Name,
		ArktosNetwork: "",
		Namespace:     service.Namespace,
		Tenant:        service.Tenant,
		Ip:            service.Spec.ClusterIP,
	}
	response := GrpcUpdateService(c.grpcHost, msg)
	code := response.Code
	if code != CodeType_OK {
		return errors.New("Service update failed on Mizar side")
	}
	return nil
}

func (c *MizarServiceController) processServiceDeletion(key string) error {
	tenant, namespace, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		return err
	}
	netName := strings.Split(name, dnsServiceDefaultName+"-")[0]

	msg := &BuiltinsServiceMessage{
		Name:          name,
		ArktosNetwork: netName,
		Namespace:     namespace,
		Tenant:        tenant,
		Ip:            "",
	}

	response := GrpcDeleteService(c.grpcHost, msg)
	code := response.Code
	if code != CodeType_OK {
		return errors.New("Service Deletion failed on mizar side")
	}

	return nil
}
