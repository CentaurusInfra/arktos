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

package customresourcedefinition

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	storageerr "k8s.io/apiserver/pkg/storage/errors"
	"k8s.io/apiserver/pkg/util/dryrun"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/labels"
)

// rest implements a RESTStorage for API services against etcd
type REST struct {
	*genericregistry.Store
}

const (
	crdSharingPolicyAnnotation = "arktos.futurewei.com/crd-sharing-policy"
	forcedSharing              = "forced"
)

// NewREST returns a RESTStorage object that will work against API services.
func NewREST(scheme *runtime.Scheme, optsGetter generic.RESTOptionsGetter) *REST {
	strategy := NewStrategy(scheme)

	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &apiextensions.CustomResourceDefinition{} },
		NewListFunc:              func() runtime.Object { return &apiextensions.CustomResourceDefinitionList{} },
		PredicateFunc:            MatchCustomResourceDefinition,
		DefaultQualifiedResource: apiextensions.Resource("customresourcedefinitions"),

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err) // TODO: Propagate error up
	}
	return &REST{store}
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"crd", "crds"}
}

// try to retrieve the forced version of CRD under the system tenant first.
// If not found, try the search under the tenant.
func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	tenant, ok := genericapirequest.TenantFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot decide the tenant")
	}

	systemContext := genericapirequest.WithTenant(ctx, metav1.TenantSystem)
	obj, err := r.Store.Get(systemContext, name, options)
	if tenant == metav1.TenantSystem {
		return obj, err
	}

	if err == nil && IsCrdSystemForced(obj.(*apiextensions.CustomResourceDefinition)) {
		return obj, nil
	}

	return r.Store.Get(ctx, name, options)
}

func (r *REST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	tenant, ok := genericapirequest.TenantFrom(ctx)
	if !ok {
		return nil, false, fmt.Errorf("cannot decide the tenant")
	}

	if tenant == metav1.TenantSystem {
		return r.Store.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
	}

	systemContext := genericapirequest.WithTenant(ctx, metav1.TenantSystem)
	sysObj, err := r.Store.Get(systemContext, name, &metav1.GetOptions{})
	if err == nil && IsCrdSystemForced(sysObj.(*apiextensions.CustomResourceDefinition)) {
		return nil, false, fmt.Errorf("%v is a system CRD, you cannot overwrite it.", name)
	}

	return r.Store.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

// Return the forced CRD under the system tenant and the CRDs under the tenant.
func (r *REST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	tenant, ok := genericapirequest.TenantFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot decide the tenant")
	}

	if tenant == metav1.TenantSystem {
		return r.Store.List(ctx, options)
	}

	resultItems := []apiextensions.CustomResourceDefinition{}
	systemCrdMap := map[string]bool{}

	systemContext := genericapirequest.WithTenant(ctx, metav1.TenantSystem)
	sysSharingOptions := &metainternalversion.ListOptions{LabelSelector: labels.Set{crdSharingPolicyAnnotation: forcedSharing}.AsSelector()}
	sysList, err := r.Store.List(systemContext, sysSharingOptions)
	if err == nil {
		sysCrdList, ok := sysList.(*apiextensions.CustomResourceDefinitionList)
		if ok {
			for _, crd := range sysCrdList.Items {
				if IsCrdSystemForced(&crd) {
					systemCrdMap[crd.Name] = true
					resultItems = append(resultItems, crd)
				}
			}
		}
	}

	tenantList, err := r.Store.List(ctx, options)
	if err != nil {
		return nil, err
	}

	tenantCrdList, ok := tenantList.(*apiextensions.CustomResourceDefinitionList)
	if !ok {
		return nil, fmt.Errorf("Failed to convert the object to CRD list")
	}

	for _, crd := range tenantCrdList.Items {
		if _, collsion := systemCrdMap[crd.Name]; !collsion {
			resultItems = append(resultItems, crd)
		}
	}

	resultList := &apiextensions.CustomResourceDefinitionList{
		TypeMeta: tenantCrdList.TypeMeta,
		ListMeta: tenantCrdList.ListMeta,
		Items:    resultItems,
	}

	return resultList, nil
}

func (r *REST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	tenant, ok := genericapirequest.TenantFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot decide the tenant")
	}

	if tenant == metav1.TenantSystem {
		return r.Store.Create(ctx, obj, createValidation, options)
	}

	crd, _ := obj.(*apiextensions.CustomResourceDefinition)
	crdName := crd.Name
	systemContext := genericapirequest.WithTenant(ctx, metav1.TenantSystem)
	sysObj, err := r.Store.Get(systemContext, crdName, &metav1.GetOptions{})
	if err == nil && IsCrdSystemForced(sysObj.(*apiextensions.CustomResourceDefinition)) {
		return nil, fmt.Errorf("There is already a system forced CRD with the name %v ", crdName)
	}

	return r.Store.Create(ctx, obj, createValidation, options)
}

// Delete adds the CRD finalizer to the list
func (r *REST) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	obj, err := r.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	crd := obj.(*apiextensions.CustomResourceDefinition)

	// Ensure we have a UID precondition
	if options == nil {
		options = metav1.NewDeleteOptions(0)
	}
	if options.Preconditions == nil {
		options.Preconditions = &metav1.Preconditions{}
	}
	if options.Preconditions.UID == nil {
		options.Preconditions.UID = &crd.UID
	} else if *options.Preconditions.UID != crd.UID {
		err = apierrors.NewConflict(
			apiextensions.Resource("customresourcedefinitions"),
			name,
			fmt.Errorf("Precondition failed: UID in precondition: %v, UID in object meta: %v", *options.Preconditions.UID, crd.UID),
		)
		return nil, false, err
	}
	if options.Preconditions.ResourceVersion != nil && *options.Preconditions.ResourceVersion != crd.ResourceVersion {
		err = apierrors.NewConflict(
			apiextensions.Resource("customresourcedefinitions"),
			name,
			fmt.Errorf("Precondition failed: ResourceVersion in precondition: %v, ResourceVersion in object meta: %v", *options.Preconditions.ResourceVersion, crd.ResourceVersion),
		)
		return nil, false, err
	}

	// upon first request to delete, add our finalizer and then delegate
	if crd.DeletionTimestamp.IsZero() {
		key, err := r.Store.KeyFunc(ctx, name)
		if err != nil {
			return nil, false, err
		}

		preconditions := storage.Preconditions{UID: options.Preconditions.UID, ResourceVersion: options.Preconditions.ResourceVersion}

		out := r.Store.NewFunc()
		err = r.Store.Storage.GuaranteedUpdate(
			ctx, key, out, false, &preconditions,
			storage.SimpleUpdate(func(existing runtime.Object) (runtime.Object, error) {
				existingCRD, ok := existing.(*apiextensions.CustomResourceDefinition)
				if !ok {
					// wrong type
					return nil, fmt.Errorf("expected *apiextensions.CustomResourceDefinition, got %v", existing)
				}
				if err := deleteValidation(existingCRD); err != nil {
					return nil, err
				}

				// Set the deletion timestamp if needed
				if existingCRD.DeletionTimestamp.IsZero() {
					now := metav1.Now()
					existingCRD.DeletionTimestamp = &now
				}

				if !apiextensions.CRDHasFinalizer(existingCRD, apiextensions.CustomResourceCleanupFinalizer) {
					existingCRD.Finalizers = append(existingCRD.Finalizers, apiextensions.CustomResourceCleanupFinalizer)
				}
				// update the status condition too
				apiextensions.SetCRDCondition(existingCRD, apiextensions.CustomResourceDefinitionCondition{
					Type:    apiextensions.Terminating,
					Status:  apiextensions.ConditionTrue,
					Reason:  "InstanceDeletionPending",
					Message: "CustomResourceDefinition marked for deletion; CustomResource deletion will begin soon",
				})
				return existingCRD, nil
			}),
			dryrun.IsDryRun(options.DryRun),
		)

		if err != nil {
			err = storageerr.InterpretGetError(err, apiextensions.Resource("customresourcedefinitions"), name)
			err = storageerr.InterpretUpdateError(err, apiextensions.Resource("customresourcedefinitions"), name)
			if _, ok := err.(*apierrors.StatusError); !ok {
				err = apierrors.NewInternalError(err)
			}
			return nil, false, err
		}

		return out, false, nil
	}

	return r.Store.Delete(ctx, name, deleteValidation, options)
}

// NewStatusREST makes a RESTStorage for status that has more limited options.
// It is based on the original REST so that we can share the same underlying store
func NewStatusREST(scheme *runtime.Scheme, rest *REST) *StatusREST {
	statusStore := *rest.Store
	statusStore.CreateStrategy = nil
	statusStore.DeleteStrategy = nil
	statusStore.UpdateStrategy = NewStatusStrategy(scheme)
	return &StatusREST{store: &statusStore}
}

type StatusREST struct {
	store *genericregistry.Store
}

var _ = rest.Patcher(&StatusREST{})

func (r *StatusREST) New() runtime.Object {
	return &apiextensions.CustomResourceDefinition{}
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

func IsCrdSystemForced(crd *apiextensions.CustomResourceDefinition) bool {
	sharingPolicy, _ := crd.GetLabels()[crdSharingPolicyAnnotation]
	result := strings.ToLower(sharingPolicy) == forcedSharing && crd.GetTenant() == metav1.TenantSystem
	return result
}
