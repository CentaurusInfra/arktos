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
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	arktosextv1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	arktosinformer "k8s.io/arktos-ext/pkg/generated/informers/externalversions/arktosextensions/v1"
	arktosv1 "k8s.io/arktos-ext/pkg/generated/listers/arktosextensions/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	controllerForMizarPod     = "mizar_pod"
	defaultNetworkName        = "default"
	vpcSuffix                 = "-default-network"
	subnetSuffix              = "-subnet"
	mizarAnnotationsVpcKey    = "mizar.com/vpc"
	mizarAnnotationsSubnetKey = "mizar.com/subnet"
)

// MizarPodController points to current controller
type MizarPodController struct {
	// Allow to update pod object's annotation to API server
	kubeClient clientset.Interface

	// A store of network objects, populated by the shared informer passed to MizarPodController
	netLister arktosv1.NetworkLister

	// A store of pod objects, populated by the shared informer passed to MizarPodController
	lister corelisters.PodLister
	// listerSynced returns true if the store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	listerSynced cache.InformerSynced

	// To allow injection for testing.
	handler func(keyWithEventType KeyWithEventType) error

	// Queue that used to hold thing to be handled.
	queue workqueue.RateLimitingInterface

	grpcHost string

	grpcAdaptor IGrpcAdaptor
}

// NewMizarPodController creates and configures a new controller instance
func NewMizarPodController(podInformer coreinformers.PodInformer, kubeClient clientset.Interface, arktosNetworkInformer arktosinformer.NetworkInformer, grpcHost string, grpcAdaptor IGrpcAdaptor) *MizarPodController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	c := &MizarPodController{
		kubeClient:   kubeClient,
		netLister:    arktosNetworkInformer.Lister(),
		lister:       podInformer.Lister(),
		listerSynced: podInformer.Informer().HasSynced,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerForMizarPod),
		grpcHost:     grpcHost,
		grpcAdaptor:  grpcAdaptor,
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.createObj,
		UpdateFunc: c.updateObj,
		DeleteFunc: c.deleteObj,
	})
	c.lister = podInformer.Lister()
	c.listerSynced = podInformer.Informer().HasSynced

	c.handler = c.handle

	return c
}

// Run begins watching and handling.
func (c *MizarPodController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %v controller", controllerForMizarPod)
	defer klog.Infof("Shutting down %v controller", controllerForMizarPod)

	if !controller.WaitForCacheSync(controllerForMizarPod, stopCh, c.listerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *MizarPodController) createObj(obj interface{}) {
	key, _ := controller.KeyFunc(obj)
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Create})
}

// When an object is updated.
func (c *MizarPodController) updateObj(old, cur interface{}) {
	curObj := cur.(*v1.Pod)
	oldObj := old.(*v1.Pod)
	if curObj.ResourceVersion == oldObj.ResourceVersion {
		// Periodic resync will send update events for all known objects.
		// Two different versions of the same object will always have different RVs.
		return
	}
	if curObj.DeletionTimestamp != nil {
		// Object is being deleted. Don't handle update anymore.
		return
	}

	key, _ := controller.KeyFunc(curObj)
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Update, ResourceVersion: curObj.ResourceVersion})
}

func (c *MizarPodController) deleteObj(obj interface{}) {
	key, _ := controller.KeyFunc(obj)
	klog.Infof("%v deleted. key %s.", controllerForMizarPod, key)
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Delete})
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the handler is never invoked concurrently with the same key.
func (c *MizarPodController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *MizarPodController) processNextWorkItem() bool {
	workItem, quit := c.queue.Get()

	if quit {
		return false
	}

	keyWithEventType := workItem.(KeyWithEventType)
	key := keyWithEventType.Key
	defer c.queue.Done(workItem)

	err := c.handler(keyWithEventType)
	if err == nil {
		c.queue.Forget(workItem)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("Handle %v of key %v failed with %v", controllerForMizarPod, key, err))
	c.queue.AddRateLimited(keyWithEventType)

	return true
}

func (c *MizarPodController) handle(keyWithEventType KeyWithEventType) error {
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType
	klog.Infof("Entering handling for %v. key %s, eventType %s", controllerForMizarPod, key, eventType)

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished handling %v %q (%v)", controllerForMizarPod, key, time.Since(startTime))
	}()

	tenant, namespace, name, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		return err
	}

	obj, err := c.lister.PodsWithMultiTenancy(namespace, tenant).Get(name)
	if err != nil {
		if eventType == EventType_Delete && errors.IsNotFound(err) {
			obj = &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Tenant:    tenant,
				},
			}
		} else {
			return err
		}
	}

	//The annotations of vpc and subnet should not be added into pods
	//which use host networking
	if eventType == EventType_Create && !obj.Spec.HostNetwork {
		network, err := c.netLister.NetworksWithMultiTenancy(tenant).Get(defaultNetworkName)

		if err != nil {
			klog.Warningf("Failed to retrieve network in local cache by tenant %s, name %s: %v", tenant, defaultNetworkName, err)
			return err
		}

		if network.Spec.Type != mizarNetworkType || network.Status.Phase != arktosextv1.NetworkReady {
			klog.Warningf("The arktos network %s is not mizar type or is not Ready.", network.Name)
			return nil
		}

		vpc := network.Spec.VPCID
		if len(vpc) == 0 {
			vpc = fmt.Sprintf("%s%s", tenant, vpcSuffix)
		}
		subnet := fmt.Sprintf("%s%s", vpc, subnetSuffix)

		needUpdate, obj := annotatePod(obj, vpc, subnet)
		if needUpdate {
			_, err := c.kubeClient.CoreV1().PodsWithMultiTenancy(obj.Namespace, obj.Tenant).Update(obj)
			if err != nil {
				klog.Errorf("Pod name (%s) - update pod's annotation to API server in error (%v) when (%v).", obj.Name, err, eventType)
				return err
			}

		}
	}

	klog.V(4).Infof("Handling %v %s/%s/%s for event %v", controllerForMizarPod, tenant, namespace, name, eventType)

	switch eventType {
	case EventType_Create:
		processPodGrpcReturnCode(c, c.grpcAdaptor.CreatePod(c.grpcHost, obj), keyWithEventType)
	case EventType_Update:
		processPodGrpcReturnCode(c, c.grpcAdaptor.UpdatePod(c.grpcHost, obj), keyWithEventType)
	case EventType_Delete:
		processPodGrpcReturnCode(c, c.grpcAdaptor.DeletePod(c.grpcHost, obj), keyWithEventType)
	default:
		panic(fmt.Sprintf("unimplemented for eventType %v", eventType))
	}

	return nil
}

func annotatePod(obj *v1.Pod, vpc, subnet string) (bool, *v1.Pod) {
	needUpdate := false
	if len(obj.Annotations) == 0 {
		obj.Annotations = map[string]string{
			mizarAnnotationsVpcKey:    vpc,
			mizarAnnotationsSubnetKey: subnet,
		}
		needUpdate = true
	} else {
		vpcName, vpcNameOk := obj.Annotations[mizarAnnotationsVpcKey]
		subnetName, subnetNameOk := obj.Annotations[mizarAnnotationsSubnetKey]

		if (!vpcNameOk && !subnetNameOk) || (!subnetNameOk && vpcName != vpc) ||
			(!vpcNameOk && subnetName != subnet) || (vpcName != vpc && subnetName != subnet) {
			obj.Annotations[mizarAnnotationsVpcKey] = vpc
			obj.Annotations[mizarAnnotationsSubnetKey] = subnet
			needUpdate = true
		} else if !vpcNameOk || vpcName != vpc {
			obj.Annotations[mizarAnnotationsVpcKey] = vpc
			needUpdate = true
		} else if !subnetNameOk || subnetName != subnet {
			obj.Annotations[mizarAnnotationsSubnetKey] = subnet
			needUpdate = true
		}
	}

	return needUpdate, obj

}

func processPodGrpcReturnCode(c *MizarPodController, returnCode *ReturnCode, keyWithEventType KeyWithEventType) {
	key := keyWithEventType.Key
	eventType := keyWithEventType.EventType
	switch returnCode.Code {
	case CodeType_OK:
		klog.Infof("Mizar handled request successfully for %v. key %s, eventType %v", controllerForMizarPod, key, eventType)
	case CodeType_TEMP_ERROR:
		klog.Warningf("Mizar hit temporary error for %v. key %s. %s, eventType %v", controllerForMizarPod, key, returnCode.Message, eventType)
		c.queue.AddRateLimited(keyWithEventType)
	case CodeType_PERM_ERROR:
		klog.Errorf("Mizar hit permanent error for %v. key %s. %s, eventType %v", controllerForMizarPod, key, returnCode.Message, eventType)
	default:
		klog.Errorf("unimplemented for CodeType %v", returnCode.Code)
	}
}
