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
	"k8s.io/client-go/informers"
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog"
)

func mockResetHander(c *ControllerBase, newLowerBound, newUpperbound int64) {
	klog.Infof("Mocked sent reset message to channel")
	return
}

func createControllerInstanceBaseAndCIM(t *testing.T, client clientset.Interface, cim *ControllerInstanceManager,
	controllerType string, stopCh chan struct{}) (*ControllerBase, *ControllerInstanceManager) {

	if cim == nil {
		cim, _ = CreateTestControllerInstanceManager(stopCh)
	}

	ResetFilterHandler = mockResetHander
	newControllerInstance1, err := NewControllerBase(controllerType, client, nil, nil)
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
	}

	return controllerInstance
}

func TestGetControllerInstanceManager(t *testing.T) {
	instance = nil
	cim := GetInstanceHandler()
	assert.Nil(t, cim)

	client := fake.NewSimpleClientset()
	informers := informers.NewSharedInformerFactory(client, 0)

	cim = NewControllerInstanceManager(informers.Core().V1().ControllerInstances(), client, nil)
	assert.NotNil(t, cim)

	checkInstanceHandler = mockCheckInstanceHander
}

func TestCreateControllerInstanceBase(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	defer close(stopCh)

	controllerInstanceBase, cim := createControllerInstanceBaseAndCIM(t, client, nil, "foo", stopCh)

	// 1st controller instance for a type needs to cover all workload
	assert.Equal(t, 0, controllerInstanceBase.curPos)
	assert.Equal(t, 1, len(controllerInstanceBase.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)

	// 1st controller instance for a different type needs to cover all workload
	controllerInstanceBase2, _ := createControllerInstanceBaseAndCIM(t, client, cim, "bar", stopCh)
	assert.NotNil(t, controllerInstanceBase2)
	assert.Equal(t, 0, controllerInstanceBase2.curPos)
	assert.Equal(t, 1, len(controllerInstanceBase2.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase2.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase2.sortedControllerInstancesLocal[0].controllerKey)
}

func TestDeleteControllerInstance(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	defer close(stopCh)

	controllerType := "foo"
	controllerInstanceBase, cim1 := createControllerInstanceBaseAndCIM(t, client, nil, controllerType, stopCh)
	controllerInstance1 := convertControllerBaseToControllerInstance(controllerInstanceBase)

	// 1st controller instance for a type needs to cover all workload
	assert.Equal(t, 0, controllerInstanceBase.curPos)
	assert.Equal(t, 1, len(controllerInstanceBase.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)

	// 2nd controller instance will split workload space with 1st one
	stopCh2 := make(chan struct{})
	controllerInstanceBase2, cim2 := createControllerInstanceBaseAndCIM(t, client, nil, controllerType, stopCh2)
	controllerInstance2 := convertControllerBaseToControllerInstance(controllerInstanceBase2)

	// notify controller creation events
	cim1.addControllerInstance(controllerInstance2)
	cim2.addControllerInstance(controllerInstance1)

	controllerInstances, err := listControllerInstancesByType(controllerType)
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstances)
	controllerInstanceBase.updateCachedControllerInstances(controllerInstances)

	expectedPos := getPosFromControllerInstances(controllerInstance1, controllerInstance1, controllerInstance2)
	assert.Equal(t, expectedPos, controllerInstanceBase.curPos)

	hashKey1 := int64(4611686018427387904) // mid point
	assert.Equal(t, 2, len(controllerInstanceBase.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[1].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey)

	// controller that takes the second half workload died, the left controller needs to take all workload
	instanceNameToDel := controllerInstanceBase.sortedControllerInstancesLocal[1].instanceName
	instanceBaseToCheck := controllerInstanceBase
	if instanceNameToDel == controllerInstance1.Name {
		cim1.deleteControllerInstance(controllerInstance1)
		cim2.deleteControllerInstance(controllerInstance1)
		instanceBaseToCheck = controllerInstanceBase2
	} else {
		cim1.deleteControllerInstance(controllerInstance2)
		cim2.deleteControllerInstance(controllerInstance2)
	}

	controllerInstances, err = listControllerInstancesByType(controllerType)
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstances)
	instanceBaseToCheck.updateCachedControllerInstances(controllerInstances)
	assert.Equal(t, 0, instanceBaseToCheck.curPos)
	assert.Equal(t, 1, len(instanceBaseToCheck.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), instanceBaseToCheck.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), instanceBaseToCheck.sortedControllerInstancesLocal[0].controllerKey)
}

func TestCreateControllerInstanceBaseInRaceCondition_2(t *testing.T) {
	controllerType := "foo"

	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	defer close(stopCh)

	// 1. Create two controller instances at the same time
	controllerInstanceBase1, cim1 := createControllerInstanceBaseAndCIM(t, client, nil, controllerType, stopCh)
	assert.Equal(t, 0, controllerInstanceBase1.curPos)
	assert.Equal(t, 1, len(controllerInstanceBase1.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase1.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase1.sortedControllerInstancesLocal[0].controllerKey)
	t.Logf("CIM 1 %v", cim1.instanceId)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBase1.sortedControllerInstancesLocal)

	controllerInstanceBase2, cim2 := createControllerInstanceBaseAndCIM(t, client, nil, controllerType, stopCh)
	assert.Equal(t, 0, controllerInstanceBase2.curPos)
	assert.Equal(t, 1, len(controllerInstanceBase2.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase2.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase2.sortedControllerInstancesLocal[0].controllerKey)
	t.Logf("CIM 2 %v", cim2.instanceId)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBase2.sortedControllerInstancesLocal)

	assert.True(t, controllerInstanceBase1.controllerKey == controllerInstanceBase2.controllerKey)

	// 2. Notify other CIM the existenc of other controller instance
	controllerInstance1 := convertControllerBaseToControllerInstance(controllerInstanceBase1)
	controllerInstance2 := convertControllerBaseToControllerInstance(controllerInstanceBase2)
	cim1.addControllerInstance(controllerInstance2)
	cim2.addControllerInstance(controllerInstance1)

	// Verify adjustment leads to same result
	controllerInstanceBase2.instanceUpdateProcess(controllerType)

	assert.Equal(t, 2, len(controllerInstanceBase2.sortedControllerInstancesLocal))
	assert.Equal(t, 2, len(controllerInstanceBase2.controllerInstanceMap))
	assert.NotEqual(t, controllerInstanceBase2.sortedControllerInstancesLocal[0].controllerKey,
		controllerInstanceBase2.sortedControllerInstancesLocal[1].controllerKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase2.sortedControllerInstancesLocal[1].controllerKey)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBase2.sortedControllerInstancesLocal)

	controllerInstanceBase1.instanceUpdateProcess(controllerType)
	assert.Equal(t, 2, len(controllerInstanceBase1.sortedControllerInstancesLocal))
	assert.Equal(t, 2, len(controllerInstanceBase1.controllerInstanceMap))
	assert.NotEqual(t, controllerInstanceBase1.sortedControllerInstancesLocal[0].controllerKey,
		controllerInstanceBase1.sortedControllerInstancesLocal[1].controllerKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase1.sortedControllerInstancesLocal[1].controllerKey)
	assertControllerKeyCoversEntireRange(t, controllerInstanceBase1.sortedControllerInstancesLocal)

	assert.NotEqual(t, controllerInstanceBase1.controllerKey, controllerInstanceBase2.controllerKey)
	assert.True(t, controllerInstanceBase1.controllerKey == int64(math.MaxInt64) || controllerInstanceBase2.controllerKey == int64(math.MaxInt64))
	assert.Equal(t, controllerInstanceBase1.sortedControllerInstancesLocal[0].instanceName,
		controllerInstanceBase2.sortedControllerInstancesLocal[0].instanceName)
	assert.Equal(t, controllerInstanceBase1.sortedControllerInstancesLocal[1].instanceName,
		controllerInstanceBase2.sortedControllerInstancesLocal[1].instanceName)
	assert.Equal(t, controllerInstanceBase1.sortedControllerInstancesLocal[0].controllerKey,
		controllerInstanceBase2.sortedControllerInstancesLocal[0].controllerKey)
	assert.Equal(t, controllerInstanceBase1.sortedControllerInstancesLocal[1].controllerKey,
		controllerInstanceBase2.sortedControllerInstancesLocal[1].controllerKey)
}

func TestConsolidateControllerInstances_Sort(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	defer close(stopCh)

	// Test case : 2nd controller instance will split workload space with 1st one
	// 1. create 1st controller
	controllerType := "foo"
	controllerInstanceBase, cim := createControllerInstanceBaseAndCIM(t, client, nil, controllerType, stopCh)
	controllerInstance1 := convertControllerBaseToControllerInstance(controllerInstanceBase)

	// 2. create 2nd controller
	cim2, _ := CreateTestControllerInstanceManager(stopCh)
	hashKey1 := int64(4611686018427387904) // mid point
	controllerInstance1_2 := newControllerInstance(cim2, controllerType, int64(10000), int32(100))
	cim.addControllerInstance(controllerInstance1_2)
	cim2.addControllerInstance(controllerInstance1_2)
	cim2.addControllerInstance(controllerInstance1)

	controllerInstances, err := listControllerInstancesByType(controllerType)
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstances)
	controllerInstanceBase.updateCachedControllerInstances(controllerInstances)

	expectedPos := getPosFromControllerInstances(controllerInstance1, controllerInstance1, controllerInstance1_2)
	assert.Equal(t, expectedPos, controllerInstanceBase.curPos)

	assert.Equal(t, 2, len(controllerInstanceBase.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[1].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey)

	// 3nd controller instance will share same workload space with the first 2 - each take 1/3
	hashKey1 = int64(3074457345618258603)
	hashKey2 := int64(6148914691236517205)
	cim3, _ := CreateTestControllerInstanceManager(stopCh)
	controllerInstance1_3 := newControllerInstance(cim3, "foo", int64(2000), 100)
	cim.addControllerInstance(controllerInstance1_3)
	cim2.addControllerInstance(controllerInstance1_3)
	cim3.addControllerInstance(controllerInstance1_3)
	cim3.addControllerInstance(controllerInstance1)
	cim3.addControllerInstance(controllerInstance1_2)
	controllerInstances, err = listControllerInstancesByType(controllerType)
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstances)
	controllerInstanceBase.updateCachedControllerInstances(controllerInstances)

	expectedPos = getPosFromControllerInstances(controllerInstance1, controllerInstance1, controllerInstance1_2, controllerInstance1_3)
	assert.Equal(t, expectedPos, controllerInstanceBase.curPos)
	assert.Equal(t, 3, len(controllerInstanceBase.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[1].lowerboundKey)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[2].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[2].controllerKey)

	// same controller instances
	controllerInstanceBase.updateCachedControllerInstances(controllerInstances)
	assert.Equal(t, expectedPos, controllerInstanceBase.curPos)
	assert.Equal(t, 3, len(controllerInstanceBase.sortedControllerInstancesLocal))
	assert.Equal(t, int64(0), controllerInstanceBase.sortedControllerInstancesLocal[0].lowerboundKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[0].controllerKey)
	assert.Equal(t, hashKey1, controllerInstanceBase.sortedControllerInstancesLocal[1].lowerboundKey)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[1].controllerKey)
	assert.Equal(t, hashKey2, controllerInstanceBase.sortedControllerInstancesLocal[2].lowerboundKey)
	assert.Equal(t, int64(math.MaxInt64), controllerInstanceBase.sortedControllerInstancesLocal[2].controllerKey)
}

func TestIsInRange(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	defer close(stopCh)

	controllerType := "foo"
	controllerInstanceBase, cim := createControllerInstanceBaseAndCIM(t, client, nil, controllerType, stopCh)
	controllerInstance1 := convertControllerBaseToControllerInstance(controllerInstanceBase)

	// check range
	assert.True(t, controllerInstanceBase.IsInRange(int64(0)))
	assert.True(t, controllerInstanceBase.IsInRange(int64(math.MaxInt64)))
	assert.False(t, controllerInstanceBase.IsInRange(int64(-1)))

	// 2 controller instances with same workload num, max interval = the first one
	workloadNum1 := int32(10000)
	//workloadNum2 := workloadNum1
	controllerInstanceBase.sortedControllerInstancesLocal[0].workloadNum = workloadNum1

	hashKey1 := int64(100000)
	cim2, _ := CreateTestControllerInstanceManager(stopCh)
	controllerInstance2 := newControllerInstance(cim2, controllerType, hashKey1, workloadNum1)
	cim.addControllerInstance(controllerInstance2)
	cim2.addControllerInstance(controllerInstance2)
	cim2.addControllerInstance(controllerInstance1)
	controllerInstanceBase.instanceUpdateProcess(controllerType)

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
	assertControllerKeyCoversEntireRange(t, controllerInstanceBase.sortedControllerInstancesLocal)
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
	defer close(stopCh)

	controllerType := "foo"
	controllerInstanceBase, _ := createControllerInstanceBaseAndCIM(t, client, nil, controllerType, stopCh)

	assert.Equal(t, int32(0), controllerInstanceBase.sortedControllerInstancesLocal[0].workloadNum)

	newWorkloadNum := 100
	controllerInstanceBase.SetWorkloadNum(newWorkloadNum)
	assert.Equal(t, int32(newWorkloadNum), controllerInstanceBase.sortedControllerInstancesLocal[0].workloadNum)

	controllerInstanceBase.ReportHealth(client)
}

func getPosFromControllerInstances(targetInstance *v1.ControllerInstance, searchInstances ...*v1.ControllerInstance) int {
	sort.Slice(searchInstances, func(i, j int) bool {
		return searchInstances[i].Name < searchInstances[j].Name
	})

	for i, instance := range searchInstances {
		if targetInstance.Name == instance.Name {
			return i
		}
	}

	return -1
}
