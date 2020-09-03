/*
Copyright 2020 Authors of Arktos

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

package generic

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"reflect"
	"testing"
)

func TestObjectMetaFieldsSet(t *testing.T) {
	tests := []struct {
		objectMeta        *metav1.ObjectMeta
		hasNamespaceField bool
		expectedResult    fields.Set
	}{
		{
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameOnly",
				Namespace: "testNS",
			},
			hasNamespaceField: false,
			expectedResult: fields.Set{
				"metadata.hashkey": "0",
				"metadata.name":    "testWitNameOnly",
			},
		},
		{
			objectMeta: &metav1.ObjectMeta{
				Name: "testWitNameOnlyHasEmptyNamespaceField",
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.hashkey":   "0",
				"metadata.name":      "testWitNameOnlyHasEmptyNamespaceField",
				"metadata.namespace": "",
			},
		},
		{
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameOnlyHasNamespaceField",
				Namespace: "testNS",
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.hashkey":   "0",
				"metadata.name":      "testWitNameOnlyHasNamespaceField",
				"metadata.namespace": "testNS",
			},
		},
		{
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameHashKeyHasNamespaceField",
				Namespace: "testNS",
				HashKey:   10000,
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.hashkey":   "10000",
				"metadata.name":      "testWitNameHashKeyHasNamespaceField",
				"metadata.namespace": "testNS",
			},
		},
		{
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameHashKeyOwnerRefHasNamespaceField",
				Namespace: "testNS",
				HashKey:   10000,
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", HashKey: 10},
				},
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.hashkey":   "10000",
				"metadata.name":      "testWitNameHashKeyOwnerRefHasNamespaceField",
				"metadata.namespace": "testNS",
				"metadata.ownerReferences.hashkey.Deployment": "10",
			},
		},
		{
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameHashKeyOwnerRefsHasNamespaceField",
				Namespace: "testNS",
				HashKey:   10000,
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", HashKey: 10},
					{Kind: "Replicaset", HashKey: 20},
				},
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.hashkey":   "10000",
				"metadata.name":      "testWitNameHashKeyOwnerRefsHasNamespaceField",
				"metadata.namespace": "testNS",
				"metadata.ownerReferences.hashkey.Deployment": "10",
				"metadata.ownerReferences.hashkey.Replicaset": "20",
			},
		},
		{
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameHashKeyDupOwnerRefsHasNamespaceField",
				Namespace: "testNS",
				HashKey:   10000,
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", HashKey: 10},
					{Kind: "Deployment", HashKey: 20},
				},
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.hashkey":   "10000",
				"metadata.name":      "testWitNameHashKeyDupOwnerRefsHasNamespaceField",
				"metadata.namespace": "testNS",
				"metadata.ownerReferences.hashkey.Deployment": "20",
			},
		},
		{
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameHashKeyOwnerRefsHaNamespaceField",
				Namespace: "testNS",
				HashKey:   10000,
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", HashKey: 20},
				},
			},
			hasNamespaceField: false,
			expectedResult: fields.Set{
				"metadata.hashkey": "10000",
				"metadata.name":    "testWitNameHashKeyOwnerRefsHaNamespaceField",
				"metadata.ownerReferences.hashkey.Deployment": "20",
			},
		},
	}
	for _, test := range tests {
		result := ObjectMetaFieldsSet(test.objectMeta, test.hasNamespaceField)
		if !reflect.DeepEqual(result, test.expectedResult) {
			t.Errorf("The test failed. The expected result is : %v. The actual result is %v", result, test.expectedResult)
		}
	}
}

func TestAddObjectMetaFieldsSet(t *testing.T) {
	tests := []struct {
		source            fields.Set
		objectMeta        *metav1.ObjectMeta
		hasNamespaceField bool
		expectedResult    fields.Set
	}{
		{
			source: fields.Set{},
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameOnlyNoSourcefield",
				Namespace: "testNS",
			},
			hasNamespaceField: false,
			expectedResult: fields.Set{
				"metadata.hashkey": "0",
				"metadata.name":    "testWitNameOnlyNoSourcefield",
			},
		},
		{
			source: fields.Set{
				"metadata.test": "testfield",
			},
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameOnly",
				Namespace: "testNS",
			},
			hasNamespaceField: false,
			expectedResult: fields.Set{
				"metadata.test":    "testfield",
				"metadata.hashkey": "0",
				"metadata.name":    "testWitNameOnly",
			},
		},
		{
			source: fields.Set{
				"metadata.test": "testfield",
			},
			objectMeta: &metav1.ObjectMeta{
				Name: "testWitNameOnlyHasEmptyNamespaceField",
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.test":      "testfield",
				"metadata.hashkey":   "0",
				"metadata.name":      "testWitNameOnlyHasEmptyNamespaceField",
				"metadata.namespace": "",
			},
		},
		{
			source: fields.Set{
				"metadata.test": "testfield",
			},
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameOnlyHasNamespaceField",
				Namespace: "testNS",
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.test":      "testfield",
				"metadata.hashkey":   "0",
				"metadata.name":      "testWitNameOnlyHasNamespaceField",
				"metadata.namespace": "testNS",
			},
		},
		{
			source: fields.Set{
				"metadata.test": "testfield",
			},
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameHashKeyHasNamespaceField",
				Namespace: "testNS",
				HashKey:   10000,
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.test":      "testfield",
				"metadata.hashkey":   "10000",
				"metadata.name":      "testWitNameHashKeyHasNamespaceField",
				"metadata.namespace": "testNS",
			},
		},
		{
			source: fields.Set{
				"metadata.test": "testfield",
			},
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameHashKeyOwnerRefHasNamespaceField",
				Namespace: "testNS",
				HashKey:   10000,
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", HashKey: 10},
				},
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.test":      "testfield",
				"metadata.hashkey":   "10000",
				"metadata.name":      "testWitNameHashKeyOwnerRefHasNamespaceField",
				"metadata.namespace": "testNS",
				"metadata.ownerReferences.hashkey.Deployment": "10",
			},
		},
		{
			source: fields.Set{
				"metadata.test": "testfield",
			},
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameHashKeyOwnerRefsHasNamespaceField",
				Namespace: "testNS",
				HashKey:   10000,
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", HashKey: 10},
					{Kind: "Replicaset", HashKey: 20},
				},
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.test":      "testfield",
				"metadata.hashkey":   "10000",
				"metadata.name":      "testWitNameHashKeyOwnerRefsHasNamespaceField",
				"metadata.namespace": "testNS",
				"metadata.ownerReferences.hashkey.Deployment": "10",
				"metadata.ownerReferences.hashkey.Replicaset": "20",
			},
		},
		{
			source: fields.Set{
				"metadata.test": "testfield",
			},
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameHashKeyDupOwnerRefsHasNamespaceField",
				Namespace: "testNS",
				HashKey:   10000,
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", HashKey: 10},
					{Kind: "Deployment", HashKey: 20},
				},
			},
			hasNamespaceField: true,
			expectedResult: fields.Set{
				"metadata.test":      "testfield",
				"metadata.hashkey":   "10000",
				"metadata.name":      "testWitNameHashKeyDupOwnerRefsHasNamespaceField",
				"metadata.namespace": "testNS",
				"metadata.ownerReferences.hashkey.Deployment": "20",
			},
		},
		{
			source: fields.Set{
				"metadata.test": "testfield",
			},
			objectMeta: &metav1.ObjectMeta{
				Name:      "testWitNameHashKeyOwnerRefsHaNamespaceField",
				Namespace: "testNS",
				HashKey:   10000,
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", HashKey: 20},
				},
			},
			hasNamespaceField: false,
			expectedResult: fields.Set{
				"metadata.test":    "testfield",
				"metadata.hashkey": "10000",
				"metadata.name":    "testWitNameHashKeyOwnerRefsHaNamespaceField",
				"metadata.ownerReferences.hashkey.Deployment": "20",
			},
		},
	}
	for _, test := range tests {
		result := AddObjectMetaFieldsSet(test.source, test.objectMeta, test.hasNamespaceField)
		if !reflect.DeepEqual(result, test.expectedResult) {
			t.Errorf("The test failed. The expected result is : %v. The actual result is %v", result, test.expectedResult)
		}
	}
}

func TestMergeFieldsSets(t *testing.T) {
	var tests = []struct {
		name           string
		source         fields.Set
		fragment       fields.Set
		expectedResult fields.Set
	}{
		{
			name:           "Empty fields merge",
			source:         fields.Set{},
			fragment:       fields.Set{},
			expectedResult: fields.Set{},
		},
		{
			name: "Empty fragment fields merge",
			source: fields.Set{
				"metadata.test": "testfield",
			},
			fragment: fields.Set{},
			expectedResult: fields.Set{
				"metadata.test": "testfield",
			},
		},
		{
			name:   "Empty source fields merge",
			source: fields.Set{},
			fragment: fields.Set{
				"metadata.test": "testfield",
			},
			expectedResult: fields.Set{
				"metadata.test": "testfield",
			},
		},
		{
			name: "Simple merge",
			source: fields.Set{
				"metadata.source1": "sv1",
			},
			fragment: fields.Set{
				"metadata.fragment": "fv1",
			},
			expectedResult: fields.Set{
				"metadata.source1":  "sv1",
				"metadata.fragment": "fv1",
			},
		},
		{
			name: "Same field key",
			source: fields.Set{
				"metadata.source1": "sv1",
			},
			fragment: fields.Set{
				"metadata.source1": "fv1",
			},
			expectedResult: fields.Set{
				"metadata.source1": "fv1",
			},
		},
		{
			name: "One same key and one different key",
			source: fields.Set{
				"metadata.source1": "sv1",
			},
			fragment: fields.Set{
				"metadata.source1":   "fv1",
				"metadata.fragment1": "fv2",
			},
			expectedResult: fields.Set{
				"metadata.source1":   "fv1",
				"metadata.fragment1": "fv2",
			},
		},
	}
	for _, test := range tests {
		result := MergeFieldsSets(test.source, test.fragment)
		if !reflect.DeepEqual(result, test.expectedResult) {
			t.Errorf("The test %v failed. The expected result is : %v. The actual result is %v", test.name, result, test.expectedResult)
		}
	}
}
