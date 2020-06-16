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

package v1beta1

import (
	v1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// PodSecurityPolicyLister helps list PodSecurityPolicies.
type PodSecurityPolicyLister interface {
	// List lists all PodSecurityPolicies in the indexer.
	List(selector labels.Selector) (ret []*v1beta1.PodSecurityPolicy, err error)
	// PodSecurityPolicies returns an object that can list and get PodSecurityPolicies.
	PodSecurityPolicies() PodSecurityPolicyTenantLister
	PodSecurityPoliciesWithMultiTenancy(tenant string) PodSecurityPolicyTenantLister
	// Get retrieves the PodSecurityPolicy from the index for a given name.
	Get(name string) (*v1beta1.PodSecurityPolicy, error)
	PodSecurityPolicyListerExpansion
}

// podSecurityPolicyLister implements the PodSecurityPolicyLister interface.
type podSecurityPolicyLister struct {
	indexer cache.Indexer
}

// NewPodSecurityPolicyLister returns a new PodSecurityPolicyLister.
func NewPodSecurityPolicyLister(indexer cache.Indexer) PodSecurityPolicyLister {
	return &podSecurityPolicyLister{indexer: indexer}
}

// List lists all PodSecurityPolicies in the indexer.
func (s *podSecurityPolicyLister) List(selector labels.Selector) (ret []*v1beta1.PodSecurityPolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.PodSecurityPolicy))
	})
	return ret, err
}

// Get retrieves the PodSecurityPolicy from the index for a given name.
func (s *podSecurityPolicyLister) Get(name string) (*v1beta1.PodSecurityPolicy, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("podsecuritypolicy"), name)
	}
	return obj.(*v1beta1.PodSecurityPolicy), nil
}

// PodSecurityPolicies returns an object that can list and get PodSecurityPolicies.
func (s *podSecurityPolicyLister) PodSecurityPolicies() PodSecurityPolicyTenantLister {
	return podSecurityPolicyTenantLister{indexer: s.indexer, tenant: "system"}
}

func (s *podSecurityPolicyLister) PodSecurityPoliciesWithMultiTenancy(tenant string) PodSecurityPolicyTenantLister {
	return podSecurityPolicyTenantLister{indexer: s.indexer, tenant: tenant}
}

// PodSecurityPolicyTenantLister helps list and get PodSecurityPolicies.
type PodSecurityPolicyTenantLister interface {
	// List lists all PodSecurityPolicies in the indexer for a given tenant/tenant.
	List(selector labels.Selector) (ret []*v1beta1.PodSecurityPolicy, err error)
	// Get retrieves the PodSecurityPolicy from the indexer for a given tenant/tenant and name.
	Get(name string) (*v1beta1.PodSecurityPolicy, error)
	PodSecurityPolicyTenantListerExpansion
}

// podSecurityPolicyTenantLister implements the PodSecurityPolicyTenantLister
// interface.
type podSecurityPolicyTenantLister struct {
	indexer cache.Indexer
	tenant  string
}

// List lists all PodSecurityPolicies in the indexer for a given tenant.
func (s podSecurityPolicyTenantLister) List(selector labels.Selector) (ret []*v1beta1.PodSecurityPolicy, err error) {
	err = cache.ListAllByTenant(s.indexer, s.tenant, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.PodSecurityPolicy))
	})
	return ret, err
}

// Get retrieves the PodSecurityPolicy from the indexer for a given tenant and name.
func (s podSecurityPolicyTenantLister) Get(name string) (*v1beta1.PodSecurityPolicy, error) {
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
		return nil, errors.NewNotFound(v1beta1.Resource("podsecuritypolicy"), name)
	}
	return obj.(*v1beta1.PodSecurityPolicy), nil
}
