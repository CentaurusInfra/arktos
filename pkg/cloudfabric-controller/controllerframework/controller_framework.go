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
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/util/metrics"
	"math"
)

const (
	ControllerStateInit   string = "init"
	ControllerStateLocked string = "locked"
	ControllerStateWait   string = "wait"
	ControllerStateActive string = "active"
	ControllerStateError  string = "error"

	controllerInstanceNamePrefix string = "ci"
)

type controllerInstanceLocal struct {
	instanceId    types.UID
	controllerKey int64
	lowerboundKey int64
	workloadNum   int32
	isLocked      bool
}

type ControllerBase struct {
	controllerType       string
	controllerInstanceId types.UID
	controllerName       string
	state                string

	// use int64 as k8s base deal with int64 better
	controllerKey int64

	sortedControllerInstancesLocal []controllerInstanceLocal
	controllerInstanceList         []v1.ControllerInstance

	curPos int

	client                                   clientset.Interface
	controllerInstanceUpdateByControllerType chan string
}

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

func NewControllerBase(controllerType string, client clientset.Interface, updateChan chan string) (*ControllerBase, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

	if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := metrics.RegisterMetricAndTrackRateLimiterUsage(controllerType+"_controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	// Get existed controller instances from registry
	controllerInstances, err := listControllerInstancesByType(controllerType)

	if err != nil {
		// TODO - add retry
		klog.Errorf("Error getting controller instances for %s. Error %v", controllerType, err)
	}

	sortedControllerInstances := SortControllerInstancesByKeyAndConvertToLocal(controllerInstances)

	controller := &ControllerBase{
		client:                                   client,
		controllerType:                           controllerType,
		state:                                    ControllerStateInit,
		controllerInstanceId:                     uuid.NewUUID(),
		controllerInstanceList:                   controllerInstances,
		sortedControllerInstancesLocal:           sortedControllerInstances,
		curPos:                                   -1,
		controllerInstanceUpdateByControllerType: updateChan,
	}

	controller.controllerKey = controller.generateKey()
	controller.controllerName = GetControllerName(controller.controllerInstanceId)

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

func (c *ControllerBase) GetControllerId() types.UID {
	return c.controllerInstanceId
}

func (c *ControllerBase) GetClient() clientset.Interface {
	return c.client
}

func (c *ControllerBase) WatchInstanceUpdate(stopCh <-chan struct{}) {
	var stopSign chan<- interface{}
	for {
		select {
		case stopSign <- stopCh:
			break
		case updatedType, ok := <-c.controllerInstanceUpdateByControllerType:
			if !ok {
				klog.Errorf("Unexpected controller instance update message")
				return
			}

			klog.Infof("Got controller instance update massage. Updated Controller Type %s, current controller instance type %s", updatedType, c.controllerType)
			if updatedType != c.controllerType {
				continue
			}

			klog.Infof("Start updating controller instance %s", c.controllerType)
			controllerInstances, err := listControllerInstancesByType(c.controllerType)
			if err != nil {
				// TODO - add retry
				klog.Errorf("Error getting controller instances for %s. Error %v", c.controllerType, err)
				continue
			}
			c.updateCachedControllerInstances(controllerInstances)
			klog.Infof("Done updating controller instance %s", c.controllerType)
		}
	}
}

func (c *ControllerBase) GetControllerType() string {
	return c.controllerType
}

func (c *ControllerBase) IsControllerActive() bool {
	return c.state == ControllerStateActive
}

func (c *ControllerBase) IsInRange(key int64) bool {
	if key < 0 {
		return false
	}

	if key > c.controllerKey || key <= c.sortedControllerInstancesLocal[c.curPos].lowerboundKey {
		return false
	}

	return true
}

func (c *ControllerBase) generateKey() int64 {
	if len(c.sortedControllerInstancesLocal) == 0 {
		return math.MaxInt64
	}

	min, max := c.getMaxInterval()
	return (max - min) / 2
}

func (c *ControllerBase) getMaxInterval() (int64, int64) {
	min := int64(0)
	max := int64(math.MaxInt64)

	maxWorkloadNum := (int32)(-1)
	intervalFound := false

	for i := 0; i < len(c.sortedControllerInstancesLocal); i++ {
		item := c.sortedControllerInstancesLocal[i]

		if item.workloadNum > maxWorkloadNum {
			maxWorkloadNum = item.workloadNum
			max = item.controllerKey
			min = item.lowerboundKey
			intervalFound = true
		}
	}

	if !intervalFound && len(c.sortedControllerInstancesLocal) > 0 {
		min = c.sortedControllerInstancesLocal[0].lowerboundKey
		max = c.sortedControllerInstancesLocal[0].controllerKey
	}

	return min, max
}

func (c *ControllerBase) updateCachedControllerInstances(controllerInstancesInStorage []v1.ControllerInstance) {
	sortedNewControllerInstancesLocal := SortControllerInstancesByKeyAndConvertToLocal(controllerInstancesInStorage)

	// Compare
	isUpdated, isSelfUpdated, newLowerBound, newUpperbound, newPos := c.tryConsolidateControllerInstancesLocal(sortedNewControllerInstancesLocal)
	if !isUpdated && !isSelfUpdated {
		return
	}

	message := fmt.Sprintf("Controller %s instance %s", c.controllerType, c.controllerName)
	if c.curPos >= 0 {
		message += fmt.Sprintf(" old range (%d, %d]", c.sortedControllerInstancesLocal[c.curPos].lowerboundKey, c.sortedControllerInstancesLocal[c.curPos].controllerKey)
	}
	message += fmt.Sprintf(" assigned range (%d, %d]", newLowerBound, newUpperbound)
	klog.Info(message)

	if isUpdated {
		c.sortedControllerInstancesLocal = sortedNewControllerInstancesLocal
	}

	if isSelfUpdated {
		c.state = ControllerStateWait
		c.curPos = newPos
	}

	if c.state == ControllerStateWait {
		// TODO - wait for current processing workloads being done
	}

	if isSelfUpdated {
		// TODO - reset filter
	}

	if c.state == ControllerStateLocked {
		// TODO - wait for unlock
		klog.Infof("Controller %s instance %v is currently locked. Waiting for unlock", c.controllerType, c.controllerName)
		return
	}

	c.state = ControllerStateActive
	return
}

// Assume both old & new controller instances are sorted by controller key
func (c *ControllerBase) tryConsolidateControllerInstancesLocal(newControllerInstancesLocal []controllerInstanceLocal) (isUpdated bool, isSelfUpdated bool, newLowerbound int64, newUpperbound int64, newPos int) {
	oldControllerInstancesLocal := c.sortedControllerInstancesLocal

	isUpdated = false
	if len(oldControllerInstancesLocal) != len(newControllerInstancesLocal) {
		isUpdated = true
	}

	// find position in new controller instances - assume current controller is in new controller instance list (deal with edge cases later)
	newPos = -1
	for i := 0; i < len(newControllerInstancesLocal); i++ {
		if newControllerInstancesLocal[i].instanceId == c.controllerInstanceId {
			newPos = i
			break
		}
	}

	// current instance not in new controller instance map, this controller instance lost connection with registry, pause processing
	if newPos == -1 {
		c.state = ControllerStateError
		klog.Errorf("Current instance not in registry. Controller type %s, instance id %v, key %v", c.controllerType, c.controllerName, c.controllerKey)
		return true, false, 0, 0, 0
	} else if newPos == len(newControllerInstancesLocal)-1 && c.controllerKey != math.MaxInt64 { // next to last become last
		c.controllerKey = math.MaxInt64
		isSelfUpdated = true
		isUpdated = true
	}

	if c.curPos == -1 {
		//c.curPos = newPos
		return true, true, newControllerInstancesLocal[newPos].lowerboundKey, c.controllerKey, newPos
	}

	if c.sortedControllerInstancesLocal[c.curPos].lowerboundKey != newControllerInstancesLocal[newPos].lowerboundKey {
		return true, true, newControllerInstancesLocal[newPos].lowerboundKey, c.controllerKey, newPos
	}

	if !isUpdated {
		for i := 0; i < len(newControllerInstancesLocal); i++ {
			if c.sortedControllerInstancesLocal[i].lowerboundKey != newControllerInstancesLocal[i].lowerboundKey ||
				c.sortedControllerInstancesLocal[i].workloadNum != newControllerInstancesLocal[i].workloadNum ||
				c.sortedControllerInstancesLocal[i].controllerKey != newControllerInstancesLocal[i].controllerKey ||
				c.sortedControllerInstancesLocal[i].instanceId != newControllerInstancesLocal[i].instanceId {
				isUpdated = true
			}
		}
	}

	return isUpdated, false, 0, 0, newPos
}

// register current controller instance in registry
func (c *ControllerBase) registControllerInstance() error {
	controllerInstance := v1.ControllerInstance{
		ControllerType: c.controllerType,
		UID:            c.controllerInstanceId,
		HashKey:        c.controllerKey,
		WorkloadNum:    0,
		IsLocked:       c.state == ControllerStateLocked,
		ObjectMeta: metav1.ObjectMeta{
			Name: c.controllerName,
		},
	}

	isExist := isControllerInstanceExisted(c.controllerInstanceList, c.controllerInstanceId)
	if isExist {
		// Error
		klog.Errorf("Trying to register new %s controller instance with id %v already existed in controller instance list", c.controllerType, c.controllerName)
		return errors.NewAlreadyExists(apps.Resource("controllerinstances"), "UID")
	} else {
		c.controllerInstanceList = append(c.controllerInstanceList, controllerInstance)

		// Write to registry
		_, err := c.client.CoreV1().ControllerInstances().Create(&controllerInstance)
		if err != nil {
			klog.Errorf("Error register controller %s instance %s, error %v", c.controllerType, c.controllerName, err)
			// TODO
			return err
		}

		// Check controllers updates
		c.updateCachedControllerInstances(c.controllerInstanceList)
	}

	return nil
}

// Periodically update controller instance in registry for two things:
//     1. Update workload # so that workload can be more evenly distributed
//     2. Renew TTL for current controller instance
func (c *ControllerBase) ReportHealth() {
	klog.Infof("Controller %s instance %s report health", c.controllerType, c.controllerName)
	controllerInstance := v1.ControllerInstance{
		ControllerType: c.controllerType,
		UID:            c.controllerInstanceId,
		HashKey:        c.controllerKey,
		WorkloadNum:    c.sortedControllerInstancesLocal[c.curPos].workloadNum,
		ObjectMeta: metav1.ObjectMeta{
			Name: c.controllerName,
		},
	}

	// Write to registry
	_, err := c.client.CoreV1().ControllerInstances().Update(&controllerInstance)
	if err != nil {
		klog.Errorf("Error update controller %s instance %s, error %v", c.controllerType, c.controllerName, err)
		//TODO
	}
}

func GetControllerName(instanceId types.UID) string {
	return fmt.Sprintf("%s-%v", controllerInstanceNamePrefix, instanceId)
}

// Get controller instances by controller type
//		Return sorted controller instance list & error if any
func listControllerInstancesByType(controllerType string) ([]v1.ControllerInstance, error) {
	var controllerInstances []v1.ControllerInstance

	cim := GetControllerInstanceManager()
	if cim == nil {
		klog.Fatalf("Unexpected reference to uninitialized controller instance manager")
	}

	controllerInstancesByType, err := cim.ListControllerInstances(controllerType)
	if err != nil {
		return nil, err
	}

	for _, controllerInstance := range controllerInstancesByType {
		controllerInstances = append(controllerInstances, controllerInstance)
	}
	return controllerInstances, nil
}
