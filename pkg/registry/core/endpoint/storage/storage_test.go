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
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistrytest "k8s.io/apiserver/pkg/registry/generic/testing"
	etcd3testing "k8s.io/apiserver/pkg/storage/etcd3/testing"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/registry/registrytest"
)

var tenant = "test-te"

func newStorage(t *testing.T) (*REST, *etcd3testing.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	restOptions := generic.RESTOptions{
		StorageConfig:           etcdStorage,
		Decorator:               generic.UndecoratedStorage,
		DeleteCollectionWorkers: 1,
		ResourcePrefix:          "endpoints",
	}
	return NewREST(restOptions), server
}

func validNewEndpoints() *api.Endpoints {
	return &api.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
			Tenant:    tenant,
		},
		Subsets: []api.EndpointSubset{{
			Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}},
			Ports:     []api.EndpointPort{{Port: 80, Protocol: "TCP"}},
		}},
	}
}

func TestCreate(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store)
	endpoints := validNewEndpoints()
	endpoints.ObjectMeta = metav1.ObjectMeta{}
	test.TestCreate(
		// valid
		endpoints,
		// invalid
		&api.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: "_-a123-a_"},
		},
	)
}

func TestUpdate(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store).AllowCreateOnUpdate()
	test.TestUpdate(
		// valid
		validNewEndpoints(),
		// updateFunc
		func(obj runtime.Object) runtime.Object {
			object := obj.(*api.Endpoints)
			object.Subsets = []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "1.2.3.4"}, {IP: "5.6.7.8"}},
				Ports:     []api.EndpointPort{{Port: 80, Protocol: "TCP"}},
			}}
			return object
		},
	)
}

func TestDelete(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store)
	test.TestDelete(validNewEndpoints())
}

func TestGet(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store)
	test.TestGet(validNewEndpoints())
}

func TestList(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store)
	test.TestList(validNewEndpoints())
}

func TestWatch(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store)
	test.TestWatch(
		validNewEndpoints(),
		// matching labels
		[]labels.Set{},
		// not matching labels
		[]labels.Set{
			{"foo": "bar"},
		},
		// matching fields
		[]fields.Set{
			{"metadata.name": "foo"},
		},
		// not matching fields
		[]fields.Set{
			{"metadata.name": "bar"},
			{"name": "foo"},
		},
	)
}

func TestGetK8SAlias(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()

	ctxCreate := genericapirequest.NewDefaultContext()
	k8s := &api.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubernetes",
		},
	}
	objSaved, err := storage.Create(ctxCreate, k8s, func(obj runtime.Object) error { return nil }, &metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to setup system k8s EP: %v", err)
	}
	k8sSaved := objSaved.(*api.Endpoints)

	ctxGet := genericapirequest.NewDefaultContext()
	ctxGet = genericapirequest.WithTenantAndNamespace(ctxGet, "bar", "default")
	obj, err := storage.Get(ctxGet, "kubernetes-foo", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ep := obj.(*api.Endpoints)

	if ep.UID != k8sSaved.UID {
		t.Errorf("expected objetct UID %s, got %s", k8sSaved.UID, ep.UID)
	}

	if ep.Tenant != "bar" {
		t.Errorf("returned ep was expected tenant %q, got %q", "bar", ep.Tenant)
	}

	if ep.Name != "kubernetes-foo" {
		t.Errorf("returned ep was expected name %q, got %q", "kubernetes-foo", ep.Name)
	}

	networkLabel := ep.Labels["arktos.futurewei.com/network"]
	if networkLabel != "foo" {
		t.Errorf("returned ep was expected network label %q, got %q", "foo", networkLabel)
	}
}

func TestCreateK8SAliasShouldBeForbidden(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()

	ctxCreate := genericapirequest.NewDefaultContext()
	k8s := &api.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubernetes-foo",
		},
	}
	_, err := storage.Create(ctxCreate, k8s, func(obj runtime.Object) error { return nil }, &metav1.CreateOptions{})
	if !strings.Contains(err.Error(), "Forbidden: read only resource not allowed to create or update") {
		t.Errorf("expected '... Forbidden: read only resource not allowed to create or update'; got %q", err)
	}
}
