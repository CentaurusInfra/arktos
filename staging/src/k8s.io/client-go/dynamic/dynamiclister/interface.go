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

package dynamiclister

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// Lister helps list resources.
type Lister interface {
	// List lists all resources in the indexer.
	List(selector labels.Selector) (ret []*unstructured.Unstructured, err error)
	// Get retrieves a resource from the indexer with the given name
	Get(name string) (*unstructured.Unstructured, error)
	// Tenant returns an object that can list and get resources in a given tenant.
	Tenant(tenant string) TenantLister
	// Namespace returns an object that can list and get resources in a given namespace.
	Namespace(namespace string) NamespaceLister
	NamespaceWithMultiTenancy(namespace string, tenant string) NamespaceLister
}

// TenantLister helps list and get resources.
type TenantLister interface {
	// List lists all resources in the indexer for a given tenant.
	List(selector labels.Selector) (ret []*unstructured.Unstructured, err error)
	// Get retrieves a resource from the indexer for a given tenant and name.
	Get(name string) (*unstructured.Unstructured, error)
}

// NamespaceLister helps list and get resources.
type NamespaceLister interface {
	// List lists all resources in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*unstructured.Unstructured, err error)
	// Get retrieves a resource from the indexer for a given namespace and name.
	Get(name string) (*unstructured.Unstructured, error)
}
