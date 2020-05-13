/*
Copyright 2014 The Kubernetes Authors.
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

package tenant

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	arktosv1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	fakearktosv1 "k8s.io/arktos-ext/pkg/generated/clientset/versioned/fake"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/controller"
)

func TestTenantCreation(t *testing.T) {

	testcases := map[string]struct {
		Tenant                  *v1.Tenant
		ExpectCreatedNamespaces []string
		NetworkTemplate         string
		NetworkTemplatePath 	string
		ExpectedNetwork         *arktosv1.Network
	}{
		"new-tenants": {
			Tenant: &v1.Tenant{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-tenant-1",
				},
			},
			ExpectCreatedNamespaces: tenant_default_namespaces[:],
			NetworkTemplate: `{"metadata":{"name":"default", "tenant":"should-be-overridden"},"spec":{"type":"test-type","vpcID":"{{.}}-12345"}}`,
			NetworkTemplatePath: "test.tmpl",
			ExpectedNetwork: &arktosv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "default",
					Tenant: "test-tenant-1",
				},
				Spec: arktosv1.NetworkSpec{
					Type:  "test-type",
					VPCID: "test-tenant-1-12345",
				},
			},
		},
		"new-tenants-with-empty-default-network-tmpl-path": {
			Tenant: &v1.Tenant{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-tenant-2",
				},
			},
			ExpectCreatedNamespaces: tenant_default_namespaces[:],
			NetworkTemplate: `{"metadata":{"name":"default"},"spec":{"type":"test-type","vpcID":"{{.}}-12345"}}`,
			NetworkTemplatePath: "",
			ExpectedNetwork: nil,
		},
		"terminating-tenant-not-create-default-network": {
			Tenant: &v1.Tenant{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-tenant-3",
				},
				Status: v1.TenantStatus {
					Phase: v1.TenantTerminating,
				},
			},
			ExpectCreatedNamespaces: tenant_default_namespaces[:],
			NetworkTemplate: `{"metadata":{"name":"default"},"spec":{"type":"test-type","vpcID":"{{.}}-12345"}}`,
			NetworkTemplatePath: "test.tmpl",
			ExpectedNetwork: nil,
		},
	}

	for k, tc := range testcases {
		client := fake.NewSimpleClientset(testcases["new-tenants"].Tenant)
		informers := informers.NewSharedInformerFactory(fake.NewSimpleClientset(), controller.NoResyncPeriodFunc())
		tnInformer := informers.Core().V1().Tenants()
		networkClient := fakearktosv1.NewSimpleClientset(&arktosv1.Network{})
		controller := NewTenantController(
			client,
			tnInformer,
			10*time.Minute,
			networkClient,
			tc.NetworkTemplatePath,
		)
		controller.listerSynced = alwaysReady

		syncCalls := make(chan struct{})
		controller.syncHandler = func(key string) error {
			err := controller.syncTenant(key)
			if err != nil {
				t.Logf("%s: %v", k, err)
			}

			syncCalls <- struct{}{}
			return err
		}

		controller.templateGetter = func(path string) (s string, err error) {
			return tc.NetworkTemplate, nil
		}

		stopCh := make(chan struct{})
		defer close(stopCh)
		go controller.Run(1, stopCh)

		tnStore := tnInformer.Informer().GetStore()
		tnStore.Add(tc.Tenant)
		controller.enqueue(tc.Tenant)

		// wait to be called
		select {
		case <-syncCalls:
		case <-time.After(10 * time.Second):
			t.Errorf("%s: took too long", k)
		}

		// verify ns actions
		actions := client.Actions()
		if len(tc.ExpectCreatedNamespaces) != len(actions) {
			t.Errorf("%s: Expected to create namespaces %#v. Actual actions were: %#v", k, tc.ExpectCreatedNamespaces, actions)
			continue
		}

		actualCreatedNamespaces := sets.NewString()
		expectCreatedNamespaces := sets.NewString()
		for _, s := range tc.ExpectCreatedNamespaces {
			expectCreatedNamespaces.Insert(s)
		}
		for i := 0; i < len(actions); i++ {
			action := actions[i]
			if !action.Matches("create", "namespaces") {
				t.Errorf("%s: Unexpected action %v", k, action)
				break
			}

			createdNamespace := action.(core.CreateAction).GetObject().(*v1.Namespace)
			if createdNamespace.Tenant != tc.Tenant.Name {
				t.Errorf("%s: Unexpected tenant name in the created namespace: %s", k, createdNamespace.Tenant)
			}

			createdNamespaceName := createdNamespace.Name
			if !expectCreatedNamespaces.Has(createdNamespaceName) {
				t.Errorf("%s: Unexpected namespace is created: %s", k, createdNamespaceName)
			}
			if actualCreatedNamespaces.Has(createdNamespaceName) {
				t.Errorf("%s: namespace is created multiple times: %s", k, createdNamespaceName)
			}

			actualCreatedNamespaces.Insert(createdNamespaceName)
		}

		// verify network CR actions
		netActions := networkClient.Actions()
		if (tc.ExpectedNetwork == nil) {
			if 0 != len(netActions) {
				t.Errorf("%s: Should have no action; got actions: %#v", k, netActions)
			}
		} else {
			if 1 != len(netActions) {
				t.Errorf("%s: Expected to create network %#v. Actual actions were: %#v", k, tc.ExpectedNetwork.Name, netActions)
			}
			if !netActions[0].Matches("create", "networks") {
				t.Errorf("%s: Unexpected action %v", k, netActions[0])
			}
			netObj := netActions[0].(core.CreateAction).GetObject().(*arktosv1.Network)
			if !reflect.DeepEqual(netObj, tc.ExpectedNetwork) {
				t.Errorf("%s: Expected network object %#v; got %#v", k, tc.ExpectedNetwork, netObj)
			}
		}
	}
}

var alwaysReady = func() bool { return true }
