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
	v1beta1 "k8s.io/api/storage/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeStorageClasses implements StorageClassInterface
type FakeStorageClasses struct {
	Fake *FakeStorageV1beta1
	te   string
}

var storageclassesResource = schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1beta1", Resource: "storageclasses"}

var storageclassesKind = schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1beta1", Kind: "StorageClass"}

// Get takes name of the storageClass, and returns the corresponding storageClass object, and an error if there is any.
func (c *FakeStorageClasses) Get(name string, options v1.GetOptions) (result *v1beta1.StorageClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantGetAction(storageclassesResource, name, c.te), &v1beta1.StorageClass{})

	if obj == nil {
		return nil, err
	}

	return obj.(*v1beta1.StorageClass), err
}

// List takes label and field selectors, and returns the list of StorageClasses that match those selectors.
func (c *FakeStorageClasses) List(opts v1.ListOptions) (result *v1beta1.StorageClassList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantListAction(storageclassesResource, storageclassesKind, opts, c.te), &v1beta1.StorageClassList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.StorageClassList{ListMeta: obj.(*v1beta1.StorageClassList).ListMeta}
	for _, item := range obj.(*v1beta1.StorageClassList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.AggregatedWatchInterface that watches the requested storageClasses.
func (c *FakeStorageClasses) Watch(opts v1.ListOptions) watch.AggregatedWatchInterface {
	aggWatch := watch.NewAggregatedWatcher()
	watcher, err := c.Fake.
		InvokesWatch(testing.NewTenantWatchAction(storageclassesResource, opts, c.te))

	aggWatch.AddWatchInterface(watcher, err)
	return aggWatch
}

// Create takes the representation of a storageClass and creates it.  Returns the server's representation of the storageClass, and an error, if there is any.
func (c *FakeStorageClasses) Create(storageClass *v1beta1.StorageClass) (result *v1beta1.StorageClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantCreateAction(storageclassesResource, storageClass, c.te), &v1beta1.StorageClass{})

	if obj == nil {
		return nil, err
	}

	return obj.(*v1beta1.StorageClass), err
}

// Update takes the representation of a storageClass and updates it. Returns the server's representation of the storageClass, and an error, if there is any.
func (c *FakeStorageClasses) Update(storageClass *v1beta1.StorageClass) (result *v1beta1.StorageClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantUpdateAction(storageclassesResource, storageClass, c.te), &v1beta1.StorageClass{})

	if obj == nil {
		return nil, err
	}

	return obj.(*v1beta1.StorageClass), err
}

// Delete takes name of the storageClass and deletes it. Returns an error if one occurs.
func (c *FakeStorageClasses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewTenantDeleteAction(storageclassesResource, name, c.te), &v1beta1.StorageClass{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeStorageClasses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {

	action := testing.NewTenantDeleteCollectionAction(storageclassesResource, listOptions, c.te)

	_, err := c.Fake.Invokes(action, &v1beta1.StorageClassList{})
	return err
}

// Patch applies the patch and returns the patched storageClass.
func (c *FakeStorageClasses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.StorageClass, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantPatchSubresourceAction(storageclassesResource, c.te, name, pt, data, subresources...), &v1beta1.StorageClass{})

	if obj == nil {
		return nil, err
	}

	return obj.(*v1beta1.StorageClass), err
}
