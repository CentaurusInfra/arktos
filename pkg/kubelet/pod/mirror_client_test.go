/*
Copyright 2015 The Kubernetes Authors.
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

package pod

import (
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
)

func TestParsePodFullName(t *testing.T) {
	type nameTuple struct {
		Name      string
		Namespace string
		Tenant    string
	}
	successfulCases := map[string]nameTuple{
		"bar_foo_baz":         {Name: "bar", Namespace: "foo", Tenant: "baz"},
		"bar.org_foo.com_baz": {Name: "bar.org", Namespace: "foo.com", Tenant: "baz"},
		"bar-bar_foo_baz":     {Name: "bar-bar", Namespace: "foo", Tenant: "baz"},
	}
	failedCases := []string{"barfoo", "bar_foo", "", "bar_", "_foo"}

	for podFullName, expected := range successfulCases {
		name, namespace, tenant, err := kubecontainer.ParsePodFullName(podFullName)
		if err != nil {
			t.Errorf("unexpected error when parsing the full name: %v", err)
			continue
		}
		if name != expected.Name || namespace != expected.Namespace || tenant != expected.Tenant {
			t.Errorf("expected name %q, namespace %q; got name %q, namespace %q",
				expected.Name, expected.Namespace, name, namespace)
		}
	}
	for _, podFullName := range failedCases {
		_, _, _, err := kubecontainer.ParsePodFullName(podFullName)
		if err == nil {
			t.Errorf("expected error when parsing the full name, got none")
		}
	}
}

func TestIsStaticPod(t *testing.T) {
	testcases := []struct {
		pod    *v1.Pod
		expect bool
	}{
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						kubetypes.ConfigSourceAnnotationKey: kubetypes.ApiserverSource,
					},
				},
			},
			expect: false,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						kubetypes.ConfigSourceAnnotationKey: kubetypes.ApiserverSource + "0",
					},
				},
			},
			expect: false,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						kubetypes.ConfigSourceAnnotationKey: kubetypes.ApiserverSource + "99",
					},
				},
			},
			expect: false,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expect: false,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"abc": "def",
					},
				},
			},
			expect: false,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						kubetypes.ConfigSourceAnnotationKey: "abc",
					},
				},
			},
			expect: true,
		},
	}

	for i, test := range testcases {
		actual := IsStaticPod(test.pod)
		if test.expect != actual {
			t.Errorf("case %d failed: expected %v, got %v", i, test.expect, actual)
		}
	}
}
