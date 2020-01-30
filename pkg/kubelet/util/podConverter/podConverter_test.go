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

package podConverter

import (
	"encoding/base64"
	"encoding/json"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

func TestPodConverter(t *testing.T) {
	sshkey := "testsshkeyvalue"
	userData := "mount /dev/testpvc, /mnt"
	testVpc := "testsVpc"

	testNicsPortId := []v1.Nic{
		{
			PortId: "04fe371d-fddf-43c4-9419-b7d9cd4a2197",
		},
	}
	testNicsSubnet := []v1.Nic{
		{
			Name:       "eth0",
			SubnetName: "subnet1",
		},
	}
	testsingleNic := []v1.Nic{
		{
			Name:        "eth0",
			SubnetName:  "subnet1",
			PortId:      "04fe371d-fddf-43c4-9419-b7d9cd4a2197",
			IpAddress:   "10.213.0.1",
			Tag:         "podConverter UT test",
			Ipv6Enabled: false,
		},
	}
	testMultipleNics := []v1.Nic{
		{
			Name:        "eth0",
			SubnetName:  "subnet1",
			PortId:      "04fe371d-fddf-43c4-9419-b7d9cd4a2197",
			IpAddress:   "10.213.0.1",
			Tag:         "podConverter UT test, nic1",
			Ipv6Enabled: false,
		},
		{
			Name:        "eth1",
			SubnetName:  "subnet1",
			PortId:      "1234abcd-fddf-43c4-9419-b7d9cd4a2197",
			IpAddress:   "10.213.0.2",
			Tag:         "podConverter UT test, nic2",
			Ipv6Enabled: false,
		},
	}

	testCases := []struct {
		description         string
		vmPod               v1.Pod
		expectedAnnotations map[string]string
		expectCpodNil       bool
	}{
		{
			"basic case",
			v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					VirtualMachine: &v1.VirtualMachine{
						Name:      "test",
						PublicKey: sshkey,
					},
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
					Conditions: []v1.PodCondition{
						{Type: v1.PodScheduled, Status: v1.ConditionUnknown},
					},
				},
			},
			map[string]string{
				rootVolumeSizeKeyName: defaultVirtletRootVolumeSize,
				VirtletRuntimeKeyName: defaultVirtletRuntimeValue,
				sshKeysKeyName:        sshkey,
			},
			false,
		},
		{
			"pod with volume",
			v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					VirtualMachine: &v1.VirtualMachine{
						Name:      "test",
						PublicKey: sshkey,
						UserData:  []byte(base64.StdEncoding.EncodeToString([]byte(userData))),
					},
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
					Conditions: []v1.PodCondition{
						{Type: v1.PodScheduled, Status: v1.ConditionTrue},
					},
				},
			},
			map[string]string{
				rootVolumeSizeKeyName:    defaultVirtletRootVolumeSize,
				VirtletRuntimeKeyName:    defaultVirtletRuntimeValue,
				sshKeysKeyName:           sshkey,
				cloudInitUserDataKeyName: userData,
			},
			false,
		},
		{
			"pod with network interface, nic with portId only",
			makeNicTestVmPod(testVpc, testNicsPortId),
			makeNicTestExpectedAnnotations(testVpc, testNicsPortId),
			false,
		},
		{
			"pod with network interface, nic with subnet",
			makeNicTestVmPod(testVpc, testNicsSubnet),
			makeNicTestExpectedAnnotations(testVpc, testNicsSubnet),
			false,
		},
		{
			"pod with network interface, single nic",
			makeNicTestVmPod(testVpc, testsingleNic),
			makeNicTestExpectedAnnotations(testVpc, testsingleNic),
			false,
		},
		{
			"pod with network interface, multiple nics",
			makeNicTestVmPod(testVpc, testMultipleNics),
			makeNicTestExpectedAnnotations(testVpc, testMultipleNics),
			false,
		},
		{
			"invalid vm pod",
			v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{},
				},
			},
			map[string]string{},
			true,
		},
	}
	for _, tc := range testCases {
		cpod := ConvertVmPodToContainerPod(&tc.vmPod)
		if cpod != nil && tc.expectCpodNil || cpod == nil && !tc.expectCpodNil {
			t.Errorf("for test case %q, unexpected contaienr pod returned", tc.description)
		}

		if !tc.expectCpodNil {
			containers := cpod.Spec.Containers
			if containers == nil || len(containers) == 0 {
				t.Errorf("for test case %q, unexpected pod returned, Spec.Containers is nil or empty", tc.description)
			}
			vm := tc.vmPod.Spec.VirtualMachine
			if vm.Name != containers[0].Name || vm.Image != containers[0].Image || vm.ImagePullPolicy != containers[0].ImagePullPolicy {
				t.Errorf("for test case %q, unexpected containers returned.", tc.description)
			}

			if !reflect.DeepEqual(tc.expectedAnnotations, cpod.Annotations) {
				t.Errorf("for test case %q, expect annotations: %q, got: %q\n", tc.description, tc.expectedAnnotations, cpod.Annotations)
			}
			// explicitly verify the Nic slices
			if nicsJson, found := cpod.Annotations[NICsKeyName]; found {
				var nics []v1.Nic
				err := json.Unmarshal([]byte(nicsJson), &nics)
				if err != nil {
					t.Errorf("for test case %q, failed to unmarshal the Nic Json string. Error : %v \n", tc.description, err)
				}
				if !reflect.DeepEqual(nics, tc.vmPod.Spec.Nics) {
					t.Errorf("for test case %q, expect Nics: %v, got: %v\n", tc.description, tc.vmPod.Spec.Nics, nics)
				}
			}

			if !reflect.DeepEqual(tc.vmPod.Status, cpod.Status) {
				t.Errorf("for test case %q, expect status: %v, got: %v\n", tc.description, tc.vmPod.Status, cpod.Status)
			}
		}
	}
}

func makeNicTestVmPod(testVpc string, nics []v1.Nic) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			VPC:  testVpc,
			Nics: nics,
			VirtualMachine: &v1.VirtualMachine{
				Name: "test",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
			Conditions: []v1.PodCondition{
				{Type: v1.PodScheduled, Status: v1.ConditionUnknown},
			},
		},
	}
}

func makeNicTestExpectedAnnotations(testVpc string, nics []v1.Nic) map[string]string {
	return map[string]string{
		rootVolumeSizeKeyName: defaultVirtletRootVolumeSize,
		VirtletRuntimeKeyName: defaultVirtletRuntimeValue,
		VPCKeyName:            testVpc,
		NICsKeyName: func() string {
			s, err := json.Marshal(nics)
			if err != nil {
				return stringEmpty
			}
			return string(s)
		}(),
	}
}
