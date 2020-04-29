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

package fake

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"
)

const (
	testGroup      = "testgroup"
	testVersion    = "testversion"
	testResource   = "testkinds"
	testTenant     = "testtenant"
	testNamespace  = "testns"
	testName       = "testname"
	testKind       = "TestKind"
	testAPIVersion = "testgroup/testversion"
)

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return newUnstructuredWithMultiTenancy(apiVersion, kind, namespace, name, metav1.TenantSystem)
}

func newUnstructuredWithMultiTenancy(apiVersion, kind, namespace, name string, tenant string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"tenant":    tenant,
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}

func newUnstructuredWithSpecWithTenant(spec map[string]interface{}) *unstructured.Unstructured {
	u := newUnstructuredWithMultiTenancy(testAPIVersion, testKind, testNamespace, testName, testTenant)
	u.Object["spec"] = spec
	return u
}

func TestListWithMultiTenancy(t *testing.T) {
	scheme := runtime.NewScheme()

	client := NewSimpleDynamicClient(scheme,
		newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "te-foo"),
		newUnstructuredWithMultiTenancy("group2/version", "TheKind", "ns-foo", "name2-foo", "te-foo"),
		newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-bar", "te-foo"),
		newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-baz", "te-foo"),
		newUnstructuredWithMultiTenancy("group2/version", "TheKind", "ns-foo", "name2-baz", "te-foo"),
	)
	listFirst, err := client.Resource(schema.GroupVersionResource{Group: "group", Version: "version", Resource: "thekinds"}).List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	expected := []unstructured.Unstructured{
		*newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "te-foo"),
		*newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-bar", "te-foo"),
		*newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-baz", "te-foo"),
	}
	if !equality.Semantic.DeepEqual(listFirst.Items, expected) {
		t.Fatal(diff.ObjectGoPrintDiff(expected, listFirst.Items))
	}
}

type patchTestCaseWithMultiTenancy struct {
	name                  string
	object                runtime.Object
	patchType             types.PatchType
	patchBytes            []byte
	wantErrMsg            string
	expectedPatchedObject runtime.Object
}

func (tc *patchTestCaseWithMultiTenancy) runner(t *testing.T) {
	client := NewSimpleDynamicClient(runtime.NewScheme(), tc.object)
	resourceInterface := client.Resource(schema.GroupVersionResource{Group: testGroup, Version: testVersion, Resource: testResource}).NamespaceWithMultiTenancy(testNamespace, testTenant)

	got, recErr := resourceInterface.Patch(testName, tc.patchType, tc.patchBytes, metav1.PatchOptions{})

	if err := tc.verifyErr(recErr); err != nil {
		t.Error(err)
	}

	if err := tc.verifyResult(got); err != nil {
		t.Error(err)
	}

}

// verifyErr verifies that the given error returned from Patch is the error
// expected by the test case.
func (tc *patchTestCaseWithMultiTenancy) verifyErr(err error) error {
	if tc.wantErrMsg != "" && err == nil {
		return fmt.Errorf("want error, got nil")
	}

	if tc.wantErrMsg == "" && err != nil {
		return fmt.Errorf("want no error, got %v", err)
	}

	if err != nil {
		if want, got := tc.wantErrMsg, err.Error(); want != got {
			return fmt.Errorf("incorrect error: want: %q got: %q", want, got)
		}
	}
	return nil
}

func (tc *patchTestCaseWithMultiTenancy) verifyResult(result *unstructured.Unstructured) error {
	if tc.expectedPatchedObject == nil && result == nil {
		return nil
	}
	if !equality.Semantic.DeepEqual(result, tc.expectedPatchedObject) {
		return fmt.Errorf("unexpected diff in received object: %s", diff.ObjectGoPrintDiff(tc.expectedPatchedObject, result))
	}
	return nil
}

func TestPatchWithMultiTenancy(t *testing.T) {
	testCases := []patchTestCaseWithMultiTenancy{
		{
			name:       "jsonpatch fails with merge type",
			object:     newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "bar"}),
			patchType:  types.StrategicMergePatchType,
			patchBytes: []byte(`[]`),
			wantErrMsg: "invalid JSON document",
		},
		{
			name:      "jsonpatch works with empty patch",
			object:    newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "bar"}),
			patchType: types.JSONPatchType,
			// No-op
			patchBytes:            []byte(`[]`),
			expectedPatchedObject: newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "bar"}),
		}, {
			name:      "jsonpatch works with simple change patch",
			object:    newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "bar"}),
			patchType: types.JSONPatchType,
			// change spec.foo from bar to foobar
			patchBytes:            []byte(`[{"op": "replace", "path": "/spec/foo", "value": "foobar"}]`),
			expectedPatchedObject: newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "foobar"}),
		}, {
			name:      "jsonpatch works with simple addition",
			object:    newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "bar"}),
			patchType: types.JSONPatchType,
			// add spec.newvalue = dummy
			patchBytes:            []byte(`[{"op": "add", "path": "/spec/newvalue", "value": "dummy"}]`),
			expectedPatchedObject: newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "bar", "newvalue": "dummy"}),
		}, {
			name:      "jsonpatch works with simple deletion",
			object:    newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "bar", "toremove": "shouldnotbehere"}),
			patchType: types.JSONPatchType,
			// remove spec.newvalue = dummy
			patchBytes:            []byte(`[{"op": "remove", "path": "/spec/toremove"}]`),
			expectedPatchedObject: newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "bar"}),
		}, {
			name:      "strategic merge patch fails with JSONPatch",
			object:    newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "bar"}),
			patchType: types.StrategicMergePatchType,
			// add spec.newvalue = dummy
			patchBytes: []byte(`[{"op": "add", "path": "/spec/newvalue", "value": "dummy"}]`),
			wantErrMsg: "invalid JSON document",
		}, {
			name:                  "merge patch works with simple replacement",
			object:                newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "bar"}),
			patchType:             types.MergePatchType,
			patchBytes:            []byte(`{ "spec": { "foo": "baz" } }`),
			expectedPatchedObject: newUnstructuredWithSpecWithTenant(map[string]interface{}{"foo": "baz"}),
		},
		// TODO: Add tests for strategic merge using v1.Pod for example to ensure the test cases
		// demonstrate expected use cases.
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.runner)
	}
}
