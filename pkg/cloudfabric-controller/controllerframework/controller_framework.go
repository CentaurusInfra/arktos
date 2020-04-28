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
	generalErrors "errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/grafov/bcast"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/metrics"
)

const (
	ControllerStateInit   string = "init"
	ControllerStateLocked string = "locked"
	ControllerStateWait   string = "wait"
	ControllerStateActive string = "active"
	ControllerStateError  string = "error"

	controllerInstanceNamePrefix string = "ci"
)

var ResetFilterHandler = resetFilter
var CreateControllerInstanceHandler = createControllerInstance

type controllerInstanceLocal struct {
	instanceName  string
	controllerKey int64
	lowerboundKey int64
	workloadNum   int32
	isLocked      bool
}

func (c *controllerInstanceLocal) Size() int64 {
	size := c.controllerKey - c.lowerboundKey
	if c.lowerboundKey == 0 && c.controllerKey != math.MaxInt64 {
		size++
	}
	return size
}

type ControllerBase struct {
	controllerType            string
	controllerName            string
	state                     string
	countOfProcessingWorkItem int
	workItemCountMux          sync.Mutex

	// use int64 as k8s base deal with int64 better
	controllerKey int64

	sortedControllerInstancesLocal []controllerInstanceLocal
	controllerInstanceMap          map[string]v1.ControllerInstance // key: name of controller instance

	curPos int

	client                          clientset.Interface
	controllerInstanceUpdateCh      *bcast.Member
	mux                             sync.Mutex
	unlockControllerInstanceHandler func(local controllerInstanceLocal) error
	createControllerInstanceHandler func(v1.ControllerInstance) (*v1.ControllerInstance, error)
	InformerResetChGrp              *bcast.Group
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

	sortedControllerInstances := SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceMap)

	controller := &ControllerBase{
		client:         client,
		controllerType: controllerType,
		//state:                                    ControllerStateInit,
		controllerInstanceMap:          controllerInstanceMap,
		sortedControllerInstancesLocal: sortedControllerInstances,
		curPos:                         -1,
		controllerInstanceUpdateCh:     cimUpdateCh,
		countOfProcessingWorkItem:      0,
		InformerResetChGrp:             informerResetChGrp,
	}

	controllerKey, err := controller.generateKey()
	if err != nil {
		return nil, err
	}
	controller.controllerKey = controllerKey
	controller.controllerName = generateControllerName()
	controller.unlockControllerInstanceHandler = controller.unlockControllerInstance

	// First controller instance. No need to wait for others
	if len(controller.sortedControllerInstancesLocal) == 0 {
		controller.state = ControllerStateActive
	} else {
		controller.state = ControllerStateLocked
	}

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
	defer c.mux.Unlock()

	_, isOK := c.controllerInstanceMap[c.controllerName]
	if isOK && c.curPos >= 0 {
		c.sortedControllerInstancesLocal[c.curPos].workloadNum = int32(workloadNum)
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
			klog.Infof("Got controller instance update massage. Updated Controller Type %s, current controller instance type %s, key %d", updatedType, c.controllerType, c.controllerKey)
			if updatedType != c.controllerType {
				continue
			}

			klog.V(4).Infof("Start updating controller instance %s", c.controllerType)
			controllerInstances, err := listControllerInstancesByType(c.controllerType)
			if err != nil {
				// TODO - add retry
				klog.Errorf("Error getting controller instances for %s. Error %v", c.controllerType, err)
				continue
			}
			c.updateCachedControllerInstances(controllerInstances)
			klog.V(4).Infof("Done updating controller instance %s", c.controllerType)
		}
	}
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

func (c *ControllerBase) IsControllerActive() bool {
	return c.state == ControllerStateActive
}

func (c *ControllerBase) IsInRange(key int64) bool {
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
			lowerBound = c.sortedControllerInstancesLocal[c.curPos-1].controllerKey
		}

		klog.V(4).Infof("Current controller %s, status %s, range [%d, %d]", c.controllerType, c.state, lowerBound, c.sortedControllerInstancesLocal[c.curPos].controllerKey)
	}
}

// Here we assume filter already being reset
func (c *ControllerBase) IsDoneProcessingCurrentWorkloads() (bool, int) {
	klog.Infof("Current controller %s instance %s state %s", c.controllerType, c.controllerName, c.state)

	if c.countOfProcessingWorkItem > 0 {
		return false, c.countOfProcessingWorkItem
	}
	if c.state == ControllerStateWait {
		c.state = ControllerStateActive
	}

	if c.state == ControllerStateActive {
		c.tryUnlockControllerInstance(c.sortedControllerInstancesLocal, c.curPos)
	}

	return true, 0
}

// Generate controllerKey for new controller instance. It is to find and split a scope for new controller instance.
// Scope Splitting Principles:
// 1. We always find existing scope with biggest size, and split it.
// 2. If there are more than one scope at the biggest size, we chose the one with most ongoing work load, and split it.
// 3. If both existing scope size and ongoing work are even, we choose first scope and split it.
func (c *ControllerBase) generateKey() (int64, error) {
	if len(c.sortedControllerInstancesLocal) == 0 {
		return math.MaxInt64, nil
	}

	candidate := c.sortedControllerInstancesLocal[0]
	for i := 1; i < len(c.sortedControllerInstancesLocal); i++ {
		item := c.sortedControllerInstancesLocal[i]

		// There are two conditions to be met then change candidate:
		// 1. if the space is bigger
		// 2. or the space size is same, but with more work load
		// When splitting odd size space, the sub spaces has 1 difference in size. So ignore difference of 1 when comparing two spaces' size.
		// Which is said, we consider it is bigger when it's bigger more than 1, and we consider both are equal even they have diff 1.
		if item.Size() > candidate.Size()+1 ||
			(math.Abs(float64(item.Size()-candidate.Size())) <= 1 && item.workloadNum > candidate.workloadNum) {
			candidate = item
		}
	}

	spaceToSplit := candidate.controllerKey - candidate.lowerboundKey

	// Add one to space to guarantee the first half will have more than second half when space to split is not even.
	// But don't apply if the scope starting from 0 because it already got extra space from number 0.
	if spaceToSplit != math.MaxInt64 && candidate.lowerboundKey != 0 {
		spaceToSplit++
	}

	if spaceToSplit <= 1 {
		return -1, generalErrors.New("no enough space to split for new controller manager instance")
	}
	return candidate.lowerboundKey + spaceToSplit/2, nil
}

func (c *ControllerBase) updateCachedControllerInstances(newControllerInstanceMap map[string]v1.ControllerInstance) {
	c.mux.Lock()
	klog.V(4).Infof("Controller %s instance %s mux accquired", c.controllerType, c.controllerName)
	defer func() {
		c.mux.Unlock()
		klog.V(4).Infof("Controller %s instance %s mux released", c.controllerType, c.controllerName)
	}()

	sortedNewControllerInstancesLocal := SortControllerInstancesByKeyAndConvertToLocal(newControllerInstanceMap)

	// Compare
	isUpdated, isSelfUpdated, newLowerBound, newUpperbound, newPos := c.tryConsolidateControllerInstancesLocal(sortedNewControllerInstancesLocal)
	if !isUpdated && !isSelfUpdated {
		return
	}

	isSelfRangeUpdated := false

	if isUpdated {
		defer func() {
			c.curPos = newPos
			c.sortedControllerInstancesLocal = sortedNewControllerInstancesLocal
			c.controllerInstanceMap = newControllerInstanceMap

			if isSelfUpdated {
				c.controllerKey = c.sortedControllerInstancesLocal[c.curPos].controllerKey
			}

			if isSelfRangeUpdated {
				ResetFilterHandler(c, newLowerBound, newUpperbound)
			}

			c.IsDoneProcessingCurrentWorkloads()
		}()
	}

	if isSelfUpdated {
		var currentControllerInstance controllerInstanceLocal
		if c.curPos < 0 {
			currentControllerInstance = sortedNewControllerInstancesLocal[newPos]
			isSelfRangeUpdated = true
		} else {
			currentControllerInstance = c.sortedControllerInstancesLocal[c.curPos]

			if currentControllerInstance.lowerboundKey != newLowerBound || currentControllerInstance.controllerKey != newUpperbound {
				isSelfRangeUpdated = true
			}
		}

		if isSelfRangeUpdated {
			klog.Infof("Self range updated %d, %d", newLowerBound, newUpperbound)
		}

		message := fmt.Sprintf("Controller %s instance %s", c.controllerType, c.controllerName)
		if currentControllerInstance.lowerboundKey != newLowerBound || currentControllerInstance.controllerKey != newUpperbound {
			if c.sortedControllerInstancesLocal[c.curPos].lowerboundKey == 0 {
				message += fmt.Sprintf(" old range [%d, %d]", c.sortedControllerInstancesLocal[c.curPos].lowerboundKey, c.sortedControllerInstancesLocal[c.curPos].controllerKey)
			} else {
				message += fmt.Sprintf(" old range (%d, %d]", c.sortedControllerInstancesLocal[c.curPos].lowerboundKey, c.sortedControllerInstancesLocal[c.curPos].controllerKey)
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

		isLowerBoundExtended := false
		isUpperBoundExtended := false
		if newLowerBound < currentControllerInstance.lowerboundKey {
			isLowerBoundExtended = true
		}
		if currentControllerInstance.controllerKey < newUpperbound {
			isUpperBoundExtended = true
		}

		// Currently, we only extend lower bound when previous controller died, and extend upper bound when last controller died.
		// Therefore, there is no need to wait for workload release

		if (isLowerBoundExtended && newPos != 0) || (isUpperBoundExtended && newPos != len(newControllerInstanceMap)-1) {
			klog.Infof("Controller %s instance %s range extended", c.controllerType, c.controllerName)
			//c.state = ControllerStateLocked
		}
		if len(sortedNewControllerInstancesLocal) == 1 && c.state != ControllerStateActive {
			c.state = ControllerStateActive // self unlock does not need to report immediately. It can be updated via health report
		}

		if c.state == ControllerStateLocked {
			if !sortedNewControllerInstancesLocal[newPos].isLocked {
				// this instance is unlocked
				klog.Infof("Controller %s instance %s is unlocked", c.controllerType, c.controllerName)
				c.state = ControllerStateActive
			} else {
				klog.Infof("Controller %s instance %s is locked, wait for unlock", c.controllerType, c.controllerName)
				// wait for unlock
				return
			}
		}

		isLowerBoundIncreased := false
		isUpperBoundLowered := false
		if currentControllerInstance.lowerboundKey < newLowerBound {
			isLowerBoundIncreased = true
		}
		if newUpperbound < currentControllerInstance.controllerKey {
			isUpperBoundLowered = true
		}

		if isLowerBoundIncreased || isUpperBoundLowered {
			c.state = ControllerStateWait
			// wait for finishing current processing items that belongs to excluded range
			klog.Infof("Controller %s instance %s status wait", c.controllerType, c.controllerName)
			return
		}

	}

	if c.state != ControllerStateActive {
		// wait for status update
		klog.Infof("Controller %s instance %v is currently %s. Wait for status update", c.controllerType, c.controllerName, c.state)
		return
	}

	// active controller instance can unlock instance 1 position ahead of it
	c.tryUnlockControllerInstance(sortedNewControllerInstancesLocal, newPos)

	return
}

func (c *ControllerBase) tryUnlockControllerInstance(sortedControllerInstances []controllerInstanceLocal, pos int) {
	klog.V(4).Infof("entering trying unlock controller instance. pos %v. current controller %s", pos, c.controllerName)
	if pos >= 1 && sortedControllerInstances[pos-1].isLocked {
		klog.V(4).Infof("Trying to unlock controller %s on position %d", sortedControllerInstances[pos-1].instanceName, pos-1)
		err := c.unlockControllerInstanceHandler(sortedControllerInstances[pos-1])
		klog.V(4).Infof("Done unlocking controller %s on position %d. err %v", sortedControllerInstances[pos-1].instanceName, pos-1, err)
		if err != nil {
			// TODO - add retry logic
			klog.Fatalf("Unable to unlock controller %s instance %s. panic for now.", c.controllerType, sortedControllerInstances[pos-1].instanceName)
		}
	}
}

// Assume both old & new controller instances are sorted by controller key
// return
// 		isUpdate: 		controller instances were changed
//		isSelfUpdate:   is current controller instance updated (include lowerbound, upperbound, islocked)
//		newLowerBound:  new lowerbound for current controller instance
//		newUpperBound:  new upperbound for current controller instance
//		newPos:			new position of current controller instance in sorted controller list (sorted by controllerKey - upperbound)
func (c *ControllerBase) tryConsolidateControllerInstancesLocal(newControllerInstancesLocal []controllerInstanceLocal) (isUpdated bool, isSelfUpdated bool, newLowerbound int64, newUpperbound int64, newPos int) {
	isUpdated = false
	if len(c.sortedControllerInstancesLocal) != len(newControllerInstancesLocal) {
		isUpdated = true
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
		return false, false, 0, 0, 0
	} else if newPos == len(newControllerInstancesLocal)-1 && c.controllerKey != math.MaxInt64 { // next to last become last
		//c.controllerKey = math.MaxInt64
		newControllerInstancesLocal[newPos].controllerKey = math.MaxInt64
		/*if c.state == ControllerStateLocked {
			c.state = ControllerStateActive
		}*/
		isSelfUpdated = true
		isUpdated = true

	}

	if c.curPos == -1 { // current controller instance is joining the pool
		return true, true, newControllerInstancesLocal[newPos].lowerboundKey, c.controllerKey, newPos
	}

	if !isSelfUpdated {
		currentInstance := c.sortedControllerInstancesLocal[c.curPos]
		newInstance := newControllerInstancesLocal[newPos]

		if currentInstance.isLocked != newInstance.isLocked || currentInstance.lowerboundKey != newInstance.lowerboundKey ||
			currentInstance.controllerKey != newInstance.controllerKey {
			isSelfUpdated = true
			isUpdated = true
		}
	}

	if !isUpdated {
		for i := 0; i < len(newControllerInstancesLocal); i++ {
			if c.sortedControllerInstancesLocal[i].lowerboundKey != newControllerInstancesLocal[i].lowerboundKey ||
				c.sortedControllerInstancesLocal[i].workloadNum != newControllerInstancesLocal[i].workloadNum ||
				c.sortedControllerInstancesLocal[i].controllerKey != newControllerInstancesLocal[i].controllerKey ||
				c.sortedControllerInstancesLocal[i].instanceName != newControllerInstancesLocal[i].instanceName ||
				c.sortedControllerInstancesLocal[i].isLocked != newControllerInstancesLocal[i].isLocked {
				isUpdated = true
				break
			}
		}
	}

	return isUpdated, isSelfUpdated, newControllerInstancesLocal[newPos].lowerboundKey, newControllerInstancesLocal[newPos].controllerKey, newPos
}

func (c *ControllerBase) unlockControllerInstance(controllerInstanceToUnlockLocal controllerInstanceLocal) error {
	oldControllerInstance, isOK := c.controllerInstanceMap[controllerInstanceToUnlockLocal.instanceName]
	if !isOK {
		err := errors.NewBadRequest(fmt.Sprintf("Need to unlock controller %s instance %s missing in controller instance map", c.controllerType, controllerInstanceToUnlockLocal.instanceName))
		klog.Error(err)
		return err
	} else if !oldControllerInstance.IsLocked {
		err := errors.NewBadRequest(fmt.Sprintf("Controller %s instance %s is not locked in instance map", c.controllerType, controllerInstanceToUnlockLocal.instanceName))
		klog.Error(err)
		return err
	}

	controllerInstance := oldControllerInstance.DeepCopy()
	controllerInstance.ResourceVersion = ""
	controllerInstance.IsLocked = false

	_, err := c.client.CoreV1().ControllerInstances().Update(controllerInstance)
	if err == nil {
		klog.Infof("Controller %s instance %s unlocked.", c.controllerType, controllerInstanceToUnlockLocal.instanceName)
		return nil
	} else {
		klog.Errorf("Error unlock controller %s instance %s, error %v. instance [%#v]", c.controllerType, controllerInstanceToUnlockLocal.instanceName, err, controllerInstance)
		return err
	}
}

// register current controller instance in registry
func (c *ControllerBase) registControllerInstance() error {
	controllerInstance := v1.ControllerInstance{
		ControllerType: c.controllerType,
		ControllerKey:  c.controllerKey,
		WorkloadNum:    0,
		IsLocked:       c.state == ControllerStateLocked,
		ObjectMeta: metav1.ObjectMeta{
			Name: c.controllerName,
		},
	}

	_, isExist := c.controllerInstanceMap[c.controllerName]
	if isExist {
		// Error
		klog.Errorf("Trying to register new %s controller instance with id %v already existed in controller instance list", c.controllerType, c.controllerName)
		return errors.NewBadRequest(fmt.Sprintf("Controllerinstances name %s already existed", c.controllerName))
	} else {
		// Write to registry
		newControllerInstance, err := CreateControllerInstanceHandler(c, controllerInstance)

		if err != nil {
			klog.Errorf("Error register controller %s instance %s, error %v", c.controllerType, c.controllerName, err)
			// TODO
			return err
		}

		c.controllerInstanceMap[c.controllerName] = *newControllerInstance

		// Check controllers updates
		c.updateCachedControllerInstances(c.controllerInstanceMap)
	}

	return nil
}

// Periodically update controller instance in registry for two things:
//     1. Update workload # so that workload can be more evenly distributed
//     2. Renew TTL for current controller instance
func (c *ControllerBase) ReportHealth() {
	oldControllerInstance := c.controllerInstanceMap[c.controllerName]
	controllerInstance := oldControllerInstance.DeepCopy()
	controllerInstance.WorkloadNum = c.sortedControllerInstancesLocal[c.curPos].workloadNum
	controllerInstance.IsLocked = c.state == ControllerStateLocked
	controllerInstance.ControllerKey = c.controllerKey
	klog.V(3).Infof("Controller %s instance %s report health. Version %s", c.controllerType, c.controllerName, controllerInstance.ResourceVersion)

	// Write to registry
	newControllerInstance, err := c.client.CoreV1().ControllerInstances().Update(controllerInstance)
	if err != nil {
		errorMessage := err.Error()
		if strings.Contains(errorMessage, genericregistry.OptimisticLockErrorMsg) && strings.Contains(errorMessage, "Operation cannot be fulfilled") {
			klog.Infof("Skip reporting health this time as registry was updated. old ResourceVersion %s", controllerInstance.ResourceVersion)
		} else {
			klog.Errorf("Error update controller %s instance %s, error %v", c.controllerType, c.controllerName, err)
		}
	} else {
		c.controllerInstanceMap[c.controllerName] = *newControllerInstance
	}
}

func generateControllerName() string {
	uid := uuid.NewUUID()
	return fmt.Sprintf("%s-%v", controllerInstanceNamePrefix, uid)
}

// Get controller instances by controller type
//		Return sorted controller instance list & error if any
func listControllerInstancesByType(controllerType string) (map[string]v1.ControllerInstance, error) {
	controllerInstanceMap := make(map[string]v1.ControllerInstance)

	cim := GetControllerInstanceManager()
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

func createControllerInstance(c *ControllerBase, controllerInstance v1.ControllerInstance) (*v1.ControllerInstance, error) {
	return c.client.CoreV1().ControllerInstances().Create(&controllerInstance)
}
