/*
Copyright 2015 The Kubernetes Authors.
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

package storage

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	arktosv1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/printers"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
	"k8s.io/kubernetes/pkg/registry/core/endpoint"
)

type REST struct {
	*genericregistry.Store
}

// NewREST returns a RESTStorage object that will work against endpoints.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &api.Endpoints{} },
		NewListFunc:              func() runtime.Object { return &api.EndpointsList{} },
		DefaultQualifiedResource: api.Resource("endpoints"),

		CreateStrategy: endpoint.Strategy,
		UpdateStrategy: endpoint.Strategy,
		DeleteStrategy: endpoint.Strategy,

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
	return []string{"ep"}
}

// Get gets endpoints resource by name in the context (whose significance is tenant and namespace).
// If the target name starts with prefix of "kubernetes-", and the contextual namespace is "default",
// this target is considered as "k8s alias", a read-only copy of kubernetes endpoints of default namespace
// in system tenant, and its content returned is based on the system kubernetes resource.
// See Endpoints section of docs/design-proposals/multi-tenancy/multi-tenancy-network.md for the rationale.
func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	tenant, ok := genericapirequest.TenantFrom(ctx)
	if !ok {
		tenant = metav1.TenantSystem
	}

	namespace, ok := genericapirequest.NamespaceFrom(ctx)
	if !ok {
		namespace = metav1.NamespaceDefault
	}

	// redirect default/kubernetes-<network> EP query to default/kubernetes of system tenant
	if isK8sAliasEP(namespace, name) {
		ctx2 := genericapirequest.WithTenantAndNamespace(ctx, metav1.TenantSystem, metav1.NamespaceDefault)
		obj, err := r.Store.Get(ctx2, "kubernetes", options)
		if err != nil {
			return obj, err
		}

		ep := obj.(*api.Endpoints)
		ep.Tenant = tenant
		ep.Name = name
		if ep.Labels == nil {
			ep.Labels = make(map[string]string)
		}
		ep.Labels[arktosv1.NetworkLabel] = name[len("kubernetes-"):]
		return ep, err
	}

	return r.Store.Get(ctx, name, options)
}

func isK8sAliasEP(namespace, name string) bool {
	const k8sAliasPrefix = "kubernetes-"
	return namespace == metav1.NamespaceDefault && strings.HasPrefix(name, k8sAliasPrefix)
}
