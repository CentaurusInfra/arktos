/*
Copyright 2015 The Kubernetes Authors.
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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/pager"
)

// Lister is any object that knows how to perform an initial list.
type Lister interface {
	// List should return a list type object; the Items field will be extracted, and the
	// ResourceVersion field will be used to start the watch in the right place.
	List(options metav1.ListOptions) (runtime.Object, error)
}

// Watcher is any object that knows how to start a watch on a resource.
type Watcher interface {
	// Watch should begin a watch at the specified version.
	Watch(options metav1.ListOptions) watch.AggregatedWatchInterface
}

// Updater is to set the search conditions of resources for listers and watchers
type Updater interface {
	// Update should change the search conditions, such as field selectors
	Update(options metav1.ListOptions)
}

// ListerWatcher is any object that knows how to perform an initial list and start a watch on a resource.
type ListerWatcher interface {
	Lister
	Watcher
	Updater
}

// ListFunc knows how to list resources
type ListFunc func(options metav1.ListOptions) (runtime.Object, error)

// WatchFunc knows how to watch resources
type WatchFunc func(options metav1.ListOptions) watch.AggregatedWatchInterface

// UpdateFunc knows how to update search resource conditions
type UpdateFunc func(options metav1.ListOptions)

// ListWatch knows how to list and watch a set of apiserver resources.  It satisfies the ListerWatcher interface.
// It is a convenience function for users of NewReflector, etc.
// ListFunc and WatchFunc must not be nil
type ListWatch struct {
	ListFunc   ListFunc
	WatchFunc  WatchFunc
	UpdateFunc UpdateFunc
	// DisableChunking requests no chunking for this list watcher.
	DisableChunking bool
	searchOptions   metav1.ListOptions
}

// Getter interface knows how to access Get method from RESTClient.
type Getter interface {
	RESTClient() restclient.Interface
	RESTClients() []restclient.Interface
}

// NewListWatchFromClient creates a new ListWatch from the specified client, resource, namespace and field selector.
func NewListWatchFromClient(c Getter, resource string, namespace string, fieldSelector fields.Selector) *ListWatch {
	return NewListWatchFromClientWithMultiTenancy(c, resource, namespace, fieldSelector, metav1.TenantSystem)
}

func NewListWatchFromClientWithMultiTenancy(c Getter, resource string, namespace string, fieldSelector fields.Selector, tenant string) *ListWatch {
	optionsModifier := func(options *metav1.ListOptions) {
		options.FieldSelector = fieldSelector.String()
	}
	return NewFilteredListWatchFromClientWithMultiTenancy(c, resource, namespace, optionsModifier, tenant)
}

// NewFilteredListWatchFromClient creates a new ListWatch from the specified client, resource, namespace, and option modifier.
// Option modifier is a function takes a ListOptions and modifies the consumed ListOptions. Provide customized modifier function
// to apply modification to ListOptions with a field selector, a label selector, or any other desired options.
func NewFilteredListWatchFromClient(c Getter, resource string, namespace string, optionsModifier func(options *metav1.ListOptions)) *ListWatch {
	return NewFilteredListWatchFromClientWithMultiTenancy(c, resource, namespace, optionsModifier, metav1.TenantSystem)
}
func NewFilteredListWatchFromClientWithMultiTenancy(c Getter, resource string, namespace string, optionsModifier func(options *metav1.ListOptions), tenant string) *ListWatch {
	listFunc := func(options metav1.ListOptions) (runtime.Object, error) {
		optionsModifier(&options)
		return c.RESTClient().Get().
			Tenant(tenant).
			Namespace(namespace).
			Resource(resource).
			VersionedParams(&options, metav1.ParameterCodec).
			Do().
			Get()
	}
	watchFunc := func(options metav1.ListOptions) watch.AggregatedWatchInterface {
		options.Watch = true
		optionsModifier(&options)

		watchClient := watch.NewAggregatedWatcher()
		for _, getter := range c.RESTClients() {
			watcher, err := getter.Get().
				Tenant(tenant).
				Namespace(namespace).
				Resource(resource).
				VersionedParams(&options, metav1.ParameterCodec).
				Watch()
			watchClient.AddWatchInterface(watcher, err)
		}
		return watchClient
	}
	return &ListWatch{ListFunc: listFunc, WatchFunc: watchFunc}
}

// List a set of apiserver resources
func (lw *ListWatch) List(options metav1.ListOptions) (runtime.Object, error) {
	if !lw.DisableChunking {
		return pager.New(pager.SimplePageFunc(lw.ListFunc)).List(context.TODO(), options)
	}
	return lw.ListFunc(lw.appendOptions(options))
}

// Watch a set of apiserver resources
func (lw *ListWatch) Watch(options metav1.ListOptions) watch.AggregatedWatchInterface {
	return lw.WatchFunc(lw.appendOptions(options))
}

// Update the resource search conditions for listers and watchers
func (lw *ListWatch) Update(options metav1.ListOptions) {
	lw.searchOptions = options
}

func (lw *ListWatch) appendOptions(options metav1.ListOptions) metav1.ListOptions {
	appended := options.DeepCopy()
	if len(appended.FieldSelector) == 0 {
		appended.FieldSelector = lw.searchOptions.FieldSelector
	} else {
		appended.FieldSelector = appended.FieldSelector + "," + lw.searchOptions.FieldSelector
	}
	return *appended
}
