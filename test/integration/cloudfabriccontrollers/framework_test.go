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

package cloudfabriccontrollers

import (
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/api/core/v1"
	"math"
	"testing"
)

func isEqual(instance1, instance2 v1.ControllerInstance) bool {
	if instance1.Name != instance2.Name || instance1.ControllerKey != instance2.ControllerKey ||
		instance1.ControllerType != instance2.ControllerType {
		return false
	}

	return true
}

func isInList(instances []v1.ControllerInstance, instance v1.ControllerInstance) bool {
	for _, item := range instances {
		if item.Name == instance.Name {
			return isEqual(instance, item)
		}
	}

	return false
}

func TestMultipleReplicaSetControllerLifeCycle(t *testing.T) {
	// start controller manager 1
	_, closeFn1, cim1, rm1, informers1, client1 := rmSetup(t)
	defer closeFn1()
	stopCh1 := runControllerAndInformers(t, cim1, rm1, informers1, 0)
	defer close(stopCh1)

	// check replicaset controller status in controller manager 1
	controllerInstanceList, err := client1.CoreV1().ControllerInstances().List(metav1.ListOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstanceList)
	assert.Equal(t, 1, len(controllerInstanceList.Items), "number of controller instance")

	rsControllerInstance1 := controllerInstanceList.Items[0]
	assert.Equal(t, int64(math.MaxInt64), rsControllerInstance1.ControllerKey)
	assert.NotNil(t, rsControllerInstance1.Name, "Nil controller instance name")
	assert.False(t, rsControllerInstance1.IsLocked, "Unexpected 1st controller instance status")
	assert.Equal(t, rm1.GetControllerType(), rsControllerInstance1.ControllerType, "Unexpected controller type")

/*
	// start controller manager 2
	cim2, rm2, informers2, client2 := rmSetupControllerMaster(t, s)
	stopCh2 := runControllerAndInformers(t, cim2, rm2, informers2, 0)
	defer close(stopCh2)

	// check replicaset controller status in controller manager 2
	t.Logf("rm 1 instance id: %v", rm1.GetControllerName())
	t.Logf("rm 2 instance id: %v", rm2.GetControllerName())

	assert.NotEqual(t, rm1.GetControllerName(), rm2.GetControllerName())
	controllerInstanceList2, err := client2.CoreV1().ControllerInstances().List(metav1.ListOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstanceList2)
	assert.Equal(t, 2, len(controllerInstanceList2.Items), "number of controller instance")
	t.Logf("new rms [%#v]", controllerInstanceList2)

	rsControllerInstanceRead1, err := client2.CoreV1().ControllerInstances().Get(rm1.GetControllerName(), metav1.GetOptions{})
	assert.Nil(t, err)
	rsControllerInstanceRead2, err := client2.CoreV1().ControllerInstances().Get(rm2.GetControllerName(), metav1.GetOptions{})
	assert.Nil(t, err)

	// check controller instance updates
	assert.Equal(t, rsControllerInstance1.Name, rsControllerInstanceRead1.Name)
	assert.Equal(t, rsControllerInstance1.ControllerType, rsControllerInstanceRead1.ControllerType)
	assert.Equal(t, int64(math.MaxInt64), rsControllerInstanceRead1.ControllerKey)
	assert.Equal(t, rsControllerInstance1.ControllerKey, rsControllerInstanceRead1.ControllerKey)

	assert.True(t, 0 < rsControllerInstanceRead1.ControllerKey)
	assert.True(t, rsControllerInstanceRead2.ControllerKey < rsControllerInstanceRead1.ControllerKey)

	assert.False(t, rsControllerInstanceRead1.IsLocked, "Unexpected 1st controller instance status")
	assert.False(t, rsControllerInstanceRead2.IsLocked, "Unexpected 2nd controller instance status")
	assert.Equal(t, rm2.GetControllerType(), rsControllerInstanceRead2.ControllerType, "Unexpected controller type")
 */
}
