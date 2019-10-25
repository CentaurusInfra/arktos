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

package controllerframework

import (
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
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
	instanceId                  types.UID
	currentControllers          map[string](map[string]v1.ControllerInstance)
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
var checkInstanceHandler = checkInstanceExistence

func GetControllerInstanceManager() *ControllerInstanceManager {
	return instance
}

func checkInstanceExistence() {
	if instance != nil {
		klog.Fatalf("Unexpected reference to controller instance manager - initialized")
	}
}

func NewControllerInstanceManager(coInformer coreinformers.ControllerInstanceInformer, kubeClient clientset.Interface, instanceChangeNotifyChan chan string) *ControllerInstanceManager {
	checkInstanceHandler()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient != nil && kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		metrics.RegisterMetricAndTrackRateLimiterUsage("job_controller", kubeClient.CoreV1().RESTClient().GetRateLimiter())
	}

	manager := &ControllerInstanceManager{
		instanceId: uuid.NewUUID(),
		kubeClient: kubeClient,
		recorder:   eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "controller-instance-manager"}),

		currentControllers:           make(map[string](map[string]v1.ControllerInstance)),
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

func (cim *ControllerInstanceManager) GetInstanceId() types.UID {
	return cim.instanceId
}

func (cim *ControllerInstanceManager) addControllerInstance(obj interface{}) {
	newControllerInstance := obj.(*v1.ControllerInstance)
	if newControllerInstance.DeletionTimestamp != nil {
		cim.deleteControllerInstance(newControllerInstance)
		return
	}
	klog.Infof("Received event for NEW controller instance %v. CIM %v", newControllerInstance.Name, cim.instanceId)

	cim.mux.Lock()
	klog.Infof("mux acquired addControllerInstance. CIM %v", cim.instanceId)
	defer func() {
		cim.mux.Unlock()
		klog.Infof("mux released addControllerInstance. CIM %v", cim.instanceId)
	}()

	if cim.currentControllers == nil {
		cim.currentControllers = make(map[string](map[string]v1.ControllerInstance))
	}
	existingInstancesForType, ok := cim.currentControllers[newControllerInstance.ControllerType]
	if !ok { // real new controller instance
		cim.currentControllers[newControllerInstance.ControllerType] = make(map[string]v1.ControllerInstance)
	} else { // check existing controller instance
		existingInstance, ok1 := existingInstancesForType[newControllerInstance.Name]
		if ok1 {
			cim.updateControllerInstance(&existingInstance, newControllerInstance)
			klog.Infof("Got existing controller instance %s in AddFunc. CIM %v", newControllerInstance.Name, cim.instanceId)
			return
		}
	}
	cim.currentControllers[newControllerInstance.ControllerType][newControllerInstance.Name] = *newControllerInstance

	// notify appropriate controller
	cim.notifyHandler(newControllerInstance)
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
			klog.Errorf("Got invalid resource version %s for controller instance %v. CIM %v", curControllerInstance.ResourceVersion, curControllerInstance, cim.instanceId)
			return
		}

		if newRev < oldRev {
			klog.Infof("Got staled controller instance %s in UpdateFunc. Existing Version %s, new instance version %s. CIM %v",
				oldControllerInstance.Name, oldControllerInstance.ResourceVersion, curControllerInstance.ResourceVersion, cim.instanceId)
			return
		}
	}

	klog.Infof("Received event for UPDATE controller instance %v", curControllerInstance.Name)

	if curControllerInstance.Name != oldControllerInstance.Name {
		klog.Fatalf("Unexpected controller instance Name changed from %v to %v. CIM %v", oldControllerInstance.Name, curControllerInstance.Name, cim.instanceId)
	}
	if curControllerInstance.ControllerType != oldControllerInstance.ControllerType {
		klog.Fatalf("Unexpected controller instance type changed from %s to %s. CIM %v", oldControllerInstance.ControllerType, curControllerInstance.ControllerType, cim.instanceId)
	}
	if curControllerInstance.DeletionTimestamp != nil {
		cim.deleteControllerInstance(curControllerInstance)
		return
	}

	cim.mux.Lock()
	klog.Infof("mux acquired updateControllerInstance. CIM %v", cim.instanceId)
	defer func() {
		cim.mux.Unlock()
		klog.Infof("mux released updateControllerInstance. CIM %v", cim.instanceId)
	}()

	cim.currentControllers[oldControllerInstance.ControllerType][oldControllerInstance.Name] = *curControllerInstance

	if curControllerInstance.WorkloadNum != oldControllerInstance.WorkloadNum || curControllerInstance.IsLocked != oldControllerInstance.IsLocked ||
		curControllerInstance.ControllerKey != oldControllerInstance.ControllerKey {
		klog.Infof("Notify controller instance %v was updated. CIM %v", curControllerInstance.Name, cim.instanceId)
		cim.notifyHandler(curControllerInstance)
	}
}

func (cim *ControllerInstanceManager) deleteControllerInstance(obj interface{}) {
	controllerinstance, ok := obj.(*v1.ControllerInstance)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v. CIM %v", obj, cim.instanceId))
			return
		}
		controllerinstance, ok = tombstone.Obj.(*v1.ControllerInstance)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a pod %#v. CIM %v", obj, cim.instanceId))
			return
		}
	}

	klog.Infof("Received event for delete controller instance %v. CIM %v", controllerinstance.Name, cim.instanceId)
	cim.mux.Lock()
	klog.Infof("mux acquired deleteControllerInstance. CIM %v", cim.instanceId)
	defer func() {
		cim.mux.Unlock()
		klog.Infof("mux released deleteControllerInstance. CIM %v", cim.instanceId)
	}()

	if l, ok := cim.currentControllers[controllerinstance.ControllerType]; ok {
		if _, ok1 := l[controllerinstance.Name]; ok1 {
			delete(l, controllerinstance.Name)

			// notify appropriate controller
			cim.notifyHandler(controllerinstance)
		}
	}
}

func (cim *ControllerInstanceManager) Run(stopCh <-chan struct{}) {
	klog.Infof("Starting controller instance manager. CIM %v", cim.instanceId)
	defer klog.Infof("Shutting down controller instance manager %v", cim.instanceId)

	if !WaitForCacheSync("Controller Instance Manager", stopCh, cim.controllerListerSynced) {
		klog.Infof("Controller instances NOT synced %v. CIM %v", cim.controllerListerSynced, cim.instanceId)
		return
	}

	<-stopCh
}

func (cim *ControllerInstanceManager) GetUpdateCh() chan string {
	return cim.controllerInstanceChangeChan
}

func (cim *ControllerInstanceManager) ListControllerInstances(controllerType string) (map[string]v1.ControllerInstance, error) {
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
	klog.Infof("mux acquired syncControllerInstances. CIM %v", cim.instanceId)
	defer func() {
		cim.mux.Unlock()
		klog.Infof("mux released syncControllerInstances. CIM %v", cim.instanceId)
	}()

	controllerInstanceList, err := cim.kubeClient.CoreV1().ControllerInstances().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	klog.Infof("Found %d of controller instances from registry. CIM %v", len(controllerInstanceList.Items), cim.instanceId)

	for _, controllerInstance := range controllerInstanceList.Items {
		controllersByType, ok := cim.currentControllers[controllerInstance.ControllerType]
		if !ok {
			controllersByType = make(map[string]v1.ControllerInstance)
			cim.currentControllers[controllerInstance.ControllerType] = controllersByType
		}
		controllersByType[controllerInstance.Name] = controllerInstance
	}

	cim.isControllerListInitialized = true

	return nil
}

func (cim *ControllerInstanceManager) notifyControllerInstanceChanges(controllerInstance *v1.ControllerInstance) {
	go func() {
		cim.controllerInstanceChangeChan <- controllerInstance.ControllerType
	}()
}
