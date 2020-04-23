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

package v1beta1

import (
	strings "strings"
	"time"

	v1beta1 "k8s.io/api/storage/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	scheme "k8s.io/client-go/kubernetes/scheme"
	rest "k8s.io/client-go/rest"
	klog "k8s.io/klog"
)

// CSIDriversGetter has a method to return a CSIDriverInterface.
// A group's client should implement this interface.
type CSIDriversGetter interface {
	CSIDrivers() CSIDriverInterface
}

// CSIDriverInterface has methods to work with CSIDriver resources.
type CSIDriverInterface interface {
	Create(*v1beta1.CSIDriver) (*v1beta1.CSIDriver, error)
	Update(*v1beta1.CSIDriver) (*v1beta1.CSIDriver, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1beta1.CSIDriver, error)
	List(opts v1.ListOptions) (*v1beta1.CSIDriverList, error)
	Watch(opts v1.ListOptions) watch.AggregatedWatchInterface
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.CSIDriver, err error)
	CSIDriverExpansion
}

// cSIDrivers implements CSIDriverInterface
type cSIDrivers struct {
	client  rest.Interface
	clients []rest.Interface
}

// newCSIDrivers returns a CSIDrivers
func newCSIDrivers(c *StorageV1beta1Client) *cSIDrivers {
	return &cSIDrivers{
		client:  c.RESTClient(),
		clients: c.RESTClients(),
	}
}

// Get takes name of the cSIDriver, and returns the corresponding cSIDriver object, and an error if there is any.
func (c *cSIDrivers) Get(name string, options v1.GetOptions) (result *v1beta1.CSIDriver, err error) {
	result = &v1beta1.CSIDriver{}
	err = c.client.Get().
		Resource("csidrivers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)

	return
}

// List takes label and field selectors, and returns the list of CSIDrivers that match those selectors.
func (c *cSIDrivers) List(opts v1.ListOptions) (result *v1beta1.CSIDriverList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1beta1.CSIDriverList{}
	err = c.client.Get().
		Resource("csidrivers").
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
			Resource("csidrivers").
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

// Watch returns a watch.Interface that watches the requested cSIDrivers.
func (c *cSIDrivers) Watch(opts v1.ListOptions) watch.AggregatedWatchInterface {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	aggWatch := watch.NewAggregatedWatcher()
	for _, client := range c.clients {
		watcher, err := client.Get().
			Resource("csidrivers").
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

// Create takes the representation of a cSIDriver and creates it.  Returns the server's representation of the cSIDriver, and an error, if there is any.
func (c *cSIDrivers) Create(cSIDriver *v1beta1.CSIDriver) (result *v1beta1.CSIDriver, err error) {
	result = &v1beta1.CSIDriver{}

	err = c.client.Post().
		Resource("csidrivers").
		Body(cSIDriver).
		Do().
		Into(result)

	return
}

// Update takes the representation of a cSIDriver and updates it. Returns the server's representation of the cSIDriver, and an error, if there is any.
func (c *cSIDrivers) Update(cSIDriver *v1beta1.CSIDriver) (result *v1beta1.CSIDriver, err error) {
	result = &v1beta1.CSIDriver{}

	err = c.client.Put().
		Resource("csidrivers").
		Name(cSIDriver.Name).
		Body(cSIDriver).
		Do().
		Into(result)

	return
}

// Delete takes name of the cSIDriver and deletes it. Returns an error if one occurs.
func (c *cSIDrivers) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("csidrivers").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *cSIDrivers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("csidrivers").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched cSIDriver.
func (c *cSIDrivers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.CSIDriver, err error) {
	result = &v1beta1.CSIDriver{}
	err = c.client.Patch(pt).
		Resource("csidrivers").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)

	return
}
