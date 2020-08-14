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

package cloudfabriccontrollers

import (
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
	"sort"
	"testing"
)

func TestMultipleReplicaSetControllerLifeCycle(t *testing.T) {
	// case 1. start controller manager 1
	_, closeFn1, cim1, rsc1, informers1, client1 := RmSetup(t)
	defer closeFn1()
	stopCh1 := RunControllerAndInformers(t, cim1, rsc1, informers1, 0)
	defer close(stopCh1)

	// check replicaset controller status in controller manager 1
	controllerInstanceList, err := client1.CoreV1().ControllerInstances().List(metav1.ListOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, controllerInstanceList)
	assert.Equal(t, 1, len(controllerInstanceList.Items), "number of controller instance")

	rsControllerInstance1 := controllerInstanceList.Items[0]
	assert.Equal(t, int64(math.MaxInt64), rsControllerInstance1.ControllerKey)
	assert.NotNil(t, rsControllerInstance1.Name, "Nil controller instance name")
	assert.Equal(t, rsc1.GetControllerType(), rsControllerInstance1.ControllerType, "Unexpected controller type")

	/* Comment out as we can no longer test this case in integration test.
	Also UT was able to simulate this scenario
	*/

	/*
		// case 2. start controller manager 2
		cim2, rsc2, informers2, client2 := RmSetupControllerMaster(t, s)
		stopCh2 := RunControllerAndInformers(t, cim2, rsc2, informers2, 0)
		defer close(stopCh2)

		// wait for controller instance to update controller key in registry
		time.Sleep(5 * time.Second)
		rsc1.ReportHealth(client1)
		rsc2.ReportHealth(client2)

		// check replicaset controller status in controller manager 2
		t.Logf("rm 1 instance id: %v", rsc1.GetControllerName())
		t.Logf("rm 2 instance id: %v", rsc2.GetControllerName())

		assert.NotEqual(t, rsc1.GetControllerName(), rsc2.GetControllerName())
		controllerInstanceList2, err := client2.CoreV1().ControllerInstances().List(metav1.ListOptions{})
		assert.Nil(t, err)
		assert.NotNil(t, controllerInstanceList2)
		assert.Equal(t, 2, len(controllerInstanceList2.Items), "number of controller instance")
		t.Logf("new rms [%#v]", controllerInstanceList2)

		rsControllerInstanceRead1, err := client2.CoreV1().ControllerInstances().Get(rsc1.GetControllerName(), metav1.GetOptions{})
		assert.Nil(t, err)
		rsControllerInstanceRead2, err := client2.CoreV1().ControllerInstances().Get(rsc2.GetControllerName(), metav1.GetOptions{})
		assert.Nil(t, err)

		instance1 := getInstanceFromControllerInstancesAndExpectedPos(0, rsControllerInstanceRead1, rsControllerInstanceRead2)
		instance2 := getInstanceFromControllerInstancesAndExpectedPos(1, rsControllerInstanceRead1, rsControllerInstanceRead2)
		t.Logf("sorted instance 1 %s controllerKey %v", instance1.Name, instance1.ControllerKey)
		t.Logf("sorted instance 2 %s controllerKey %v", instance2.Name, instance2.ControllerKey)
		// check controller instance updates
		assert.Equal(t, rsControllerInstance1.ControllerType, instance1.ControllerType)
		assert.Equal(t, rsControllerInstance1.ControllerType, instance2.ControllerType)

		assert.Equal(t, int64(4611686018427387904), instance1.ControllerKey) // consistent hash
		assert.Equal(t, int64(math.MaxInt64), instance2.ControllerKey)

		// case 3. start controller manager 3
		cim3, rsc3, informers3, client3 := RmSetupControllerMaster(t, s)
		stopCh3 := RunControllerAndInformers(t, cim3, rsc3, informers3, 0)
		defer close(stopCh3)
		time.Sleep(5 * time.Second)
		t.Logf("rm 3 instance id: %v", rsc3.GetControllerName())
		rsc1.ReportHealth(client1)
		rsc2.ReportHealth(client2)
		rsc3.ReportHealth(client3)

		// check replicaset controller status in controller manager 2
		assert.NotEqual(t, rsc1.GetControllerName(), rsc3.GetControllerName())
		assert.NotEqual(t, rsc2.GetControllerName(), rsc3.GetControllerName())
		controllerInstanceList3, err := client2.CoreV1().ControllerInstances().List(metav1.ListOptions{})
		assert.Nil(t, err)
		assert.NotNil(t, controllerInstanceList3)
		assert.Equal(t, 3, len(controllerInstanceList3.Items), "number of controller instance")

		rsControllerInstanceRead1, err = client3.CoreV1().ControllerInstances().Get(rsc1.GetControllerName(), metav1.GetOptions{})
		assert.Nil(t, err)
		rsControllerInstanceRead2, err = client3.CoreV1().ControllerInstances().Get(rsc2.GetControllerName(), metav1.GetOptions{})
		assert.Nil(t, err)
		rsControllerInstanceRead3, err := client3.CoreV1().ControllerInstances().Get(rsc3.GetControllerName(), metav1.GetOptions{})
		assert.Nil(t, err)

		instance1 = getInstanceFromControllerInstancesAndExpectedPos(0, rsControllerInstanceRead1, rsControllerInstanceRead2, rsControllerInstanceRead3)
		instance2 = getInstanceFromControllerInstancesAndExpectedPos(1, rsControllerInstanceRead1, rsControllerInstanceRead2, rsControllerInstanceRead3)
		instance3 := getInstanceFromControllerInstancesAndExpectedPos(2, rsControllerInstanceRead1, rsControllerInstanceRead2, rsControllerInstanceRead3)
		t.Logf("sorted instance 1 %s controllerKey %v", instance1.Name, instance1.ControllerKey)
		t.Logf("sorted instance 2 %s controllerKey %v", instance2.Name, instance2.ControllerKey)
		t.Logf("sorted instance 3 %s controllerKey %v", instance3.Name, instance3.ControllerKey)

		// check controller instance updates
		assert.True(t, instance1.ControllerKey < instance2.ControllerKey)
		assert.True(t, instance2.ControllerKey < instance3.ControllerKey)
		assert.Equal(t, int64(3074457345618258603), instance1.ControllerKey)
		assert.Equal(t, int64(6148914691236517205), instance2.ControllerKey)
		assert.Equal(t, int64(math.MaxInt64), instance3.ControllerKey)

		assert.Equal(t, rsControllerInstance1.Name, rsControllerInstanceRead1.Name)
		assert.Equal(t, rsControllerInstance1.ControllerType, rsControllerInstanceRead1.ControllerType)

		assert.Equal(t, rsc1.GetControllerType(), rsControllerInstanceRead1.ControllerType, "Unexpected controller type")
		assert.Equal(t, rsc2.GetControllerType(), rsControllerInstanceRead2.ControllerType, "Unexpected controller type")
		assert.Equal(t, rsc3.GetControllerType(), rsControllerInstanceRead3.ControllerType, "Unexpected controller type")
		t.Logf("new rms [%#v]", controllerInstanceList3)

		CleanupControllers(rsc1.ControllerBase, rsc2.ControllerBase, rsc3.ControllerBase)
		//assert.False(t, rsControllerInstanceRead3.IsLocked, "Unexpected 3rd controller instance status")
	*/
	/*
		// case 4. 1st controller instance died - This needs to be done in unit test as integration test would be flaky
		close(stopCh1)
		// need to manually delete the controller instance from registry as default 5 minute timeout will cause test timeout during batch test.
		err = client3.CoreV1().ControllerInstances().Delete(rsc1.GetControllerName(), &metav1.DeleteOptions{})
		assert.Nil(t, err)

		controllerInstanceList4, err := client2.CoreV1().ControllerInstances().List(metav1.ListOptions{})
		assert.Nil(t, err)
		assert.NotNil(t, controllerInstanceList4)
		assert.Equal(t, 2, len(controllerInstanceList4.Items))
		t.Logf("new rms [%#v]", controllerInstanceList4)

		rsControllerInstanceRead2, err = client2.CoreV1().ControllerInstances().Get(rsc2.GetControllerName(), metav1.GetOptions{})
		assert.Nil(t, err)
		assert.NotNil(t, rsControllerInstanceRead2)
		rsControllerInstanceRead3, err = client2.CoreV1().ControllerInstances().Get(rsc3.GetControllerName(), metav1.GetOptions{})
		assert.Nil(t, err)
		assert.NotNil(t, rsControllerInstanceRead3)

		// check replicaset controller status in controller manager 2
		assert.NotEqual(t, rsControllerInstanceRead2.ControllerKey, rsControllerInstanceRead3.ControllerKey)
		if rsControllerInstanceRead2.ControllerKey < rsControllerInstanceRead3.ControllerKey {
			assert.Equal(t, int64(math.MaxInt64), rsControllerInstanceRead3.ControllerKey)
		} else {
			assert.Equal(t, int64(math.MaxInt64), rsControllerInstanceRead2.ControllerKey)
		}
		assert.False(t, rsControllerInstanceRead2.IsLocked, "Unexpected 2st controller instance status")
		assert.False(t, rsControllerInstanceRead3.IsLocked, "Unexpected 3rd controller instance status")
	*/
}

func getInstanceFromControllerInstancesAndExpectedPos(index int, searchInstances ...*v1.ControllerInstance) *v1.ControllerInstance {
	if len(searchInstances) <= index {
		return nil
	}

	sort.Slice(searchInstances, func(i, j int) bool {
		return searchInstances[i].Name < searchInstances[j].Name
	})

	return searchInstances[index]
}
