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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	apiregistration "k8s.io/kube-aggregator/pkg/apis/apiregistration"
)

// FakeAPIServices implements APIServiceInterface
type FakeAPIServices struct {
	Fake *FakeApiregistration
	te   string
}

var apiservicesResource = schema.GroupVersionResource{Group: "apiregistration.k8s.io", Version: "", Resource: "apiservices"}

var apiservicesKind = schema.GroupVersionKind{Group: "apiregistration.k8s.io", Version: "", Kind: "APIService"}

// Get takes name of the aPIService, and returns the corresponding aPIService object, and an error if there is any.
func (c *FakeAPIServices) Get(name string, options v1.GetOptions) (result *apiregistration.APIService, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantGetAction(apiservicesResource, name, c.te), &apiregistration.APIService{})

	if obj == nil {
		return nil, err
	}

	return obj.(*apiregistration.APIService), err
}

// List takes label and field selectors, and returns the list of APIServices that match those selectors.
func (c *FakeAPIServices) List(opts v1.ListOptions) (result *apiregistration.APIServiceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantListAction(apiservicesResource, apiservicesKind, opts, c.te), &apiregistration.APIServiceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &apiregistration.APIServiceList{ListMeta: obj.(*apiregistration.APIServiceList).ListMeta}
	for _, item := range obj.(*apiregistration.APIServiceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested aPIServices.
func (c *FakeAPIServices) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewTenantWatchAction(apiservicesResource, opts, c.te))

}

// Create takes the representation of a aPIService and creates it.  Returns the server's representation of the aPIService, and an error, if there is any.
func (c *FakeAPIServices) Create(aPIService *apiregistration.APIService) (result *apiregistration.APIService, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantCreateAction(apiservicesResource, aPIService, c.te), &apiregistration.APIService{})

	if obj == nil {
		return nil, err
	}

	return obj.(*apiregistration.APIService), err
}

// Update takes the representation of a aPIService and updates it. Returns the server's representation of the aPIService, and an error, if there is any.
func (c *FakeAPIServices) Update(aPIService *apiregistration.APIService) (result *apiregistration.APIService, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantUpdateAction(apiservicesResource, aPIService, c.te), &apiregistration.APIService{})

	if obj == nil {
		return nil, err
	}

	return obj.(*apiregistration.APIService), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeAPIServices) UpdateStatus(aPIService *apiregistration.APIService) (*apiregistration.APIService, error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantUpdateSubresourceAction(apiservicesResource, "status", aPIService, c.te), &apiregistration.APIService{})

	if obj == nil {
		return nil, err
	}
	return obj.(*apiregistration.APIService), err
}

// Delete takes name of the aPIService and deletes it. Returns an error if one occurs.
func (c *FakeAPIServices) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewTenantDeleteAction(apiservicesResource, name, c.te), &apiregistration.APIService{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAPIServices) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {

	action := testing.NewTenantDeleteCollectionAction(apiservicesResource, listOptions, c.te)

	_, err := c.Fake.Invokes(action, &apiregistration.APIServiceList{})
	return err
}

// Patch applies the patch and returns the patched aPIService.
func (c *FakeAPIServices) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *apiregistration.APIService, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewTenantPatchSubresourceAction(apiservicesResource, c.te, name, pt, data, subresources...), &apiregistration.APIService{})

	if obj == nil {
		return nil, err
	}

	return obj.(*apiregistration.APIService), err
}
