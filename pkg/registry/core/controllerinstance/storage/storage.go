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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/printers"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
	"k8s.io/kubernetes/pkg/registry/core/controllerinstance"
)

// REST implements a RESTStorage for ControllerInstance
type REST struct {
	*genericregistry.Store
}

const TTL = 30 // time-to-live in seconds

// NewREST returns a RESTStorage object that will work with ControllerInstance objects.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:     func() runtime.Object { return &api.ControllerInstance{} },
		NewListFunc: func() runtime.Object { return &api.ControllerInstanceList{} },
		TTLFunc: func(runtime.Object, uint64, bool) (uint64, error) {
			return TTL, nil
		},
		TTLOnUpdateFunc: func(runtime.Object, uint64) (uint64, error) {
			return TTL, nil
		},
		DefaultQualifiedResource: api.Resource("controllerinstances"),

		CreateStrategy: controllerinstance.Strategy,
		UpdateStrategy: controllerinstance.Strategy,
		DeleteStrategy: controllerinstance.Strategy,

		TableConvertor: printerstorage.TableConvertor{TableGenerator: printers.NewTableGenerator().With(printersinternal.AddHandlers)},
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err) // TODO: Propagate error up
	}
	return &REST{store}
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"co"}
}
