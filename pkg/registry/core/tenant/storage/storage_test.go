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

package storage

import (
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistrytest "k8s.io/apiserver/pkg/registry/generic/testing"
	"k8s.io/apiserver/pkg/registry/rest"
	etcd3testing "k8s.io/apiserver/pkg/storage/etcd3/testing"

	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/registry/registrytest"
)

func newStorage(t *testing.T) (*REST, *etcd3testing.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	restOptions := generic.RESTOptions{StorageConfig: etcdStorage, Decorator: generic.UndecoratedStorage, DeleteCollectionWorkers: 1, ResourcePrefix: "tenants"}
	tenantStorage, _, _ := NewREST(restOptions)
	return tenantStorage, server
}

func validNewTenant() *api.Tenant {
	return &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: api.TenantSpec{
			StorageClusterId: "1",
		},
	}
}

func TestCreate(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	test := genericregistrytest.New(t, storage.store).ClusterScope()
	tenant := validNewTenant()
	tenant.ObjectMeta = metav1.ObjectMeta{GenerateName: "foo"}
	test.TestCreate(
		// valid
		tenant,
		// invalid
		&api.Tenant{
			ObjectMeta: metav1.ObjectMeta{Name: "bad value"},
			Spec: api.TenantSpec{
				StorageClusterId: "1",
			},
		},
	)
}

func TestCreateSetsFields(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	tenant := validNewTenant()
	ctx := genericapirequest.NewContext()
	_, err := storage.Create(ctx, tenant, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	object, err := storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actual := object.(*api.Tenant)
	if actual.Name != tenant.Name {
		t.Errorf("unexpected tenant: %#v", actual)
	}
	if len(actual.UID) == 0 {
		t.Errorf("expected tenant UID to be set: %#v", actual)
	}
	if actual.Status.Phase != api.TenantActive {
		t.Errorf("expected tenant phase to be set to active, but %v", actual.Status.Phase)
	}
}

func TestDelete(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	test := genericregistrytest.New(t, storage.store).ClusterScope().ReturnDeletedObject()
	test.TestDelete(validNewTenant())
}

func TestGet(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	test := genericregistrytest.New(t, storage.store).ClusterScope()
	test.TestGet(validNewTenant())
}

func TestList(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	test := genericregistrytest.New(t, storage.store).ClusterScope()
	test.TestList(validNewTenant())
}

func TestWatch(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	test := genericregistrytest.New(t, storage.store).ClusterScope()
	test.TestWatch(
		validNewTenant(),
		// matching labels
		[]labels.Set{},
		// not matching labels
		[]labels.Set{
			{"foo": "bar"},
		},
		// matching fields
		[]fields.Set{
			{"metadata.name": "foo"},
			{"name": "foo"},
		},
		// not matching fields
		[]fields.Set{
			{"metadata.name": "bar"},
		},
	)
}

func TestDeleteTenantWithIncompleteFinalizers(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	key := "tenants/foo"
	ctx := genericapirequest.NewContext()
	now := metav1.Now()
	tenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "foo",
			DeletionTimestamp: &now,
		},
		Spec: api.TenantSpec{
			StorageClusterId: "1",
			Finalizers:       []api.FinalizerName{api.FinalizerKubernetes},
		},
		Status: api.TenantStatus{Phase: api.TenantActive},
	}
	if err := storage.store.Storage.Create(ctx, key, tenant, nil, 0, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := storage.Delete(ctx, "foo", rest.ValidateAllObjectFunc, nil); err == nil {
		t.Errorf("unexpected no error")
	}
	// should still exist
	_, err := storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateDeletingTenantWithIncompleteMetadataFinalizers(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	key := "tenants/foo"
	ctx := genericapirequest.NewContext()
	now := metav1.Now()
	tenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "foo",
			DeletionTimestamp: &now,
			Finalizers:        []string{"example.com/foo"},
		},
		Spec: api.TenantSpec{
			StorageClusterId: "1",
			Finalizers:       []api.FinalizerName{},
		},
		Status: api.TenantStatus{Phase: api.TenantActive},
	}
	if err := storage.store.Storage.Create(ctx, key, tenant, nil, 0, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ns, err := storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err = storage.Update(ctx, "foo", rest.DefaultUpdatedObjectInfo(ns), rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc, false, &metav1.UpdateOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// should still exist
	_, err = storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateDeletingTenantWithIncompleteSpecFinalizers(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	key := "tenants/foo"
	ctx := genericapirequest.NewContext()
	now := metav1.Now()
	tenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "foo",
			DeletionTimestamp: &now,
		},
		Spec: api.TenantSpec{
			StorageClusterId: "1",
			Finalizers:       []api.FinalizerName{api.FinalizerKubernetes},
		},
		Status: api.TenantStatus{Phase: api.TenantActive},
	}
	if err := storage.store.Storage.Create(ctx, key, tenant, nil, 0, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ns, err := storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err = storage.Update(ctx, "foo", rest.DefaultUpdatedObjectInfo(ns), rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc, false, &metav1.UpdateOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// should still exist
	_, err = storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateDeletingTenantWithCompleteFinalizers(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	key := "tenants/foo"
	ctx := genericapirequest.NewContext()
	now := metav1.Now()
	tenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "foo",
			DeletionTimestamp: &now,
			Finalizers:        []string{"example.com/foo"},
		},
		Spec: api.TenantSpec{
			StorageClusterId: "1",
		},
		Status: api.TenantStatus{Phase: api.TenantActive},
	}
	if err := storage.store.Storage.Create(ctx, key, tenant, nil, 0, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ns, err := storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ns.(*api.Tenant).Finalizers = nil
	if _, _, err = storage.Update(ctx, "foo", rest.DefaultUpdatedObjectInfo(ns), rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc, false, &metav1.UpdateOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// should not exist
	_, err = storage.Get(ctx, "foo", &metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestFinalizeDeletingTenantWithCompleteFinalizers(t *testing.T) {
	// get finalize storage
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	restOptions := generic.RESTOptions{StorageConfig: etcdStorage, Decorator: generic.UndecoratedStorage, DeleteCollectionWorkers: 1, ResourcePrefix: "tenants"}
	storage, _, finalizeStorage := NewREST(restOptions)

	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	defer finalizeStorage.store.DestroyFunc()
	key := "tenants/foo"
	ctx := genericapirequest.NewContext()
	now := metav1.Now()
	tenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "foo",
			DeletionTimestamp: &now,
		},
		Spec: api.TenantSpec{
			StorageClusterId: "1",
			Finalizers:       []api.FinalizerName{api.FinalizerKubernetes},
		},
		Status: api.TenantStatus{Phase: api.TenantActive},
	}
	if err := storage.store.Storage.Create(ctx, key, tenant, nil, 0, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ns, err := storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ns.(*api.Tenant).Spec.Finalizers = nil
	if _, _, err = finalizeStorage.Update(ctx, "foo", rest.DefaultUpdatedObjectInfo(ns), rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc, false, &metav1.UpdateOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// should not exist
	_, err = storage.Get(ctx, "foo", &metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestFinalizeDeletingTenantWithIncompleteMetadataFinalizers(t *testing.T) {
	// get finalize storage
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	restOptions := generic.RESTOptions{StorageConfig: etcdStorage, Decorator: generic.UndecoratedStorage, DeleteCollectionWorkers: 1, ResourcePrefix: "tenants"}
	storage, _, finalizeStorage := NewREST(restOptions)

	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	defer finalizeStorage.store.DestroyFunc()
	key := "tenants/foo"
	ctx := genericapirequest.NewContext()
	now := metav1.Now()
	tenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "foo",
			DeletionTimestamp: &now,
			Finalizers:        []string{"example.com/foo"},
		},
		Spec: api.TenantSpec{
			StorageClusterId: "1",
			Finalizers:       []api.FinalizerName{api.FinalizerKubernetes},
		},
		Status: api.TenantStatus{Phase: api.TenantActive},
	}
	if err := storage.store.Storage.Create(ctx, key, tenant, nil, 0, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ns, err := storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ns.(*api.Tenant).Spec.Finalizers = nil
	if _, _, err = finalizeStorage.Update(ctx, "foo", rest.DefaultUpdatedObjectInfo(ns), rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc, false, &metav1.UpdateOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// should still exist
	_, err = storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteTenantWithCompleteFinalizers(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	key := "tenants/foo"
	ctx := genericapirequest.NewContext()
	now := metav1.Now()
	tenant := &api.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "foo",
			DeletionTimestamp: &now,
		},
		Spec: api.TenantSpec{
			StorageClusterId: "1",
			Finalizers:       []api.FinalizerName{},
		},
		Status: api.TenantStatus{Phase: api.TenantActive},
	}
	if err := storage.store.Storage.Create(ctx, key, tenant, nil, 0, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := storage.Delete(ctx, "foo", rest.ValidateAllObjectFunc, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// should not exist
	_, err := storage.Get(ctx, "foo", &metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestDeleteWithGCFinalizers(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()

	propagationBackground := metav1.DeletePropagationBackground
	propagationForeground := metav1.DeletePropagationForeground
	propagationOrphan := metav1.DeletePropagationOrphan
	trueVar := true

	var tests = []struct {
		name          string
		deleteOptions *metav1.DeleteOptions

		existingFinalizers  []string
		remainingFinalizers map[string]bool
	}{
		{
			name:          "nil-with-orphan",
			deleteOptions: nil,
			existingFinalizers: []string{
				metav1.FinalizerOrphanDependents,
			},
			remainingFinalizers: map[string]bool{
				metav1.FinalizerOrphanDependents: true,
			},
		},
		{
			name:          "nil-with-delete",
			deleteOptions: nil,
			existingFinalizers: []string{
				metav1.FinalizerDeleteDependents,
			},
			remainingFinalizers: map[string]bool{
				metav1.FinalizerDeleteDependents: true,
			},
		},
		{
			name:                "nil-without-finalizers",
			deleteOptions:       nil,
			existingFinalizers:  []string{},
			remainingFinalizers: map[string]bool{},
		},
		{
			name: "propagation-background-with-orphan",
			deleteOptions: &metav1.DeleteOptions{
				PropagationPolicy: &propagationBackground,
			},
			existingFinalizers: []string{
				metav1.FinalizerOrphanDependents,
			},
			remainingFinalizers: map[string]bool{},
		},
		{
			name: "propagation-background-with-delete",
			deleteOptions: &metav1.DeleteOptions{
				PropagationPolicy: &propagationBackground,
			},
			existingFinalizers: []string{
				metav1.FinalizerDeleteDependents,
			},
			remainingFinalizers: map[string]bool{},
		},
		{
			name: "propagation-background-without-finalizers",
			deleteOptions: &metav1.DeleteOptions{
				PropagationPolicy: &propagationBackground,
			},
			existingFinalizers:  []string{},
			remainingFinalizers: map[string]bool{},
		},
		{
			name: "propagation-foreground-with-orphan",
			deleteOptions: &metav1.DeleteOptions{
				PropagationPolicy: &propagationForeground,
			},
			existingFinalizers: []string{
				metav1.FinalizerOrphanDependents,
			},
			remainingFinalizers: map[string]bool{
				metav1.FinalizerDeleteDependents: true,
			},
		},
		{
			name: "propagation-foreground-with-delete",
			deleteOptions: &metav1.DeleteOptions{
				PropagationPolicy: &propagationForeground,
			},
			existingFinalizers: []string{
				metav1.FinalizerDeleteDependents,
			},
			remainingFinalizers: map[string]bool{
				metav1.FinalizerDeleteDependents: true,
			},
		},
		{
			name: "propagation-foreground-without-finalizers",
			deleteOptions: &metav1.DeleteOptions{
				PropagationPolicy: &propagationForeground,
			},
			existingFinalizers: []string{},
			remainingFinalizers: map[string]bool{
				metav1.FinalizerDeleteDependents: true,
			},
		},
		{
			name: "propagation-orphan-with-orphan",
			deleteOptions: &metav1.DeleteOptions{
				PropagationPolicy: &propagationOrphan,
			},
			existingFinalizers: []string{
				metav1.FinalizerOrphanDependents,
			},
			remainingFinalizers: map[string]bool{
				metav1.FinalizerOrphanDependents: true,
			},
		},
		{
			name: "propagation-orphan-with-delete",
			deleteOptions: &metav1.DeleteOptions{
				PropagationPolicy: &propagationOrphan,
			},
			existingFinalizers: []string{
				metav1.FinalizerDeleteDependents,
			},
			remainingFinalizers: map[string]bool{
				metav1.FinalizerOrphanDependents: true,
			},
		},
		{
			name: "propagation-orphan-without-finalizers",
			deleteOptions: &metav1.DeleteOptions{
				PropagationPolicy: &propagationOrphan,
			},
			existingFinalizers: []string{},
			remainingFinalizers: map[string]bool{
				metav1.FinalizerOrphanDependents: true,
			},
		},
		{
			name: "orphan-dependents-with-orphan",
			deleteOptions: &metav1.DeleteOptions{
				OrphanDependents: &trueVar,
			},
			existingFinalizers: []string{
				metav1.FinalizerOrphanDependents,
			},
			remainingFinalizers: map[string]bool{
				metav1.FinalizerOrphanDependents: true,
			},
		},
		{
			name: "orphan-dependents-with-delete",
			deleteOptions: &metav1.DeleteOptions{
				OrphanDependents: &trueVar,
			},
			existingFinalizers: []string{
				metav1.FinalizerDeleteDependents,
			},
			remainingFinalizers: map[string]bool{
				metav1.FinalizerOrphanDependents: true,
			},
		},
		{
			name: "orphan-dependents-without-finalizers",
			deleteOptions: &metav1.DeleteOptions{
				OrphanDependents: &trueVar,
			},
			existingFinalizers: []string{},
			remainingFinalizers: map[string]bool{
				metav1.FinalizerOrphanDependents: true,
			},
		},
	}

	for _, test := range tests {
		key := "tenants/" + test.name
		ctx := genericapirequest.NewContext()
		tenant := &api.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name:       test.name,
				Finalizers: test.existingFinalizers,
			},
			Spec: api.TenantSpec{
				StorageClusterId: "1",
				Finalizers:       []api.FinalizerName{},
			},
			Status: api.TenantStatus{Phase: api.TenantActive},
		}
		if err := storage.store.Storage.Create(ctx, key, tenant, nil, 0, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var obj runtime.Object
		var err error
		if obj, _, err = storage.Delete(ctx, test.name, rest.ValidateAllObjectFunc, test.deleteOptions); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ns, ok := obj.(*api.Tenant)
		if !ok {
			t.Errorf("unexpected object kind: %+v", obj)
		}
		if len(ns.Finalizers) != len(test.remainingFinalizers) {
			t.Errorf("%s: unexpected remaining finalizers: %v", test.name, ns.Finalizers)
		}
		for _, f := range ns.Finalizers {
			if test.remainingFinalizers[f] != true {
				t.Errorf("%s: unexpected finalizer %s", test.name, f)
			}
		}
	}
}

func TestShortNames(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.store.DestroyFunc()
	expected := []string{"te"}
	registrytest.AssertShortNames(t, storage, expected)
}
