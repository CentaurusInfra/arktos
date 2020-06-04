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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Lister = &dynamicLister{}
var _ NamespaceLister = &dynamicNamespaceLister{}

// dynamicLister implements the Lister interface.
type dynamicLister struct {
	indexer cache.Indexer
	gvr     schema.GroupVersionResource
}

// New returns a new Lister.
func New(indexer cache.Indexer, gvr schema.GroupVersionResource) Lister {
	return &dynamicLister{indexer: indexer, gvr: gvr}
}

// List lists all resources in the indexer.
func (l *dynamicLister) List(selector labels.Selector) (ret []*unstructured.Unstructured, err error) {
	err = cache.ListAll(l.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*unstructured.Unstructured))
	})
	return ret, err
}

// Get retrieves a resource from the indexer with the given name
func (l *dynamicLister) Get(name string) (*unstructured.Unstructured, error) {
	obj, exists, err := l.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(l.gvr.GroupResource(), name)
	}
	return obj.(*unstructured.Unstructured), nil
}

// Tenant returns an object that can list and get resources from a given tenant.
func (l *dynamicLister) Tenant(tenant string) TenantLister {
	return &dynamicTenantLister{indexer: l.indexer, tenant: tenant, gvr: l.gvr}
}

// dynamicTenantLister implements the TenantLister interface.
type dynamicTenantLister struct {
	indexer cache.Indexer
	tenant  string
	gvr     schema.GroupVersionResource
}

// List lists all resources in the indexer for a given tenant.
func (l *dynamicTenantLister) List(selector labels.Selector) (ret []*unstructured.Unstructured, err error) {
	err = cache.ListAllByTenant(l.indexer, l.tenant, selector, func(m interface{}) {
		ret = append(ret, m.(*unstructured.Unstructured))
	})
	return ret, err
}

// Get retrieves a resource from the indexer for a given tenant and name.
func (l *dynamicTenantLister) Get(name string) (*unstructured.Unstructured, error) {
	key := l.tenant + "/" + name
	if l.tenant == metav1.TenantSystem {
		key = name
	}
	obj, exists, err := l.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(l.gvr.GroupResource(), name)
	}
	return obj.(*unstructured.Unstructured), nil
}

// Namespace returns an object that can list and get resources from a given namespace.
func (l *dynamicLister) Namespace(namespace string) NamespaceLister {
	return &dynamicNamespaceLister{indexer: l.indexer, tenant: metav1.TenantSystem, namespace: namespace, gvr: l.gvr}
}

func (l *dynamicLister) NamespaceWithMultiTenancy(namespace string, tenant string) NamespaceLister {
	return &dynamicNamespaceLister{indexer: l.indexer, tenant: tenant, namespace: namespace, gvr: l.gvr}
}

// dynamicNamespaceLister implements the NamespaceLister interface.
type dynamicNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
	tenant    string
	gvr       schema.GroupVersionResource
}

// List lists all resources in the indexer for a given namespace.
func (l *dynamicNamespaceLister) List(selector labels.Selector) (ret []*unstructured.Unstructured, err error) {
	err = cache.ListAllByNamespace(l.indexer, l.tenant, l.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*unstructured.Unstructured))
	})
	return ret, err
}

// Get retrieves a resource from the indexer for a given namespace and name.
func (l *dynamicNamespaceLister) Get(name string) (*unstructured.Unstructured, error) {
	key := l.tenant + "/" + l.namespace + "/" + name
	if l.tenant == metav1.TenantSystem {
		key = l.namespace + "/" + name
	}
	obj, exists, err := l.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(l.gvr.GroupResource(), name)
	}
	return obj.(*unstructured.Unstructured), nil
}
