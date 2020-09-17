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
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	arktoscheme "k8s.io/arktos-ext/pkg/generated/clientset/versioned/scheme"
	arktosinformer "k8s.io/arktos-ext/pkg/generated/informers/externalversions/arktosextensions/v1"
	arktosv1 "k8s.io/arktos-ext/pkg/generated/listers/arktosextensions/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	mizarNetworkType = "mizar"
)

// MizarArktosNetworkController delivers grpc message to Mizar to update VPC with arktos network name
type MizarArktosNetworkController struct {
	netClientset    *arktos.Clientset
	kubeClientset   *kubernetes.Clientset
	netLister       arktosv1.NetworkLister
	netListerSynced cache.InformerSynced
	syncHandler     func(eventKeyWithType KeyWithEventType) error
	queue           workqueue.RateLimitingInterface
	recorder        record.EventRecorder
	grpcHost        string
}

// NewMizarArktosNetworkController starts arktos network controller for mizar
func NewMizarArktosNetworkController(netClientset *arktos.Clientset, kubeClientset *kubernetes.Clientset, informer arktosinformer.NetworkInformer, grpcHost string) *MizarArktosNetworkController {
	utilruntime.Must(arktoscheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClientset.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	c := &MizarArktosNetworkController{
		netClientset:    netClientset,
		kubeClientset:   kubeClientset,
		netLister:       informer.Lister(),
		netListerSynced: informer.Informer().HasSynced,
		queue:           workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		recorder:        eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "mizar-arktos-network-controller"}),
		grpcHost:        grpcHost,
	}

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.createNetwork,
	})

	c.netLister = informer.Lister()
	c.netListerSynced = informer.Informer().HasSynced
	c.syncHandler = c.syncNetwork

	return c
}

// Run update from mizar cluster
func (c *MizarArktosNetworkController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Info("Starting Mizar arktos network controller")
	klog.V(5).Info("waiting for informer caches to sync")
	if !cache.WaitForCacheSync(stopCh, c.netListerSynced) {
		klog.Error("failed to wait for cache to sync")
		return
	}
	klog.V(5).Info("staring workers of network controller")
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	klog.V(5).Infof("%d workers started", workers)
	<-stopCh
	klog.Info("shutting down mizar arktos network controller")
}

func (c *MizarArktosNetworkController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *MizarArktosNetworkController) processNextWorkItem() bool {
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

	utilruntime.HandleError(fmt.Errorf("Handle arktos network of key %q failed with %v", key, err))
	c.queue.AddRateLimited(eventKeyWithType)

	return true
}

func (c *MizarArktosNetworkController) createNetwork(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", obj, err))
		return
	}
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Create})
}

func (c *MizarArktosNetworkController) syncNetwork(eventKeyWithType KeyWithEventType) error {
	key := eventKeyWithType.Key
	event := eventKeyWithType.EventType

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing service %q (%v)", key, time.Since(startTime))
	}()

	tenant, name, err := cache.SplitMetaTenantKey(key)
	if err != nil {
		return err
	}

	net, err := c.netLister.NetworksWithMultiTenancy(tenant).Get(name)
	if err != nil {
		return err
	}

	klog.Infof("Mizar-Arktos-Network-controller - get network: %#v.", net)

	switch event {
	case EventType_Create:
		err = c.processNetworkCreation(net, eventKeyWithType)
	default:
		panic(fmt.Sprintf("unimplemented for eventType %v", event))
	}
	if err != nil {
		return err
	}
	return nil
}

func (c *MizarArktosNetworkController) processNetworkCreation(network *v1.Network, eventKeyWithType KeyWithEventType) error {
	//skip update or create if type is not mizar or network status is ready
	key := eventKeyWithType.Key
	if network.Spec.Type != mizarNetworkType || network.Status.Phase == v1.NetworkReady {
		c.recorder.Eventf(network, corev1.EventTypeNormal, "processNetworkCreation", "Type is not mizar, nothing to be done in mizar cluster: %v.", network)
		return nil
	}

	msg := &BuiltinsArktosMessage{
		Name: network.Name,
		Vpc:  network.Spec.VPCID,
	}

	response := GrpcCreateArktosNetwork(c.grpcHost, msg)

	code := response.Code
	context := response.Message

	switch code {
	case CodeType_OK:
		klog.Infof("Mizar handled arktos network and vpc id update successfully: %s", key)
	case CodeType_TEMP_ERROR:
		klog.Warningf("Mizar hit temporary error for arktos network and vpc id update: %s", key)
		c.queue.AddRateLimited(eventKeyWithType)
		return errors.New("Arktos network and vpc id update failed on mizar side, will try again.....")
	case CodeType_PERM_ERROR:
		klog.Errorf("Mizar hit permanent error for Arktos network creation for Arktos network: %s", key)
		return errors.New("Arktos network and vpc id update failed permanently on mizar side")
	}

	c.recorder.Eventf(network, corev1.EventTypeNormal, "processNetworkCreation", "successfully created network from mizar cluster: %v.", context)
	return nil
}
