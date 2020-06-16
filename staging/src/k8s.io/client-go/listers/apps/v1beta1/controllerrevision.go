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
	v1beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ControllerRevisionLister helps list ControllerRevisions.
type ControllerRevisionLister interface {
	// List lists all ControllerRevisions in the indexer.
	List(selector labels.Selector) (ret []*v1beta1.ControllerRevision, err error)
	// ControllerRevisions returns an object that can list and get ControllerRevisions.
	ControllerRevisions(namespace string) ControllerRevisionNamespaceLister
	ControllerRevisionsWithMultiTenancy(namespace string, tenant string) ControllerRevisionNamespaceLister
	ControllerRevisionListerExpansion
}

// controllerRevisionLister implements the ControllerRevisionLister interface.
type controllerRevisionLister struct {
	indexer cache.Indexer
}

// NewControllerRevisionLister returns a new ControllerRevisionLister.
func NewControllerRevisionLister(indexer cache.Indexer) ControllerRevisionLister {
	return &controllerRevisionLister{indexer: indexer}
}

// List lists all ControllerRevisions in the indexer.
func (s *controllerRevisionLister) List(selector labels.Selector) (ret []*v1beta1.ControllerRevision, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.ControllerRevision))
	})
	return ret, err
}

// ControllerRevisions returns an object that can list and get ControllerRevisions.
func (s *controllerRevisionLister) ControllerRevisions(namespace string) ControllerRevisionNamespaceLister {
	return controllerRevisionNamespaceLister{indexer: s.indexer, namespace: namespace, tenant: "system"}
}

func (s *controllerRevisionLister) ControllerRevisionsWithMultiTenancy(namespace string, tenant string) ControllerRevisionNamespaceLister {
	return controllerRevisionNamespaceLister{indexer: s.indexer, namespace: namespace, tenant: tenant}
}

// ControllerRevisionNamespaceLister helps list and get ControllerRevisions.
type ControllerRevisionNamespaceLister interface {
	// List lists all ControllerRevisions in the indexer for a given tenant/namespace.
	List(selector labels.Selector) (ret []*v1beta1.ControllerRevision, err error)
	// Get retrieves the ControllerRevision from the indexer for a given tenant/namespace and name.
	Get(name string) (*v1beta1.ControllerRevision, error)
	ControllerRevisionNamespaceListerExpansion
}

// controllerRevisionNamespaceLister implements the ControllerRevisionNamespaceLister
// interface.
type controllerRevisionNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
	tenant    string
}

// List lists all ControllerRevisions in the indexer for a given namespace.
func (s controllerRevisionNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.ControllerRevision, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.tenant, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.ControllerRevision))
	})
	return ret, err
}

// Get retrieves the ControllerRevision from the indexer for a given namespace and name.
func (s controllerRevisionNamespaceLister) Get(name string) (*v1beta1.ControllerRevision, error) {
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
		return nil, errors.NewNotFound(v1beta1.Resource("controllerrevision"), name)
	}
	return obj.(*v1beta1.ControllerRevision), nil
}
