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

package defaultpodnetworkreadiness

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	admissiontesting "k8s.io/apiserver/pkg/admission/testing"
	api "k8s.io/kubernetes/pkg/apis/core"
)

func TestAdmit(t *testing.T) {
	tcs := []struct {
		desc                   string
		pod                    *api.Pod
		notExpectingReadiness  bool
		expectedReadinessValue string
	}{
		{
			desc: "annotate false value to applicable pod",
			pod: &api.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "emptyPod", Namespace: "test-ne", Tenant: "test-te"},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{Name: "ctr1", Image: "image"},
					},
				},
			},
			expectedReadinessValue: "false",
		},
		{
			desc: "not to change pod already has readiness annotated",
			pod: &api.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "readyPod", Namespace: "test-ne", Tenant: "test-te", Annotations: map[string]string{"arktos.futurewei.com/network-readiness": "true"}},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{Name: "ctr1", Image: "image"},
					},
				},
			},
			expectedReadinessValue: "true",
		},
		{
			desc: "not to annotate pod of host network",
			pod: &api.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "readyPod", Namespace: "test-ne", Tenant: "test-te"},
				Spec: api.PodSpec{
					SecurityContext: &api.PodSecurityContext{
						HostNetwork: true,
					},
					Containers: []api.Container{
						{Name: "ctr1", Image: "image"},
					},
				},
			},
			notExpectingReadiness: true,
		},
	}

	handler := admissiontesting.WithReinvocationTesting(t, &plugin{})

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			pod := tc.pod
			attributes := admission.NewAttributesRecord(pod, nil, api.Kind("Pod").WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, api.Resource("pods").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := handler.Admit(attributes, nil)
			if err != nil {
				t.Fatalf("Unexpected error returned from admission handler: %v", err)
			}

			v, ok := pod.Annotations["arktos.futurewei.com/network-readiness"]
			if tc.notExpectingReadiness && ok {
				t.Fatalf("should not have added readiness annotation")
			}
			if v != tc.expectedReadinessValue {
				t.Fatalf("unexpected readiness annotation: expected %q, got %q", tc.expectedReadinessValue, v)
			}
		})
	}
}
