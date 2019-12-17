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
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog"
)

func mockResetHander(c *ControllerBase, newLowerBound, newUpperbound int64) {
	klog.Infof("Mocked sent reset message to channel")
	return
}

func createControllerInstanceBaseAndCIM(t *testing.T, client clientset.Interface, cim *ControllerInstanceManager, controllerType string, stopCh chan struct{},
	updateCh chan string, resetCh chan interface{}) (*ControllerBase, *ControllerInstanceManager) {

	if cim == nil {
		cim, _ = CreateTestControllerInstanceManager(stopCh, updateCh)
		go cim.Run(stopCh)
	}

	ResetFilterHandler = mockResetHander
	newControllerInstance1, err := NewControllerBase(controllerType, client, updateCh, resetCh)
	newControllerInstance1.unlockControllerInstanceHandler = mockUnlockcontrollerInstanceHandler
	cim.addControllerInstance(convertControllerBaseToControllerInstance(newControllerInstance1))

	assert.Nil(t, err)
	assert.NotNil(t, newControllerInstance1)
	assert.NotNil(t, newControllerInstance1.GetControllerName())
	assert.Equal(t, controllerType, newControllerInstance1.GetControllerType())

	return newControllerInstance1, cim
}

func convertControllerBaseToControllerInstance(controllerBase *ControllerBase) *v1.ControllerInstance {
	controllerInstance := &v1.ControllerInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllerBase.GetControllerName(),
		},
		ControllerType: controllerBase.controllerType,
		ControllerKey:  controllerBase.controllerKey,
		WorkloadNum:    0,
		IsLocked:       controllerBase.state == ControllerStateLocked,
	}

	return controllerInstance
}

var unlockedControllerInstanceName string

func mockUnlockcontrollerInstanceHandler(local controllerInstanceLocal) error {
	unlockedControllerInstanceName = local.instanceName
	return nil
}

func TestGetControllerInstanceManager(t *testing.T) {
	instance = nil
	cim := GetControllerInstanceManager()
	assert.Nil(t, cim)

	client := fake.NewSimpleClientset()
	informers := informers.NewSharedInformerFactory(client, 0)
	updateCh := make(chan string)
	cim = NewControllerInstanceManager(informers.Core().V1().ControllerInstances(), client, updateCh)
	assert.NotNil(t, cim)

	checkInstanceHandler = mockCheckInstanceHander
}

func TestGenerateKey(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	resetCh := make(chan interface{})
	defer close(stopCh)
	defer close(updateCh)
	defer close(resetCh)

	controllerInstanceBase, cim := createControllerInstanceBaseAndCIM(t, client, nil, "foo", stopCh, updateCh, resetCh)

	// 1st controller instance for a type needs to cover all workload
	assert.Equal(t, 0, controllerInstanceBase.curPos)
	assert.Equal(t, 1, len(controllerInstanceBase.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
	assert.False(t, controllerInstanceBase.sortedControllerInstancesLocal[0].isLocked)

	// 1st controller instance for a different type needs to cover all workload
	controllerInstanceBase2, _ := createControllerInstanceBaseAndCIM(t, client, cim, "bar", stopCh, updateCh, resetCh)
	assert.NotNil(t, controllerInstanceBase2)
	assert.Equal(t, 0, controllerInstanceBase2.curPos)
	assert.Equal(t, 1, len(controllerInstanceBase2.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase2.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase2.sortedControllerInstancesLocal[0].controllerKey)
	assert.False(t, controllerInstanceBase2.sortedControllerInstancesLocal[0].isLocked)
}

func TestConsolidateControllerInstances_Sort(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	resetCh := make(chan interface{})
	defer close(stopCh)
	defer close(updateCh)
	defer close(resetCh)

	// 2nd controller instance will share same workload space with 1st one
	controllerType := "foo"
	controllerInstanceBase, cim := createControllerInstanceBaseAndCIM(t, client, nil, controllerType, stopCh, updateCh, resetCh)
	assert.True(t, controllerInstanceBase.IsControllerActive())

	hashKey1 := int64(10000)
	controllerInstance1_2 := newControllerInstance(controllerType, hashKey1, int32(100), true)
	cim.addControllerInstance(controllerInstance1_2)

	controllerInstances, err := listControllerInstancesByType(controllerType)
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstances)
	controllerInstanceBase.updateCachedControllerInstances(controllerInstances)
	assert.Equal(t, 1, controllerInstanceBase.curPos)
	assert.Equal(t, 2, len(controllerInstanceBase.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[1].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey)
	assert.Equal(t, 2, len(controllerInstanceBase.controllerInstanceMap))

	// 3nd controller instance will share same workload space with the first 2
	hashKey2 := hashKey1 + 20000
	controllerInstance1_3 := newControllerInstance("foo", hashKey2, 100, true)
	cim.addControllerInstance(controllerInstance1_3)
	controllerInstances, err = listControllerInstancesByType(controllerType)
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstances)
	controllerInstanceBase.updateCachedControllerInstances(controllerInstances)
	assert.Equal(t, 2, controllerInstanceBase.curPos)
	assert.Equal(t, 3, len(controllerInstanceBase.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[1].lowerboundKey)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[2].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[2].controllerKey)
	assert.Equal(t, 3, len(controllerInstanceBase.controllerInstanceMap))

	// same controller instances
	controllerInstanceBase.updateCachedControllerInstances(controllerInstances)
	assert.Equal(t, 2, controllerInstanceBase.curPos)
	assert.Equal(t, 3, len(controllerInstanceBase.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[1].lowerboundKey)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[2].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[2].controllerKey)
	assert.Equal(t, 3, len(controllerInstanceBase.controllerInstanceMap))
}

func TestConsolidateControllerInstances_ReturnValues_MergeAndAutoExtends(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	resetCh := make(chan interface{})
	defer close(stopCh)
	defer close(updateCh)
	defer close(resetCh)

	controllerType := "foo"
	controllerInstanceBase, _ := createControllerInstanceBaseAndCIM(t, client, nil, controllerType, stopCh, updateCh, resetCh)
	assert.True(t, controllerInstanceBase.IsControllerActive())

	// current controller instance A has range [0, maxInt64]
	assert.Equal(t, 0, controllerInstanceBase.curPos)
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
	assert.Equal(t, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey, controllerInstanceBase.controllerKey)
	assert.Equal(t, 1, len(controllerInstanceBase.sortedControllerInstancesLocal))
	controllerInstanceNameA := controllerInstanceBase.controllerName

	// Add 2nd controller instance B with hashkey 100000,
	// return isUpdate=true, isSelfUpdate=true, newLowerbound=controller key of 2nd controller instance, newUpperbound=maxInt64, newPos=1
	// controller instance B: [0, 10000]
	// controller instance A: (10000, maxInt64]
	hashKey1 := int64(10000)

	controllerInstanceB := newControllerInstance(controllerType, hashKey1, 100, true)
	controllerInstanceNameB := controllerInstanceB.Name
	controllerInstanceBase.controllerInstanceMap[controllerInstanceNameB] = *controllerInstanceB
	sortedControllerInstanceLocal := SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceBase.controllerInstanceMap)
	isUpdated, isSelfUpdated, newLowerbound, newUpperBound, newPos := controllerInstanceBase.tryConsolidateControllerInstancesLocal(sortedControllerInstanceLocal)
	assert.True(t, isUpdated)
	assert.True(t, isSelfUpdated)
	assert.Equal(t, hashKey1, newLowerbound)
	assert.Equal(t, int64(math.MaxInt64), newUpperBound)
	assert.Equal(t, 1, newPos)
	// update current controller instance
	controllerInstanceBase.curPos = newPos
	controllerInstanceBase.sortedControllerInstancesLocal = sortedControllerInstanceLocal

	assert.Equal(t, controllerInstanceNameB, controllerInstanceBase.sortedControllerInstancesLocal[0].instanceName)
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)

	assert.Equal(t, controllerInstanceNameA, controllerInstanceBase.sortedControllerInstancesLocal[1].instanceName)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[1].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey)

	// Add 3nd controlller instance C with hashkey 100,
	// return isUpdate=true, isSelfUpdate=false, newLowerbound=hashKey1, newUpperbound=maxInt64, newPos=2
	// controller instance C: [0, 100]
	// controller instance B: (100, 10000]
	// controller instance A: (10000, maxInt64]
	hashKey2 := int64(100)
	controllerInstanceC := newControllerInstance(controllerType, hashKey2, 100, true)
	controllerInstanceNameC := controllerInstanceC.Name
	controllerInstanceBase.controllerInstanceMap[controllerInstanceNameC] = *controllerInstanceC
	sortedControllerInstanceLocal = SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceBase.controllerInstanceMap)
	isUpdated, isSelfUpdated, newLowerbound, newUpperBound, newPos = controllerInstanceBase.tryConsolidateControllerInstancesLocal(sortedControllerInstanceLocal)
	assert.True(t, isUpdated)
	assert.False(t, isSelfUpdated)
	assert.Equal(t, hashKey1, newLowerbound, "lower bound key")
	assert.Equal(t, int64(math.MaxInt64), newUpperBound, "upper bound key")
	assert.Equal(t, 2, newPos)
	controllerInstanceBase.curPos = newPos
	controllerInstanceBase.sortedControllerInstancesLocal = sortedControllerInstanceLocal

	assert.Equal(t, controllerInstanceNameC, controllerInstanceBase.sortedControllerInstancesLocal[0].instanceName)
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)

	assert.Equal(t, controllerInstanceNameB, controllerInstanceBase.sortedControllerInstancesLocal[1].instanceName)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[1].lowerboundKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey)

	assert.Equal(t, controllerInstanceNameA, controllerInstanceBase.sortedControllerInstancesLocal[2].instanceName)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[2].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[2].controllerKey)

	// one controller instance died, left two, hash range needs to be reorganized
	// controller instance C: [0, 100]
	// controller instance A: (100, maxInt64] - automatically merge to instance behind
	// return isUpdate = true, isSelfUpdate=, newLowerbound=0, newUpperbound=maxInt64, newPos=0
	controllerInstanceMapNew := make(map[string]v1.ControllerInstance)
	controllerInstanceMapNew[controllerInstanceNameA] = controllerInstanceBase.controllerInstanceMap[controllerInstanceNameA]
	controllerInstanceMapNew[controllerInstanceNameC] = controllerInstanceBase.controllerInstanceMap[controllerInstanceNameC]
	sortedControllerInstanceLocal = SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceMapNew)
	isUpdated, isSelfUpdated, newLowerbound, newUpperBound, newPos = controllerInstanceBase.tryConsolidateControllerInstancesLocal(sortedControllerInstanceLocal)
	assert.True(t, isUpdated)
	assert.True(t, isSelfUpdated)
	assert.Equal(t, hashKey2, newLowerbound, "lower bound key")
	assert.Equal(t, int64(math.MaxInt64), newUpperBound, "upper bound key")
	assert.Equal(t, 1, newPos)
	controllerInstanceBase.curPos = newPos
	controllerInstanceBase.sortedControllerInstancesLocal = sortedControllerInstanceLocal
	controllerInstanceBase.controllerInstanceMap = controllerInstanceMapNew

	assert.Equal(t, controllerInstanceNameC, controllerInstanceBase.sortedControllerInstancesLocal[0].instanceName)
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)

	assert.Equal(t, controllerInstanceNameA, controllerInstanceBase.sortedControllerInstancesLocal[1].instanceName)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[1].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey)

	// one more controller instances died, left one, hash range should be [0, maxInt64]
	// controller instance A: [0, maxInt64] - above tested automatically merge to instance behind
	// return isUpdate = true, isSelfUpdate=true, newLowerbound=0, newUpperbound=maxInt64, newPos=0
	delete(controllerInstanceMapNew, controllerInstanceNameC)
	sortedControllerInstanceLocal = SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceMapNew)
	isUpdated, isSelfUpdated, newLowerbound, newUpperBound, newPos = controllerInstanceBase.tryConsolidateControllerInstancesLocal(sortedControllerInstanceLocal)
	assert.True(t, isUpdated)
	assert.True(t, isSelfUpdated)
	assert.Equal(t, int64(0), newLowerbound)
	assert.Equal(t, int64(math.MaxInt64), newUpperBound)
	assert.Equal(t, 0, newPos)
	controllerInstanceBase.curPos = newPos
	controllerInstanceBase.sortedControllerInstancesLocal = sortedControllerInstanceLocal
	controllerInstanceBase.controllerInstanceMap = controllerInstanceMapNew

	assert.Equal(t, controllerInstanceNameA, controllerInstanceBase.sortedControllerInstancesLocal[0].instanceName)
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
}

func TestGetMaxInterval(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	resetCh := make(chan interface{})
	defer close(stopCh)
	defer close(updateCh)
	defer close(resetCh)

	controllerType := "foo"
	controllerInstanceBase, _ := createControllerInstanceBaseAndCIM(t, client, nil, controllerType, stopCh, updateCh, resetCh)
	assert.True(t, controllerInstanceBase.IsControllerActive())

	// Single controller instance, max interval always (0, maxInt64)
	min, max := controllerInstanceBase.getMaxInterval()
	assert.Equal(t, int64(0), min)
	assert.Equal(t, int64(math.MaxInt64), max)

	controllerInstanceBase.sortedControllerInstancesLocal[0].workloadNum = int32(-1)
	min, max = controllerInstanceBase.getMaxInterval()
	assert.Equal(t, int64(0), min)
	assert.Equal(t, int64(math.MaxInt64), max)

	// check range
	assert.True(t, controllerInstanceBase.IsInRange(int64(0)))
	assert.True(t, controllerInstanceBase.IsInRange(int64(math.MaxInt64)))
	assert.False(t, controllerInstanceBase.IsInRange(int64(-1)))

	// 2 controller instances with same workload num, max interval = the first one
	workloadNum1 := int32(10000)
	//workloadNum2 := workloadNum1
	controllerInstanceBase.sortedControllerInstancesLocal[0].workloadNum = workloadNum1

	hashKey1 := int64(100000)
	controllerInstance2 := newControllerInstance(controllerType, hashKey1, workloadNum1, true)
	controllerInstanceBase.controllerInstanceMap[controllerInstance2.Name] = *controllerInstance2
	controllerInstanceBase.sortedControllerInstancesLocal = SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceBase.controllerInstanceMap)
	min, max = controllerInstanceBase.getMaxInterval()
	assert.Equal(t, int64(0), min)
	assert.Equal(t, hashKey1, max)

	// check range
	controllerInstanceBase.curPos = 0
	controllerInstanceBase.controllerKey = controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey
	assert.True(t, controllerInstanceBase.IsInRange(int64(0)))
	assert.True(t, controllerInstanceBase.IsInRange(hashKey1))
	assert.False(t, controllerInstanceBase.IsInRange(int64(math.MaxInt64)))

	controllerInstanceBase.curPos = 1
	controllerInstanceBase.controllerKey = controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey
	assert.False(t, controllerInstanceBase.IsInRange(int64(0)))
	assert.False(t, controllerInstanceBase.IsInRange(hashKey1))
	assert.True(t, controllerInstanceBase.IsInRange(int64(math.MaxInt64)))

	// 2 controller instances with workloadNum1 < workloadNum2 => (min, max) = controller instance 2 range
	controllerInstanceBase.sortedControllerInstancesLocal[1].workloadNum = workloadNum1 + 1
	min, max = controllerInstanceBase.getMaxInterval()
	assert.Equal(t, hashKey1, min)
	assert.Equal(t, int64(math.MaxInt64), max)

	// 2 controller instances with workloadNum1 > workloadNum2 => (min, max) = controller instance 1 range
	controllerInstanceBase.sortedControllerInstancesLocal[1].workloadNum = workloadNum1 - 1
	min, max = controllerInstanceBase.getMaxInterval()
	assert.Equal(t, int64(0), min)
	assert.Equal(t, hashKey1, max)
}

func TestControllerInstanceLifeCycle(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	resetCh := make(chan interface{})
	defer close(stopCh)
	defer close(updateCh)
	defer close(resetCh)

	// 1st controller instance
	controllerType1 := "foo"
	controllerInstanceBaseFoo1, cim := createControllerInstanceBaseAndCIM(t, client, nil, controllerType1, stopCh, updateCh, resetCh)

	// 2nd controller instance
	stopCh2 := make(chan struct{})
	updateCh2 := make(chan string)
	resetCh2 := make(chan interface{})
	defer close(stopCh2)
	defer close(updateCh2)
	defer close(resetCh2)

	controllerInstanceBaseFoo2, _ := createControllerInstanceBaseAndCIM(t, client, cim, controllerType1, stopCh2, updateCh2, resetCh2)
	assert.NotNil(t, controllerInstanceBaseFoo2)
	assert.Equal(t, controllerType1, controllerInstanceBaseFoo2.GetControllerType())
	assert.True(t, controllerInstanceBaseFoo1.controllerKey > controllerInstanceBaseFoo2.controllerKey)
	assert.False(t, controllerInstanceBaseFoo2.IsControllerActive())
	assert.True(t, controllerInstanceBaseFoo2.state == ControllerStateLocked)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal)

	// 1st controller instance got update event
	// lowerbound increased, set state to wait
	updatedControllerInstancelist, err := listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(updatedControllerInstancelist))
	controllerInstanceBaseFoo1.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo1.state == ControllerStateWait || controllerInstanceBaseFoo1.state == ControllerStateActive)
	assert.Equal(t, 1, controllerInstanceBaseFoo1.curPos)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBaseFoo1.controllerKey)
	assert.Equal(t, controllerInstanceBaseFoo2.controllerKey, controllerInstanceBaseFoo1.sortedControllerInstancesLocal[controllerInstanceBaseFoo1.curPos].lowerboundKey)
	assert.Equal(t, int64(0), controllerInstanceBaseFoo1.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.True(t, controllerInstanceBaseFoo1.sortedControllerInstancesLocal[0].isLocked)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo1.sortedControllerInstancesLocal)

	// 2nd controller instance got update event
	controllerInstanceBaseFoo2.updateCachedControllerInstances(updatedControllerInstancelist)

	// 1st controller instance done processing current workload
	unlockedControllerInstanceName = ""
	controllerInstanceBaseFoo1.IsDoneProcessingCurrentWorkloads()
	assert.True(t, controllerInstanceBaseFoo1.IsControllerActive())
	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)
	controllerInstanceBaseFoo1.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo1.state == ControllerStateActive)
	assert.False(t, controllerInstanceBaseFoo1.sortedControllerInstancesLocal[1].isLocked)
	assert.Equal(t, controllerInstanceBaseFoo2.controllerName, unlockedControllerInstanceName)

	//assert.False(t, controllerInstanceBaseFoo1.sortedControllerInstancesLocal[0].isLocked)
	// mock controller instance 2 received unlock message
	controllerInstanceFoo2 := convertControllerBaseToControllerInstance(controllerInstanceBaseFoo2)
	controllerInstanceFoo2Copy := convertControllerBaseToControllerInstance(controllerInstanceBaseFoo2)
	controllerInstanceFoo2.ResourceVersion = "100"
	controllerInstanceFoo2Copy.ResourceVersion = "101"
	controllerInstanceFoo2Copy.IsLocked = false
	cim.updateControllerInstance(controllerInstanceFoo2, controllerInstanceFoo2Copy)
	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)
	controllerInstanceBaseFoo2.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo2.state == ControllerStateActive)
	assert.False(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[0].isLocked)
	assert.False(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[1].isLocked)

	// start 3rd controller instance
	stopCh3 := make(chan struct{})
	updateCh3 := make(chan string)
	resetCh3 := make(chan interface{})
	defer close(stopCh3)
	defer close(updateCh3)
	defer close(resetCh3)

	controllerInstanceBaseFoo3, _ := createControllerInstanceBaseAndCIM(t, client, cim, controllerType1, stopCh3, updateCh3, resetCh3)
	assert.NotNil(t, controllerInstanceBaseFoo3)
	assert.Equal(t, controllerType1, controllerInstanceBaseFoo3.GetControllerType())
	assert.True(t, controllerInstanceBaseFoo3.controllerKey < controllerInstanceBaseFoo1.controllerKey)
	assert.False(t, controllerInstanceBaseFoo3.IsControllerActive())
	assert.True(t, controllerInstanceBaseFoo3.state == ControllerStateLocked)

	// 2nd controller received update event, lowerbound increased, set state to wait
	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(updatedControllerInstancelist))
	controllerInstanceBaseFoo2.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo2.state == ControllerStateWait || controllerInstanceBaseFoo2.state == ControllerStateActive)
	assert.Equal(t, 1, controllerInstanceBaseFoo2.curPos)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal)
	assert.True(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[0].isLocked)
	assert.False(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[1].isLocked)
	assert.False(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[2].isLocked)

	// 2nd controller instance done processing current workload
	unlockedControllerInstanceName = ""
	controllerInstanceBaseFoo2.IsDoneProcessingCurrentWorkloads()
	assert.True(t, controllerInstanceBaseFoo2.IsControllerActive())
	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)
	controllerInstanceBaseFoo2.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo2.state == ControllerStateActive)
	assert.True(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[0].isLocked)
	assert.Equal(t, controllerInstanceBaseFoo3.controllerName, unlockedControllerInstanceName)

	// 3rd controller instance got unlock event
	controllerInstanceFoo3 := convertControllerBaseToControllerInstance(controllerInstanceBaseFoo3)
	controllerInstanceFoo3Copy := convertControllerBaseToControllerInstance(controllerInstanceBaseFoo3)
	controllerInstanceFoo3.ResourceVersion = "200"
	controllerInstanceFoo3Copy.ResourceVersion = "201"
	controllerInstanceFoo3Copy.IsLocked = false
	cim.updateControllerInstance(controllerInstanceFoo3, controllerInstanceFoo3Copy)

	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)
	controllerInstanceBaseFoo3.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo3.state == ControllerStateActive)
	assert.False(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal[0].isLocked)
	assert.False(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal[1].isLocked)
	assert.False(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal[2].isLocked)

	// 2nd controller instance got update event
	controllerInstanceBaseFoo2.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo2.state == ControllerStateActive)
	assert.False(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[0].isLocked)
	assert.False(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[1].isLocked)
	assert.False(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[2].isLocked)
	assert.True(t, int64(math.MaxInt64) > controllerInstanceBaseFoo2.sortedControllerInstancesLocal[1].controllerKey)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal)

	// 1st controller instance died - make sure the hashkey range can be auto-extended when the rightmost controller instance dieded
	controllerInstanceFoo1 := convertControllerBaseToControllerInstance(controllerInstanceBaseFoo1)
	cim.deleteControllerInstance(controllerInstanceFoo1)
	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)

	// 2nd controller instance received update event
	controllerInstanceBaseFoo2.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo2.state == ControllerStateActive)
	assert.Equal(t, 2, len(controllerInstanceBaseFoo2.sortedControllerInstancesLocal))
	assert.False(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[0].isLocked)
	assert.False(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[1].isLocked)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBaseFoo2.sortedControllerInstancesLocal[1].controllerKey)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal)

	// 2nd controller sent update event
	controllerInstanceFoo2 = convertControllerBaseToControllerInstance(controllerInstanceBaseFoo2)
	controllerInstanceFoo2Copy = convertControllerBaseToControllerInstance(controllerInstanceBaseFoo2)
	controllerInstanceFoo2.ResourceVersion = "300"
	controllerInstanceFoo2Copy.ResourceVersion = "301"
	cim.updateControllerInstance(controllerInstanceFoo2, controllerInstanceFoo2Copy)
	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)

	// 3rd controller received update event
	controllerInstanceBaseFoo3.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo3.state == ControllerStateActive)
	assert.Equal(t, 2, len(controllerInstanceBaseFoo3.sortedControllerInstancesLocal))
	assert.False(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal[0].isLocked)
	assert.False(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal[1].isLocked)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal)

	// 3rd controller instance died - make sure lowerbound can also be extended if becomes the frontmost controller instance
	controllerInstanceFoo3 = convertControllerBaseToControllerInstance(controllerInstanceBaseFoo3)
	cim.deleteControllerInstance(controllerInstanceFoo3)
	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)

	// 2nd controller instance received update event
	controllerInstanceBaseFoo2.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo2.state == ControllerStateActive)
	assert.Equal(t, 1, len(controllerInstanceBaseFoo2.sortedControllerInstancesLocal))
	assert.False(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[0].isLocked)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal)
}

// test case: there are one controller instance A, a new controller instance B just joined. B locked self and wait for A to unlock it.
//            A died, B can unlocked itself
func TestControllerInstanceLifeCycle2(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	resetCh := make(chan interface{})
	defer close(stopCh)
	defer close(updateCh)
	defer close(resetCh)

	// create instance A
	controllerType1 := "foo"
	controllerInstanceBaseFoo1, cim := createControllerInstanceBaseAndCIM(t, client, nil, controllerType1, stopCh, updateCh, resetCh)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo1.sortedControllerInstancesLocal)

	// create instance B
	stopCh2 := make(chan struct{})
	updateCh2 := make(chan string)
	resetCh2 := make(chan interface{})
	defer close(stopCh2)
	defer close(updateCh2)
	defer close(resetCh2)

	controllerInstanceBaseFoo2, _ := createControllerInstanceBaseAndCIM(t, client, cim, controllerType1, stopCh2, updateCh2, resetCh2)
	assert.NotNil(t, controllerInstanceBaseFoo2)
	assert.Equal(t, controllerType1, controllerInstanceBaseFoo2.GetControllerType())
	assert.False(t, controllerInstanceBaseFoo2.IsControllerActive())
	assert.True(t, controllerInstanceBaseFoo2.state == ControllerStateLocked)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal)

	// instance A died
	controllerInstanceFoo1 := convertControllerBaseToControllerInstance(controllerInstanceBaseFoo1)
	cim.deleteControllerInstance(controllerInstanceFoo1)
	updatedControllerInstancelist, err := listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)

	// instance B received update event
	controllerInstanceBaseFoo2.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo2.state == ControllerStateActive)
	assert.Equal(t, 1, len(controllerInstanceBaseFoo2.sortedControllerInstancesLocal))
	// assert.False(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal[0].isLocked) - this is unnecessary as the self unlock won't be reported immediately
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal)
}

// test case : there are two controller instances A and B. Key B < Key A.
//             a new controller instance C just joined. C locked self and wait for B to unlock it.
//             B died, C can be unlocked by C.
func TestControllerInstanceLifeCycle3(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	resetCh := make(chan interface{})
	defer close(stopCh)
	defer close(updateCh)
	defer close(resetCh)

	// create instance A
	controllerType1 := "foo"
	controllerInstanceBaseFoo1, cim := createControllerInstanceBaseAndCIM(t, client, nil, controllerType1, stopCh, updateCh, resetCh)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo1.sortedControllerInstancesLocal)

	// create instance B
	stopCh2 := make(chan struct{})
	updateCh2 := make(chan string)
	resetCh2 := make(chan interface{})
	defer close(stopCh2)
	defer close(updateCh2)
	defer close(resetCh2)

	controllerInstanceBaseFoo2, _ := createControllerInstanceBaseAndCIM(t, client, cim, controllerType1, stopCh2, updateCh2, resetCh2)
	assert.NotNil(t, controllerInstanceBaseFoo2)
	assert.Equal(t, controllerType1, controllerInstanceBaseFoo2.GetControllerType())
	assert.False(t, controllerInstanceBaseFoo2.IsControllerActive())
	assert.True(t, controllerInstanceBaseFoo2.state == ControllerStateLocked)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal)

	// instance A received B creation event
	updatedControllerInstancelist, err := listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)
	controllerInstanceBaseFoo1.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo1.state == ControllerStateWait || controllerInstanceBaseFoo1.state == ControllerStateActive)

	// create instance C
	stopCh3 := make(chan struct{})
	updateCh3 := make(chan string)
	resetCh3 := make(chan interface{})
	defer close(stopCh3)
	defer close(updateCh3)
	defer close(resetCh3)

	controllerInstanceBaseFoo3, _ := createControllerInstanceBaseAndCIM(t, client, cim, controllerType1, stopCh3, updateCh3, resetCh3)
	assert.NotNil(t, controllerInstanceBaseFoo3)
	assert.Equal(t, controllerType1, controllerInstanceBaseFoo3.GetControllerType())
	assert.True(t, controllerInstanceBaseFoo3.controllerKey < controllerInstanceBaseFoo1.controllerKey)
	assert.False(t, controllerInstanceBaseFoo3.IsControllerActive())
	assert.True(t, controllerInstanceBaseFoo3.state == ControllerStateLocked)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo2.sortedControllerInstancesLocal)

	// instance A received C creation event
	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)
	controllerInstanceBaseFoo1.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo1.state == ControllerStateWait || controllerInstanceBaseFoo1.state == ControllerStateActive)

	// instance B died
	controllerInstanceFoo2 := convertControllerBaseToControllerInstance(controllerInstanceBaseFoo2)
	cim.deleteControllerInstance(controllerInstanceFoo2)
	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)

	// instance C received update event
	controllerInstanceBaseFoo3.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.Equal(t, 2, len(controllerInstanceBaseFoo3.sortedControllerInstancesLocal))
	assert.True(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal[0].isLocked)
	assert.False(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal[1].isLocked)
	assert.True(t, controllerInstanceBaseFoo3.state == ControllerStateLocked)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal)

	// instance A received delete event
	controllerInstanceBaseFoo1.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.Equal(t, 2, len(controllerInstanceBaseFoo1.sortedControllerInstancesLocal))
	assert.True(t, controllerInstanceBaseFoo1.sortedControllerInstancesLocal[0].isLocked)
	assert.False(t, controllerInstanceBaseFoo1.sortedControllerInstancesLocal[1].isLocked)
	assert.True(t, controllerInstanceBaseFoo1.state == ControllerStateWait || controllerInstanceBaseFoo1.state == ControllerStateActive)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo1.sortedControllerInstancesLocal)

	// instance A done processing current workload
	unlockedControllerInstanceName = ""
	controllerInstanceBaseFoo1.IsDoneProcessingCurrentWorkloads()
	assert.True(t, controllerInstanceBaseFoo1.IsControllerActive())
	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)
	controllerInstanceBaseFoo1.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo1.state == ControllerStateActive)
	assert.True(t, controllerInstanceBaseFoo1.sortedControllerInstancesLocal[0].isLocked)
	assert.Equal(t, controllerInstanceBaseFoo3.controllerName, unlockedControllerInstanceName)

	// instance A unlock instance C
	controllerInstanceFoo3 := convertControllerBaseToControllerInstance(controllerInstanceBaseFoo3)
	controllerInstanceFoo3Copy := convertControllerBaseToControllerInstance(controllerInstanceBaseFoo3)
	controllerInstanceFoo3.ResourceVersion = "100"
	controllerInstanceFoo3Copy.ResourceVersion = "110"
	controllerInstanceFoo3Copy.IsLocked = false
	cim.updateControllerInstance(controllerInstanceFoo3, controllerInstanceFoo3Copy)

	updatedControllerInstancelist, err = listControllerInstancesByType(controllerType1)
	assert.Nil(t, err)
	controllerInstanceBaseFoo3.updateCachedControllerInstances(updatedControllerInstancelist)
	assert.True(t, controllerInstanceBaseFoo3.state == ControllerStateActive)
	assert.False(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal[0].isLocked)
	assert.False(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal[1].isLocked)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBaseFoo3.sortedControllerInstancesLocal)
}

func assertControllerKeyCoversEntireRange(t *testing.T, sortedControllerInstanceLocal []controllerInstanceLocal) {
	numofControllers := len(sortedControllerInstanceLocal)
	assert.Equal(t, int64(0), sortedControllerInstanceLocal[0].lowerboundKey)

	for i := 0; i < numofControllers-1; i++ {
		if i+1 < numofControllers {
			assert.Equal(t, sortedControllerInstanceLocal[i].controllerKey, sortedControllerInstanceLocal[i+1].lowerboundKey)
		}
	}

	assert.Equal(t, int64(math.MaxInt64), sortedControllerInstanceLocal[numofControllers-1].controllerKey)
}

func TestSetWorkloadNum(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	resetCh := make(chan interface{})
	defer close(stopCh)
	defer close(updateCh)
	defer close(resetCh)

	controllerType := "foo"
	controllerInstanceBase, _ := createControllerInstanceBaseAndCIM(t, client, nil, controllerType, stopCh, updateCh, resetCh)
	assert.True(t, controllerInstanceBase.IsControllerActive())

	assert.Equal(t, int32(0), controllerInstanceBase.sortedControllerInstancesLocal[0].workloadNum)

	newWorkloadNum := 100
	controllerInstanceBase.SetWorkloadNum(newWorkloadNum)
	assert.Equal(t, int32(newWorkloadNum), controllerInstanceBase.sortedControllerInstancesLocal[0].workloadNum)

	controllerInstanceBase.ReportHealth()
}
