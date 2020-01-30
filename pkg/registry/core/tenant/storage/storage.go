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
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	storageerr "k8s.io/apiserver/pkg/storage/errors"
	"k8s.io/apiserver/pkg/util/dryrun"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/printers"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
	"k8s.io/kubernetes/pkg/registry/core/tenant"
)

// rest implements a RESTStorage for tenants
type REST struct {
	store  *genericregistry.Store
	status *genericregistry.Store
}

// StatusREST implements the REST endpoint for changing the status of a tenant.
type StatusREST struct {
	store *genericregistry.Store
}

// FinalizeREST implements the REST endpoint for finalizing a tenant.
type FinalizeREST struct {
	store *genericregistry.Store
}

// NewREST returns a RESTStorage object that will work against tenants.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST, *FinalizeREST) {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &api.Tenant{} },
		NewListFunc:              func() runtime.Object { return &api.TenantList{} },
		PredicateFunc:            tenant.MatchTenant,
		DefaultQualifiedResource: api.Resource("tenants"),

		CreateStrategy:      tenant.Strategy,
		UpdateStrategy:      tenant.Strategy,
		DeleteStrategy:      tenant.Strategy,
		ReturnDeletedObject: true,

		ShouldDeleteDuringUpdate: ShouldDeleteTenantDuringUpdate,

		TableConvertor: printerstorage.TableConvertor{TableGenerator: printers.NewTableGenerator().With(printersinternal.AddHandlers)},
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: tenant.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err) // TODO: Propagate error up
	}

	statusStore := *store
	statusStore.UpdateStrategy = tenant.StatusStrategy

	finalizeStore := *store
	finalizeStore.UpdateStrategy = tenant.FinalizeStrategy

	return &REST{store: store, status: &statusStore}, &StatusREST{store: &statusStore}, &FinalizeREST{store: &finalizeStore}
}

func (r *REST) NamespaceScoped() bool {
	return r.store.NamespaceScoped()
}

func (r *REST) TenantScoped() bool {
	return r.store.TenantScoped()
}

func (r *REST) New() runtime.Object {
	return r.store.New()
}

func (r *REST) NewList() runtime.Object {
	return r.store.NewList()
}

func (r *REST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	return r.store.List(ctx, options)
}

func (r *REST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	return r.store.Create(ctx, obj, createValidation, options)
}

func (r *REST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

func (r *REST) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return r.store.Watch(ctx, options)
}

func (r *REST) Export(ctx context.Context, name string, opts metav1.ExportOptions) (runtime.Object, error) {
	return r.store.Export(ctx, name, opts)
}

// Delete enforces life-cycle rules for tenant termination
func (r *REST) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	nsObj, err := r.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	tenant := nsObj.(*api.Tenant)

	// Ensure we have a UID precondition
	if options == nil {
		options = metav1.NewDeleteOptions(0)
	}
	if options.Preconditions == nil {
		options.Preconditions = &metav1.Preconditions{}
	}
	if options.Preconditions.UID == nil {
		options.Preconditions.UID = &tenant.UID
	} else if *options.Preconditions.UID != tenant.UID {
		err = apierrors.NewConflict(
			api.Resource("tenants"),
			name,
			fmt.Errorf("Precondition failed: UID in precondition: %v, UID in object meta: %v", *options.Preconditions.UID, tenant.UID),
		)
		return nil, false, err
	}
	if options.Preconditions.ResourceVersion != nil && *options.Preconditions.ResourceVersion != tenant.ResourceVersion {
		err = apierrors.NewConflict(
			api.Resource("tenants"),
			name,
			fmt.Errorf("Precondition failed: ResourceVersion in precondition: %v, ResourceVersion in object meta: %v", *options.Preconditions.ResourceVersion, tenant.ResourceVersion),
		)
		return nil, false, err
	}

	// upon first request to delete, we switch the phase to start tenant termination
	// TODO: enhance graceful deletion's calls to DeleteStrategy to allow phase change and finalizer patterns
	if tenant.DeletionTimestamp.IsZero() {
		key, err := r.store.KeyFunc(ctx, name)
		if err != nil {
			return nil, false, err
		}

		preconditions := storage.Preconditions{UID: options.Preconditions.UID, ResourceVersion: options.Preconditions.ResourceVersion}

		out := r.store.NewFunc()
		err = r.store.Storage.GuaranteedUpdate(
			ctx, key, out, false, &preconditions,
			storage.SimpleUpdate(func(existing runtime.Object) (runtime.Object, error) {
				existingTenant, ok := existing.(*api.Tenant)
				if !ok {
					// wrong type
					return nil, fmt.Errorf("expected *api.Tenant, got %v", existing)
				}
				if err := deleteValidation(existingTenant); err != nil {
					return nil, err
				}
				// Set the deletion timestamp if needed
				if existingTenant.DeletionTimestamp.IsZero() {
					now := metav1.Now()
					existingTenant.DeletionTimestamp = &now
				}
				// Set the tenant phase to terminating, if needed
				if existingTenant.Status.Phase != api.TenantTerminating {
					existingTenant.Status.Phase = api.TenantTerminating
				}

				// the current finalizers which are on tenant
				currentFinalizers := map[string]bool{}
				for _, f := range existingTenant.Finalizers {
					currentFinalizers[f] = true
				}
				// the finalizers we should ensure on tenant
				shouldHaveFinalizers := map[string]bool{
					metav1.FinalizerOrphanDependents: shouldHaveOrphanFinalizer(options, currentFinalizers[metav1.FinalizerOrphanDependents]),
					metav1.FinalizerDeleteDependents: shouldHaveDeleteDependentsFinalizer(options, currentFinalizers[metav1.FinalizerDeleteDependents]),
				}
				// determine whether there are changes
				changeNeeded := false
				for finalizer, shouldHave := range shouldHaveFinalizers {
					changeNeeded = currentFinalizers[finalizer] != shouldHave || changeNeeded
					if shouldHave {
						currentFinalizers[finalizer] = true
					} else {
						delete(currentFinalizers, finalizer)
					}
				}
				// make the changes if needed
				if changeNeeded {
					newFinalizers := []string{}
					for f := range currentFinalizers {
						newFinalizers = append(newFinalizers, f)
					}
					existingTenant.Finalizers = newFinalizers
				}
				return existingTenant, nil
			}),
			dryrun.IsDryRun(options.DryRun),
		)

		if err != nil {
			err = storageerr.InterpretGetError(err, api.Resource("tenants"), name)
			err = storageerr.InterpretUpdateError(err, api.Resource("tenants"), name)
			if _, ok := err.(*apierrors.StatusError); !ok {
				err = apierrors.NewInternalError(err)
			}
			return nil, false, err
		}

		return out, false, nil
	}

	// prior to final deletion, we must ensure that finalizers is empty
	if len(tenant.Spec.Finalizers) != 0 {
		err = apierrors.NewConflict(api.Resource("tenants"), tenant.Name, fmt.Errorf("The system is ensuring all content is removed from this tenant.  Upon completion, this tenant will automatically be purged by the system."))
		return nil, false, err
	}
	return r.store.Delete(ctx, name, deleteValidation, options)
}

// ShouldDeleteTenantDuringUpdate adds tenant-specific spec.finalizer checks on top of the default generic ShouldDeleteDuringUpdate behavior
func ShouldDeleteTenantDuringUpdate(ctx context.Context, key string, obj, existing runtime.Object) bool {
	ns, ok := obj.(*api.Tenant)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("unexpected type %T", obj))
		return false
	}
	return len(ns.Spec.Finalizers) == 0 && genericregistry.ShouldDeleteDuringUpdate(ctx, key, obj, existing)
}

func shouldHaveOrphanFinalizer(options *metav1.DeleteOptions, haveOrphanFinalizer bool) bool {
	if options.OrphanDependents != nil {
		return *options.OrphanDependents
	}
	if options.PropagationPolicy != nil {
		return *options.PropagationPolicy == metav1.DeletePropagationOrphan
	}
	return haveOrphanFinalizer
}

func shouldHaveDeleteDependentsFinalizer(options *metav1.DeleteOptions, haveDeleteDependentsFinalizer bool) bool {
	if options.OrphanDependents != nil {
		return *options.OrphanDependents == false
	}
	if options.PropagationPolicy != nil {
		return *options.PropagationPolicy == metav1.DeletePropagationForeground
	}
	return haveDeleteDependentsFinalizer
}

func (e *REST) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1beta1.Table, error) {
	return e.store.ConvertToTable(ctx, object, tableOptions)
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"te"}
}

var _ rest.StorageVersionProvider = &REST{}

func (r *REST) StorageVersion() runtime.GroupVersioner {
	return r.store.StorageVersion()
}

func (r *StatusREST) New() runtime.Object {
	return r.store.New()
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	// We are explicitly setting forceAllowCreate to false in the call to the underlying storage because
	// subresources should never allow create on update.
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, false, options)
}

func (r *FinalizeREST) New() runtime.Object {
	return r.store.New()
}

// Update alters the status finalizers subset of an object.
func (r *FinalizeREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	// We are explicitly setting forceAllowCreate to false in the call to the underlying storage because
	// subresources should never allow create on update.
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, false, options)
}
