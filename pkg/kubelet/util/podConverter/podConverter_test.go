/*
Copyright 2015 The Kubernetes Authors.

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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

func TestPodConverter(t *testing.T) {
	sshkey := "testsshvalue"
	userData := "mount /dev/testpvc, /mnt"

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
				rootVolumeSizeKeyName:        defaultVirtletRootVolumeSize,
				defaultVirtletRuntimeKeyName: defaultVirtletRuntimeValue,
				sshKeysKeyName:               sshkey,
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
				rootVolumeSizeKeyName:        defaultVirtletRootVolumeSize,
				defaultVirtletRuntimeKeyName: defaultVirtletRuntimeValue,
				sshKeysKeyName:               sshkey,
				cloudInitUserDataKeyName:     userData,
			},
			false,
		},
		// TODO: add network support with Cloud Fabric CNI
		//{
		//	"pod with network interface",
		//	v1.Pod{},
		//	map[string]string{},
		//	false,
		//},
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
			if !reflect.DeepEqual(tc.vmPod.Status, cpod.Status) {
				t.Errorf("for test case %q, expect status: %v, got: %v\n", tc.description, tc.vmPod.Status, cpod.Status)
			}
		}
	}
}
