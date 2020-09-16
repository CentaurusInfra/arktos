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

package v1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ConfigMapLister helps list ConfigMaps.
type ConfigMapLister interface {
	// List lists all ConfigMaps in the indexer.
	List(selector labels.Selector) (ret []*v1.ConfigMap, err error)
	// ConfigMaps returns an object that can list and get ConfigMaps.
	ConfigMaps(namespace string) ConfigMapNamespaceLister
	ConfigMapsWithMultiTenancy(namespace string, tenant string) ConfigMapNamespaceLister
	ConfigMapListerExpansion
}

// configMapLister implements the ConfigMapLister interface.
type configMapLister struct {
	indexer cache.Indexer
}

// NewConfigMapLister returns a new ConfigMapLister.
func NewConfigMapLister(indexer cache.Indexer) ConfigMapLister {
	return &configMapLister{indexer: indexer}
}

// List lists all ConfigMaps in the indexer.
func (s *configMapLister) List(selector labels.Selector) (ret []*v1.ConfigMap, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.ConfigMap))
	})
	return ret, err
}

// ConfigMaps returns an object that can list and get ConfigMaps.
func (s *configMapLister) ConfigMaps(namespace string) ConfigMapNamespaceLister {
	return configMapNamespaceLister{indexer: s.indexer, namespace: namespace, tenant: ""}
}

func (s *configMapLister) ConfigMapsWithMultiTenancy(namespace string, tenant string) ConfigMapNamespaceLister {
	return configMapNamespaceLister{indexer: s.indexer, namespace: namespace, tenant: tenant}
}

// ConfigMapNamespaceLister helps list and get ConfigMaps.
type ConfigMapNamespaceLister interface {
	// List lists all ConfigMaps in the indexer for a given tenant/namespace.
	List(selector labels.Selector) (ret []*v1.ConfigMap, err error)
	// Get retrieves the ConfigMap from the indexer for a given tenant/namespace and name.
	Get(name string) (*v1.ConfigMap, error)
	ConfigMapNamespaceListerExpansion
}

// configMapNamespaceLister implements the ConfigMapNamespaceLister
// interface.
type configMapNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
	tenant    string
}

// List lists all ConfigMaps in the indexer for a given namespace.
func (s configMapNamespaceLister) List(selector labels.Selector) (ret []*v1.ConfigMap, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.tenant, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.ConfigMap))
	})
	return ret, err
}

// Get retrieves the ConfigMap from the indexer for a given namespace and name.
func (s configMapNamespaceLister) Get(name string) (*v1.ConfigMap, error) {
	key := s.tenant + "/" + s.namespace + "/" + name
	if s.tenant == "system" {
		key = s.namespace + "/" + name
	}
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("configmap"), name)
	}
	return obj.(*v1.ConfigMap), nil
}
