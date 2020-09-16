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

// Code generated by lister-gen. DO NOT EDIT.

package v1beta1

import (
	v1beta1 "k8s.io/api/authorization/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// SelfSubjectAccessReviewLister helps list SelfSubjectAccessReviews.
type SelfSubjectAccessReviewLister interface {
	// List lists all SelfSubjectAccessReviews in the indexer.
	List(selector labels.Selector) (ret []*v1beta1.SelfSubjectAccessReview, err error)
	// SelfSubjectAccessReviews returns an object that can list and get SelfSubjectAccessReviews.
	SelfSubjectAccessReviews() SelfSubjectAccessReviewTenantLister
	SelfSubjectAccessReviewsWithMultiTenancy(tenant string) SelfSubjectAccessReviewTenantLister
	// Get retrieves the SelfSubjectAccessReview from the index for a given name.
	Get(name string) (*v1beta1.SelfSubjectAccessReview, error)
	SelfSubjectAccessReviewListerExpansion
}

// selfSubjectAccessReviewLister implements the SelfSubjectAccessReviewLister interface.
type selfSubjectAccessReviewLister struct {
	indexer cache.Indexer
}

// NewSelfSubjectAccessReviewLister returns a new SelfSubjectAccessReviewLister.
func NewSelfSubjectAccessReviewLister(indexer cache.Indexer) SelfSubjectAccessReviewLister {
	return &selfSubjectAccessReviewLister{indexer: indexer}
}

// List lists all SelfSubjectAccessReviews in the indexer.
func (s *selfSubjectAccessReviewLister) List(selector labels.Selector) (ret []*v1beta1.SelfSubjectAccessReview, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.SelfSubjectAccessReview))
	})
	return ret, err
}

// Get retrieves the SelfSubjectAccessReview from the index for a given name.
func (s *selfSubjectAccessReviewLister) Get(name string) (*v1beta1.SelfSubjectAccessReview, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("selfsubjectaccessreview"), name)
	}
	return obj.(*v1beta1.SelfSubjectAccessReview), nil
}

// SelfSubjectAccessReviews returns an object that can list and get SelfSubjectAccessReviews.
func (s *selfSubjectAccessReviewLister) SelfSubjectAccessReviews() SelfSubjectAccessReviewTenantLister {
	return selfSubjectAccessReviewTenantLister{indexer: s.indexer, tenant: ""}
}

func (s *selfSubjectAccessReviewLister) SelfSubjectAccessReviewsWithMultiTenancy(tenant string) SelfSubjectAccessReviewTenantLister {
	return selfSubjectAccessReviewTenantLister{indexer: s.indexer, tenant: tenant}
}

// SelfSubjectAccessReviewTenantLister helps list and get SelfSubjectAccessReviews.
type SelfSubjectAccessReviewTenantLister interface {
	// List lists all SelfSubjectAccessReviews in the indexer for a given tenant/tenant.
	List(selector labels.Selector) (ret []*v1beta1.SelfSubjectAccessReview, err error)
	// Get retrieves the SelfSubjectAccessReview from the indexer for a given tenant/tenant and name.
	Get(name string) (*v1beta1.SelfSubjectAccessReview, error)
	SelfSubjectAccessReviewTenantListerExpansion
}

// selfSubjectAccessReviewTenantLister implements the SelfSubjectAccessReviewTenantLister
// interface.
type selfSubjectAccessReviewTenantLister struct {
	indexer cache.Indexer
	tenant  string
}

// List lists all SelfSubjectAccessReviews in the indexer for a given tenant.
func (s selfSubjectAccessReviewTenantLister) List(selector labels.Selector) (ret []*v1beta1.SelfSubjectAccessReview, err error) {
	err = cache.ListAllByTenant(s.indexer, s.tenant, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.SelfSubjectAccessReview))
	})
	return ret, err
}

// Get retrieves the SelfSubjectAccessReview from the indexer for a given tenant and name.
func (s selfSubjectAccessReviewTenantLister) Get(name string) (*v1beta1.SelfSubjectAccessReview, error) {
	key := s.tenant + "/" + name
	if s.tenant == "system" {
		key = name
	}
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("selfsubjectaccessreview"), name)
	}
	return obj.(*v1beta1.SelfSubjectAccessReview), nil
}
