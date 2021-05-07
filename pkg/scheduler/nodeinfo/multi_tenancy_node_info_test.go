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

package nodeinfo

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func makeBasePodWithMultiTenancy(t testingMode, nodeName, objName, cpu, mem, extended string, ports []v1.ContainerPort) *v1.Pod {
	req := v1.ResourceList{}
	if cpu != "" {
		req = v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(cpu),
			v1.ResourceMemory: resource.MustParse(mem),
		}
		if extended != "" {
			parts := strings.Split(extended, ":")
			if len(parts) != 2 {
				t.Fatalf("Invalid extended resource string: \"%s\"", extended)
			}
			req[v1.ResourceName(parts[0])] = resource.MustParse(parts[1])
		}
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID(objName),
			Tenant:    "test-te",
			Namespace: "node_info_cache_test",
			Name:      objName,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Resources: v1.ResourceRequirements{
					Requests: req,
				},
				ResourcesAllocated: req,
				Ports:              ports,
			}},
			NodeName: nodeName,
		},
	}
}

func TestNewNodeInfoWithMultiTenancy(t *testing.T) {
	nodeName := "test-node"
	pods := []*v1.Pod{
		makeBasePodWithMultiTenancy(t, nodeName, "test-1", "100m", "500", "", []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 80, Protocol: "TCP"}}),
		makeBasePodWithMultiTenancy(t, nodeName, "test-2", "200m", "1Ki", "", []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 8080, Protocol: "TCP"}}),
	}

	expected := &NodeInfo{
		requestedResource: &Resource{
			MilliCPU:         300,
			Memory:           1524,
			EphemeralStorage: 0,
			AllowedPodNumber: 0,
			ScalarResources:  map[v1.ResourceName]int64(nil),
		},
		nonzeroRequest: &Resource{
			MilliCPU:         300,
			Memory:           1524,
			EphemeralStorage: 0,
			AllowedPodNumber: 0,
			ScalarResources:  map[v1.ResourceName]int64(nil),
		},
		TransientInfo:       NewTransientSchedulerInfo(),
		allocatableResource: &Resource{},
		generation:          2,
		usedPorts: HostPortInfo{
			"127.0.0.1": map[ProtocolPort]struct{}{
				{Protocol: "TCP", Port: 80}:   {},
				{Protocol: "TCP", Port: 8080}: {},
			},
		},
		imageStates: map[string]*ImageStateSummary{},
		pods: []*v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Tenant:    "test-te",
					Namespace: "node_info_cache_test",
					Name:      "test-1",
					UID:       types.UID("test-1"),
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("100m"),
									v1.ResourceMemory: resource.MustParse("500"),
								},
							},
							ResourcesAllocated: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("100m"),
								v1.ResourceMemory: resource.MustParse("500"),
							},
							Ports: []v1.ContainerPort{
								{
									HostIP:   "127.0.0.1",
									HostPort: 80,
									Protocol: "TCP",
								},
							},
						},
					},
					NodeName: nodeName,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Tenant:    "test-te",
					Namespace: "node_info_cache_test",
					Name:      "test-2",
					UID:       types.UID("test-2"),
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("200m"),
									v1.ResourceMemory: resource.MustParse("1Ki"),
								},
							},
							ResourcesAllocated: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("200m"),
								v1.ResourceMemory: resource.MustParse("1Ki"),
							},
							Ports: []v1.ContainerPort{
								{
									HostIP:   "127.0.0.1",
									HostPort: 8080,
									Protocol: "TCP",
								},
							},
						},
					},
					NodeName: nodeName,
				},
			},
		},
	}

	gen := generation
	ni := NewNodeInfo(pods...)
	if ni.generation <= gen {
		t.Errorf("generation is not incremented. previous: %v, current: %v", gen, ni.generation)
	}
	for i := range expected.pods {
		_ = expected.pods[i].Spec.Workloads()
	}
	expected.generation = ni.generation
	if !reflect.DeepEqual(expected, ni) {
		t.Errorf("\nEXPECT: %#v\nACTUAL: %#v\n", expected, ni)
	}
}

func TestNodeInfoCloneWithMultiTenancy(t *testing.T) {
	nodeName := "test-node"
	tests := []struct {
		nodeInfo *NodeInfo
		expected *NodeInfo
	}{
		{
			nodeInfo: &NodeInfo{
				requestedResource:   &Resource{},
				nonzeroRequest:      &Resource{},
				TransientInfo:       NewTransientSchedulerInfo(),
				allocatableResource: &Resource{},
				generation:          2,
				usedPorts: HostPortInfo{
					"127.0.0.1": map[ProtocolPort]struct{}{
						{Protocol: "TCP", Port: 80}:   {},
						{Protocol: "TCP", Port: 8080}: {},
					},
				},
				imageStates: map[string]*ImageStateSummary{},
				pods: []*v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Tenant:    "test-te",
							Namespace: "node_info_cache_test",
							Name:      "test-1",
							UID:       types.UID("test-1"),
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    resource.MustParse("100m"),
											v1.ResourceMemory: resource.MustParse("500"),
										},
									},
									ResourcesAllocated: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("100m"),
										v1.ResourceMemory: resource.MustParse("500"),
									},
									Ports: []v1.ContainerPort{
										{
											HostIP:   "127.0.0.1",
											HostPort: 80,
											Protocol: "TCP",
										},
									},
								},
							},
							NodeName: nodeName,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Tenant:    "test-te",
							Namespace: "node_info_cache_test",
							Name:      "test-2",
							UID:       types.UID("test-2"),
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    resource.MustParse("200m"),
											v1.ResourceMemory: resource.MustParse("1Ki"),
										},
									},
									ResourcesAllocated: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("200m"),
										v1.ResourceMemory: resource.MustParse("1Ki"),
									},
									Ports: []v1.ContainerPort{
										{
											HostIP:   "127.0.0.1",
											HostPort: 8080,
											Protocol: "TCP",
										},
									},
								},
							},
							NodeName: nodeName,
						},
					},
				},
			},
			expected: &NodeInfo{
				requestedResource:   &Resource{},
				nonzeroRequest:      &Resource{},
				TransientInfo:       NewTransientSchedulerInfo(),
				allocatableResource: &Resource{},
				generation:          2,
				usedPorts: HostPortInfo{
					"127.0.0.1": map[ProtocolPort]struct{}{
						{Protocol: "TCP", Port: 80}:   {},
						{Protocol: "TCP", Port: 8080}: {},
					},
				},
				imageStates: map[string]*ImageStateSummary{},
				pods: []*v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Tenant:    "test-te",
							Namespace: "node_info_cache_test",
							Name:      "test-1",
							UID:       types.UID("test-1"),
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    resource.MustParse("100m"),
											v1.ResourceMemory: resource.MustParse("500"),
										},
									},
									ResourcesAllocated: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("100m"),
										v1.ResourceMemory: resource.MustParse("500"),
									},
									Ports: []v1.ContainerPort{
										{
											HostIP:   "127.0.0.1",
											HostPort: 80,
											Protocol: "TCP",
										},
									},
								},
							},
							NodeName: nodeName,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Tenant:    "test-te",
							Namespace: "node_info_cache_test",
							Name:      "test-2",
							UID:       types.UID("test-2"),
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    resource.MustParse("200m"),
											v1.ResourceMemory: resource.MustParse("1Ki"),
										},
									},
									ResourcesAllocated: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("200m"),
										v1.ResourceMemory: resource.MustParse("1Ki"),
									},
									Ports: []v1.ContainerPort{
										{
											HostIP:   "127.0.0.1",
											HostPort: 8080,
											Protocol: "TCP",
										},
									},
								},
							},
							NodeName: nodeName,
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		ni := test.nodeInfo.Clone()
		// Modify the field to check if the result is a clone of the origin one.
		test.nodeInfo.generation += 10
		test.nodeInfo.usedPorts.Remove("127.0.0.1", "TCP", 80)
		if !reflect.DeepEqual(test.expected, ni) {
			t.Errorf("expected: %#v, got: %#v", test.expected, ni)
		}
	}
}

func TestNodeInfoAddPodWithMultiTenancy(t *testing.T) {
	nodeName := "test-node"
	pods := []*v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Tenant:    "test-te",
				Namespace: "node_info_cache_test",
				Name:      "test-1",
				UID:       types.UID("test-1"),
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("100m"),
								v1.ResourceMemory: resource.MustParse("500"),
							},
						},
						ResourcesAllocated: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("500"),
						},
						Ports: []v1.ContainerPort{
							{
								HostIP:   "127.0.0.1",
								HostPort: 80,
								Protocol: "TCP",
							},
						},
					},
				},
				NodeName: nodeName,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Tenant:    "test-te",
				Namespace: "node_info_cache_test",
				Name:      "test-2",
				UID:       types.UID("test-2"),
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("200m"),
								v1.ResourceMemory: resource.MustParse("1Ki"),
							},
						},
						ResourcesAllocated: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("200m"),
							v1.ResourceMemory: resource.MustParse("1Ki"),
						},
						Ports: []v1.ContainerPort{
							{
								HostIP:   "127.0.0.1",
								HostPort: 8080,
								Protocol: "TCP",
							},
						},
					},
				},
				NodeName: nodeName,
			},
		},
	}
	expected := &NodeInfo{
		node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
		},
		requestedResource: &Resource{
			MilliCPU:         300,
			Memory:           1524,
			EphemeralStorage: 0,
			AllowedPodNumber: 0,
			ScalarResources:  map[v1.ResourceName]int64(nil),
		},
		nonzeroRequest: &Resource{
			MilliCPU:         300,
			Memory:           1524,
			EphemeralStorage: 0,
			AllowedPodNumber: 0,
			ScalarResources:  map[v1.ResourceName]int64(nil),
		},
		TransientInfo:       NewTransientSchedulerInfo(),
		allocatableResource: &Resource{},
		generation:          2,
		usedPorts: HostPortInfo{
			"127.0.0.1": map[ProtocolPort]struct{}{
				{Protocol: "TCP", Port: 80}:   {},
				{Protocol: "TCP", Port: 8080}: {},
			},
		},
		imageStates: map[string]*ImageStateSummary{},
		pods: []*v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Tenant:    "test-te",
					Namespace: "node_info_cache_test",
					Name:      "test-1",
					UID:       types.UID("test-1"),
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("100m"),
									v1.ResourceMemory: resource.MustParse("500"),
								},
							},
							ResourcesAllocated: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("100m"),
								v1.ResourceMemory: resource.MustParse("500"),
							},
							Ports: []v1.ContainerPort{
								{
									HostIP:   "127.0.0.1",
									HostPort: 80,
									Protocol: "TCP",
								},
							},
						},
					},
					NodeName: nodeName,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Tenant:    "test-te",
					Namespace: "node_info_cache_test",
					Name:      "test-2",
					UID:       types.UID("test-2"),
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("200m"),
									v1.ResourceMemory: resource.MustParse("1Ki"),
								},
							},
							ResourcesAllocated: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("200m"),
								v1.ResourceMemory: resource.MustParse("1Ki"),
							},
							Ports: []v1.ContainerPort{
								{
									HostIP:   "127.0.0.1",
									HostPort: 8080,
									Protocol: "TCP",
								},
							},
						},
					},
					NodeName: nodeName,
				},
			},
		},
	}

	ni := fakeNodeInfo()
	gen := ni.generation
	for _, pod := range pods {
		ni.AddPod(pod)
		if ni.generation <= gen {
			t.Errorf("generation is not incremented. Prev: %v, current: %v", gen, ni.generation)
		}
		gen = ni.generation
	}
	for i := range expected.pods {
		_ = expected.pods[i].Spec.Workloads()
	}

	expected.generation = ni.generation
	if !reflect.DeepEqual(expected, ni) {
		t.Errorf("expected: %#v, got: %#v", expected, ni)
	}
}

func TestNodeInfoRemovePodWithMultiTenancy(t *testing.T) {
	nodeName := "test-node"
	pods := []*v1.Pod{
		makeBasePodWithMultiTenancy(t, nodeName, "test-1", "100m", "500", "", []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 80, Protocol: "TCP"}}),
		makeBasePodWithMultiTenancy(t, nodeName, "test-2", "200m", "1Ki", "", []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 8080, Protocol: "TCP"}}),
	}

	tests := []struct {
		pod              *v1.Pod
		errExpected      bool
		expectedNodeInfo *NodeInfo
	}{
		{
			pod:         makeBasePodWithMultiTenancy(t, nodeName, "non-exist", "0", "0", "", []v1.ContainerPort{{}}),
			errExpected: true,
			expectedNodeInfo: &NodeInfo{
				node: &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
					},
				},
				requestedResource: &Resource{
					MilliCPU:         300,
					Memory:           1524,
					EphemeralStorage: 0,
					AllowedPodNumber: 0,
					ScalarResources:  map[v1.ResourceName]int64(nil),
				},
				nonzeroRequest: &Resource{
					MilliCPU:         300,
					Memory:           1524,
					EphemeralStorage: 0,
					AllowedPodNumber: 0,
					ScalarResources:  map[v1.ResourceName]int64(nil),
				},
				TransientInfo:       NewTransientSchedulerInfo(),
				allocatableResource: &Resource{},
				generation:          2,
				usedPorts: HostPortInfo{
					"127.0.0.1": map[ProtocolPort]struct{}{
						{Protocol: "TCP", Port: 80}:   {},
						{Protocol: "TCP", Port: 8080}: {},
					},
				},
				imageStates: map[string]*ImageStateSummary{},
				pods: []*v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Tenant:    "test-te",
							Namespace: "node_info_cache_test",
							Name:      "test-1",
							UID:       types.UID("test-1"),
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    resource.MustParse("100m"),
											v1.ResourceMemory: resource.MustParse("500"),
										},
									},
									ResourcesAllocated: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("100m"),
										v1.ResourceMemory: resource.MustParse("500"),
									},
									Ports: []v1.ContainerPort{
										{
											HostIP:   "127.0.0.1",
											HostPort: 80,
											Protocol: "TCP",
										},
									},
								},
							},
							NodeName: nodeName,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Tenant:    "test-te",
							Namespace: "node_info_cache_test",
							Name:      "test-2",
							UID:       types.UID("test-2"),
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    resource.MustParse("200m"),
											v1.ResourceMemory: resource.MustParse("1Ki"),
										},
									},
									ResourcesAllocated: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("200m"),
										v1.ResourceMemory: resource.MustParse("1Ki"),
									},
									Ports: []v1.ContainerPort{
										{
											HostIP:   "127.0.0.1",
											HostPort: 8080,
											Protocol: "TCP",
										},
									},
								},
							},
							NodeName: nodeName,
						},
					},
				},
			},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant:    "test-te",
					Namespace: "node_info_cache_test",
					Name:      "test-1",
					UID:       types.UID("test-1"),
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("100m"),
									v1.ResourceMemory: resource.MustParse("500"),
								},
							},
							ResourcesAllocated: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("100m"),
								v1.ResourceMemory: resource.MustParse("500"),
							},
							Ports: []v1.ContainerPort{
								{
									HostIP:   "127.0.0.1",
									HostPort: 80,
									Protocol: "TCP",
								},
							},
						},
					},
					NodeName: nodeName,
				},
			},
			errExpected: false,
			expectedNodeInfo: &NodeInfo{
				node: &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
					},
				},
				requestedResource: &Resource{
					MilliCPU:         200,
					Memory:           1024,
					EphemeralStorage: 0,
					AllowedPodNumber: 0,
					ScalarResources:  map[v1.ResourceName]int64(nil),
				},
				nonzeroRequest: &Resource{
					MilliCPU:         200,
					Memory:           1024,
					EphemeralStorage: 0,
					AllowedPodNumber: 0,
					ScalarResources:  map[v1.ResourceName]int64(nil),
				},
				TransientInfo:       NewTransientSchedulerInfo(),
				allocatableResource: &Resource{},
				generation:          3,
				usedPorts: HostPortInfo{
					"127.0.0.1": map[ProtocolPort]struct{}{
						{Protocol: "TCP", Port: 8080}: {},
					},
				},
				imageStates: map[string]*ImageStateSummary{},
				pods: []*v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Tenant:    "test-te",
							Namespace: "node_info_cache_test",
							Name:      "test-2",
							UID:       types.UID("test-2"),
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    resource.MustParse("200m"),
											v1.ResourceMemory: resource.MustParse("1Ki"),
										},
									},
									ResourcesAllocated: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("200m"),
										v1.ResourceMemory: resource.MustParse("1Ki"),
									},
									Ports: []v1.ContainerPort{
										{
											HostIP:   "127.0.0.1",
											HostPort: 8080,
											Protocol: "TCP",
										},
									},
								},
							},
							NodeName: nodeName,
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		ni := fakeNodeInfo(pods...)

		gen := ni.generation
		err := ni.RemovePod(test.pod)
		if err != nil {
			if test.errExpected {
				expectedErrorMsg := fmt.Errorf("no corresponding pod %s in pods of node %s", test.pod.Name, ni.Node().Name)
				if expectedErrorMsg == err {
					t.Errorf("expected error: %v, got: %v", expectedErrorMsg, err)
				}
			} else {
				t.Errorf("expected no error, got: %v", err)
			}
		} else {
			if ni.generation <= gen {
				t.Errorf("generation is not incremented. Prev: %v, current: %v", gen, ni.generation)
			}
		}
		for i := range test.expectedNodeInfo.pods {
			_ = test.expectedNodeInfo.pods[i].Spec.Workloads()
		}

		test.expectedNodeInfo.generation = ni.generation
		if !reflect.DeepEqual(test.expectedNodeInfo, ni) {
			t.Errorf("expected: %#v, got: %#v", test.expectedNodeInfo, ni)
		}
	}
}
