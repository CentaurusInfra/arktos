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
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"
)

const testTenant = "testte"

func newPartialObjectMetadataWithMultitenancy(apiVersion, kind, te, namespace, name string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Tenant:    te,
			Namespace: namespace,
			Name:      name,
		},
	}
}

func TestListWithMultiTenancy(t *testing.T) {
	client := NewSimpleMetadataClient(scheme,
		newPartialObjectMetadataWithMultitenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo"),
		newPartialObjectMetadataWithMultitenancy("group2/version", "TheKind", "te-foo", "ns-foo", "name2-foo"),
		newPartialObjectMetadataWithMultitenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-bar"),
		newPartialObjectMetadataWithMultitenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-baz"),
		newPartialObjectMetadataWithMultitenancy("group2/version", "TheKind", "te-foo", "ns-foo", "name2-baz"),
	)
	listFirst, err := client.Resource(schema.GroupVersionResource{Group: "group", Version: "version", Resource: "thekinds"}).List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	expected := []metav1.PartialObjectMetadata{
		*newPartialObjectMetadataWithMultitenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo"),
		*newPartialObjectMetadataWithMultitenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-bar"),
		*newPartialObjectMetadataWithMultitenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-baz"),
	}
	if !equality.Semantic.DeepEqual(listFirst.Items, expected) {
		t.Fatal(diff.ObjectGoPrintDiff(expected, listFirst.Items))
	}
}

func newPartialObjectMetadataWithAnnotationsAndMultiTenancy(annotations map[string]string) *metav1.PartialObjectMetadata {
	u := newPartialObjectMetadataWithMultitenancy(testAPIVersion, testKind, testTenant, testNamespace, testName)
	u.Annotations = annotations
	return u
}

func TestPatchWithMultiTenancy(t *testing.T) {
	testCases := []patchTestCase{
		{
			name:       "jsonpatch fails with merge type",
			object:     newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "bar"}),
			patchType:  types.StrategicMergePatchType,
			patchBytes: []byte(`[]`),
			wantErrMsg: "invalid JSON document",
		}, {
			name:      "jsonpatch works with empty patch",
			object:    newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "bar"}),
			patchType: types.JSONPatchType,
			// No-op
			patchBytes:            []byte(`[]`),
			expectedPatchedObject: newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "bar"}),
		}, {
			name:      "jsonpatch works with simple change patch",
			object:    newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "bar"}),
			patchType: types.JSONPatchType,
			// change spec.foo from bar to foobar
			patchBytes:            []byte(`[{"op": "replace", "path": "/metadata/annotations/foo", "value": "foobar"}]`),
			expectedPatchedObject: newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "foobar"}),
		}, {
			name:      "jsonpatch works with simple addition",
			object:    newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "bar"}),
			patchType: types.JSONPatchType,
			// add spec.newvalue = dummy
			patchBytes:            []byte(`[{"op": "add", "path": "/metadata/annotations/newvalue", "value": "dummy"}]`),
			expectedPatchedObject: newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "bar", "newvalue": "dummy"}),
		}, {
			name:      "jsonpatch works with simple deletion",
			object:    newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "bar", "toremove": "shouldnotbehere"}),
			patchType: types.JSONPatchType,
			// remove spec.newvalue = dummy
			patchBytes:            []byte(`[{"op": "remove", "path": "/metadata/annotations/toremove"}]`),
			expectedPatchedObject: newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "bar"}),
		}, {
			name:      "strategic merge patch fails with JSONPatch",
			object:    newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "bar"}),
			patchType: types.StrategicMergePatchType,
			// add spec.newvalue = dummy
			patchBytes: []byte(`[{"op": "add", "path": "/metadata/annotations/newvalue", "value": "dummy"}]`),
			wantErrMsg: "invalid JSON document",
		}, {
			name:                  "merge patch works with simple replacement",
			object:                newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "bar"}),
			patchType:             types.MergePatchType,
			patchBytes:            []byte(`{ "metadata": {"annotations": { "foo": "baz" } } }`),
			expectedPatchedObject: newPartialObjectMetadataWithAnnotationsAndMultiTenancy(map[string]string{"foo": "baz"}),
		},
		// TODO: Add tests for strategic merge using v1.Pod for example to ensure the test cases
		// demonstrate expected use cases.
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.runnerWithMultiTenancy)
	}
}

func (tc *patchTestCase) runnerWithMultiTenancy(t *testing.T) {
	client := NewSimpleMetadataClient(scheme, tc.object)
	resourceInterface := client.Resource(schema.GroupVersionResource{Group: testGroup, Version: testVersion, Resource: testResource}).
		NamespaceWithMultiTenancy(testNamespace, testTenant)

	got, recErr := resourceInterface.Patch(testName, tc.patchType, tc.patchBytes, metav1.PatchOptions{})

	if err := tc.verifyErr(recErr); err != nil {
		t.Error(err)
	}

	if err := tc.verifyResult(got); err != nil {
		t.Error(err)
	}

}
