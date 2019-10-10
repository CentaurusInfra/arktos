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
}

func TestConsolidateControllerInstances_ReturnValues(t *testing.T) {
	client := fake.NewSimpleClientset()
	stopCh := make(chan struct{})
	updateCh := make(chan string)
	defer close(stopCh)
	defer close(updateCh)

	controllerType := "foo"
	controllerInstanceBase, _, _ := createControllerInstanceBase(t, client, nil, controllerType, stopCh, updateCh)

	// Add 2nd controller instance with hashkey 100000,
	// return isUpdate=true, isSelfUpdate=true, newLowerbound=controller key of 2nd controller instance, newUpperbound=maxInt64, newPos=1
	hashKey1 := int64(10000)

	newControllerInstance := newControllerInstance(controllerType, hashKey1, 100, true)
	controllerInstanceBase.controllerInstanceList = append(controllerInstanceBase.controllerInstanceList, *newControllerInstance)
	sortedControllerInstanceLocal := SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceBase.controllerInstanceList)
	isUpdated, isSelfUpdated, newLowerbound, newUpperBound, newPos := controllerInstanceBase.tryConsolidateControllerInstancesLocal(sortedControllerInstanceLocal)
	assert.True(t, isUpdated)
	assert.True(t, isSelfUpdated)
	assert.Equal(t, hashKey1, newLowerbound)
	assert.Equal(t, int64(math.MaxInt64), newUpperBound)
	assert.Equal(t, 1, newPos)

	// Add 3nd controlller instance with hashkey 100,
}
