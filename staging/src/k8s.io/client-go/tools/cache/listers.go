/*
Copyright 2014 The Kubernetes Authors.
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

package cache

import (
	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AppendFunc is used to add a matching item to whatever list the caller is using
type AppendFunc func(interface{})

func ListAll(store Store, selector labels.Selector, appendFn AppendFunc) error {
	selectAll := selector.Empty()
	for _, m := range store.List() {
		if selectAll {
			// Avoid computing labels of the objects to speed up common flows
			// of listing all objects.
			appendFn(m)
			continue
		}
		metadata, err := meta.Accessor(m)
		if err != nil {
			return err
		}
		if selector.Matches(labels.Set(metadata.GetLabels())) {
			appendFn(m)
		}
	}
	return nil
}

func ListAllByTenant(indexer Indexer, tenant string, selector labels.Selector, appendFn AppendFunc) error {
	selectAll := selector.Empty()
	if tenant == metav1.TenantAll {
		return ListAll(indexer, selector, appendFn)
	}

	items, err := indexer.Index(TenantIndex, &metav1.ObjectMeta{Tenant: tenant})
	if err != nil {
		// Ignore error; do slow search without index.
		klog.Warningf("can not retrieve list of objects using index : %v", err)
		for _, m := range indexer.List() {
			metadata, err := meta.Accessor(m)
			if err != nil {
				return err
			}
			if (metadata.GetTenant() == tenant || tenant == metav1.TenantAll) && selector.Matches(labels.Set(metadata.GetLabels())) {
				appendFn(m)
			}

		}
		return nil
	}
	for _, m := range items {
		if selectAll {
			// Avoid computing labels of the objects to speed up common flows
			// of listing all objects.
			appendFn(m)
			continue
		}
		metadata, err := meta.Accessor(m)
		if err != nil {
			return err
		}
		if selector.Matches(labels.Set(metadata.GetLabels())) {
			appendFn(m)
		}
	}

	return nil
}

func ListAllByNamespace(indexer Indexer, tenant string, namespace string, selector labels.Selector, appendFn AppendFunc) error {
	selectAll := selector.Empty()

	if tenant == metav1.TenantAll && namespace == metav1.NamespaceAll {
		return ListAll(indexer, selector, appendFn)
	}

	items, err := indexer.Index(NamespaceIndex, &metav1.ObjectMeta{Namespace: namespace, Tenant: tenant})
	if err != nil {
		// Ignore error; do slow search without index.
		klog.Warningf("can not retrieve list of objects using index : %v", err)
		for _, m := range indexer.List() {
			metadata, err := meta.Accessor(m)
			if err != nil {
				return err
			}
			if (metadata.GetNamespace() == namespace || namespace == metav1.NamespaceAll) && (metadata.GetTenant() == tenant || tenant == metav1.TenantAll) && selector.Matches(labels.Set(metadata.GetLabels())) {
				appendFn(m)
			}

		}
		return nil
	}
	for _, m := range items {
		if selectAll {
			// Avoid computing labels of the objects to speed up common flows
			// of listing all objects.
			appendFn(m)
			continue
		}
		metadata, err := meta.Accessor(m)
		if err != nil {
			return err
		}
		if selector.Matches(labels.Set(metadata.GetLabels())) {
			appendFn(m)
		}
	}

	return nil
}

// GenericLister is a lister skin on a generic Indexer
type GenericLister interface {
	// List will return all objects across namespaces
	List(selector labels.Selector) (ret []runtime.Object, err error)
	// Get will attempt to retrieve assuming that name==key
	Get(name string) (runtime.Object, error)
	// ByTenant will give you a GenericTenantLister for one tenant
	ByTenant(tenant string) GenericTenantLister
	// ByNamespace will give you a GenericNamespaceLister for one namespace
	ByNamespace(namespace string) GenericNamespaceLister
	ByNamespaceWithMultiTenancy(namespace string, tenant string) GenericNamespaceLister
}

// GenericTenantLister is a lister skin on a generic Indexer
type GenericTenantLister interface {
	// List will return all objects in this tenant
	List(selector labels.Selector) (ret []runtime.Object, err error)
	// Get will attempt to retrieve by tenant and name
	Get(name string) (runtime.Object, error)
}

// GenericNamespaceLister is a lister skin on a generic Indexer
type GenericNamespaceLister interface {
	// List will return all objects in this namespace
	List(selector labels.Selector) (ret []runtime.Object, err error)
	// Get will attempt to retrieve by namespace and name
	Get(name string) (runtime.Object, error)
}

func NewGenericLister(indexer Indexer, resource schema.GroupResource) GenericLister {
	return &genericLister{indexer: indexer, resource: resource}
}

type genericLister struct {
	indexer  Indexer
	resource schema.GroupResource
}

func (s *genericLister) List(selector labels.Selector) (ret []runtime.Object, err error) {
	err = ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(runtime.Object))
	})
	return ret, err
}

func (s *genericLister) ByNamespace(namespace string) GenericNamespaceLister {
	return &genericNamespaceLister{indexer: s.indexer, tenant: metav1.TenantSystem, namespace: namespace, resource: s.resource}
}

func (s *genericLister) ByNamespaceWithMultiTenancy(namespace string, tenant string) GenericNamespaceLister {
	return &genericNamespaceLister{indexer: s.indexer, tenant: tenant, namespace: namespace, resource: s.resource}
}

func (s *genericLister) ByTenant(tenant string) GenericTenantLister {
	return &genericTenantLister{indexer: s.indexer, tenant: tenant, resource: s.resource}
}

func (s *genericLister) Get(name string) (runtime.Object, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(s.resource, name)
	}
	return obj.(runtime.Object), nil
}

type genericTenantLister struct {
	indexer  Indexer
	tenant   string
	resource schema.GroupResource
}

func (s *genericTenantLister) List(selector labels.Selector) (ret []runtime.Object, err error) {
	err = ListAllByTenant(s.indexer, s.tenant, selector, func(m interface{}) {
		ret = append(ret, m.(runtime.Object))
	})
	return ret, err
}

func (s *genericTenantLister) Get(name string) (runtime.Object, error) {
	key := s.tenant + "/" + name
	if s.tenant == metav1.TenantSystem {
		key = name
	}
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(s.resource, name)
	}
	return obj.(runtime.Object), nil
}

type genericNamespaceLister struct {
	indexer   Indexer
	namespace string
	tenant    string
	resource  schema.GroupResource
}

func (s *genericNamespaceLister) List(selector labels.Selector) (ret []runtime.Object, err error) {
	err = ListAllByNamespace(s.indexer, s.tenant, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(runtime.Object))
	})
	return ret, err
}

func (s *genericNamespaceLister) Get(name string) (runtime.Object, error) {
	key := s.tenant + "/" + s.namespace + "/" + name
	if s.tenant == metav1.TenantSystem {
		key = s.namespace + "/" + name
	}
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(s.resource, name)
	}
	return obj.(runtime.Object), nil
}
