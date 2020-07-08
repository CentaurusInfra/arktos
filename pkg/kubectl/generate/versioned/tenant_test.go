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

package versioned

import (
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//api "k8s.io/kubernetes/pkg/apis/core"
)

func TestTenantGenerate(t *testing.T) {
	tests := []struct {
		name      string
		params    map[string]interface{}
		expected  *v1.Tenant
		expectErr bool
	}{
		{
			name: "test1",
			params: map[string]interface{}{
				"name":           "foo",
				"storagecluster": "1",
			},
			expected: &v1.Tenant{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: v1.TenantSpec{
					StorageClusterId: "1",
				},
			},
			expectErr: false,
		},
		{
			name:      "test2",
			params:    map[string]interface{}{},
			expectErr: true,
		},
		{
			name: "test3",
			params: map[string]interface{}{
				"name":           1,
				"storagecluster": "1",
			},
			expectErr: true,
		},
		{
			name: "test4",
			params: map[string]interface{}{
				"name":           "",
				"storagecluster": "1",
			},
			expectErr: true,
		},
		{
			name: "test5",
			params: map[string]interface{}{
				"name":           nil,
				"storagecluster": "1",
			},
			expectErr: true,
		},
		{
			name: "test6",
			params: map[string]interface{}{
				"name_wrong_key": "some_value",
				"storagecluster": "1",
			},
			expectErr: true,
		},
		{
			name: "test7",
			params: map[string]interface{}{
				"NAME":           "some_value",
				"storagecluster": "1",
			},
			expectErr: true,
		},
		{
			name: "test8",
			params: map[string]interface{}{
				"name": "foo",
			},
			expectErr: true,
		},
		{
			name: "test9",
			params: map[string]interface{}{
				"name":           "foo",
				"storagecluster": "",
			},
			expectErr: true,
		},
		{
			name: "test10",
			params: map[string]interface{}{
				"name":           "foo",
				"storagecluster": "     ",
			},
			expectErr: true,
		},
		{
			name: "test10",
			params: map[string]interface{}{
				"name":           "foo",
				"storagecluster": nil,
			},
			expectErr: true,
		},
		{
			name: "test11",
			params: map[string]interface{}{
				"name":           "foo",
				"storagecluster": "non-integer",
			},
			expectErr: true,
		},
		{
			name: "test12",
			params: map[string]interface{}{
				"name": "foo",
				// the value must be 0-63
				"storagecluster": "-1",
			},
			expectErr: true,
		},
		{
			name: "test13",
			params: map[string]interface{}{
				"name": "foo",
				// the value must be 0-63
				"storagecluster": "64",
			},
			expectErr: true,
		},
	}
	generator := TenantGeneratorV1{}
	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := generator.Generate(tt.params)
			switch {
			case tt.expectErr && err != nil:
				return // loop, since there's no output to check
			case tt.expectErr && err == nil:
				t.Errorf("%v: expected error and didn't get one", index)
				return // loop, no expected output object
			case !tt.expectErr && err != nil:
				t.Errorf("%v: unexpected error %v", index, err)
				return // loop, no output object
			case !tt.expectErr && err == nil:
				// do nothing and drop through
			}
			if !reflect.DeepEqual(obj.(*v1.Tenant), tt.expected) {
				t.Errorf("\nexpected:\n%#v\nsaw:\n%#v", tt.expected, obj.(*v1.Tenant))
			}
		})
	}
}
