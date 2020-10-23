/*
Copyright 2014 The Kubernetes Authors.
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

package resource

import (
	"fmt"
	"math/rand"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// Selector is a Visitor for resources that match a label selector.
type Selector struct {
	Clients       []RESTClient
	Client        RESTClient
	Mapping       *meta.RESTMapping
	Tenant        string
	Namespace     string
	LabelSelector string
	FieldSelector string
	Export        bool
	LimitChunks   int64
}

// NewSelector creates a resource selector which hides details of getting items by their label selector.
func NewSelector(clients []RESTClient, mapping *meta.RESTMapping, tenant, namespace, labelSelector, fieldSelector string, export bool, limitChunks int64) *Selector {
	s := &Selector{
		Clients:       clients,
		Mapping:       mapping,
		Tenant:        tenant,
		Namespace:     namespace,
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
		Export:        export,
		LimitChunks:   limitChunks,
	}

	max := len(clients)
	if max == 1 {
		s.Client = clients[0]
	}

	if max > 1 {
		rand.Seed(time.Now().UnixNano())
		ran := rand.Intn(max)
		s.Client = clients[ran]
	}

	return s
}

// Visit implements Visitor and uses request chunking by default.
func (r *Selector) Visit(fn VisitorFunc) error {
	var continueToken string
	for {
		list, err := NewHelper(r.Clients, r.Mapping).ListWithMultiTenancy(
			r.Tenant,
			r.Namespace,
			r.ResourceMapping().GroupVersionKind.GroupVersion().String(),
			r.Export,
			&metav1.ListOptions{
				LabelSelector: r.LabelSelector,
				FieldSelector: r.FieldSelector,
				Limit:         r.LimitChunks,
				Continue:      continueToken,
			},
		)
		if err != nil {
			if errors.IsResourceExpired(err) {
				return err
			}
			if errors.IsBadRequest(err) || errors.IsNotFound(err) {
				if se, ok := err.(*errors.StatusError); ok {
					// modify the message without hiding this is an API error
					if len(r.LabelSelector) == 0 && len(r.FieldSelector) == 0 {
						se.ErrStatus.Message = fmt.Sprintf("Unable to list %q: %v", r.Mapping.Resource, se.ErrStatus.Message)
					} else {
						se.ErrStatus.Message = fmt.Sprintf("Unable to find %q that match label selector %q, field selector %q: %v", r.Mapping.Resource, r.LabelSelector, r.FieldSelector, se.ErrStatus.Message)
					}
					return se
				}
				if len(r.LabelSelector) == 0 && len(r.FieldSelector) == 0 {
					return fmt.Errorf("Unable to list %q: %v", r.Mapping.Resource, err)
				}
				return fmt.Errorf("Unable to find %q that match label selector %q, field selector %q: %v", r.Mapping.Resource, r.LabelSelector, r.FieldSelector, err)
			}
			return err
		}
		resourceVersion, _ := metadataAccessor.ResourceVersion(list)
		nextContinueToken, _ := metadataAccessor.Continue(list)
		info := &Info{
			Clients:         r.Clients,
			Mapping:         r.Mapping,
			Tenant:          r.Tenant,
			Namespace:       r.Namespace,
			ResourceVersion: resourceVersion,

			Object: list,
		}

		if err := fn(info, nil); err != nil {
			return err
		}
		if len(nextContinueToken) == 0 {
			return nil
		}
		continueToken = nextContinueToken
	}
}

func (r *Selector) Watch(resourceVersion string) (watch.Interface, error) {
	return NewHelper(r.Clients, r.Mapping).Watch(r.Namespace, r.ResourceMapping().GroupVersionKind.GroupVersion().String(),
		&metav1.ListOptions{ResourceVersion: resourceVersion, LabelSelector: r.LabelSelector, FieldSelector: r.FieldSelector})
}

func (r *Selector) WatchWithMultiTenancy(resourceVersion string) (watch.Interface, error) {
	return NewHelper(r.Clients, r.Mapping).WatchWithMultiTenancy(r.Tenant, r.Namespace, r.ResourceMapping().GroupVersionKind.GroupVersion().String(),
		&metav1.ListOptions{ResourceVersion: resourceVersion, LabelSelector: r.LabelSelector, FieldSelector: r.FieldSelector})
}

// ResourceMapping returns the mapping for this resource and implements ResourceMapping
func (r *Selector) ResourceMapping() *meta.RESTMapping {
	return r.Mapping
}
