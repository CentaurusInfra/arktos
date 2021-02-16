/*
Copyright 2018 The Kubernetes Authors.
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

package etcd

import (
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
)

// GetEtcdStorageData returns etcd data for all persisted objects.
// It is exported so that it can be reused across multiple tests.
// It returns a new map on every invocation to prevent different tests from mutating shared state.
func GetEtcdStorageData() map[schema.GroupVersionResource]StorageData {
	return GetEtcdStorageDataForNamespace("etcdstoragepathtestnamespace")
}

// GetEtcdStorageDataForNamespace returns etcd data for all persisted objects.
// It is exported so that it can be reused across multiple tests.
// It returns a new map on every invocation to prevent different tests from mutating shared state.
// Namespaced objects keys are computed for the specified namespace.
func GetEtcdStorageDataForNamespace(namespace string) map[schema.GroupVersionResource]StorageData {
	return GetEtcdStorageDataForNamespaceWithMultiTenancy(metav1.TenantSystem, namespace)
}

// StorageData contains information required to create an object and verify its storage in etcd
// It must be paired with a specific resource
type StorageData struct {
	Stub             string                   // Valid JSON stub to use during create
	Prerequisites    []Prerequisite           // Optional, ordered list of JSON objects to create before stub
	ExpectedEtcdPath string                   // Expected location of object in etcd, do not use any variables, constants, etc to derive this value - always supply the full raw string
	ExpectedGVK      *schema.GroupVersionKind // The GVK that we expect this object to be stored as - leave this nil to use the default
}

// Prerequisite contains information required to create a resource (but not verify it)
type Prerequisite struct {
	GvrData schema.GroupVersionResource
	Stub    string
}

// GetCustomResourceDefinitionData returns the resource definitions that back the custom resources
// included in GetEtcdStorageData.  They should be created using CreateTestCRDs before running any tests.
func GetCustomResourceDefinitionData() []*apiextensionsv1beta1.CustomResourceDefinition {
	return []*apiextensionsv1beta1.CustomResourceDefinition{
		// namespaced with legacy version field
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "foos.cr.bar.com",
				Tenant: metav1.TenantSystem,
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   "cr.bar.com",
				Version: "v1",
				Scope:   apiextensionsv1beta1.NamespaceScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Plural: "foos",
					Kind:   "Foo",
				},
			},
		},
		// Tenant-Scoped with legacy version field
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "moons.dreamwalk.com",
				Tenant: metav1.TenantSystem,
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   "dreamwalk.com",
				Version: "v1",
				Scope:   apiextensionsv1beta1.TenantScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Plural: "moons",
					Kind:   "moon",
				},
			},
		},
		// cluster scoped with legacy version field
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pants.custom.fancy.com",
				Tenant: metav1.TenantSystem,
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   "custom.fancy.com",
				Version: "v2",
				Scope:   apiextensionsv1beta1.ClusterScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Plural: "pants",
					Kind:   "Pant",
				},
			},
		},
		// cluster scoped with legacy version field and pruning.
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "integers.random.numbers.com",
				Tenant: metav1.TenantSystem,
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   "random.numbers.com",
				Version: "v1",
				Scope:   apiextensionsv1beta1.ClusterScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Plural: "integers",
					Kind:   "Integer",
				},
				Validation: &apiextensionsv1beta1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1beta1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]apiextensionsv1beta1.JSONSchemaProps{
							"value": {
								Type: "number",
							},
						},
					},
				},
				PreserveUnknownFields: pointer.BoolPtr(false),
			},
		},
		// cluster scoped with versions field
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pandas.awesome.bears.com",
				Tenant: metav1.TenantSystem,
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group: "awesome.bears.com",
				Versions: []apiextensionsv1beta1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
					},
					{
						Name:    "v2",
						Served:  false,
						Storage: false,
					},
					{
						Name:    "v3",
						Served:  true,
						Storage: false,
					},
				},
				Scope: apiextensionsv1beta1.ClusterScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Plural: "pandas",
					Kind:   "Panda",
				},
				Subresources: &apiextensionsv1beta1.CustomResourceSubresources{
					Status: &apiextensionsv1beta1.CustomResourceSubresourceStatus{},
					Scale: &apiextensionsv1beta1.CustomResourceSubresourceScale{
						SpecReplicasPath:   ".spec.replicas",
						StatusReplicasPath: ".status.replicas",
						LabelSelectorPath:  func() *string { path := ".status.selector"; return &path }(),
					},
				},
			},
		},
	}
}

func gvr(g, v, r string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: g, Version: v, Resource: r}
}

func gvkP(g, v, k string) *schema.GroupVersionKind {
	return &schema.GroupVersionKind{Group: g, Version: v, Kind: k}
}
