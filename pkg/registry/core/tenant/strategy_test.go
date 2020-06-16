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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	api "k8s.io/kubernetes/pkg/apis/core"

	// install all api groups for testing
	_ "k8s.io/kubernetes/pkg/api/testapi"
)

func TestTenantStrategy(t *testing.T) {
	ctx := genericapirequest.NewDefaultContext()
	if Strategy.NamespaceScoped() {
		t.Errorf("Tenants should not be namespace scoped")
	}
	if Strategy.TenantScoped() {
		t.Errorf("Tenants should not be tenant scoped")
	}
	if Strategy.AllowCreateOnUpdate() {
		t.Errorf("Tenants should not allow create on update")
	}
	tenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "10"},
		Spec: api.TenantSpec{
			StorageClusterId: "1",
		},
		Status: api.TenantStatus{Phase: api.TenantTerminating},
	}
	Strategy.PrepareForCreate(ctx, tenant)
	if tenant.Status.Phase != api.TenantActive {
		t.Errorf("Tenants do not allow setting phase on create")
	}
	if len(tenant.Spec.Finalizers) != 1 || tenant.Spec.Finalizers[0] != api.FinalizerArktos {
		t.Errorf("Prepare For Create should have added kubernetes finalizer")
	}
	errs := Strategy.Validate(ctx, tenant)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}
	invalidTenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{Name: "bar", ResourceVersion: "4"},
	}
	// ensure we copy spec.finalizers from old to new
	Strategy.PrepareForUpdate(ctx, invalidTenant, tenant)
	if len(invalidTenant.Spec.Finalizers) != 1 || invalidTenant.Spec.Finalizers[0] != api.FinalizerArktos {
		t.Errorf("PrepareForUpdate should have preserved old.spec.finalizers")
	}
	errs = Strategy.ValidateUpdate(ctx, invalidTenant, tenant)
	if len(errs) == 0 {
		t.Errorf("Expected a validation error")
	}
	if invalidTenant.ResourceVersion != "4" {
		t.Errorf("Incoming resource version on update should not be mutated")
	}
}

func TestTenantStatusStrategy(t *testing.T) {
	ctx := genericapirequest.NewDefaultContext()
	if StatusStrategy.NamespaceScoped() {
		t.Errorf("Tenants should not be namespace scoped")
	}
	if StatusStrategy.TenantScoped() {
		t.Errorf("Tenants should not be tenant scoped")
	}
	if StatusStrategy.AllowCreateOnUpdate() {
		t.Errorf("Tenants should not allow create on update")
	}
	now := metav1.Now()
	oldTenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "10", DeletionTimestamp: &now},
		Spec:       api.TenantSpec{Finalizers: []api.FinalizerName{"arktos"}},
		Status:     api.TenantStatus{Phase: api.TenantActive},
	}
	tenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "9", DeletionTimestamp: &now},
		Status:     api.TenantStatus{Phase: api.TenantTerminating},
	}
	StatusStrategy.PrepareForUpdate(ctx, tenant, oldTenant)
	if tenant.Status.Phase != api.TenantTerminating {
		t.Errorf("Tenant status updates should allow change of phase: %v", tenant.Status.Phase)
	}
	if len(tenant.Spec.Finalizers) != 1 || tenant.Spec.Finalizers[0] != api.FinalizerArktos {
		t.Errorf("PrepareForUpdate should have preserved old finalizers")
	}
	errs := StatusStrategy.ValidateUpdate(ctx, tenant, oldTenant)
	if len(errs) != 0 {
		t.Errorf("Unexpected error %v", errs)
	}
	if tenant.ResourceVersion != "9" {
		t.Errorf("Incoming resource version on update should not be mutated")
	}
}

func TestTenantFinalizeStrategy(t *testing.T) {
	ctx := genericapirequest.NewDefaultContext()
	if FinalizeStrategy.NamespaceScoped() {
		t.Errorf("Tenants should not be namespace scoped")
	}
	if FinalizeStrategy.TenantScoped() {
		t.Errorf("Tenants should not be tenant scoped")
	}
	if FinalizeStrategy.AllowCreateOnUpdate() {
		t.Errorf("Tenants should not allow create on update")
	}
	oldTenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "10"},
		Spec:       api.TenantSpec{Finalizers: []api.FinalizerName{"arktos", "example.com/org"}},
		Status:     api.TenantStatus{Phase: api.TenantActive},
	}
	tenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", ResourceVersion: "9"},
		Spec:       api.TenantSpec{Finalizers: []api.FinalizerName{"example.com/foo"}},
		Status:     api.TenantStatus{Phase: api.TenantTerminating},
	}
	FinalizeStrategy.PrepareForUpdate(ctx, tenant, oldTenant)
	if tenant.Status.Phase != api.TenantActive {
		t.Errorf("finalize updates should not allow change of phase: %v", tenant.Status.Phase)
	}
	if len(tenant.Spec.Finalizers) != 1 || string(tenant.Spec.Finalizers[0]) != "example.com/foo" {
		t.Errorf("PrepareForUpdate should have modified finalizers")
	}
	errs := StatusStrategy.ValidateUpdate(ctx, tenant, oldTenant)
	if len(errs) != 0 {
		t.Errorf("Unexpected error %v", errs)
	}
	if tenant.ResourceVersion != "9" {
		t.Errorf("Incoming resource version on update should not be mutated")
	}
}

func TestSelectableFieldLabelConversions(t *testing.T) {
	apitesting.TestSelectableFieldLabelConversionsOfKind(t,
		"v1",
		"Tenant",
		TenantToSelectableFields(&api.Tenant{}),
		map[string]string{"name": "metadata.name"},
	)
}
