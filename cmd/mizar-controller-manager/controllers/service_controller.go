package app

import (
	"fmt"
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

type Controller struct {
	kubeclientset  *kubernetes.Clientset
	informer       coreinformers.ServiceInformer
	informerSynced cache.InformerSynced
	lister         corelisters.ServiceLister
	recorder       record.EventRecorder
	workqueue      workqueue.RateLimitingInterface
}

func NewServiceController(kubeclientset *kubernetes.Clientset, serviceInformer coreinformers.ServiceInformer) (*Controller, error) {
	informer := serviceInformer
	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "mizar-service-controller"})
	eventBroadcaster.StartLogging(klog.Infof)
	klog.Infof("Sending events to api server.")
	eventBroadcaster.StartRecordingToSink(
		&v1core.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	workqueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	c := &Controller{
		kubeclientset:  kubeclientset,
		informer:       informer,
		informerSynced: informer.Informer().HasSynced,
		lister:         informer.Lister(),
		recorder:       recorder,
		workqueue:      workqueue,
	}

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			fmt.Println("Service Added")
		},
		UpdateFunc: func(oldObject, newObject interface{}) {
			fmt.Println("Service Updated")
		},
	})
	return c, nil
}

func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
	klog.Infof("Starting node controller")
	klog.Infoln("Waiting cache to be synced.")
	if ok := cache.WaitForCacheSync(stopCh, nc.informerSynced); !ok {
		klog.Fatalln("Timeout expired during waiting for caches to sync.")
	}
	klog.Infoln("Starting custom controller.")
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	<-stopCh
	klog.Info("shutting down node controller")
}

func (c *Controller) runWorker() {
	for {
		item, queueIsEmpty := nc.workqueue.Get()
		if queueIsEmpty {
			break
		}
		c.process(item)
	}
}

func (c *Controller) process(item interface{}) {
	defer c.workqueue.Done(item)
	c.workqueue.Forget(item)
}
