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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/printers"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
	"k8s.io/kubernetes/pkg/registry/core/action"
)

type REST struct {
	*genericregistry.Store
	status *genericregistry.Store
}

// StatusREST implements the REST endpoint for changing the status of a Action object.
type StatusREST struct {
	store *genericregistry.Store
}

// NewREST returns a RESTStorage object that will work against actions.
func NewREST(optsGetter generic.RESTOptionsGetter, ttl uint64) (*REST, *StatusREST) {
	resource := api.Resource("actions")
	opts, err := optsGetter.GetRESTOptions(resource)
	if err != nil {
		panic(err) // TODO: Propagate error up
	}

	store := &genericregistry.Store{
		NewFunc:       func() runtime.Object { return &api.Action{} },
		NewListFunc:   func() runtime.Object { return &api.ActionList{} },
		PredicateFunc: action.MatchAction,
		TTLFunc: func(runtime.Object, uint64, bool) (uint64, error) {
			return ttl, nil
		},
		DefaultQualifiedResource: resource,

		CreateStrategy: action.Strategy,
		UpdateStrategy: action.Strategy,
		DeleteStrategy: action.Strategy,

		TableConvertor: printerstorage.TableConvertor{TableGenerator: printers.NewTableGenerator().With(printersinternal.AddHandlers)},
	}

	options := &generic.StoreOptions{
		RESTOptions: opts,
		AttrFunc:    action.GetAttrs,
		TriggerFunc: map[string]storage.IndexerFunc{"spec.nodeName": action.NodeNameTriggerFunc},
	}

	if err := store.CompleteWithOptions(options); err != nil {
		panic(err) // TODO: Propagate error up
	}

	statusStore := *store
	statusStore.UpdateStrategy = action.StatusStrategy

	return &REST{Store: store, status: &statusStore}, &StatusREST{store: &statusStore}
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"ac"}
}

// New creates a new pod resource
func (r *StatusREST) New() runtime.Object {
	return &api.Action{}
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
