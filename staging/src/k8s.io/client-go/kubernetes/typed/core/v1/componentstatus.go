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

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	strings "strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	scheme "k8s.io/client-go/kubernetes/scheme"
	rest "k8s.io/client-go/rest"
	klog "k8s.io/klog"
)

// ComponentStatusesGetter has a method to return a ComponentStatusInterface.
// A group's client should implement this interface.
type ComponentStatusesGetter interface {
	ComponentStatuses() ComponentStatusInterface
}

// ComponentStatusInterface has methods to work with ComponentStatus resources.
type ComponentStatusInterface interface {
	Create(*v1.ComponentStatus) (*v1.ComponentStatus, error)
	Update(*v1.ComponentStatus) (*v1.ComponentStatus, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Get(name string, options metav1.GetOptions) (*v1.ComponentStatus, error)
	List(opts metav1.ListOptions) (*v1.ComponentStatusList, error)
	Watch(opts metav1.ListOptions) watch.AggregatedWatchInterface
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ComponentStatus, err error)
	ComponentStatusExpansion
}

// componentStatuses implements ComponentStatusInterface
type componentStatuses struct {
	client  rest.Interface
	clients []rest.Interface
}

// newComponentStatuses returns a ComponentStatuses
func newComponentStatuses(c *CoreV1Client) *componentStatuses {
	return &componentStatuses{
		client:  c.RESTClient(),
		clients: c.RESTClients(),
	}
}

// Get takes name of the componentStatus, and returns the corresponding componentStatus object, and an error if there is any.
func (c *componentStatuses) Get(name string, options metav1.GetOptions) (result *v1.ComponentStatus, err error) {
	result = &v1.ComponentStatus{}
	err = c.client.Get().
		Resource("componentstatuses").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)

	return
}

// List takes label and field selectors, and returns the list of ComponentStatuses that match those selectors.
func (c *componentStatuses) List(opts metav1.ListOptions) (result *v1.ComponentStatusList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.ComponentStatusList{}
	err = c.client.Get().
		Resource("componentstatuses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	if err == nil {
		return
	}

	if !strings.Contains(err.Error(), "forbidden") ||
		!strings.Contains(err.Error(), "no relationship found between node") {
		return
	}

	// Found api server that works with this list, keep the client
	for _, client := range c.clients {
		if client == c.client {
			continue
		}

		err = client.Get().
			Resource("componentstatuses").
			VersionedParams(&opts, scheme.ParameterCodec).
			Timeout(timeout).
			Do().
			Into(result)

		if err == nil {
			c.client = client
			return
		}

		if err != nil && strings.Contains(err.Error(), "forbidden") &&
			strings.Contains(err.Error(), "no relationship found between node") {
			klog.V(6).Infof("Skip error %v in list", err)
			continue
		}
	}

	return
}

// Watch returns a watch.Interface that watches the requested componentStatuses.
func (c *componentStatuses) Watch(opts metav1.ListOptions) watch.AggregatedWatchInterface {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	aggWatch := watch.NewAggregatedWatcher()
	for _, client := range c.clients {
		watcher, err := client.Get().
			Resource("componentstatuses").
			VersionedParams(&opts, scheme.ParameterCodec).
			Timeout(timeout).
			Watch()
		if err != nil && opts.AllowPartialWatch {
			// watch error was not returned properly in error message. Skip when partial watch is allowed
			klog.V(6).Infof("Watch error for partial watch %v. options [%+v]", err, opts)
			continue
		}
		aggWatch.AddWatchInterface(watcher, err)
	}
	return aggWatch
}

// Create takes the representation of a componentStatus and creates it.  Returns the server's representation of the componentStatus, and an error, if there is any.
func (c *componentStatuses) Create(componentStatus *v1.ComponentStatus) (result *v1.ComponentStatus, err error) {
	result = &v1.ComponentStatus{}

	err = c.client.Post().
		Resource("componentstatuses").
		Body(componentStatus).
		Do().
		Into(result)

	return
}

// Update takes the representation of a componentStatus and updates it. Returns the server's representation of the componentStatus, and an error, if there is any.
func (c *componentStatuses) Update(componentStatus *v1.ComponentStatus) (result *v1.ComponentStatus, err error) {
	result = &v1.ComponentStatus{}

	err = c.client.Put().
		Resource("componentstatuses").
		Name(componentStatus.Name).
		Body(componentStatus).
		Do().
		Into(result)

	return
}

// Delete takes name of the componentStatus and deletes it. Returns an error if one occurs.
func (c *componentStatuses) Delete(name string, options *metav1.DeleteOptions) error {
	return c.client.Delete().
		Resource("componentstatuses").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *componentStatuses) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("componentstatuses").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched componentStatus.
func (c *componentStatuses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ComponentStatus, err error) {
	result = &v1.ComponentStatus{}
	err = c.client.Patch(pt).
		Resource("componentstatuses").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)

	return
}
