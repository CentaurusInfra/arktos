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

package action

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/diff"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	api "k8s.io/kubernetes/pkg/apis/core"

	// install all api groups for testing
	_ "k8s.io/kubernetes/pkg/api/testapi"
)

func TestGetAttrs(t *testing.T) {
	actionA := &api.Action{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a0118",
			Namespace: "default",
			HashKey:   20,
		},
		Spec: api.ActionSpec{
			NodeName: "fooNode",
		},
		Status: api.ActionStatus{
			Complete: true,
		},
	}
	field := ActionToSelectableFields(actionA)
	expect := fields.Set{
		"metadata.name":      "a0118",
		"metadata.namespace": "default",
		"metadata.hashkey":   "20",
		"spec.nodeName":      "fooNode",
		"status.complete":    "true",
	}
	if e, a := expect, field; !reflect.DeepEqual(e, a) {
		t.Errorf("E: %+v\nA: %+v\ndiff: %s", e, a, diff.ObjectDiff(e, a))
	}
}

func TestSelectableFieldLabelConversions(t *testing.T) {
	fset := ActionToSelectableFields(&api.Action{})
	apitesting.TestSelectableFieldLabelConversionsOfKind(t,
		"v1",
		"Action",
		fset,
		nil,
	)
}
