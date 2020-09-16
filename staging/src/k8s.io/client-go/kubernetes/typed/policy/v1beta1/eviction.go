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
	rest "k8s.io/client-go/rest"
)

// EvictionsGetter has a method to return a EvictionInterface.
// A group's client should implement this interface.
type EvictionsGetter interface {
	Evictions(namespace string) EvictionInterface
	EvictionsWithMultiTenancy(namespace string, tenant string) EvictionInterface
}

// EvictionInterface has methods to work with Eviction resources.
type EvictionInterface interface {
	EvictionExpansion
}

// evictions implements EvictionInterface
type evictions struct {
	client  rest.Interface
	clients []rest.Interface
	ns      string
	te      string
}

// newEvictions returns a Evictions
func newEvictions(c *PolicyV1beta1Client, namespace string) *evictions {
	return newEvictionsWithMultiTenancy(c, namespace, "")
}

func newEvictionsWithMultiTenancy(c *PolicyV1beta1Client, namespace string, tenant string) *evictions {
	return &evictions{
		client:  c.RESTClient(),
		clients: c.RESTClients(),
		ns:      namespace,
		te:      tenant,
	}
}
