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
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/controller"
)

func TestTenantCreation(t *testing.T) {

	testcases := map[string]struct {
		Tenant                  *v1.Tenant
		ExpectCreatedNamespaces []string
	}{
		"new-tenants": {
			Tenant: &v1.Tenant{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-tenant-1",
				},
			},
			ExpectCreatedNamespaces: tenant_default_namespaces[:],
		},
	}

	for k, tc := range testcases {
		client := fake.NewSimpleClientset(testcases["new-tenants"].Tenant)
		informers := informers.NewSharedInformerFactory(fake.NewSimpleClientset(), controller.NoResyncPeriodFunc())
		tnInformer := informers.Core().V1().Tenants()
		controller := NewTenantController(
			client,
			tnInformer,
			10*time.Minute,
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
	}
}

var alwaysReady = func() bool { return true }
