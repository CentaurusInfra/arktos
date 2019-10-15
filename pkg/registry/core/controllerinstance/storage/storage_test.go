/*
Copyright 2015 The Kubernetes Authors.

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
	etcdtesting "k8s.io/apiserver/pkg/storage/etcd/testing"

	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/registry/registrytest"
)

func newStorage(t *testing.T) (*REST, *etcdtesting.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	restOptions := generic.RESTOptions{StorageConfig: etcdStorage, Decorator: generic.UndecoratedStorage, DeleteCollectionWorkers: 1, ResourcePrefix: "controllerinstances"}
	controllerTypeStorage := NewREST(restOptions)
	return controllerTypeStorage, server
}

func validNewControllerInstance() *api.ControllerInstance {
	return &api.ControllerInstance{
		ControllerType: "hel",
		ControllerKey:  1,
		IsLocked:       false,
		WorkloadNum:    100,
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
			UID:  "112",
		},
	}
}

func TestCreate(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store).ClusterScope()
	controllerInstance := validNewControllerInstance()
	test.TestCreate(
		// valid
		controllerInstance,
		// invalid
		&api.ControllerInstance{
			ObjectMeta: metav1.ObjectMeta{Name: "bad value"},
		},
	)
}

func TestCreateSetsFields(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.DestroyFunc()
	controllerInstance := validNewControllerInstance()
	ctx := genericapirequest.NewContext()
	_, err := storage.Store.Create(ctx, controllerInstance, rest.ValidateAllObjectFunc, &metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	object, err := storage.Get(ctx, "foo", &metav1.GetOptions{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actual := object.(*api.ControllerInstance)
	if actual.IsLocked != controllerInstance.IsLocked {
		t.Errorf("unexpected controller lock: %#v", actual)
	}
	if actual.UID != controllerInstance.UID {
		t.Errorf("unexpected controller uid: %#v", actual)
	}
	if actual.HashKey != controllerInstance.HashKey {
		t.Errorf("unexpected controller key: %#v", actual)
	}
	if actual.WorkloadNum != controllerInstance.WorkloadNum {
		t.Errorf("unexpected controller worload num: %#v", actual)
	}
	if actual.Name != controllerInstance.Name {
		t.Errorf("unexpected controller name: %#v", actual)
	}

	if actual.ControllerType != controllerInstance.ControllerType {
		t.Errorf("unexpected controller name: %#v", actual)
	}
}

func TestGet(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store).ClusterScope()
	test.TestGet(validNewControllerInstance())
}

func TestList(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store).ClusterScope()
	test.TestList(validNewControllerInstance())
}
