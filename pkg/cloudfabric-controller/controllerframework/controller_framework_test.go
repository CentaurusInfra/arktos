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
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"math"
	"testing"
)

func createControllerInstanceBase(t *testing.T, client clientset.Interface, cim *ControllerInstanceManager, controllerType string, stopCh chan struct{},
	updateCh chan string) (*ControllerBase, *ControllerInstanceManager, error) {

	if cim == nil {
		cim, _ = createControllerInstanceManager(stopCh, updateCh)
	}
	go cim.Run(stopCh)

	newControllerInstance1, err := NewControllerBase(controllerType, client, updateCh)
	assert.Nil(t, err)
	assert.NotNil(t, newControllerInstance1)
	assert.NotNil(t, newControllerInstance1.GetControllerName())
	assert.Equal(t, controllerType, newControllerInstance1.GetControllerType())
	assert.True(t, newControllerInstance1.IsControllerActive())

	return newControllerInstance1, cim, err
}

func TestGenerateKey(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	defer close(stopCh)
	defer close(updateCh)

	controllerInstanceBase, cim, err := createControllerInstanceBase(t, client, nil, "foo", stopCh, updateCh)

	// 1st controller instance for a type needs to cover all workload
	assert.Equal(t, 0, controllerInstanceBase.curPos)
	assert.Equal(t, 1, len(controllerInstanceBase.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
	assert.False(t, controllerInstanceBase.sortedControllerInstancesLocal[0].isLocked)

	// 1st controller instance for a different type needs to cover all workload
	controllerInstanceBase2, _, err := createControllerInstanceBase(t, client, cim, "bar", stopCh, updateCh)
	assert.Nil(t, err)
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
	defer close(stopCh)
	defer close(updateCh)

	// 2nd controller instance will share same workload space with 1st one
	controllerType := "foo"
	controllerInstanceBase, cim, err := createControllerInstanceBase(t, client, nil, controllerType, stopCh, updateCh)

	hashKey1 := int64(10000)
	controllerInstance1_2 := newControllerInstance(controllerType, hashKey1, int32(100), true)
	cim.addControllerInstance(&controllerInstanceBase.controllerInstanceList[0])
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
	assert.Equal(t, 2, len(controllerInstanceBase.controllerInstanceList))

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
	assert.Equal(t, 3, len(controllerInstanceBase.controllerInstanceList))

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
	assert.Equal(t, 3, len(controllerInstanceBase.controllerInstanceList))
}

func TestConsolidateControllerInstances_ReturnValues_MergeAndAutoExtends(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	defer close(stopCh)
	defer close(updateCh)

	controllerType := "foo"
	controllerInstanceBase, _, _ := createControllerInstanceBase(t, client, nil, controllerType, stopCh, updateCh)

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
	controllerInstanceBase.controllerInstanceList = append(controllerInstanceBase.controllerInstanceList, *controllerInstanceB)
	sortedControllerInstanceLocal := SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceBase.controllerInstanceList)
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
	controllerInstanceBase.controllerInstanceList = append(controllerInstanceBase.controllerInstanceList, *controllerInstanceC)
	sortedControllerInstanceLocal = SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceBase.controllerInstanceList)
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
	controllerInstanceListNew := []v1.ControllerInstance{controllerInstanceBase.controllerInstanceList[0], controllerInstanceBase.controllerInstanceList[2]}
	sortedControllerInstanceLocal = SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceListNew)
	isUpdated, isSelfUpdated, newLowerbound, newUpperBound, newPos = controllerInstanceBase.tryConsolidateControllerInstancesLocal(sortedControllerInstanceLocal)
	assert.True(t, isUpdated)
	assert.True(t, isSelfUpdated)
	assert.Equal(t, hashKey2, newLowerbound, "lower bound key")
	assert.Equal(t, int64(math.MaxInt64), newUpperBound, "upper bound key")
	assert.Equal(t, 1, newPos)
	controllerInstanceBase.curPos = newPos
	controllerInstanceBase.sortedControllerInstancesLocal = sortedControllerInstanceLocal
	controllerInstanceBase.controllerInstanceList = controllerInstanceListNew

	assert.Equal(t, controllerInstanceNameC, controllerInstanceBase.sortedControllerInstancesLocal[0].instanceName)
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)

	assert.Equal(t, controllerInstanceNameA, controllerInstanceBase.sortedControllerInstancesLocal[1].instanceName)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[1].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey)

	// one more controller instances died, left one, hash range should be [0, maxInt64]
	// controller instance A: [0, maxInt64] - above tested automatically merge to instance behind
	// return isUpdate = true, isSelfUpdate=true, newLowerbound=0, newUpperbound=maxInt64, newPos=0
	controllerInstanceListNew = []v1.ControllerInstance{controllerInstanceBase.controllerInstanceList[0]}
	sortedControllerInstanceLocal = SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceListNew)
	isUpdated, isSelfUpdated, newLowerbound, newUpperBound, newPos = controllerInstanceBase.tryConsolidateControllerInstancesLocal(sortedControllerInstanceLocal)
	assert.True(t, isUpdated)
	assert.True(t, isSelfUpdated)
	assert.Equal(t, int64(0), newLowerbound)
	assert.Equal(t, int64(math.MaxInt64), newUpperBound)
	assert.Equal(t, 0, newPos)
	controllerInstanceBase.curPos = newPos
	controllerInstanceBase.sortedControllerInstancesLocal = sortedControllerInstanceLocal
	controllerInstanceBase.controllerInstanceList = controllerInstanceListNew

	assert.Equal(t, controllerInstanceNameA, controllerInstanceBase.sortedControllerInstancesLocal[0].instanceName)
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
}

func TestGetMaxInterval(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	defer close(stopCh)
	defer close(updateCh)

	controllerType := "foo"
	controllerInstanceBase, _, _ := createControllerInstanceBase(t, client, nil, controllerType, stopCh, updateCh)

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
	controllerInstanceBase.controllerInstanceList = append(controllerInstanceBase.controllerInstanceList, *controllerInstance2)
	controllerInstanceBase.sortedControllerInstancesLocal = SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceBase.controllerInstanceList)
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
