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
	"strings"
	"sync"

	"github.com/grafov/bcast"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/metrics"
)

var ResetFilterHandler = resetFilter

type controllerInstanceLocal struct {
	instanceName  string
	controllerKey int64
	lowerboundKey int64
	workloadNum   int32
}

type ControllerBase struct {
	controllerType            string
	controllerName            string
	state                     string
	countOfProcessingWorkItem int
	workItemCountMux          sync.Mutex

	// use int64 as k8s base deal with int64 better
	controllerKey int64

	sortedControllerInstancesLocal []controllerInstanceLocal        // locally sorted instances and keys
	controllerInstanceMap          map[string]v1.ControllerInstance // what is in registry key: name of controller instance

	curPos int

	client                     clientset.Interface
	controllerInstanceUpdateCh *bcast.Member
	mux                        sync.Mutex
	InformerResetChGrp         *bcast.Group
}

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

func NewControllerBase(controllerType string, client clientset.Interface, cimUpdateCh *bcast.Member, informerResetChGrp *bcast.Group) (*ControllerBase, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := metrics.RegisterMetricAndTrackRateLimiterUsage(controllerType+"_controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	// Get existed controller instances from registry
	controllerInstanceMap, err := listControllerInstancesByType(controllerType)

	if err != nil {
		// TODO - add retry
		klog.Errorf("Error getting controller instances for %s. Error %v", controllerType, err)
	}

	controller := &ControllerBase{
		client:                     client,
		controllerType:             controllerType,
		controllerName:             generateControllerName(controllerType, controllerInstanceMap),
		controllerInstanceMap:      controllerInstanceMap,
		curPos:                     -1,
		controllerInstanceUpdateCh: cimUpdateCh,
		countOfProcessingWorkItem:  0,
		InformerResetChGrp:         informerResetChGrp,
	}

	controllerInstanceMap[controller.controllerName] = v1.ControllerInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: controller.controllerName,
		},
		ControllerType: controllerType,
		ControllerKey:  0,
		WorkloadNum:    0,
	}
	controller.updateCachedControllerInstances(controllerInstanceMap)

	err = controller.registControllerInstance()
	if err != nil {
		klog.Fatalf("Controller %s cannot be registered.", controllerType)
	}

	return controller, err
}

func (c *ControllerBase) GetControllerName() string {
	return c.controllerName
}

func (c *ControllerBase) GetClient() clientset.Interface {
	return c.client
}

func (c *ControllerBase) GetCountOfProcessingWorkItem() int {
	return c.countOfProcessingWorkItem
}

func (c *ControllerBase) AddProcessingWorkItem() {
	c.workItemCountMux.Lock()
	c.countOfProcessingWorkItem++
	c.workItemCountMux.Unlock()
}

func (c *ControllerBase) DoneProcessingWorkItem() {
	c.workItemCountMux.Lock()
	if c.countOfProcessingWorkItem == 0 {
		klog.Infof("Current work item under processing is 0. Done processing error. Controller %s %s", c.controllerType, c.controllerName)
	} else {
		c.countOfProcessingWorkItem--
	}
	c.workItemCountMux.Unlock()
}

func (c *ControllerBase) SetWorkloadNum(workloadNum int) {
	c.mux.Lock()
	_, isOK := c.controllerInstanceMap[c.controllerName]

	if isOK && c.curPos >= 0 {
		c.sortedControllerInstancesLocal[c.curPos].workloadNum = int32(workloadNum)
		c.mux.Unlock()
	} else {
		klog.Fatalf("Current controller instance not in map")
	}
}

func (c *ControllerBase) WatchInstanceUpdate(stopCh <-chan struct{}) {
	var stopSign chan<- interface{}
	for {
		select {
		case stopSign <- stopCh:
			break
		case updatedType, ok := <-c.controllerInstanceUpdateCh.Read:
			if !ok {
				klog.Error("Unexpected controller instance update message")
				return
			}
			c.instanceUpdateProcess(updatedType.(string))
		}
	}
}

func (c *ControllerBase) instanceUpdateProcess(updatedType string) {
	klog.Infof("Got controller instance update massage. Updated Controller Type %s, current controller instance type %s, key %d", updatedType, c.controllerType, c.controllerKey)
	if updatedType != c.controllerType {
		return
	}

	klog.V(4).Infof("Start updating controller instance type %s", c.controllerType)
	controllerInstances, err := listControllerInstancesByType(c.controllerType)
	if err != nil {
		// TODO - add retry
		klog.Errorf("Error getting controller instances for %s. Error %v", c.controllerType, err)
		return
	}
	c.updateCachedControllerInstances(controllerInstances)
	klog.V(4).Infof("Done updating controller instance type %s", c.controllerType)
}

// This method is for integration test only.
// During the teardown phase of one integration test, the other will start. The previous controller instance are still live in ETCD.
// If second integration test happens to generate workload with hashkey belongs to the dieing controller instance, without actively delete it from ETCD,
// the second integration test needs to wait 5 min to become the owner of the workload and timeout.
func (c *ControllerBase) DeleteController() {
	klog.Infof("Start deleting controller %s %s key %d", c.controllerType, c.controllerName, c.controllerKey)
	err := c.client.CoreV1().ControllerInstances().Delete(c.controllerName, &metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("Error deleting controller %s %s key %d", c.controllerType, c.controllerName, c.controllerKey)
	} else {
		klog.Infof("Successfully deleted controller %s %s key %d", c.controllerType, c.controllerName, c.controllerKey)
	}
}

func (c *ControllerBase) GetControllerType() string {
	return c.controllerType
}

func (c *ControllerBase) GetControllerKey() int64 {
	return c.controllerKey
}

func (c *ControllerBase) IsInRange(key int64) bool {
	if c.curPos == -1 { // do not process until registration complete
		return false
	}

	if key < 0 {
		return false
	}

	if key == 0 && c.sortedControllerInstancesLocal[c.curPos].lowerboundKey == 0 {
		return true
	}

	if key > c.controllerKey || key <= c.sortedControllerInstancesLocal[c.curPos].lowerboundKey {
		return false
	}

	return true
}

func (c *ControllerBase) PrintRangeAndStatus() {
	c.mux.Lock()
	defer c.mux.Unlock()

	if c.curPos >= 0 && len(c.sortedControllerInstancesLocal) > 0 {
		lowerBound := int64(0)
		if c.curPos > 0 {
			lowerBound = c.sortedControllerInstancesLocal[c.curPos].lowerboundKey
		}

		klog.V(4).Infof("Current controller %s, status %s, range [%d, %d]", c.controllerType, c.state, lowerBound, c.controllerKey)
	}
}

// Here we assume filter already being reset
func (c *ControllerBase) IsDoneProcessingCurrentWorkloads() (bool, int) {
	klog.Infof("Current controller %s instance %s state %s", c.controllerType, c.controllerName, c.state)

	if c.countOfProcessingWorkItem > 0 {
		return false, c.countOfProcessingWorkItem
	}
	return true, 0
}

func (c *ControllerBase) updateCachedControllerInstances(newControllerInstanceMap map[string]v1.ControllerInstance) {
	c.mux.Lock()
	klog.V(4).Infof("Controller %s instance %s mux accquired", c.controllerType, c.controllerName)
	defer func() {
		c.mux.Unlock()
		klog.V(4).Infof("Controller %s instance %s mux released", c.controllerType, c.controllerName)
	}()

	sortedNewControllerInstancesLocal := sortControllerInstancesByKeyAndConvertToLocal(newControllerInstanceMap)

	// Compare
	isUpdated, isSelfUpdated, newLowerBound, newUpperbound, newPos, newSortedControllerInstanceLocal :=
		c.tryConsolidateControllerInstancesLocal(sortedNewControllerInstancesLocal)
	if !isUpdated && !isSelfUpdated {
		return
	}

	if isSelfUpdated {
		var currentControllerInstance controllerInstanceLocal
		if c.curPos < 0 { // new instance during registration process
			currentControllerInstance = newSortedControllerInstanceLocal[newPos]
		} else {
			currentControllerInstance = c.sortedControllerInstancesLocal[c.curPos]
		}

		message := fmt.Sprintf("Controller %s instance %s", c.controllerType, c.controllerName)
		if currentControllerInstance.lowerboundKey != newLowerBound || currentControllerInstance.controllerKey != newUpperbound {
			if c.curPos >= 0 {
				if currentControllerInstance.lowerboundKey == 0 {
					message += fmt.Sprintf(" old range [%d, %d]", c.sortedControllerInstancesLocal[c.curPos].lowerboundKey, c.sortedControllerInstancesLocal[c.curPos].controllerKey)
				} else {
					message += fmt.Sprintf(" old range (%d, %d]", c.sortedControllerInstancesLocal[c.curPos].lowerboundKey, c.sortedControllerInstancesLocal[c.curPos].controllerKey)
				}
			}
			if newPos == 0 {
				message += fmt.Sprintf(" assigned new range [%d, %d]", newLowerBound, newUpperbound)
			} else {
				message += fmt.Sprintf(" assigned new range (%d, %d]", newLowerBound, newUpperbound)
			}
		} else {
			if newPos == 0 {
				message += fmt.Sprintf(" keeps same range [%d, %d]", newLowerBound, newUpperbound)
			} else {
				message += fmt.Sprintf(" keeps same range (%d, %d]", newLowerBound, newUpperbound)
			}
		}
		klog.Info(message)
	}

	if isUpdated {
		c.curPos = newPos
		c.sortedControllerInstancesLocal = newSortedControllerInstanceLocal
		c.controllerInstanceMap = newControllerInstanceMap

		if isSelfUpdated {
			c.controllerKey = newUpperbound
			ResetFilterHandler(c, newLowerBound, newUpperbound)
		}
	}
}

func (c *ControllerBase) needToUpdateControllerKeys(newControllerInstancesLocal []controllerInstanceLocal) (bool, []controllerInstanceLocal) {
	newCandidates := reassignControllerKeys(newControllerInstancesLocal)
	if len(c.sortedControllerInstancesLocal) != len(newCandidates) {
		return true, newCandidates
	}

	for i := 0; i < len(newCandidates); i++ {
		if c.sortedControllerInstancesLocal[i].controllerKey != newCandidates[i].controllerKey ||
			c.sortedControllerInstancesLocal[i].instanceName != newCandidates[i].instanceName {
			return true, newCandidates
		}
	}

	return false, nil
}

// Assume both old & new controller instances are sorted by controller key
// return
// 		isUpdate: 					controller instances were changed
//		isSelfUpdate:   			is current controller instance updated (include lowerbound, upperbound, islocked)
//		newLowerBound:  			new lowerbound for current controller instance
//		newUpperBound:  			new upperbound for current controller instance
//		newPos:						new position of current controller instance in sorted controller list (sorted by controllerKey - upperbound)
//		updatedControllerInstances: new sorted controller instances
//
// Note 08-06-2020: Simplify the scenario to always evenly redistribute the entire problem space first
// Will need to revisit and design for long term solution later after release 0.3 (2020/0730 release)
func (c *ControllerBase) tryConsolidateControllerInstancesLocal(newControllerInstancesLocal []controllerInstanceLocal) (isUpdated bool,
	isSelfUpdated bool, newLowerbound int64, newUpperbound int64, newPos int, updatedControllerInstances []controllerInstanceLocal) {

	isUpdated, newControllerInstancesLocal = c.needToUpdateControllerKeys(newControllerInstancesLocal)
	if !isUpdated {
		return false, false, 0, 0, 0, nil
	}

	// find position in new controller instances - assume current controller is in new controller instance list (deal with edge cases later)
	newPos = -1
	for i := 0; i < len(newControllerInstancesLocal); i++ {
		if newControllerInstancesLocal[i].instanceName == c.controllerName {
			newPos = i
			break
		}
	}

	// current instance not in new controller instance map, this controller instance lost connection with registry, pause processing, force restart
	if newPos == -1 {
		klog.Errorf("Current instance not in registry. Controller type %s, instance id %v, key %v. Needs restart", c.controllerType, c.controllerName, c.controllerKey)
		return false, false, 0, 0, 0, nil
	}
	if c.controllerKey != newControllerInstancesLocal[newPos].controllerKey ||
		c.curPos < 0 || c.sortedControllerInstancesLocal[c.curPos].lowerboundKey != newControllerInstancesLocal[newPos].lowerboundKey {
		isSelfUpdated = true
		isUpdated = true
	}

	if c.curPos == -1 { // current controller instance is joining the pool
		return true, true, newControllerInstancesLocal[newPos].lowerboundKey, newControllerInstancesLocal[newPos].controllerKey, newPos, newControllerInstancesLocal
	}

	if !isUpdated {
		for i := 0; i < len(newControllerInstancesLocal); i++ {
			if c.sortedControllerInstancesLocal[i].lowerboundKey != newControllerInstancesLocal[i].lowerboundKey ||
				c.sortedControllerInstancesLocal[i].workloadNum != newControllerInstancesLocal[i].workloadNum ||
				c.sortedControllerInstancesLocal[i].controllerKey != newControllerInstancesLocal[i].controllerKey ||
				c.sortedControllerInstancesLocal[i].instanceName != newControllerInstancesLocal[i].instanceName {
				isUpdated = true
				break
			}
		}
	}

	return isUpdated, isSelfUpdated, newControllerInstancesLocal[newPos].lowerboundKey, newControllerInstancesLocal[newPos].controllerKey, newPos, newControllerInstancesLocal
}

// register current controller instance in registry
func (c *ControllerBase) registControllerInstance() error {
	controllerInstance := v1.ControllerInstance{
		ControllerType: c.controllerType,
		ControllerKey:  c.controllerKey,
		WorkloadNum:    0,
		ObjectMeta: metav1.ObjectMeta{
			Name: c.controllerName,
		},
	}

	// Write to registry
	newControllerInstance, err := c.createControllerInstance(controllerInstance)

	if err != nil {
		klog.Errorf("Error register controller %s instance %s, error %v", c.controllerType, c.controllerName, err)
		// TODO
		return err
	}

	c.controllerInstanceMap[c.controllerName] = *newControllerInstance

	return nil
}

func (c *ControllerBase) createControllerInstance(controllerInstance v1.ControllerInstance) (*v1.ControllerInstance, error) {
	return c.client.CoreV1().ControllerInstances().Create(&controllerInstance)
}

// Periodically update controller instance in registry for two things:
//     1. Update workload # so that workload can be more evenly distributed
//     2. Renew TTL for current controller instance
func (c *ControllerBase) ReportHealth(client clientset.Interface) {
	oldControllerInstance := c.controllerInstanceMap[c.controllerName]
	controllerInstance := oldControllerInstance.DeepCopy()
	controllerInstance.WorkloadNum = c.sortedControllerInstancesLocal[c.curPos].workloadNum
	controllerInstance.ControllerKey = c.controllerKey
	controllerInstance.ResourceVersion = "" // remove resource version. Force report health

	// Write to registry
	newControllerInstance, err := client.CoreV1().ControllerInstances().Update(controllerInstance)
	if err != nil {
		klog.Errorf("Error update controller %s instance %s, error %v", c.controllerType, c.controllerName, err)
	} else {
		c.mux.Lock()
		c.controllerInstanceMap[c.controllerName] = *newControllerInstance
		c.mux.Unlock()
		klog.V(3).Infof("Controller %s instance %s report health (controller key %v). Version %s",
			newControllerInstance.ControllerType, newControllerInstance.Name, newControllerInstance.ControllerKey, newControllerInstance.ResourceVersion)
	}
}

func generateControllerName(controllerType string, existedInstanceMap map[string]v1.ControllerInstance) string {
	cimInstanceId := GetInstanceId()
	if cimInstanceId == "" {
		klog.Fatalf("Controller Instance Manager not available.")
	}

	name := fmt.Sprintf("%s-%v", strings.ToLower(controllerType), cimInstanceId)
	_, isExist := existedInstanceMap[name]
	if isExist {
		klog.Fatalf("Controller instance name %s conflict. Need to restart process to get a new one ", name)
	}

	return name
}

// Get controller instances by controller type
//		Return sorted controller instance list & error if any
func listControllerInstancesByType(controllerType string) (map[string]v1.ControllerInstance, error) {
	controllerInstanceMap := make(map[string]v1.ControllerInstance)

	cim := GetInstanceHandler()
	if cim == nil {
		klog.Fatalf("Unexpected reference to uninitialized controller instance manager")
	}

	controllerInstancesByType, err := cim.ListControllerInstances(controllerType)
	if err != nil {
		return nil, err
	}

	for _, controllerInstance := range controllerInstancesByType {
		controllerInstanceMap[controllerInstance.Name] = controllerInstance
	}
	return controllerInstanceMap, nil
}

func resetFilter(c *ControllerBase, newLowerBound, newUpperbound int64) {
	go func() {
		klog.Infof("%s: The new range %+v is going to send. obj %v", c.controllerType, []int64{newLowerBound, newUpperbound}, c.controllerName)
		resetMessage := cache.FilterBound{
			OwnerName:  c.controllerType + "_Controller",
			LowerBound: newLowerBound,
			UpperBound: newUpperbound,
		}

		c.InformerResetChGrp.Send(resetMessage)
		klog.Infof("%s: The new range %+v has been sent. resetMessage %+v", c.controllerType, []int64{newLowerBound, newUpperbound}, resetMessage)
	}()
}
