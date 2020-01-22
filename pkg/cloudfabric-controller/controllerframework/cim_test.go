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
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func newControllerInstance(controllerType string, controllerKey int64, workloadNum int32, isLocked bool) *v1.ControllerInstance {
	controllerInstance := &v1.ControllerInstance{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "100",
		},
		ControllerType: controllerType,
		ControllerKey:  controllerKey,
		WorkloadNum:    workloadNum,
		IsLocked:       isLocked,
	}

	controllerInstance.Name = generateControllerName()

	return controllerInstance
}

func testAddEvent(t *testing.T, cim *ControllerInstanceManager, notifyTimes int) (*v1.ControllerInstance, string, map[string]v1.ControllerInstance) {
	// add event
	controllerType := "foo"
	controllerInstance1 := newControllerInstance(controllerType, 10000, 999, false)
	cim.addControllerInstance(controllerInstance1)

	controllerInstanceMap, err := cim.ListControllerInstances(controllerType)
	assert.NotNil(t, controllerInstanceMap)
	assert.Nil(t, err)
	controllerInstanceRead, isOK := controllerInstanceMap[controllerInstance1.Name]
	assert.True(t, isOK)
	assert.NotNil(t, controllerInstanceRead)
	assert.Equal(t, controllerInstance1.Name, controllerInstanceRead.Name)
	assert.Equal(t, controllerInstance1.ControllerType, controllerInstanceRead.ControllerType)
	assert.Equal(t, controllerInstance1.ControllerKey, controllerInstanceRead.ControllerKey)
	assert.Equal(t, controllerInstance1.WorkloadNum, controllerInstanceRead.WorkloadNum)
	assert.Equal(t, controllerInstance1.IsLocked, controllerInstanceRead.IsLocked)
	assert.Equal(t, notifyTimes, notifyTimes, "Unexpected notify times")

	return &controllerInstanceRead, controllerType, controllerInstanceMap
}

func TestSyncControllerInstances_Nil(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	cim, _ := CreateTestControllerInstanceManager(stopCh)
	assert.True(t, cim.isControllerListInitialized, "Expect controller list is initialized")

	// invalid controller type
	controllerType := "foo"
	controllerInstanceMap, err := cim.ListControllerInstances(controllerType)
	assert.Nil(t, controllerInstanceMap, "Expecting nil controller map for controller type not in map")
	assert.Nil(t, err, "Expecting no error for not existed controller")
}

func TestControllerInstancesAddAndUpdateEventHandler(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	cim, _ := CreateTestControllerInstanceManager(stopCh)
	notifyTimes = 0

	// add event
	controllerInstance1, controllerType, controllerInstanceMap := testAddEvent(t, cim, 1)

	// update event
	controllerInstance2 := controllerInstance1.DeepCopy()
	controllerInstance2.IsLocked = !controllerInstance1.IsLocked
	controllerInstance2.WorkloadNum = controllerInstance1.WorkloadNum + 101
	controllerInstance2.ControllerKey = controllerInstance1.ControllerKey - 100
	controllerInstance2.ResourceVersion = "101"
	cim.updateControllerInstance(controllerInstance1, controllerInstance2)

	controllerInstanceMapNew, err := cim.ListControllerInstances(controllerType)
	assert.NotNil(t, controllerInstanceMapNew)
	assert.Nil(t, err)
	assert.Equal(t, len(controllerInstanceMap), len(controllerInstanceMapNew), "Unexpected length of controller instance map")
	controllerInstanceRead2, isOK := controllerInstanceMapNew[controllerInstance1.Name]
	assert.True(t, isOK)
	assert.NotNil(t, controllerInstanceRead2)
	assert.Equal(t, controllerInstance1.Name, controllerInstanceRead2.Name)
	assert.Equal(t, controllerInstance1.ControllerType, controllerInstanceRead2.ControllerType)
	assert.NotEqual(t, controllerInstance1.IsLocked, controllerInstanceRead2.IsLocked)
	assert.Equal(t, controllerInstance1.WorkloadNum+101, controllerInstanceRead2.WorkloadNum)
	assert.Equal(t, controllerInstance1.ControllerKey-100, controllerInstanceRead2.ControllerKey)
	assert.Equal(t, 2, notifyTimes, "Unexpected notify times")
}

func TestControllerInstanceDeleteEventHandler(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	cim, _ := CreateTestControllerInstanceManager(stopCh)
	notifyTimes = 0

	// add event
	controllerInstance1, controllerType, _ := testAddEvent(t, cim, 1)

	// delete event
	cim.deleteControllerInstance(controllerInstance1)
	controllerInstanceMapAfterDelete, err := cim.ListControllerInstances(controllerType)
	assert.NotNil(t, controllerInstanceMapAfterDelete)
	assert.Nil(t, err)
	_, isOK := controllerInstanceMapAfterDelete[controllerInstance1.Name]
	assert.False(t, isOK)
	assert.Equal(t, 2, notifyTimes, "Unexpected notify times")
}

func TestDeletedControllerInstanceSentToAddEventHandler(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	cim, _ := CreateTestControllerInstanceManager(stopCh)
	notifyTimes = 0

	// add event
	controllerInstance1, controllerType, _ := testAddEvent(t, cim, 1)

	controllerInstance2 := controllerInstance1.DeepCopy()
	now := metav1.Now()
	controllerInstance2.DeletionTimestamp = &now
	cim.addControllerInstance(controllerInstance2)
	controllerInstanceMapNew, err := cim.ListControllerInstances(controllerType)

	assert.NotNil(t, controllerInstanceMapNew)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(controllerInstanceMapNew))
	assert.Equal(t, 2, notifyTimes, "Unexpected notify times")
}

func TestDeleteControllerInstanceDoesNotExist(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	cim, _ := CreateTestControllerInstanceManager(stopCh)
	notifyTimes = 0

	controllerInstance1 := newControllerInstance("bar", 10000, 999, false)
	cim.deleteControllerInstance(controllerInstance1)

	controllerInstanceMap, err := cim.ListControllerInstances(controllerInstance1.ControllerType)
	assert.Nil(t, controllerInstanceMap)
	assert.Nil(t, err)
	assert.Equal(t, 0, notifyTimes, "Unexpected notify times")
}

func TestAddMultipleControllerInstancesForSameControllerType(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	cim, _ := CreateTestControllerInstanceManager(stopCh)
	notifyTimes = 0

	// add event
	controllerInstance1, controllerType1, _ := testAddEvent(t, cim, 1)
	controllerInstance2, controllerType2, _ := testAddEvent(t, cim, 2)
	assert.Equal(t, controllerType1, controllerType2)
	assert.NotEqual(t, controllerInstance1.Name, controllerInstance2.Name)

	controllerInstanceMap, err := cim.ListControllerInstances(controllerType1)
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstanceMap)
	controllerInstanceRead1, isOK1 := controllerInstanceMap[controllerInstance1.Name]
	assert.True(t, isOK1)
	assert.NotNil(t, controllerInstanceRead1)
	assert.Equal(t, controllerInstance1.Name, controllerInstanceRead1.Name)

	controllerInstanceRead2, isOK2 := controllerInstanceMap[controllerInstance2.Name]
	assert.True(t, isOK2)
	assert.NotNil(t, controllerInstanceRead2)
	assert.Equal(t, controllerInstance2.Name, controllerInstanceRead2.Name)
}

func TestUpdateHandlerWithOldEvents(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	cim, _ := CreateTestControllerInstanceManager(stopCh)
	notifyTimes = 0

	// add event
	controllerInstance1, controllerType, _ := testAddEvent(t, cim, 1)

	// update event
	controllerInstance2 := controllerInstance1.DeepCopy()
	controllerInstance2.IsLocked = !controllerInstance1.IsLocked
	controllerInstance2.WorkloadNum = controllerInstance1.WorkloadNum + 101
	controllerInstance2.ControllerKey = controllerInstance1.ControllerKey - 100
	controllerInstance2.ResourceVersion = "99"
	cim.updateControllerInstance(controllerInstance1, controllerInstance2)

	// check controller instance in the map
	controllerInstanceMap2, err := cim.ListControllerInstances(controllerType)
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstanceMap2)
	controllerInstanceRead2, isOK := controllerInstanceMap2[controllerInstance1.Name]
	assert.True(t, isOK)
	assert.NotNil(t, controllerInstanceRead2)

	assert.Equal(t, controllerInstance1.Name, controllerInstanceRead2.Name)
	assert.Equal(t, controllerInstance1.ResourceVersion, controllerInstanceRead2.ResourceVersion)
	assert.Equal(t, controllerInstance1.IsLocked, controllerInstanceRead2.IsLocked)
	assert.Equal(t, controllerInstance1.WorkloadNum, controllerInstanceRead2.WorkloadNum)
	assert.Equal(t, controllerInstance1.ControllerKey, controllerInstanceRead2.ControllerKey)
}

func TestErrorHandlingInListControllerInstances(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	cim, _ := CreateTestControllerInstanceManager(stopCh)
	notifyTimes = 0

	_, controllerType, _ := testAddEvent(t, cim, 1)
	testAddEvent(t, cim, 2)

	controllerInstance3 := newControllerInstance("foo2", 10000, 999, false)
	cim.addControllerInstance(controllerInstance3)

	cim.isControllerListInitialized = false

	controllerInstanceMap1, err := cim.ListControllerInstances(controllerType)
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstanceMap1)
	assert.Equal(t, 2, len(controllerInstanceMap1))

	controllerInstanceMap2, err := cim.ListControllerInstances("foo2")
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstanceMap2)
	assert.Equal(t, 1, len(controllerInstanceMap2))
}
