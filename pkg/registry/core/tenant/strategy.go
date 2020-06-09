/*
Copyright 2020 Authors of Arktos.

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

package tenant

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	apistorage "k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/validation"
)

// tenantStrategy implements behavior for Tenants
type tenantStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Tenant
// objects via the REST API.
var Strategy = tenantStrategy{legacyscheme.Scheme, names.SimpleNameGenerator}

// NamespaceScoped is false for tenants.
func (tenantStrategy) NamespaceScoped() bool {
	return false
}

//TenantScoped is false as it is cluster-scoped
func (tenantStrategy) TenantScoped() bool {
	return false
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (tenantStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	// on create, status is active
	tenant := obj.(*api.Tenant)
	tenant.Status = api.TenantStatus{
		Phase: api.TenantActive,
	}
	// on create, we require the kubernetes value
	// we cannot use this in defaults conversion because we let it get removed over life of object
	hasKubeFinalizer := false
	for i := range tenant.Spec.Finalizers {
		if tenant.Spec.Finalizers[i] == api.FinalizerArktos {
			hasKubeFinalizer = true
			break
		}
	}
	if !hasKubeFinalizer {
		if len(tenant.Spec.Finalizers) == 0 {
			tenant.Spec.Finalizers = []api.FinalizerName{api.FinalizerArktos}
		} else {
			tenant.Spec.Finalizers = append(tenant.Spec.Finalizers, api.FinalizerArktos)
		}
	}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (tenantStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newTenant := obj.(*api.Tenant)
	oldTenant := old.(*api.Tenant)
	newTenant.Spec.Finalizers = oldTenant.Spec.Finalizers
	newTenant.Status = oldTenant.Status
}

// Validate validates a new tenant.
func (tenantStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	tenant := obj.(*api.Tenant)
	return validation.ValidateTenant(tenant)
}

// Canonicalize normalizes the object after validation.
func (tenantStrategy) Canonicalize(obj runtime.Object) {
}

// AllowCreateOnUpdate is false for tenants.
func (tenantStrategy) AllowCreateOnUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (tenantStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	errorList := validation.ValidateTenant(obj.(*api.Tenant))
	return append(errorList, validation.ValidateTenantUpdate(obj.(*api.Tenant), old.(*api.Tenant))...)
}

func (tenantStrategy) AllowUnconditionalUpdate() bool {
	return true
}

type tenantStatusStrategy struct {
	tenantStrategy
}

var StatusStrategy = tenantStatusStrategy{Strategy}

func (tenantStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newTenant := obj.(*api.Tenant)
	oldTenant := old.(*api.Tenant)
	newTenant.Spec = oldTenant.Spec
}

func (tenantStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateTenantStatusUpdate(obj.(*api.Tenant), old.(*api.Tenant))
}

type tenantFinalizeStrategy struct {
	tenantStrategy
}

var FinalizeStrategy = tenantFinalizeStrategy{Strategy}

func (tenantFinalizeStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateTenantFinalizeUpdate(obj.(*api.Tenant), old.(*api.Tenant))
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (tenantFinalizeStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newTenant := obj.(*api.Tenant)
	oldTenant := old.(*api.Tenant)
	newTenant.Status = oldTenant.Status
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	tenantObj, ok := obj.(*api.Tenant)
	if !ok {
		return nil, nil, fmt.Errorf("not a tenant")
	}
	return labels.Set(tenantObj.Labels), TenantToSelectableFields(tenantObj), nil
}

// MatchTenant returns a generic matcher for a given label and field selector.
func MatchTenant(label labels.Selector, field fields.Selector) apistorage.SelectionPredicate {
	return apistorage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// TenantToSelectableFields returns a field set that represents the object
func TenantToSelectableFields(tenant *api.Tenant) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&tenant.ObjectMeta, false)
	specificFieldsSet := fields.Set{
		"status.phase": string(tenant.Status.Phase),
		// This is a bug, but we need to support it for backward compatibility.
		"name": tenant.Name,
	}
	return generic.MergeFieldsSets(objectMetaFieldsSet, specificFieldsSet)
}
