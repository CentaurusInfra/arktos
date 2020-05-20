/*
Copyright 2018 The Kubernetes Authors.
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

package fake

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/testing"
)

func NewSimpleDynamicClient(scheme *runtime.Scheme, objects ...runtime.Object) *FakeDynamicClient {
	// In order to use List with this client, you have to have the v1.List registered in your scheme. Neat thing though
	// it does NOT have to be the *same* list
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "fake-dynamic-client-group", Version: "v1", Kind: "List"}, &unstructured.UnstructuredList{})

	codecs := serializer.NewCodecFactory(scheme)
	o := testing.NewObjectTracker(scheme, codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &FakeDynamicClient{scheme: scheme}
	cs.AddReactor("*", "*", testing.ObjectReaction(o))
	cs.AddWatchReactor("*", func(action testing.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := o.Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	return cs
}

// Clientset implements clientset.Interface. Meant to be embedded into a
// struct to get a default implementation. This makes faking out just the method
// you want to test easier.
type FakeDynamicClient struct {
	testing.Fake
	scheme *runtime.Scheme
}

type dynamicResourceClient struct {
	client    *FakeDynamicClient
	tenant    string
	namespace string
	resource  schema.GroupVersionResource
}

var _ dynamic.Interface = &FakeDynamicClient{}

func (c *FakeDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &dynamicResourceClient{client: c, resource: resource}
}

func (c *dynamicResourceClient) Namespace(ns string) dynamic.ResourceInterface {
	return c.NamespaceWithMultiTenancy(ns, metav1.TenantSystem)
}

func (c *dynamicResourceClient) NamespaceWithMultiTenancy(ns string, tenant string) dynamic.ResourceInterface {
	ret := *c
	ret.tenant = tenant
	ret.namespace = ns
	return &ret
}

func (c *dynamicResourceClient) Tenant(te string) dynamic.ResourceInterface {
	ret := *c
	ret.tenant = te
	return &ret
}

func (c *dynamicResourceClient) Create(obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	var uncastRet runtime.Object
	var err error
	switch {
	case len(c.tenant) == 0 && len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewRootCreateAction(c.resource, obj), obj)

	case len(c.tenant) == 0 && len(c.namespace) == 0 && len(subresources) > 0:
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		name := accessor.GetName()
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewRootCreateSubresourceAction(c.resource, name, strings.Join(subresources, "/"), obj), obj)

	case len(c.tenant) > 0 && len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewTenantCreateAction(c.resource, obj, c.tenant), obj)

	case len(c.tenant) > 0 && len(c.namespace) == 0 && len(subresources) > 0:
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		name := accessor.GetName()
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewTenantCreateSubresourceAction(c.resource, name, strings.Join(subresources, "/"), obj, c.tenant), obj)

	case len(c.tenant) > 0 && len(c.namespace) > 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewCreateActionWithMultiTenancy(c.resource, c.namespace, obj, c.tenant), obj)

	case len(c.tenant) > 0 && len(c.namespace) > 0 && len(subresources) > 0:
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		name := accessor.GetName()
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewCreateSubresourceActionWithMultiTenancy(c.resource, name, strings.Join(subresources, "/"), c.namespace, obj, c.tenant), obj)
	case len(c.tenant) == 0 && len(c.namespace) > 0:
		return nil, fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	if err != nil {
		return nil, err
	}
	if uncastRet == nil {
		return nil, err
	}

	ret := &unstructured.Unstructured{}
	if err := c.client.scheme.Convert(uncastRet, ret, nil); err != nil {
		return nil, err
	}
	return ret, err
}

func (c *dynamicResourceClient) Update(obj *unstructured.Unstructured, opts metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	var uncastRet runtime.Object
	var err error
	switch {
	case len(c.tenant) == 0 && len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewRootUpdateAction(c.resource, obj), obj)

	case len(c.tenant) == 0 && len(c.namespace) == 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewRootUpdateSubresourceAction(c.resource, strings.Join(subresources, "/"), obj), obj)

	case len(c.tenant) > 0 && len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewTenantUpdateAction(c.resource, obj, c.tenant), obj)

	case len(c.tenant) > 0 && len(c.namespace) == 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewTenantUpdateSubresourceAction(c.resource, strings.Join(subresources, "/"), obj, c.tenant), obj)

	case len(c.tenant) > 0 && len(c.namespace) > 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewUpdateActionWithMultiTenancy(c.resource, c.namespace, obj, c.tenant), obj)

	case len(c.tenant) > 0 && len(c.namespace) > 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewUpdateSubresourceActionWithMultiTenancy(c.resource, strings.Join(subresources, "/"), c.namespace, obj, c.tenant), obj)

	case len(c.tenant) == 0 && len(c.namespace) > 0:
		return nil, fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	if err != nil {
		return nil, err
	}
	if uncastRet == nil {
		return nil, err
	}

	ret := &unstructured.Unstructured{}
	if err := c.client.scheme.Convert(uncastRet, ret, nil); err != nil {
		return nil, err
	}
	return ret, err
}

func (c *dynamicResourceClient) UpdateStatus(obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	var uncastRet runtime.Object
	var err error
	switch {
	case len(c.tenant) == 0 && len(c.namespace) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewRootUpdateSubresourceAction(c.resource, "status", obj), obj)

	case len(c.tenant) > 0 && len(c.namespace) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewTenantUpdateSubresourceAction(c.resource, "status", obj, c.tenant), obj)

	case len(c.tenant) > 0 && len(c.namespace) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewUpdateSubresourceActionWithMultiTenancy(c.resource, "status", c.namespace, obj, c.tenant), obj)

	case len(c.tenant) == 0 && len(c.namespace) > 0:
		return nil, fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	if err != nil {
		return nil, err
	}
	if uncastRet == nil {
		return nil, err
	}

	ret := &unstructured.Unstructured{}
	if err := c.client.scheme.Convert(uncastRet, ret, nil); err != nil {
		return nil, err
	}
	return ret, err
}

func (c *dynamicResourceClient) Delete(name string, opts *metav1.DeleteOptions, subresources ...string) error {
	var err error
	switch {
	case len(c.tenant) == 0 && len(c.namespace) == 0 && len(subresources) == 0:
		_, err = c.client.Fake.
			Invokes(testing.NewRootDeleteAction(c.resource, name), &metav1.Status{Status: "dynamic delete fail"})

	case len(c.tenant) == 0 && len(c.namespace) == 0 && len(subresources) > 0:
		_, err = c.client.Fake.
			Invokes(testing.NewRootDeleteSubresourceAction(c.resource, strings.Join(subresources, "/"), name), &metav1.Status{Status: "dynamic delete fail"})

	case len(c.tenant) > 0 && len(c.namespace) == 0 && len(subresources) == 0:
		_, err = c.client.Fake.
			Invokes(testing.NewTenantDeleteAction(c.resource, name, c.tenant), &metav1.Status{Status: "dynamic delete fail"})

	case len(c.tenant) > 0 && len(c.namespace) == 0 && len(subresources) > 0:
		_, err = c.client.Fake.
			Invokes(testing.NewTenantDeleteSubresourceAction(c.resource, strings.Join(subresources, "/"), name, c.tenant), &metav1.Status{Status: "dynamic delete fail"})

	case len(c.tenant) > 0 && len(c.namespace) > 0 && len(subresources) == 0:
		_, err = c.client.Fake.
			Invokes(testing.NewDeleteActionWithMultiTenancy(c.resource, c.namespace, name, c.tenant), &metav1.Status{Status: "dynamic delete fail"})

	case len(c.tenant) > 0 && len(c.namespace) > 0 && len(subresources) > 0:
		_, err = c.client.Fake.
			Invokes(testing.NewDeleteSubresourceActionWithMultiTenancy(c.resource, strings.Join(subresources, "/"), c.namespace, name, c.tenant), &metav1.Status{Status: "dynamic delete fail"})

	case len(c.tenant) == 0 && len(c.namespace) > 0:
		return fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	return err
}

func (c *dynamicResourceClient) DeleteCollection(opts *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	var err error
	switch {
	case len(c.tenant) == 0 && len(c.namespace) == 0:
		action := testing.NewRootDeleteCollectionAction(c.resource, listOptions)
		_, err = c.client.Fake.Invokes(action, &metav1.Status{Status: "dynamic deletecollection fail"})

	case len(c.tenant) > 0 && len(c.namespace) == 0:
		action := testing.NewTenantDeleteCollectionAction(c.resource, listOptions, c.tenant)
		_, err = c.client.Fake.Invokes(action, &metav1.Status{Status: "dynamic deletecollection fail"})

	case len(c.tenant) > 0 && len(c.namespace) > 0:
		action := testing.NewDeleteCollectionActionWithMultiTenancy(c.resource, c.namespace, listOptions, c.tenant)
		_, err = c.client.Fake.Invokes(action, &metav1.Status{Status: "dynamic deletecollection fail"})

	case len(c.tenant) == 0 && len(c.namespace) > 0:
		return fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	return err
}

func (c *dynamicResourceClient) Get(name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	var uncastRet runtime.Object
	var err error
	switch {
	case len(c.tenant) == 0 && len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewRootGetAction(c.resource, name), &metav1.Status{Status: "dynamic get fail"})

	case len(c.tenant) == 0 && len(c.namespace) == 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewRootGetSubresourceAction(c.resource, strings.Join(subresources, "/"), name), &metav1.Status{Status: "dynamic get fail"})

	case len(c.tenant) > 0 && len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewTenantGetAction(c.resource, name, c.tenant), &metav1.Status{Status: "dynamic get fail"})

	case len(c.tenant) > 0 && len(c.namespace) == 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewTenantGetSubresourceAction(c.resource, strings.Join(subresources, "/"), name, c.tenant), &metav1.Status{Status: "dynamic get fail"})

	case len(c.tenant) > 0 && len(c.namespace) > 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewGetActionWithMultiTenancy(c.resource, c.namespace, name, c.tenant), &metav1.Status{Status: "dynamic get fail"})

	case len(c.tenant) > 0 && len(c.namespace) > 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewGetSubresourceActionWithMultiTenancy(c.resource, c.namespace, strings.Join(subresources, "/"), name, c.tenant), &metav1.Status{Status: "dynamic get fail"})

	case len(c.tenant) == 0 && len(c.namespace) > 0:
		return nil, fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	if err != nil {
		return nil, err
	}
	if uncastRet == nil {
		return nil, err
	}

	ret := &unstructured.Unstructured{}
	if err := c.client.scheme.Convert(uncastRet, ret, nil); err != nil {
		return nil, err
	}
	return ret, err
}

func (c *dynamicResourceClient) List(opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	var obj runtime.Object
	var err error
	switch {
	case len(c.tenant) == 0 && len(c.namespace) == 0:
		obj, err = c.client.Fake.
			Invokes(testing.NewRootListAction(c.resource, schema.GroupVersionKind{Group: "fake-dynamic-client-group", Version: "v1", Kind: "" /*List is appended by the tracker automatically*/}, opts), &metav1.Status{Status: "dynamic list fail"})

	case len(c.tenant) > 0 && len(c.namespace) == 0:
		obj, err = c.client.Fake.
			Invokes(testing.NewTenantListAction(c.resource, schema.GroupVersionKind{Group: "fake-dynamic-client-group", Version: "v1", Kind: "" /*List is appended by the tracker automatically*/}, opts, c.tenant), &metav1.Status{Status: "dynamic list fail"})

	case len(c.tenant) > 0 && len(c.namespace) > 0:
		obj, err = c.client.Fake.
			Invokes(testing.NewListActionWithMultiTenancy(c.resource, schema.GroupVersionKind{Group: "fake-dynamic-client-group", Version: "v1", Kind: "" /*List is appended by the tracker automatically*/}, c.namespace, opts, c.tenant), &metav1.Status{Status: "dynamic list fail"})

	case len(c.tenant) == 0 && len(c.namespace) > 0:
		return nil, fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}

	retUnstructured := &unstructured.Unstructured{}
	if err := c.client.scheme.Convert(obj, retUnstructured, nil); err != nil {
		return nil, err
	}
	entireList, err := retUnstructured.ToList()
	if err != nil {
		return nil, err
	}

	list := &unstructured.UnstructuredList{}
	list.SetResourceVersion(entireList.GetResourceVersion())
	for i := range entireList.Items {
		item := &entireList.Items[i]
		metadata, err := meta.Accessor(item)
		if err != nil {
			return nil, err
		}
		if label.Matches(labels.Set(metadata.GetLabels())) {
			list.Items = append(list.Items, *item)
		}
	}
	return list, nil
}

func (c *dynamicResourceClient) Watch(opts metav1.ListOptions) watch.AggregatedWatchInterface {
	aggWatch := watch.NewAggregatedWatcher()

	switch {
	case len(c.tenant) == 0 && len(c.namespace) == 0:
		aggWatch.AddWatchInterface(c.client.Fake.
			InvokesWatch(testing.NewRootWatchAction(c.resource, opts)))

	case len(c.tenant) > 0 && len(c.namespace) == 0:
		aggWatch.AddWatchInterface(c.client.Fake.
			InvokesWatch(testing.NewTenantWatchAction(c.resource, opts, c.tenant)))

	case len(c.tenant) > 0 && len(c.namespace) > 0:
		aggWatch.AddWatchInterface(c.client.Fake.
			InvokesWatch(testing.NewWatchActionWithMultiTenancy(c.resource, c.namespace, opts, c.tenant)))

	case len(c.tenant) == 0 && len(c.namespace) > 0:
		aggWatch.AddWatchInterface(nil, fmt.Errorf("namespace is not-empty but tenant is empty"))
	default:
		panic("math broke")
	}

	return aggWatch
}

// TODO: opts are currently ignored.
func (c *dynamicResourceClient) Patch(name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	var uncastRet runtime.Object
	var err error
	switch {
	case len(c.tenant) == 0 && len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewRootPatchAction(c.resource, name, pt, data), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.tenant) == 0 && len(c.namespace) == 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewRootPatchSubresourceAction(c.resource, name, pt, data, subresources...), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.tenant) > 0 && len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewTenantPatchAction(c.resource, name, pt, data, c.tenant), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.tenant) > 0 && len(c.namespace) == 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewTenantPatchSubresourceAction(c.resource, c.tenant, name, pt, data, subresources...), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.tenant) > 0 && len(c.namespace) > 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewPatchActionWithMultiTenancy(c.resource, c.namespace, name, pt, data, c.tenant), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.tenant) > 0 && len(c.namespace) > 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(testing.NewPatchSubresourceActionWithMultiTenancy(c.resource, c.tenant, c.namespace, name, pt, data, subresources...), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.tenant) == 0 && len(c.namespace) > 0:
		return nil, fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	if err != nil {
		return nil, err
	}
	if uncastRet == nil {
		return nil, err
	}

	ret := &unstructured.Unstructured{}
	if err := c.client.scheme.Convert(uncastRet, ret, nil); err != nil {
		return nil, err
	}
	return ret, err
}
