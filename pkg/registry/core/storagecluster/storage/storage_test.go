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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistrytest "k8s.io/apiserver/pkg/registry/generic/testing"
	"k8s.io/apiserver/pkg/registry/rest"
	etcdtesting "k8s.io/apiserver/pkg/storage/etcd3/testing"

	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/registry/registrytest"
)

func newStorage(t *testing.T) (*REST, *etcdtesting.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	restOptions := generic.RESTOptions{StorageConfig: etcdStorage, Decorator: generic.UndecoratedStorage, DeleteCollectionWorkers: 1, ResourcePrefix: "storageclusters"}
	tenantStorage := NewREST(restOptions)
	return tenantStorage, server
}

func validNewTenantStorage() *api.StorageCluster {
	return &api.StorageCluster{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		StorageClusterId: "1",
		ServiceAddress:   "10.10.1.5:2379",
	}
}

func TestCreate(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store).ClusterScope()
	tenantStorage := validNewTenantStorage()
	test.TestCreate(
		// valid
		tenantStorage,
		// invalid
		&api.StorageCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "bad value"},
		},
	)
}

func TestCreateSetsFields(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.DestroyFunc()
	tenantStorage := validNewTenantStorage()
	ctx := genericapirequest.NewContext()
	_, err := storage.Store.Create(ctx, tenantStorage, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	object, err := storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actual := object.(*api.StorageCluster)
	if actual.Name != tenantStorage.Name {
		t.Errorf("unexpected tenant storage name: %#v", actual)
	}
	if actual.StorageClusterId != tenantStorage.StorageClusterId {
		t.Errorf("unexpected tenant storage id: %#v", actual)
	}
	if actual.ServiceAddress != tenantStorage.ServiceAddress {
		t.Errorf("unexpected serviceAddress: %#v", actual)
	}
}

func TestGet(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store).ClusterScope()
	test.TestGet(validNewTenantStorage())
}

func TestList(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store).ClusterScope()
	test.TestList(validNewTenantStorage())
}
