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

package internalversion

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	crdregistry "k8s.io/apiextensions-apiserver/pkg/registry/customresourcedefinition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CustomResourceDefinitionListerExpansion allows custom methods to be added to
// CustomResourceDefinitionLister.
type CustomResourceDefinitionListerExpansion interface {
}

// CustomResourceDefinitionNamespaceListerExpansion allows custom methods to be added to
// CustomResourceDefinitionNamespaceLister.
type CustomResourceDefinitionTenantListerExpansion interface {
	// Get retrieves the CustomResourceDefinition from the indexer for a given tenant and name.
	GetAccessibleCrd(name string) (*apiextensions.CustomResourceDefinition, error)
}

// GetAccessibleCrd tries to retrieve the forced version of CRD under the system tenant first.
// If not found, try the search under the tenant.
func (s customResourceDefinitionTenantLister) GetAccessibleCrd(name string) (*apiextensions.CustomResourceDefinition, error) {
	if s.tenant == metav1.TenantSystem {
		return s.Get(name)
	}

	// try to get the system CRD of the given name. System_Tenant_CD_Key = name.
	sysObj, exists, err := s.indexer.GetByKey(name)
	if exists && err == nil {
		if sysCrd, ok := sysObj.(*apiextensions.CustomResourceDefinition); ok {
			if crdregistry.IsCrdSystemForced(sysCrd) {
				return sysCrd, nil
			}
		}
	}

	return s.Get(name)
}
