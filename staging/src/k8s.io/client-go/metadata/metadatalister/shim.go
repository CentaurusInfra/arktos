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

package metadatalister

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

var _ cache.GenericLister = &metadataListerShim{}
var _ cache.GenericNamespaceLister = &metadataNamespaceListerShim{}

// metadataListerShim implements the cache.GenericLister interface.
type metadataListerShim struct {
	lister Lister
}

// NewRuntimeObjectShim returns a new shim for Lister.
// It wraps Lister so that it implements cache.GenericLister interface
func NewRuntimeObjectShim(lister Lister) cache.GenericLister {
	return &metadataListerShim{lister: lister}
}

// List will return all objects across namespaces
func (s *metadataListerShim) List(selector labels.Selector) (ret []runtime.Object, err error) {
	objs, err := s.lister.List(selector)
	if err != nil {
		return nil, err
	}

	ret = make([]runtime.Object, len(objs))
	for index, obj := range objs {
		ret[index] = obj
	}
	return ret, err
}

// Get will attempt to retrieve assuming that name==key
func (s *metadataListerShim) Get(name string) (runtime.Object, error) {
	return s.lister.Get(name)
}

func (s *metadataListerShim) ByTenant(tenant string) cache.GenericTenantLister {
	return &metadataTenantListerShim{
		tenantLister: s.lister.Tenant(tenant),
	}
}
func (s *metadataListerShim) ByNamespace(namespace string) cache.GenericNamespaceLister {
	return s.ByNamespaceWithMultiTenancy(namespace, metav1.TenantSystem)
}

func (s *metadataListerShim) ByNamespaceWithMultiTenancy(namespace string, tenant string) cache.GenericNamespaceLister {
	return &metadataNamespaceListerShim{
		namespaceLister: s.lister.NamespaceWithMultiTenancy(namespace, tenant),
	}
}

// metadataTenantListerShim implements the TenantLister interface.
// It wraps TenantLister so that it implements cache.GenericTenantLister interface
type metadataTenantListerShim struct {
	tenantLister TenantLister
}

// List will return all objects in this tenant
func (ns *metadataTenantListerShim) List(selector labels.Selector) (ret []runtime.Object, err error) {
	objs, err := ns.tenantLister.List(selector)
	if err != nil {
		return nil, err
	}

	ret = make([]runtime.Object, len(objs))
	for index, obj := range objs {
		ret[index] = obj
	}
	return ret, err
}

// Get will attempt to retrieve by tenant and name
func (ns *metadataTenantListerShim) Get(name string) (runtime.Object, error) {
	return ns.tenantLister.Get(name)
}

// metadataNamespaceListerShim implements the NamespaceLister interface.
// It wraps NamespaceLister so that it implements cache.GenericNamespaceLister interface
type metadataNamespaceListerShim struct {
	namespaceLister NamespaceLister
}

// List will return all objects in this namespace
func (ns *metadataNamespaceListerShim) List(selector labels.Selector) (ret []runtime.Object, err error) {
	objs, err := ns.namespaceLister.List(selector)
	if err != nil {
		return nil, err
	}

	ret = make([]runtime.Object, len(objs))
	for index, obj := range objs {
		ret[index] = obj
	}
	return ret, err
}

// Get will attempt to retrieve by namespace and name
func (ns *metadataNamespaceListerShim) Get(name string) (runtime.Object, error) {
	return ns.namespaceLister.Get(name)
}
