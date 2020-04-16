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
	"time"

	v1beta1 "k8s.io/api/apps/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	scheme "k8s.io/client-go/kubernetes/scheme"
	rest "k8s.io/client-go/rest"
)

// ControllerRevisionsGetter has a method to return a ControllerRevisionInterface.
// A group's client should implement this interface.
type ControllerRevisionsGetter interface {
	ControllerRevisions(namespace string) ControllerRevisionInterface
	ControllerRevisionsWithMultiTenancy(namespace string, tenant string) ControllerRevisionInterface
}

// ControllerRevisionInterface has methods to work with ControllerRevision resources.
type ControllerRevisionInterface interface {
	Create(*v1beta1.ControllerRevision) (*v1beta1.ControllerRevision, error)
	Update(*v1beta1.ControllerRevision) (*v1beta1.ControllerRevision, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1beta1.ControllerRevision, error)
	List(opts v1.ListOptions) (*v1beta1.ControllerRevisionList, error)
	Watch(opts v1.ListOptions) watch.AggregatedWatchInterface
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ControllerRevision, err error)
	ControllerRevisionExpansion
}

// controllerRevisions implements ControllerRevisionInterface
type controllerRevisions struct {
	client  rest.Interface
	clients []rest.Interface
	ns      string
	te      string
}

// newControllerRevisions returns a ControllerRevisions
func newControllerRevisions(c *AppsV1beta1Client, namespace string) *controllerRevisions {
	return newControllerRevisionsWithMultiTenancy(c, namespace, "default")
}

func newControllerRevisionsWithMultiTenancy(c *AppsV1beta1Client, namespace string, tenant string) *controllerRevisions {
	return &controllerRevisions{
		client:  c.RESTClient(),
		clients: c.RESTClients(),
		ns:      namespace,
		te:      tenant,
	}
}

// Get takes name of the controllerRevision, and returns the corresponding controllerRevision object, and an error if there is any.
func (c *controllerRevisions) Get(name string, options v1.GetOptions) (result *v1beta1.ControllerRevision, err error) {
	result = &v1beta1.ControllerRevision{}
	err = c.client.Get().
		Tenant(c.te).
		Namespace(c.ns).
		Resource("controllerrevisions").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)

	return
}

// List takes label and field selectors, and returns the list of ControllerRevisions that match those selectors.
func (c *controllerRevisions) List(opts v1.ListOptions) (result *v1beta1.ControllerRevisionList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1beta1.ControllerRevisionList{}
	err = c.client.Get().
		Tenant(c.te).Namespace(c.ns).
		Resource("controllerrevisions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)

	return
}

// Watch returns a watch.Interface that watches the requested controllerRevisions.
func (c *controllerRevisions) Watch(opts v1.ListOptions) watch.AggregatedWatchInterface {
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
			Resource("controllerrevisions").
			VersionedParams(&opts, scheme.ParameterCodec).
			Timeout(timeout).
			Watch()
		aggWatch.AddWatchInterface(watcher, err)
	}
	return aggWatch
}

// Create takes the representation of a controllerRevision and creates it.  Returns the server's representation of the controllerRevision, and an error, if there is any.
func (c *controllerRevisions) Create(controllerRevision *v1beta1.ControllerRevision) (result *v1beta1.ControllerRevision, err error) {
	result = &v1beta1.ControllerRevision{}

	objectTenant := controllerRevision.ObjectMeta.Tenant
	if objectTenant == "" {
		objectTenant = c.te
	}

	err = c.client.Post().
		Tenant(objectTenant).
		Namespace(c.ns).
		Resource("controllerrevisions").
		Body(controllerRevision).
		Do().
		Into(result)

	return
}

// Update takes the representation of a controllerRevision and updates it. Returns the server's representation of the controllerRevision, and an error, if there is any.
func (c *controllerRevisions) Update(controllerRevision *v1beta1.ControllerRevision) (result *v1beta1.ControllerRevision, err error) {
	result = &v1beta1.ControllerRevision{}

	objectTenant := controllerRevision.ObjectMeta.Tenant
	if objectTenant == "" {
		objectTenant = c.te
	}

	err = c.client.Put().
		Tenant(objectTenant).
		Namespace(c.ns).
		Resource("controllerrevisions").
		Name(controllerRevision.Name).
		Body(controllerRevision).
		Do().
		Into(result)

	return
}

// Delete takes name of the controllerRevision and deletes it. Returns an error if one occurs.
func (c *controllerRevisions) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Tenant(c.te).
		Namespace(c.ns).
		Resource("controllerrevisions").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *controllerRevisions) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Tenant(c.te).
		Namespace(c.ns).
		Resource("controllerrevisions").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched controllerRevision.
func (c *controllerRevisions) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.ControllerRevision, err error) {
	result = &v1beta1.ControllerRevision{}
	err = c.client.Patch(pt).
		Tenant(c.te).
		Namespace(c.ns).
		Resource("controllerrevisions").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)

	return
}
