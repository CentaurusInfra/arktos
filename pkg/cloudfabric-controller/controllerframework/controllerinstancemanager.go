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

package controllerframework

import (
	"fmt"
	"github.com/grafov/bcast"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"
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
	controller "k8s.io/kubernetes/pkg/cloudfabric-controller"

	//"k8s.io/kubernetes/pkg/cloudfabric-controller"
	"sync"

	"k8s.io/kubernetes/pkg/util/metrics"
)

type ControllerInstanceManager struct {
	instanceId                  types.UID
	currentControllers          map[string](map[string]v1.ControllerInstance)
	controllerLister            corelisters.ControllerInstanceLister
	controllerListerSynced      cache.InformerSynced
	isControllerListInitialized bool

	recorder record.EventRecorder

	kubeClient     clientset.Interface
	cimUpdateChGrp *bcast.Group

	mux           sync.Mutex
	notifyHandler func(controllerInstance *v1.ControllerInstance)
}

var instance *ControllerInstanceManager
var checkInstanceHandler = checkInstanceExistence
var GetInstanceHandler = getControllerInstanceManager

func getControllerInstanceManager() *ControllerInstanceManager {
	return instance
}

func checkInstanceExistence() {
	if instance != nil {
		klog.Fatalf("Unexpected reference to controller instance manager - initialized")
	}
}

func NewControllerInstanceManager(coInformer coreinformers.ControllerInstanceInformer, kubeClient clientset.Interface, cimUpdateChGrp *bcast.Group) *ControllerInstanceManager {
	checkInstanceHandler()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	if kubeClient != nil && kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		metrics.RegisterMetricAndTrackRateLimiterUsage("controller_instance_manager", kubeClient.CoreV1().RESTClient().GetRateLimiter())
	}

	manager := &ControllerInstanceManager{
		instanceId: uuid.NewUUID(),
		kubeClient: kubeClient,
		recorder:   eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "controller-instance-manager"}),

		currentControllers:          make(map[string](map[string]v1.ControllerInstance)),
		isControllerListInitialized: false,
		cimUpdateChGrp:              cimUpdateChGrp,
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

func GetInstanceId() types.UID {
	cim := GetInstanceHandler()
	if cim != nil {
		return cim.instanceId
	} else {
		return ""
	}
}

func (cim *ControllerInstanceManager) addControllerInstance(obj interface{}) {
	newControllerInstance := obj.(*v1.ControllerInstance)
	if newControllerInstance.DeletionTimestamp != nil {
		cim.deleteControllerInstance(newControllerInstance)
		return
	}
	klog.V(3).Infof("Received event for NEW controller instance %v. CIM %v", newControllerInstance.Name, cim.instanceId)

	cim.mux.Lock()
	klog.V(4).Infof("mux acquired addControllerInstance. CIM %v", cim.instanceId)

	if cim.currentControllers == nil {
		cim.currentControllers = make(map[string](map[string]v1.ControllerInstance))
	}
	existingInstancesForType, ok := cim.currentControllers[newControllerInstance.ControllerType]

	if !ok { // real new controller instance
		cim.currentControllers[newControllerInstance.ControllerType] = make(map[string]v1.ControllerInstance)
	} else { // check existing controller instance
		existingInstance, ok1 := existingInstancesForType[newControllerInstance.Name]
		if ok1 {
			cim.mux.Unlock()
			klog.V(4).Infof("mux released addControllerInstance. CIM %v", cim.instanceId)
			cim.updateControllerInstance(&existingInstance, newControllerInstance)
			klog.V(4).Infof("Got existing controller instance %s in AddFunc. CIM %v", newControllerInstance.Name, cim.instanceId)
			return
		}
	}
	cim.currentControllers[newControllerInstance.ControllerType][newControllerInstance.Name] = *newControllerInstance

	// notify appropriate controller
	cim.notifyHandler(newControllerInstance)

	cim.mux.Unlock()
	klog.V(4).Infof("mux released addControllerInstance. CIM %v", cim.instanceId)
}

func (cim *ControllerInstanceManager) updateControllerInstance(old, cur interface{}) {
	curControllerInstance := cur.(*v1.ControllerInstance)
	oldControllerInstance := old.(*v1.ControllerInstance)

	if curControllerInstance.ResourceVersion == oldControllerInstance.ResourceVersion {
		return
	} else {
		isNewer, err := diff.RevisionStrIsNewer(curControllerInstance.ResourceVersion, oldControllerInstance.ResourceVersion)
		if err != nil {
			klog.Errorf("Update controller instance got invalid resource version. Controller Type %s; instance id %s, old rv [%s], new rv [%s]. CIM %v",
				curControllerInstance.ControllerType, curControllerInstance.Name, oldControllerInstance.ResourceVersion, curControllerInstance.ResourceVersion, cim.instanceId)
			return
		}

		if !isNewer {
			klog.V(3).Infof("Got staled controller instance %s in UpdateFunc. Existing Version %s, new instance version %s. CIM %v",
				oldControllerInstance.Name, oldControllerInstance.ResourceVersion, curControllerInstance.ResourceVersion, cim.instanceId)
			return
		}
	}

	klog.V(4).Infof("Received event for UPDATE controller instance %v", curControllerInstance.Name)

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
	klog.V(4).Infof("mux acquired updateControllerInstance. CIM %v", cim.instanceId)

	cim.currentControllers[oldControllerInstance.ControllerType][oldControllerInstance.Name] = *curControllerInstance

	if curControllerInstance.WorkloadNum != oldControllerInstance.WorkloadNum ||
		curControllerInstance.ControllerKey != oldControllerInstance.ControllerKey {
		klog.V(4).Infof("Notify controller instance %v was updated. CIM %v", curControllerInstance.Name, cim.instanceId)
		cim.notifyHandler(curControllerInstance)
	} else {
		klog.V(4).Infof("No update for controller %s instance %v. Skip updating controller instance map. CIM %v",
			curControllerInstance.ControllerType, curControllerInstance.Name, cim.instanceId)
	}

	cim.mux.Unlock()
	klog.V(4).Infof("mux released updateControllerInstance. CIM %v", cim.instanceId)
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
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a controller instance %#v. CIM %v", obj, cim.instanceId))
			return
		}
	}

	klog.V(3).Infof("Received event for deleting controller instance %v. CIM %v", controllerinstance.Name, cim.instanceId)
	cim.mux.Lock()
	klog.V(4).Infof("mux acquired deleteControllerInstance. CIM %v", cim.instanceId)

	if l, ok := cim.currentControllers[controllerinstance.ControllerType]; ok {
		if _, ok1 := l[controllerinstance.Name]; ok1 {
			delete(l, controllerinstance.Name)

			// notify appropriate controller
			cim.notifyHandler(controllerinstance)
		}
	}
	cim.mux.Unlock()
	klog.V(4).Infof("mux released deleteControllerInstance. CIM %v", cim.instanceId)
}

func (cim *ControllerInstanceManager) Run(stopCh <-chan struct{}) {
	klog.Infof("Starting controller instance manager. CIM %v", cim.instanceId)
	defer klog.Infof("Shutting down controller instance manager %v", cim.instanceId)

	if !controller.WaitForCacheSync("Controller Instance Manager", stopCh, cim.controllerListerSynced) {
		klog.Infof("Controller instances NOT synced %v. CIM %v", cim.controllerListerSynced, cim.instanceId)
		return
	}

	<-stopCh
}

func (cim *ControllerInstanceManager) GetUpdateChGrp() *bcast.Group {
	return cim.cimUpdateChGrp
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
	klog.V(4).Infof("mux acquired syncControllerInstances. CIM %v", cim.instanceId)
	defer func() {
		cim.mux.Unlock()
		klog.V(4).Infof("mux released syncControllerInstances. CIM %v", cim.instanceId)
	}()
	controllerInstanceList, err := cim.kubeClient.CoreV1().ControllerInstances().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	klog.V(3).Infof("Found %d of controller instances from registry. CIM %v", len(controllerInstanceList.Items), cim.instanceId)

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
		cim.cimUpdateChGrp.Send(controllerInstance.ControllerType)
	}()
}
