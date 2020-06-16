/*
Copyright The Kubernetes Authors.
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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "k8s.io/api/rbac/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ClusterRoleBindingLister helps list ClusterRoleBindings.
type ClusterRoleBindingLister interface {
	// List lists all ClusterRoleBindings in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.ClusterRoleBinding, err error)
	// ClusterRoleBindings returns an object that can list and get ClusterRoleBindings.
	ClusterRoleBindings() ClusterRoleBindingTenantLister
	ClusterRoleBindingsWithMultiTenancy(tenant string) ClusterRoleBindingTenantLister
	// Get retrieves the ClusterRoleBinding from the index for a given name.
	Get(name string) (*v1alpha1.ClusterRoleBinding, error)
	ClusterRoleBindingListerExpansion
}

// clusterRoleBindingLister implements the ClusterRoleBindingLister interface.
type clusterRoleBindingLister struct {
	indexer cache.Indexer
}

// NewClusterRoleBindingLister returns a new ClusterRoleBindingLister.
func NewClusterRoleBindingLister(indexer cache.Indexer) ClusterRoleBindingLister {
	return &clusterRoleBindingLister{indexer: indexer}
}

// List lists all ClusterRoleBindings in the indexer.
func (s *clusterRoleBindingLister) List(selector labels.Selector) (ret []*v1alpha1.ClusterRoleBinding, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ClusterRoleBinding))
	})
	return ret, err
}

// Get retrieves the ClusterRoleBinding from the index for a given name.
func (s *clusterRoleBindingLister) Get(name string) (*v1alpha1.ClusterRoleBinding, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("clusterrolebinding"), name)
	}
	return obj.(*v1alpha1.ClusterRoleBinding), nil
}

// ClusterRoleBindings returns an object that can list and get ClusterRoleBindings.
func (s *clusterRoleBindingLister) ClusterRoleBindings() ClusterRoleBindingTenantLister {
	return clusterRoleBindingTenantLister{indexer: s.indexer, tenant: "system"}
}

func (s *clusterRoleBindingLister) ClusterRoleBindingsWithMultiTenancy(tenant string) ClusterRoleBindingTenantLister {
	return clusterRoleBindingTenantLister{indexer: s.indexer, tenant: tenant}
}

// ClusterRoleBindingTenantLister helps list and get ClusterRoleBindings.
type ClusterRoleBindingTenantLister interface {
	// List lists all ClusterRoleBindings in the indexer for a given tenant/tenant.
	List(selector labels.Selector) (ret []*v1alpha1.ClusterRoleBinding, err error)
	// Get retrieves the ClusterRoleBinding from the indexer for a given tenant/tenant and name.
	Get(name string) (*v1alpha1.ClusterRoleBinding, error)
	ClusterRoleBindingTenantListerExpansion
}

// clusterRoleBindingTenantLister implements the ClusterRoleBindingTenantLister
// interface.
type clusterRoleBindingTenantLister struct {
	indexer cache.Indexer
	tenant  string
}

// List lists all ClusterRoleBindings in the indexer for a given tenant.
func (s clusterRoleBindingTenantLister) List(selector labels.Selector) (ret []*v1alpha1.ClusterRoleBinding, err error) {
	err = cache.ListAllByTenant(s.indexer, s.tenant, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ClusterRoleBinding))
	})
	return ret, err
}

// Get retrieves the ClusterRoleBinding from the indexer for a given tenant and name.
func (s clusterRoleBindingTenantLister) Get(name string) (*v1alpha1.ClusterRoleBinding, error) {
	key := s.tenant + "/" + name
	// The backward-compatible informer may have the tenant set as "system" or "all",
	// Yet when it comes to get an object, the tenant can only be "system", where the key is {name}
	if s.tenant == "system" || s.tenant == "all" {
		key = name
	}
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("clusterrolebinding"), name)
	}
	return obj.(*v1alpha1.ClusterRoleBinding), nil
}
