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
	v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// NetworkPolicyLister helps list NetworkPolicies.
type NetworkPolicyLister interface {
	// List lists all NetworkPolicies in the indexer.
	List(selector labels.Selector) (ret []*v1beta1.NetworkPolicy, err error)
	// NetworkPolicies returns an object that can list and get NetworkPolicies.
	NetworkPolicies(namespace string) NetworkPolicyNamespaceLister
	NetworkPoliciesWithMultiTenancy(namespace string, tenant string) NetworkPolicyNamespaceLister
	NetworkPolicyListerExpansion
}

// networkPolicyLister implements the NetworkPolicyLister interface.
type networkPolicyLister struct {
	indexer cache.Indexer
}

// NewNetworkPolicyLister returns a new NetworkPolicyLister.
func NewNetworkPolicyLister(indexer cache.Indexer) NetworkPolicyLister {
	return &networkPolicyLister{indexer: indexer}
}

// List lists all NetworkPolicies in the indexer.
func (s *networkPolicyLister) List(selector labels.Selector) (ret []*v1beta1.NetworkPolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.NetworkPolicy))
	})
	return ret, err
}

// NetworkPolicies returns an object that can list and get NetworkPolicies.
func (s *networkPolicyLister) NetworkPolicies(namespace string) NetworkPolicyNamespaceLister {
	return networkPolicyNamespaceLister{indexer: s.indexer, namespace: namespace, tenant: "system"}
}

func (s *networkPolicyLister) NetworkPoliciesWithMultiTenancy(namespace string, tenant string) NetworkPolicyNamespaceLister {
	return networkPolicyNamespaceLister{indexer: s.indexer, namespace: namespace, tenant: tenant}
}

// NetworkPolicyNamespaceLister helps list and get NetworkPolicies.
type NetworkPolicyNamespaceLister interface {
	// List lists all NetworkPolicies in the indexer for a given tenant/namespace.
	List(selector labels.Selector) (ret []*v1beta1.NetworkPolicy, err error)
	// Get retrieves the NetworkPolicy from the indexer for a given tenant/namespace and name.
	Get(name string) (*v1beta1.NetworkPolicy, error)
	NetworkPolicyNamespaceListerExpansion
}

// networkPolicyNamespaceLister implements the NetworkPolicyNamespaceLister
// interface.
type networkPolicyNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
	tenant    string
}

// List lists all NetworkPolicies in the indexer for a given namespace.
func (s networkPolicyNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.NetworkPolicy, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.tenant, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.NetworkPolicy))
	})
	return ret, err
}

// Get retrieves the NetworkPolicy from the indexer for a given namespace and name.
func (s networkPolicyNamespaceLister) Get(name string) (*v1beta1.NetworkPolicy, error) {
	key := s.tenant + "/" + s.namespace + "/" + name
	// The backward-compatible informer may have the tenant set as "system" or "all",
	// Yet when it comes to get an object, the tenant can only be "system", where the key is the {namespace}/{name}
	if s.tenant == "system" || s.tenant == "all" {
		key = s.namespace + "/" + name
	}
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("networkpolicy"), name)
	}
	return obj.(*v1beta1.NetworkPolicy), nil
}
