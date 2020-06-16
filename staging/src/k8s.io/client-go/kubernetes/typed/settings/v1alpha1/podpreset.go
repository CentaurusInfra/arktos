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

package v1alpha1

import (
	strings "strings"
	"time"

	v1alpha1 "k8s.io/api/settings/v1alpha1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	scheme "k8s.io/client-go/kubernetes/scheme"
	rest "k8s.io/client-go/rest"
	klog "k8s.io/klog"
)

// PodPresetsGetter has a method to return a PodPresetInterface.
// A group's client should implement this interface.
type PodPresetsGetter interface {
	PodPresets(namespace string) PodPresetInterface
	PodPresetsWithMultiTenancy(namespace string, tenant string) PodPresetInterface
}

// PodPresetInterface has methods to work with PodPreset resources.
type PodPresetInterface interface {
	Create(*v1alpha1.PodPreset) (*v1alpha1.PodPreset, error)
	Update(*v1alpha1.PodPreset) (*v1alpha1.PodPreset, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.PodPreset, error)
	List(opts v1.ListOptions) (*v1alpha1.PodPresetList, error)
	Watch(opts v1.ListOptions) watch.AggregatedWatchInterface
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PodPreset, err error)
	PodPresetExpansion
}

// podPresets implements PodPresetInterface
type podPresets struct {
	client  rest.Interface
	clients []rest.Interface
	ns      string
	te      string
}

// newPodPresets returns a PodPresets
func newPodPresets(c *SettingsV1alpha1Client, namespace string) *podPresets {
	return newPodPresetsWithMultiTenancy(c, namespace, "system")
}

func newPodPresetsWithMultiTenancy(c *SettingsV1alpha1Client, namespace string, tenant string) *podPresets {
	return &podPresets{
		client:  c.RESTClient(),
		clients: c.RESTClients(),
		ns:      namespace,
		te:      tenant,
	}
}

// Get takes name of the podPreset, and returns the corresponding podPreset object, and an error if there is any.
func (c *podPresets) Get(name string, options v1.GetOptions) (result *v1alpha1.PodPreset, err error) {
	result = &v1alpha1.PodPreset{}

	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}
	err = c.client.Get().
		Tenant(tenant).
		Namespace(c.ns).
		Resource("podpresets").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)

	return
}

// List takes label and field selectors, and returns the list of PodPresets that match those selectors.
func (c *podPresets) List(opts v1.ListOptions) (result *v1alpha1.PodPresetList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.PodPresetList{}
	err = c.client.Get().
		Tenant(c.te).Namespace(c.ns).
		Resource("podpresets").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	if err == nil {
		return
	}

	if !(errors.IsForbidden(err) && strings.Contains(err.Error(), "no relationship found between node")) {
		return
	}

	// Found api server that works with this list, keep the client
	for _, client := range c.clients {
		if client == c.client {
			continue
		}

		err = client.Get().
			Tenant(c.te).Namespace(c.ns).
			Resource("podpresets").
			VersionedParams(&opts, scheme.ParameterCodec).
			Timeout(timeout).
			Do().
			Into(result)

		if err == nil {
			c.client = client
			return
		}

		if err != nil && errors.IsForbidden(err) &&
			strings.Contains(err.Error(), "no relationship found between node") {
			klog.V(6).Infof("Skip error %v in list", err)
			continue
		}
	}

	return
}

// Watch returns a watch.Interface that watches the requested podPresets.
func (c *podPresets) Watch(opts v1.ListOptions) watch.AggregatedWatchInterface {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	aggWatch := watch.NewAggregatedWatcher()
	for _, client := range c.clients {
		watcher, err := client.Get().
			Tenant(c.te).
			Namespace(c.ns).
			Resource("podpresets").
			VersionedParams(&opts, scheme.ParameterCodec).
			Timeout(timeout).
			Watch()
		if err != nil && opts.AllowPartialWatch && errors.IsForbidden(err) {
			// watch error was not returned properly in error message. Skip when partial watch is allowed
			klog.V(6).Infof("Watch error for partial watch %v. options [%+v]", err, opts)
			continue
		}
		aggWatch.AddWatchInterface(watcher, err)
	}
	return aggWatch
}

// Create takes the representation of a podPreset and creates it.  Returns the server's representation of the podPreset, and an error, if there is any.
func (c *podPresets) Create(podPreset *v1alpha1.PodPreset) (result *v1alpha1.PodPreset, err error) {
	result = &v1alpha1.PodPreset{}

	objectTenant := podPreset.ObjectMeta.Tenant
	if objectTenant == "" {
		objectTenant = c.te
		if c.te == "all" {
			objectTenant = "system"
		}
	}

	err = c.client.Post().
		Tenant(objectTenant).
		Namespace(c.ns).
		Resource("podpresets").
		Body(podPreset).
		Do().
		Into(result)

	return
}

// Update takes the representation of a podPreset and updates it. Returns the server's representation of the podPreset, and an error, if there is any.
func (c *podPresets) Update(podPreset *v1alpha1.PodPreset) (result *v1alpha1.PodPreset, err error) {
	result = &v1alpha1.PodPreset{}

	objectTenant := podPreset.ObjectMeta.Tenant
	if objectTenant == "" {
		objectTenant = c.te
		if c.te == "all" {
			objectTenant = "system"
		}
	}

	err = c.client.Put().
		Tenant(objectTenant).
		Namespace(c.ns).
		Resource("podpresets").
		Name(podPreset.Name).
		Body(podPreset).
		Do().
		Into(result)

	return
}

// Delete takes name of the podPreset and deletes it. Returns an error if one occurs.
func (c *podPresets) Delete(name string, options *v1.DeleteOptions) error {

	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}
	return c.client.Delete().
		Tenant(tenant).
		Namespace(c.ns).
		Resource("podpresets").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *podPresets) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Tenant(c.te).
		Namespace(c.ns).
		Resource("podpresets").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched podPreset.
func (c *podPresets) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PodPreset, err error) {
	result = &v1alpha1.PodPreset{}

	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}
	err = c.client.Patch(pt).
		Tenant(tenant).
		Namespace(c.ns).
		Resource("podpresets").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)

	return
}
