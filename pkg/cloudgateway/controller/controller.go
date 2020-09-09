package controller

import (
	"fmt"
	"github.com/kubeedge/beehive/pkg/core"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset/versioned"
	informers "k8s.io/kubernetes/pkg/client/informers/externalversions"
	listers "k8s.io/kubernetes/pkg/client/listers/cloudgateway/v1"
	"k8s.io/kubernetes/pkg/cloudgateway/common/modules"
	"k8s.io/kubernetes/pkg/cloudgateway/controller/config"
	"k8s.io/kubernetes/pkg/cloudgateway/controller/handler"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// start parameters
var (
	onlyOneSignalHandler = make(chan struct{})
	shutdownSignals      = []os.Signal{os.Interrupt, syscall.SIGTERM}
)

func setupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler)
	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1)
	}()

	return stop
}

// Controller use beehive context message layer
type Controller struct {
	enable                      bool
	clientset                   clientset.Interface
	informerFactory             informers.SharedInformerFactory
	serviceExposeInformerLister listers.ServiceExposeLister
	policyInformerLister        listers.EPolicyLister
	// service lister can list/get service from the shared informer's store
	serviceLister               listers.EServiceLister
	serverLister                listers.EServerLister
	serviceExposeInformerSynced cache.InformerSynced
	policyInformerSynced        cache.InformerSynced
	serviceInformerSynced       cache.InformerSynced
	serverInformerSynced        cache.InformerSynced
	serviceExposeQueue          workqueue.RateLimitingInterface
	policyQueue                 workqueue.RateLimitingInterface
	serviceQueue                workqueue.RateLimitingInterface
	serverQueue                 workqueue.RateLimitingInterface
	serviceExposeHandler        handler.Handler
	policyHandler               handler.Handler
	serviceHandler              handler.Handler
	serverHandler               handler.Handler
}

func newController(enable bool, kc *v1.KubeAPIConfig) *Controller {
	klog.V(4).Infof("Controller building")
	cfg, err := clientcmd.BuildConfigFromFlags(kc.Master, kc.KubeConfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	gatewayClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building gateway clientset: %s", err.Error())
	}

	informerFactory := informers.NewSharedInformerFactory(gatewayClient, time.Second*30)
	serviceExposeInformer := informerFactory.Cloudgateway().V1().ServiceExposes()
	policyInformer := informerFactory.Cloudgateway().V1().EPolicies()
	serviceInformer := informerFactory.Cloudgateway().V1().EServices()
	siteInformer := informerFactory.Cloudgateway().V1().ESites()
	serverInformer := informerFactory.Cloudgateway().V1().EServers()
	gatewayInformer := informerFactory.Cloudgateway().V1().EGateways()
	c := &Controller{
		enable:                      enable,
		clientset:                   gatewayClient,
		informerFactory:             informerFactory,
		serviceExposeInformerLister: serviceExposeInformer.Lister(),
		policyInformerLister:        policyInformer.Lister(),
		serviceLister:               serviceInformer.Lister(),
		serverLister:                serverInformer.Lister(),
		serviceExposeInformerSynced: serviceExposeInformer.Informer().HasSynced,
		policyInformerSynced:        policyInformer.Informer().HasSynced,
		serviceInformerSynced:       serviceInformer.Informer().HasSynced,
		serverInformerSynced:        serverInformer.Informer().HasSynced,
		serviceExposeQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ServiceExpose"),
		policyQueue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "EPolicy"),
		serviceQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "EService"),
		serverQueue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "EServer"),
		serviceExposeHandler: handler.NewServiceExposeHandler(serviceInformer.Lister(), siteInformer.Lister(),
			policyInformer.Lister(), serverInformer.Lister(), gatewayInformer.Lister()),
		policyHandler: &handler.EPolicyHandler{},
		serviceHandler: handler.NewEServiceHandler(serviceInformer.Lister(), siteInformer.Lister(),
			gatewayClient),
		serverHandler: handler.NewEServerHandler(serverInformer.Lister(), siteInformer.Lister(),
			gatewayClient),
	}

	// Add serviceExpose event handler
	serviceExposeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueServiceExpose,
		UpdateFunc: func(old, new interface{}) {
			oldv := old.(*v1.ServiceExpose)
			newv := new.(*v1.ServiceExpose)
			if oldv.ResourceVersion == newv.ResourceVersion {
				return
			}

			c.enqueueServiceExpose(new)
		},
		DeleteFunc: c.enqueueServiceExposeForDelete,
	})

	// Add policy event handler
	policyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueuePolicy,
		UpdateFunc: func(old, new interface{}) {
			oldv := old.(*v1.EPolicy)
			newv := new.(*v1.EPolicy)
			if oldv.ResourceVersion == newv.ResourceVersion {
				return
			}

			c.enqueuePolicy(new)
		},
		DeleteFunc: c.enqueuePolicyForDelete,
	})

	// Add service event handler
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueService,
		UpdateFunc: func(old, new interface{}) {
			oldv := old.(*v1.EService)
			newv := new.(*v1.EService)
			if oldv.ResourceVersion == newv.ResourceVersion {
				return
			}

			c.enqueueService(new)
		},
		DeleteFunc: c.enqueueServiceForDelete,
	})

	// Add server event handler
	serverInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueServer,
		UpdateFunc: func(old, new interface{}) {
			oldv := old.(*v1.EServer)
			newv := new.(*v1.EServer)
			if oldv.ResourceVersion == newv.ResourceVersion {
				return
			}

			c.enqueueServer(new)
		},
		DeleteFunc: c.enqueueServerForDelete,
	})

	return c
}

func (c *Controller) enqueueServiceExpose(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	c.serviceExposeQueue.AddRateLimited(key)
	klog.V(4).Infof("Try to enqueueServiceExpose key: %#v ...", key)
}

func (c *Controller) enqueueServiceExposeForDelete(obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	c.serviceExposeQueue.AddRateLimited(key)
	klog.V(4).Infof("Try to enqueueServiceExposeForDelete key: %#v ...", key)
}

func (c *Controller) enqueueService(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	c.serviceQueue.AddRateLimited(key)
	klog.V(4).Infof("Try to enqueueService key: %#v ...", key)
}

func (c *Controller) enqueueServiceForDelete(obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	c.serviceQueue.AddRateLimited(key)
	klog.V(4).Infof("Try to enqueueServiceForDelete key: %#v ...", key)
}

func (c *Controller) enqueueServer(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	c.serverQueue.AddRateLimited(key)
	klog.V(4).Infof("Try to enqueueServer key: %#v ...", key)
}

func (c *Controller) enqueueServerForDelete(obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	c.serverQueue.AddRateLimited(key)
	klog.V(4).Infof("Try to enqueueServerForDelete key: %#v ...", key)
}

func (c *Controller) enqueuePolicy(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	c.policyQueue.AddRateLimited(key)
	klog.V(4).Infof("Try to enqueuePolicy key: %#v ...", key)
}

func (c *Controller) enqueuePolicyForDelete(obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	c.policyQueue.AddRateLimited(key)
	klog.V(4).Infof("Try to enqueuePolicyForDelete key: %#v ...", key)
}

func Register(c *v1.Controller, kc *v1.KubeAPIConfig) {
	config.InitConfigure(c)
	core.Register(newController(c.Enable, kc))
}

func (c *Controller) Name() string {
	return modules.ControllerModuleName
}

func (c *Controller) Group() string {
	return modules.ControllerGroup
}

// Enable indicates whether enable this module
func (c *Controller) Enable() bool {
	return c.enable
}

func (c *Controller) Start() {
	stopCh := setupSignalHandler()
	go c.informerFactory.Start(stopCh)

	if err := c.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.serviceExposeQueue.ShutDown()
	defer c.policyQueue.ShutDown()
	defer c.serviceQueue.ShutDown()
	defer c.serverQueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.V(4).Info("Starting gateway controller control loop")
	if ok := cache.WaitForCacheSync(stopCh, c.serviceExposeInformerSynced); !ok {
		return fmt.Errorf("failed to wait for gateway servcie expose caches to sync")
	}

	if ok := cache.WaitForCacheSync(stopCh, c.policyInformerSynced); !ok {
		return fmt.Errorf("failed to wait for gateway policy caches to sync")
	}

	if ok := cache.WaitForCacheSync(stopCh, c.serviceInformerSynced); !ok {
		return fmt.Errorf("failed to wait for gateway service caches to sync")
	}

	if ok := cache.WaitForCacheSync(stopCh, c.serverInformerSynced); !ok {
		return fmt.Errorf("failed to wait for gateway server caches to sync")
	}

	klog.V(4).Info("Starting gateway workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorkerForServiceExpose, time.Second, stopCh)
		go wait.Until(c.runWorkerForPolicy, time.Second, stopCh)
		go wait.Until(c.runWorkerForService, time.Second, stopCh)
		go wait.Until(c.runWorkerForServer, time.Second, stopCh)
	}

	klog.V(4).Info("Starting gateway workers")
	<-stopCh
	klog.V(4).Info("Shutting down gateway workers")
	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue
func (c *Controller) runWorkerForServiceExpose() {
	klog.V(4).Info("Gateway controller.runWorkerForServiceExpose: starting")

	// invoke processNextItem to fetch and consume the next change
	// to a watched or listed resource
	for c.processNextItemForServiceExpose() {
		klog.V(4).Info("Gateway controller.runWorkerForServiceExpose: processing next item")
	}

	klog.V(4).Info("Gateway controller.runWorkerForServiceExpose: completed")
}

func (c *Controller) processNextItemForServiceExpose() bool {
	klog.V(4).Info("Gateway controller.processNextItemForServiceExpose: start")

	// fetch the next item from the workqueue to process or
	// if a shutdown iss requested then return out of this to stop
	// processing
	obj, shutdown := c.serviceExposeQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.serviceExposeQueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.serviceExposeQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in serviceExposeQueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandlerForServiceExpose(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.serviceExposeQueue.Forget(obj)
		klog.V(4).Infof("Successfully gateway service expose synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) runWorkerForPolicy() {
	klog.V(4).Info("Gateway controller.runWorkerForPolicy: starting")

	// invoke processNextItem to fetch and consume the next change
	// to a watched or listed resource
	for c.processNextItemForPolicy() {
		klog.V(4).Info("Gateway controller.runWorkerForPolicy: processing next item")
	}

	klog.V(4).Info("Gateway controller.runWorkerForPolicy: completed")
}

func (c *Controller) runWorkerForServer() {
	klog.V(4).Info("Gateway controller.runWorkerForServer: starting")

	// invoke processNextItem to fetch and consume the next change
	// to a watched or listed resource
	for c.processNextItemForServer() {
		klog.V(4).Info("Gateway controller.runWorkerForServer: processing next item")
	}

	klog.V(4).Info("Gateway controller.runWorkerForServer: completed")
}

func (c *Controller) processNextItemForServer() bool {
	klog.V(4).Info("Gateway controller.processNextItemForServer: start")

	// fetch the next item from the workqueue to process or
	// if a shutdown iss requested then return out of this to stop
	// processing
	obj, shutdown := c.serverQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.serverQueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.serverQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in serverQueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandlerForServer(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.serverQueue.Forget(obj)
		klog.V(4).Infof("Successfully gateway server synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandlerForServer(key string) error {
	// convert the tenant/namespace/name string into a distinct namespace and name
	tenant, namespace, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	p, err := c.serverLister.EServersWithMultiTenancy(namespace, tenant).Get(name)
	if errors.IsNotFound(err) {
		klog.V(4).Infof("%v has been deleted", key)
		c.serverHandler.ObjectDeleted(tenant, namespace, p)
		return nil
	} else if err != nil {
		runtime.HandleError(fmt.Errorf("failed to list service by: %s/%s/%s", tenant, namespace, name))
		return err
	}

	// Add or update cases
	klog.V(4).Infof("%v has been added/updated", key)
	c.serverHandler.ObjectCreated(tenant, namespace, p)
	return nil
}

func (c *Controller) runWorkerForService() {
	klog.V(4).Info("Gateway controller.runWorkerForService: starting")

	// invoke processNextItem to fetch and consume the next change
	// to a watched or listed resource
	for c.processNextItemForService() {
		klog.V(4).Info("Gateway controller.runWorkerForService: processing next item")
	}

	klog.V(4).Info("Gateway controller.runWorkerForService: completed")
}

func (c *Controller) processNextItemForService() bool {
	klog.V(4).Info("Gateway controller.processNextItemForService: start")

	// fetch the next item from the workqueue to process or
	// if a shutdown iss requested then return out of this to stop
	// processing
	obj, shutdown := c.serviceQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.serviceQueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.serviceQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in serviceQueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandlerForService(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.serviceQueue.Forget(obj)
		klog.V(4).Infof("Successfully gateway service synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandlerForService(key string) error {
	// convert the tenant/namespace/name string into a distinct namespace and name
	tenant, namespace, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	p, err := c.serviceLister.EServicesWithMultiTenancy(namespace, tenant).Get(name)
	if errors.IsNotFound(err) {
		klog.V(4).Infof("%v has been deleted", key)
		c.serviceHandler.ObjectDeleted(tenant, namespace, p)
		return nil
	} else if err != nil {
		runtime.HandleError(fmt.Errorf("failed to list service by: %s/%s/%s", tenant, namespace, name))
		return err
	}

	// Add or update cases
	klog.V(4).Infof("%v has been added/updated", key)
	c.serviceHandler.ObjectCreated(tenant, namespace, p)
	return nil
}

func (c *Controller) processNextItemForPolicy() bool {
	klog.V(4).Info("Gateway controller.processNextItemForPolicy: start")

	// fetch the next item from the workqueue to process or
	// if a shutdown iss requested then return out of this to stop
	// processing
	obj, shutdown := c.policyQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.policyQueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.policyQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in policyQueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandlerForPolicy(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.policyQueue.Forget(obj)
		klog.V(4).Infof("Successfully gateway policy synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandlerForServiceExpose(key string) error {
	// convert the tenant/namespace/name string into a distinct namespace and name
	tenant, namespace, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	se, err := c.serviceExposeInformerLister.ServiceExposesWithMultiTenancy(namespace, tenant).Get(name)
	if errors.IsNotFound(err) {
		klog.V(4).Infof("%v has been deleted", key)
		c.serviceExposeHandler.ObjectDeleted(tenant, namespace, se)
		return nil
	} else if err != nil {
		runtime.HandleError(fmt.Errorf("failed to list service expose by: %s/%s/%s", tenant, namespace, name))
		return err
	}

	// Add or update cases
	klog.V(4).Infof("%v has been added/updated", key)
	c.serviceExposeHandler.ObjectCreated(tenant, namespace, se)
	return nil
}

func (c *Controller) syncHandlerForPolicy(key string) error {
	// convert the tenant/namespace/name string into a distinct namespace and name
	tenant, namespace, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	p, err := c.policyInformerLister.EPoliciesWithMultiTenancy(namespace, tenant).Get(name)
	if errors.IsNotFound(err) {
		klog.V(4).Infof("%v has been deleted", key)
		c.policyHandler.ObjectDeleted(tenant, namespace, p)
		return nil
	} else if err != nil {
		runtime.HandleError(fmt.Errorf("failed to list policy by: %s/%s/%s", tenant, namespace, name))
		return err
	}

	// Add or update cases
	klog.V(4).Infof("%v has been added/updated", key)
	c.policyHandler.ObjectCreated(tenant, namespace, p)
	return nil
}
