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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	v1beta1 "k8s.io/sample-apiserver/pkg/apis/wardle/v1beta1"
)

// FlunderLister helps list Flunders.
type FlunderLister interface {
	// List lists all Flunders in the indexer.
	List(selector labels.Selector) (ret []*v1beta1.Flunder, err error)
	// Flunders returns an object that can list and get Flunders.
	Flunders(namespace string) FlunderNamespaceLister
	FlundersWithMultiTenancy(namespace string, tenant string) FlunderNamespaceLister
	FlunderListerExpansion
}

// flunderLister implements the FlunderLister interface.
type flunderLister struct {
	indexer cache.Indexer
}

// NewFlunderLister returns a new FlunderLister.
func NewFlunderLister(indexer cache.Indexer) FlunderLister {
	return &flunderLister{indexer: indexer}
}

// List lists all Flunders in the indexer.
func (s *flunderLister) List(selector labels.Selector) (ret []*v1beta1.Flunder, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Flunder))
	})
	return ret, err
}

// Flunders returns an object that can list and get Flunders.
func (s *flunderLister) Flunders(namespace string) FlunderNamespaceLister {
	return flunderNamespaceLister{indexer: s.indexer, namespace: namespace, tenant: ""}
}

func (s *flunderLister) FlundersWithMultiTenancy(namespace string, tenant string) FlunderNamespaceLister {
	return flunderNamespaceLister{indexer: s.indexer, namespace: namespace, tenant: tenant}
}

// FlunderNamespaceLister helps list and get Flunders.
type FlunderNamespaceLister interface {
	// List lists all Flunders in the indexer for a given tenant/namespace.
	List(selector labels.Selector) (ret []*v1beta1.Flunder, err error)
	// Get retrieves the Flunder from the indexer for a given tenant/namespace and name.
	Get(name string) (*v1beta1.Flunder, error)
	FlunderNamespaceListerExpansion
}

// flunderNamespaceLister implements the FlunderNamespaceLister
// interface.
type flunderNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
	tenant    string
}

// List lists all Flunders in the indexer for a given namespace.
func (s flunderNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.Flunder, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.tenant, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Flunder))
	})
	return ret, err
}

// Get retrieves the Flunder from the indexer for a given namespace and name.
func (s flunderNamespaceLister) Get(name string) (*v1beta1.Flunder, error) {
	key := s.tenant + "/" + s.namespace + "/" + name
	if s.tenant == "system" {
		key = s.namespace + "/" + name
	}
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("flunder"), name)
	}
	return obj.(*v1beta1.Flunder), nil
}
