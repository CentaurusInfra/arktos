/*
Copyright 2019 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

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

// File modified by backporting scheduler 1.18.5 from kubernetes on 05/04/2021
package noderuntimenotready

import (
	"context"
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
)

func TestPodSchedulesOnRuntimeNotReadyCondition(t *testing.T) {
	notReadyNode := &v1.Node{
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeVmRuntimeReady,
					Status: v1.ConditionFalse,
				},
				{
					Type:   v1.NodeContainerRuntimeReady,
					Status: v1.ConditionFalse,
				},
			},
		},
	}

	tests := []struct {
		name                   string
		pod                    *v1.Pod
		fits                   bool
		expectedFailureReasons []string
	}{
		{
			name: "regular pod does not fit",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod1",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{Image: "pod1:V1"}},
				},
			},
			fits:                   false,
			expectedFailureReasons: []string{ErrNodeRuntimeNotReady},
		},
		{
			name: "noschedule-tolerant pod fits",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod2",
				},
				Spec: v1.PodSpec{
					Containers:  []v1.Container{{Image: "pod2:V2"}},
					Tolerations: []v1.Toleration{{Operator: "Exists", Effect: "NoSchedule"}},
				},
			},
			fits: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodeInfo := schedulernodeinfo.NewNodeInfo()
			nodeInfo.SetNode(notReadyNode)
			p, _ := New(nil, nil)
			gotStatus := p.(framework.FilterPlugin).Filter(context.Background(), nil, test.pod, nodeInfo)

			if gotStatus.IsSuccess() != test.fits {
				t.Fatalf("unexpected fits: %v, want: %v", gotStatus.IsSuccess(), test.fits)
			}
			if !gotStatus.IsSuccess() && !reflect.DeepEqual(gotStatus.Reasons(), test.expectedFailureReasons) {
				t.Fatalf("unexpected failure reasons: %v, want: %v", gotStatus.Reasons(), test.expectedFailureReasons)
			}
		})
	}
}
