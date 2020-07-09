/*
Copyright 2017 The Kubernetes Authors.
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

// Package fake provides a fake client interface to arbitrary Kubernetes
// APIs that exposes common high level operations and exposes common
// metadata.
package fake

import (
	autoscalingapi "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/scale"
	"k8s.io/client-go/testing"
)

// FakeScaleClient provides a fake implementation of scale.ScalesGetter.
type FakeScaleClient struct {
	testing.Fake
}

func (f *FakeScaleClient) Scales(namespace string) scale.ScaleInterface {
	return f.ScalesWithMultiTenancy(namespace, metav1.TenantSystem)
}

func (f *FakeScaleClient) ScalesWithMultiTenancy(namespace, tenant string) scale.ScaleInterface {
	return &fakeNamespacedScaleClient{
		tenant:    tenant,
		namespace: namespace,
		fake:      &f.Fake,
	}
}

type fakeNamespacedScaleClient struct {
	tenant    string
	namespace string
	fake      *testing.Fake
}

func (f *fakeNamespacedScaleClient) Get(resource schema.GroupResource, name string) (*autoscalingapi.Scale, error) {
	obj, err := f.fake.
		Invokes(testing.NewGetSubresourceActionWithMultiTenancy(resource.WithVersion(""), f.namespace, "scale", name, f.tenant), &autoscalingapi.Scale{})

	if err != nil {
		return nil, err
	}

	return obj.(*autoscalingapi.Scale), err
}

func (f *fakeNamespacedScaleClient) Update(resource schema.GroupResource, scale *autoscalingapi.Scale) (*autoscalingapi.Scale, error) {
	obj, err := f.fake.
		Invokes(testing.NewUpdateSubresourceActionWithMultiTenancy(resource.WithVersion(""), f.namespace, "scale", scale, f.tenant), &autoscalingapi.Scale{})

	if err != nil {
		return nil, err
	}

	return obj.(*autoscalingapi.Scale), err

}
