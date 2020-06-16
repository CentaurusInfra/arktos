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

package fake

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeNamespaces implements NamespaceInterface
type FakeNamespaces struct {
	Fake *FakeCoreV1
	te   string
}

var namespacesResource = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}

var namespacesKind = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}

// Get takes name of the namespace, and returns the corresponding namespace object, and an error if there is any.
func (c *FakeNamespaces) Get(name string, options v1.GetOptions) (result *corev1.Namespace, err error) {
	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}

	obj, err := c.Fake.
		Invokes(testing.NewTenantGetAction(namespacesResource, name, tenant), &corev1.Namespace{})

	if obj == nil {
		return nil, err
	}

	return obj.(*corev1.Namespace), err
}

// List takes label and field selectors, and returns the list of Namespaces that match those selectors.
func (c *FakeNamespaces) List(opts v1.ListOptions) (result *corev1.NamespaceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantListAction(namespacesResource, namespacesKind, opts, c.te), &corev1.NamespaceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &corev1.NamespaceList{ListMeta: obj.(*corev1.NamespaceList).ListMeta}
	for _, item := range obj.(*corev1.NamespaceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.AggregatedWatchInterface that watches the requested namespaces.
func (c *FakeNamespaces) Watch(opts v1.ListOptions) watch.AggregatedWatchInterface {
	aggWatch := watch.NewAggregatedWatcher()
	watcher, err := c.Fake.
		InvokesWatch(testing.NewTenantWatchAction(namespacesResource, opts, c.te))

	aggWatch.AddWatchInterface(watcher, err)
	return aggWatch
}

// Create takes the representation of a namespace and creates it.  Returns the server's representation of the namespace, and an error, if there is any.
func (c *FakeNamespaces) Create(namespace *corev1.Namespace) (result *corev1.Namespace, err error) {
	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}

	obj, err := c.Fake.
		Invokes(testing.NewTenantCreateAction(namespacesResource, namespace, tenant), &corev1.Namespace{})

	if obj == nil {
		return nil, err
	}

	return obj.(*corev1.Namespace), err
}

// Update takes the representation of a namespace and updates it. Returns the server's representation of the namespace, and an error, if there is any.
func (c *FakeNamespaces) Update(namespace *corev1.Namespace) (result *corev1.Namespace, err error) {
	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}

	obj, err := c.Fake.
		Invokes(testing.NewTenantUpdateAction(namespacesResource, namespace, tenant), &corev1.Namespace{})

	if obj == nil {
		return nil, err
	}

	return obj.(*corev1.Namespace), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeNamespaces) UpdateStatus(namespace *corev1.Namespace) (*corev1.Namespace, error) {
	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}

	obj, err := c.Fake.
		Invokes(testing.NewTenantUpdateSubresourceAction(namespacesResource, "status", namespace, tenant), &corev1.Namespace{})

	if obj == nil {
		return nil, err
	}
	return obj.(*corev1.Namespace), err
}

// Delete takes name of the namespace and deletes it. Returns an error if one occurs.
func (c *FakeNamespaces) Delete(name string, options *v1.DeleteOptions) error {
	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}

	_, err := c.Fake.
		Invokes(testing.NewTenantDeleteAction(namespacesResource, name, tenant), &corev1.Namespace{})

	return err
}

// Patch applies the patch and returns the patched namespace.
func (c *FakeNamespaces) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *corev1.Namespace, err error) {
	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}

	obj, err := c.Fake.
		Invokes(testing.NewTenantPatchSubresourceAction(namespacesResource, tenant, name, pt, data, subresources...), &corev1.Namespace{})

	if obj == nil {
		return nil, err
	}

	return obj.(*corev1.Namespace), err
}
