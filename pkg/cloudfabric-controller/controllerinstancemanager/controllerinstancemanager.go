/*
Copyright 2019 The Kubernetes Authors.

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

package controllerinstancemanager

import (
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"strconv"

	//"k8s.io/kubernetes/pkg/cloudfabric-controller"
	"k8s.io/kubernetes/pkg/util/metrics"
	"sync"
)

type ControllerInstanceManager struct {
	currentControllers          map[string](map[types.UID]v1.ControllerInstance)
	controllerLister            corelisters.ControllerInstanceLister
	controllerListerSynced      cache.InformerSynced
	isControllerListInitialized bool

	recorder record.EventRecorder

	kubeClient                   clientset.Interface
	controllerInstanceChangeChan chan string

	mux           sync.Mutex
	notifyHandler func(controllerInstance *v1.ControllerInstance)
}

var instance *ControllerInstanceManager

func GetInstance() *ControllerInstanceManager {
	if instance == nil {
		klog.Fatalf("Unexpected reference to controller instance manager - uninitialized")
		return nil
	}

	return instance
}

func NewControllerInstanceManager(coInformer coreinformers.ControllerInstanceInformer, kubeClient clientset.Interface, instanceChangeNotifyChan chan string) *ControllerInstanceManager {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient != nil && kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		metrics.RegisterMetricAndTrackRateLimiterUsage("job_controller", kubeClient.CoreV1().RESTClient().GetRateLimiter())
	}

	manager := &ControllerInstanceManager{
		kubeClient: kubeClient,
		recorder:   eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "controller-instance-manager"}),

		currentControllers:           make(map[string](map[types.UID]v1.ControllerInstance)),
		isControllerListInitialized:  false,
		controllerInstanceChangeChan: instanceChangeNotifyChan,
	}

	coInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    manager.addControllerInstance,
		UpdateFunc: manager.updateControllerInstance,
		DeleteFunc: manager.deleteControllerInstance,
	})

	manager.controllerLister = coInformer.Lister()
	manager.controllerListerSynced = coInformer.Informer().HasSynced
	err := manager.syncControllerInstances()
	if err != nil {
		klog.Fatalf("Unable to get controller instances from registry. Error %v", err)
	}

	manager.notifyHandler = manager.notifyControllerInstanceChanges
	instance = manager

	return instance
}

func (cim *ControllerInstanceManager) addControllerInstance(obj interface{}) {
	newControllerInstance := obj.(*v1.ControllerInstance)
	if newControllerInstance.DeletionTimestamp != nil {
		cim.deleteControllerInstance(newControllerInstance)
		return
	}
	klog.Infof("Received event for NEW controller instance %v", newControllerInstance.UID)

	cim.mux.Lock()
	klog.Info("mux locked addControllerInstance")
	defer cim.mux.Unlock()

	if cim.currentControllers == nil {
		cim.currentControllers = make(map[string](map[types.UID]v1.ControllerInstance))
	}
	existingInstancesForType, ok := cim.currentControllers[newControllerInstance.ControllerType]
	if !ok { // real new controller instance
		cim.currentControllers[newControllerInstance.ControllerType] = make(map[types.UID]v1.ControllerInstance)
	} else { // check existing controller instance
		existingInstance, ok1 := existingInstancesForType[newControllerInstance.UID]
		if ok1 {
			cim.updateControllerInstance(existingInstance, newControllerInstance)
			klog.Infof("Got existing controller instance %s in AddFunc", newControllerInstance.Name)
			return
		}
	}
	cim.currentControllers[newControllerInstance.ControllerType][newControllerInstance.UID] = *newControllerInstance

	// notify appropriate controller
	cim.notifyHandler(newControllerInstance)
	klog.Infof("mux unlocked addControllerInstance")
}

func (cim *ControllerInstanceManager) updateControllerInstance(old, cur interface{}) {
	curControllerInstance := cur.(*v1.ControllerInstance)
	oldControllerInstance := old.(*v1.ControllerInstance)

	if curControllerInstance.ResourceVersion == oldControllerInstance.ResourceVersion {
		return
	} else {
		oldRev, _ := strconv.Atoi(oldControllerInstance.ResourceVersion)
		newRev, err := strconv.Atoi(curControllerInstance.ResourceVersion)
		if err != nil {
			klog.Errorf("Got invalid resource version %s for controller instance %v", curControllerInstance.ResourceVersion, curControllerInstance)
			return
		}

		if newRev < oldRev {
			klog.Infof("Got staled controller instance %s in UpdateFunc. Existing Version %s, new instance version %s",
				oldControllerInstance.Name, oldControllerInstance.ResourceVersion, curControllerInstance.ResourceVersion)
			return
		}
	}

	klog.Infof("Received event for UPDATE controller instance %v", curControllerInstance.UID)

	if curControllerInstance.UID != oldControllerInstance.UID {
		klog.Fatalf("Unexpected controller instance UID changed from %v to %v.", oldControllerInstance.UID, curControllerInstance.UID)
	}
	if curControllerInstance.Name != oldControllerInstance.Name {
		klog.Fatalf("Unexpected controller instance Name changed from %v to %v.", oldControllerInstance.Name, curControllerInstance.Name)
	}
	if curControllerInstance.ControllerType != oldControllerInstance.ControllerType {
		klog.Fatalf("Unexpected controller instance type changed from %s to %s.", oldControllerInstance.ControllerType, curControllerInstance.ControllerType)
	}
	if curControllerInstance.DeletionTimestamp != nil {
		cim.deleteControllerInstance(curControllerInstance)
		return
	}

	cim.mux.Lock()
	klog.Infof("mux locked updateControllerInstance")
	defer cim.mux.Unlock()

	cim.currentControllers[oldControllerInstance.ControllerType][oldControllerInstance.UID] = *curControllerInstance

	if curControllerInstance.WorkloadNum != oldControllerInstance.WorkloadNum || curControllerInstance.IsLocked != oldControllerInstance.IsLocked ||
		curControllerInstance.HashKey != oldControllerInstance.HashKey {
		klog.Infof("Notify controller instance %v was updated", curControllerInstance.UID)
		cim.notifyHandler(curControllerInstance)
	}

	klog.Infof("mux unlocked updateControllerInstance")
}

func (cim *ControllerInstanceManager) deleteControllerInstance(obj interface{}) {
	controllerinstance, ok := obj.(*v1.ControllerInstance)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v", obj))
			return
		}
		controllerinstance, ok = tombstone.Obj.(*v1.ControllerInstance)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a pod %#v", obj))
			return
		}
	}

	klog.Infof("Received event for delete controller instance %v", controllerinstance.UID)
	cim.mux.Lock()
	klog.Infof("mux locked deleteControllerInstance")
	defer cim.mux.Unlock()

	if l, ok := cim.currentControllers[controllerinstance.ControllerType]; ok {
		if _, ok1 := l[controllerinstance.UID]; ok1 {
			delete(l, controllerinstance.UID)

			// notify appropriate controller
			cim.notifyHandler(controllerinstance)
		}
	}
	klog.Infof("mux unlocked deleteControllerInstance")
}

func (cim *ControllerInstanceManager) Run(stopCh <-chan struct{}) {
	klog.Infof("Starting controller instance manager")
	defer klog.Infof("Shutting down controller instance manager")

	if !WaitForCacheSync("Controller Instance Manager", stopCh, cim.controllerListerSynced) {
		klog.Infof("Controller instances NOT synced %v", cim.controllerListerSynced)
		return
	}

	<-stopCh
}

func (cim *ControllerInstanceManager) ListControllerInstances(controllerType string) (map[types.UID]v1.ControllerInstance, error) {
	if !cim.isControllerListInitialized {
		err := cim.syncControllerInstances()
		if err != nil {
			return nil, err
		}
	}
	return cim.currentControllers[controllerType], nil
}

func (cim *ControllerInstanceManager) syncControllerInstances() error {
	cim.mux.Lock()
	klog.Infof("mux locked syncControllerInstances")
	defer cim.mux.Unlock()

	controllerInstanceList, err := cim.kubeClient.CoreV1().ControllerInstances().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	klog.Infof("Found %d of controller instances from registry", len(controllerInstanceList.Items))

	for _, controllerInstance := range controllerInstanceList.Items {
		controllersByType, ok := cim.currentControllers[controllerInstance.ControllerType]
		if !ok {
			controllersByType = make(map[types.UID]v1.ControllerInstance)
			cim.currentControllers[controllerInstance.ControllerType] = controllersByType
		}
		controllersByType[controllerInstance.UID] = controllerInstance
	}

	cim.isControllerListInitialized = true

	klog.Infof("mux unlocked syncControllerInstances")
	return nil
}

func (cim *ControllerInstanceManager) notifyControllerInstanceChanges(controllerInstance *v1.ControllerInstance) {
	cim.controllerInstanceChangeChan <- controllerInstance.ControllerType
}

// WaitForCacheSync is a wrapper around cache.WaitForCacheSync that generates log messages
// indicating that the controller identified by controllerName is waiting for syncs, followed by
// either a successful or failed sync.
func WaitForCacheSync(controllerName string, stopCh <-chan struct{}, cacheSyncs ...cache.InformerSynced) bool {
	klog.Infof("Waiting for caches to sync for %s controller", controllerName)

	if !cache.WaitForCacheSync(stopCh, cacheSyncs...) {
		utilruntime.HandleError(fmt.Errorf("unable to sync caches for %s controller", controllerName))
		return false
	}

	klog.Infof("Caches are synced for %s controller", controllerName)
	return true
}
