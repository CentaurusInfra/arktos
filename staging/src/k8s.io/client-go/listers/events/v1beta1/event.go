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
	v1beta1 "k8s.io/api/events/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// EventLister helps list Events.
type EventLister interface {
	// List lists all Events in the indexer.
	List(selector labels.Selector) (ret []*v1beta1.Event, err error)
	// Events returns an object that can list and get Events.
	Events(namespace string) EventNamespaceLister
	EventsWithMultiTenancy(namespace string, tenant string) EventNamespaceLister
	EventListerExpansion
}

// eventLister implements the EventLister interface.
type eventLister struct {
	indexer cache.Indexer
}

// NewEventLister returns a new EventLister.
func NewEventLister(indexer cache.Indexer) EventLister {
	return &eventLister{indexer: indexer}
}

// List lists all Events in the indexer.
func (s *eventLister) List(selector labels.Selector) (ret []*v1beta1.Event, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Event))
	})
	return ret, err
}

// Events returns an object that can list and get Events.
func (s *eventLister) Events(namespace string) EventNamespaceLister {
	return eventNamespaceLister{indexer: s.indexer, namespace: namespace, tenant: "system"}
}

func (s *eventLister) EventsWithMultiTenancy(namespace string, tenant string) EventNamespaceLister {
	return eventNamespaceLister{indexer: s.indexer, namespace: namespace, tenant: tenant}
}

// EventNamespaceLister helps list and get Events.
type EventNamespaceLister interface {
	// List lists all Events in the indexer for a given tenant/namespace.
	List(selector labels.Selector) (ret []*v1beta1.Event, err error)
	// Get retrieves the Event from the indexer for a given tenant/namespace and name.
	Get(name string) (*v1beta1.Event, error)
	EventNamespaceListerExpansion
}

// eventNamespaceLister implements the EventNamespaceLister
// interface.
type eventNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
	tenant    string
}

// List lists all Events in the indexer for a given namespace.
func (s eventNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.Event, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.tenant, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Event))
	})
	return ret, err
}

// Get retrieves the Event from the indexer for a given namespace and name.
func (s eventNamespaceLister) Get(name string) (*v1beta1.Event, error) {
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
		return nil, errors.NewNotFound(v1beta1.Resource("event"), name)
	}
	return obj.(*v1beta1.Event), nil
}
