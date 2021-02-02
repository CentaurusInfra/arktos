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

package metadatalister

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/tools/cache"
)

func TestNamespaceGetMethodWithMultiTenacy(t *testing.T) {
	tests := []struct {
		name            string
		existingObjects []runtime.Object
		namespaceToSync string
		tenantToSync    string
		gvrToSync       schema.GroupVersionResource
		objectToGet     string
		expectedObject  *metav1.PartialObjectMetadata
		expectError     bool
	}{
		{
			name: "scenario 1: gets name-foo1 resource from the indexer from te-foo tenant, ns-foo namespace",
			existingObjects: []runtime.Object{
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo"),
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo1"),
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-bar", "ns-bar", "name-bar"),
			},
			namespaceToSync: "ns-foo",
			tenantToSync:    "te-foo",
			gvrToSync:       schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"},
			objectToGet:     "name-foo1",
			expectedObject:  newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo1"),
		},
		{
			name: "scenario 2: gets name-foo-non-existing resource from the indexer from te-foo tenant, ns-foo namespace",
			existingObjects: []runtime.Object{
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo"),
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo1"),
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-bar", "ns-bar", "name-bar"),
			},
			namespaceToSync: "ns-foo",
			tenantToSync:    "te-foo",
			gvrToSync:       schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"},
			objectToGet:     "name-foo-non-existing",
			expectError:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// test data
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			for _, obj := range test.existingObjects {
				err := indexer.Add(obj)
				if err != nil {
					t.Fatal(err)
				}
			}
			// act
			target := New(indexer, test.gvrToSync).NamespaceWithMultiTenancy(test.namespaceToSync, test.tenantToSync)
			actualObject, err := target.Get(test.objectToGet)

			// validate
			if test.expectError {
				if err == nil {
					t.Fatal("expected to get an error but non was returned")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(test.expectedObject, actualObject) {
				t.Fatalf("unexpected object has been returned expected = %v actual = %v, diff = %v", test.expectedObject, actualObject, diff.ObjectDiff(test.expectedObject, actualObject))
			}
		})
	}
}

func TestNamespaceListMethodWithMultiTenacy(t *testing.T) {
	// test data
	objs := []runtime.Object{
		newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo"),
		newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo1"),
		newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-bar", "ns-bar", "name-bar"),
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, obj := range objs {
		err := indexer.Add(obj)
		if err != nil {
			t.Fatal(err)
		}
	}
	expectedOutput := []*metav1.PartialObjectMetadata{
		newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo"),
		newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo1"),
	}
	namespaceToList := "ns-foo"
	tenantToList := "te-foo"

	// act
	target := New(indexer, schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"}).NamespaceWithMultiTenancy(namespaceToList, tenantToList)
	actualOutput, err := target.List(labels.Everything())

	// validate
	if err != nil {
		t.Fatal(err)
	}
	assertListOrDie(expectedOutput, actualOutput, t)
}

func TestListerGetMethodWithMultiTenancy(t *testing.T) {
	tests := []struct {
		name            string
		existingObjects []runtime.Object
		gvrToSync       schema.GroupVersionResource
		objectToGet     string
		expectedObject  *metav1.PartialObjectMetadata
		expectError     bool
	}{
		{
			name: "scenario 1: gets name-foo1 resource from the indexer",
			existingObjects: []runtime.Object{
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo"),
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "", "", "name-foo1"),
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-bar", "ns-bar", "name-bar"),
			},
			gvrToSync:      schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"},
			objectToGet:    "name-foo1",
			expectedObject: newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "", "", "name-foo1"),
		},
		{
			name: "scenario 2: doesn't get name-foo resource from the indexer from ns-foo namespace",
			existingObjects: []runtime.Object{
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo"),
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo1"),
				newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-bar", "ns-bar", "name-bar"),
			},
			gvrToSync:   schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"},
			objectToGet: "name-foo",
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// test data
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			for _, obj := range test.existingObjects {
				err := indexer.Add(obj)
				if err != nil {
					t.Fatal(err)
				}
			}
			// act
			target := New(indexer, test.gvrToSync)
			actualObject, err := target.Get(test.objectToGet)

			// validate
			if test.expectError {
				if err == nil {
					t.Fatal("expected to get an error but non was returned")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(test.expectedObject, actualObject) {
				t.Fatalf("unexpected object has been returned expected = %v actual = %v, diff = %v", test.expectedObject, actualObject, diff.ObjectDiff(test.expectedObject, actualObject))
			}
		})
	}
}

func TestListerListMethodWithMultiTenancy(t *testing.T) {
	// test data
	objs := []runtime.Object{
		newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo"),
		newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo1", "ns-foo", "name-bar"),
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, obj := range objs {
		err := indexer.Add(obj)
		if err != nil {
			t.Fatal(err)
		}
	}
	expectedOutput := []*metav1.PartialObjectMetadata{
		newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo", "ns-foo", "name-foo"),
		newPartialObjectMetadataWithMultiTenancy("group/version", "TheKind", "te-foo1", "ns-foo", "name-bar"),
	}

	// act
	target := New(indexer, schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"})
	actualOutput, err := target.List(labels.Everything())

	// validate
	if err != nil {
		t.Fatal(err)
	}
	assertListOrDie(expectedOutput, actualOutput, t)
}

func newPartialObjectMetadataWithMultiTenancy(apiVersion, kind, tenant, namespace, name string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Tenant:    tenant,
			Namespace: namespace,
			Name:      name,
		},
	}
}
