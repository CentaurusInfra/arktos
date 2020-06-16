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

// FakePodTemplates implements PodTemplateInterface
type FakePodTemplates struct {
	Fake *FakeCoreV1
	ns   string
	te   string
}

var podtemplatesResource = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "podtemplates"}

var podtemplatesKind = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PodTemplate"}

// Get takes name of the podTemplate, and returns the corresponding podTemplate object, and an error if there is any.
func (c *FakePodTemplates) Get(name string, options v1.GetOptions) (result *corev1.PodTemplate, err error) {

	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithMultiTenancy(podtemplatesResource, c.ns, name, tenant), &corev1.PodTemplate{})

	if obj == nil {
		return nil, err
	}

	return obj.(*corev1.PodTemplate), err
}

// List takes label and field selectors, and returns the list of PodTemplates that match those selectors.
func (c *FakePodTemplates) List(opts v1.ListOptions) (result *corev1.PodTemplateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithMultiTenancy(podtemplatesResource, podtemplatesKind, c.ns, opts, c.te), &corev1.PodTemplateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &corev1.PodTemplateList{ListMeta: obj.(*corev1.PodTemplateList).ListMeta}
	for _, item := range obj.(*corev1.PodTemplateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.AggregatedWatchInterface that watches the requested podTemplates.
func (c *FakePodTemplates) Watch(opts v1.ListOptions) watch.AggregatedWatchInterface {
	aggWatch := watch.NewAggregatedWatcher()
	watcher, err := c.Fake.
		InvokesWatch(testing.NewWatchActionWithMultiTenancy(podtemplatesResource, c.ns, opts, c.te))

	aggWatch.AddWatchInterface(watcher, err)
	return aggWatch
}

// Create takes the representation of a podTemplate and creates it.  Returns the server's representation of the podTemplate, and an error, if there is any.
func (c *FakePodTemplates) Create(podTemplate *corev1.PodTemplate) (result *corev1.PodTemplate, err error) {

	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithMultiTenancy(podtemplatesResource, c.ns, podTemplate, tenant), &corev1.PodTemplate{})

	if obj == nil {
		return nil, err
	}

	return obj.(*corev1.PodTemplate), err
}

// Update takes the representation of a podTemplate and updates it. Returns the server's representation of the podTemplate, and an error, if there is any.
func (c *FakePodTemplates) Update(podTemplate *corev1.PodTemplate) (result *corev1.PodTemplate, err error) {

	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithMultiTenancy(podtemplatesResource, c.ns, podTemplate, tenant), &corev1.PodTemplate{})

	if obj == nil {
		return nil, err
	}

	return obj.(*corev1.PodTemplate), err
}

// Delete takes name of the podTemplate and deletes it. Returns an error if one occurs.
func (c *FakePodTemplates) Delete(name string, options *v1.DeleteOptions) error {

	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithMultiTenancy(podtemplatesResource, c.ns, name, tenant), &corev1.PodTemplate{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePodTemplates) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithMultiTenancy(podtemplatesResource, c.ns, listOptions, c.te)

	_, err := c.Fake.Invokes(action, &corev1.PodTemplateList{})
	return err
}

// Patch applies the patch and returns the patched podTemplate.
func (c *FakePodTemplates) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *corev1.PodTemplate, err error) {

	tenant := c.te
	if tenant == "all" {
		tenant = "system"
	}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithMultiTenancy(podtemplatesResource, tenant, c.ns, name, pt, data, subresources...), &corev1.PodTemplate{})

	if obj == nil {
		return nil, err
	}

	return obj.(*corev1.PodTemplate), err
}
