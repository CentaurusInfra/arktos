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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	arktosv1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	fakearktosv1 "k8s.io/arktos-ext/pkg/generated/clientset/versioned/fake"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	roleActionCountPerBootstrap = 2
)

func TestTenantCreation(t *testing.T) {

	testcases := map[string]struct {
		Tenant                   *v1.Tenant
		ExpectCreatedNamespaces  []string
		NetworkTemplate          string
		NetworkTemplatePath      string
		ExpectedNetwork          *arktosv1.Network
		ExpectInitialRole        *rbacv1.ClusterRole
		ExpectInitialRoleBinding *rbacv1.ClusterRoleBinding
		ExpectActionCount        int
	}{
		"new-tenants": {
			Tenant: &v1.Tenant{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-tenant-1",
				},
			},
			ExpectCreatedNamespaces: tenantDefaultNamespaces[:],
			NetworkTemplate:         `{"metadata":{"name":"default", "tenant":"should-be-overridden"},"spec":{"type":"test-type","vpcID":"{{.}}-12345"}}`,
			NetworkTemplatePath:     "test.tmpl",
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
			ExpectInitialRole:        initialClusterRole("test-tenant-1"),
			ExpectInitialRoleBinding: initialClusterRoleBinding("test-tenant-1"),
			ExpectActionCount:        len(tenantDefaultNamespaces) + roleActionCountPerBootstrap,
		},
		"new-tenants-with-empty-default-network-tmpl-path": {
			Tenant: &v1.Tenant{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-tenant-2",
				},
			},
			ExpectCreatedNamespaces:  tenantDefaultNamespaces[:],
			NetworkTemplate:          `{"metadata":{"name":"default"},"spec":{"type":"test-type","vpcID":"{{.}}-12345"}}`,
			NetworkTemplatePath:      "",
			ExpectedNetwork:          nil,
			ExpectInitialRole:        initialClusterRole("test-tenant-2"),
			ExpectInitialRoleBinding: initialClusterRoleBinding("test-tenant-2"),
			ExpectActionCount:        len(tenantDefaultNamespaces) + roleActionCountPerBootstrap,
		},
		"terminating-tenant-not-create-default-network": {
			Tenant: &v1.Tenant{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-tenant-3",
				},
				Status: v1.TenantStatus{
					Phase: v1.TenantTerminating,
				},
			},
			ExpectCreatedNamespaces:  tenantDefaultNamespaces[:],
			NetworkTemplate:          `{"metadata":{"name":"default"},"spec":{"type":"test-type","vpcID":"{{.}}-12345"}}`,
			NetworkTemplatePath:      "test.tmpl",
			ExpectedNetwork:          nil,
			ExpectInitialRole:        initialClusterRole("test-tenant-3"),
			ExpectInitialRoleBinding: initialClusterRoleBinding("test-tenant-3"),
			ExpectActionCount:        len(tenantDefaultNamespaces) + roleActionCountPerBootstrap,
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

		clientActions := client.Actions()
		if len(clientActions) != tc.ExpectActionCount {
			t.Errorf("unmatched action counts, expect: %d, actual %d", tc.ExpectActionCount, len(clientActions))
		}

		t.Run("bootstrap namespaces", func(t *testing.T) {
			actualCreatedNamespaces := sets.NewString()
			expectCreatedNamespaces := sets.NewString()
			for _, s := range tc.ExpectCreatedNamespaces {
				expectCreatedNamespaces.Insert(s)
			}
			for _, action := range clientActions {
				if !action.Matches("create", "namespaces") {
					continue
				}

				createdNamespace := action.(core.CreateAction).GetObject().(*v1.Namespace)
				if createdNamespace.Tenant != tc.Tenant.Name {
					t.Errorf("%s: Unexpected tenant name in the created namespace: %s",
						k, createdNamespace.Tenant)
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
			if len(actualCreatedNamespaces) != len(expectCreatedNamespaces) {
				t.Errorf("%s: not all namespaces are created: expected: %v, actual: %v", k,
					expectCreatedNamespaces, actualCreatedNamespaces)
			}
		})

		t.Run("bootstrap cluster role and binding", func(t *testing.T) {
			for _, action := range clientActions {
				if action.Matches("create", "clusterroles") {
					createdClusterRole := action.(core.CreateAction).GetObject().(*rbacv1.ClusterRole)
					if !reflect.DeepEqual(tc.ExpectInitialRole, createdClusterRole) {
						t.Errorf("%s: Unexpected cluster role created, expect: %v\nactual: %v",
							k, tc.ExpectInitialRole, createdClusterRole)
					}
					continue
				}
				if action.Matches("create", "clusterrolebindings") {
					createdClusterRoleBinding := action.(core.CreateAction).GetObject().(*rbacv1.ClusterRoleBinding)
					if !reflect.DeepEqual(tc.ExpectInitialRoleBinding, createdClusterRoleBinding) {
						t.Errorf("%s: Unexpected cluster role binding created, expect: %v\nactual: %v",
							k, tc.ExpectInitialRoleBinding, createdClusterRoleBinding)
					}
				}
			}
		})

		t.Run("network CR", func(t *testing.T) {
			// verify network CR actions
			netActions := networkClient.Actions()
			if tc.ExpectedNetwork == nil {
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
		})
	}
}

func initialClusterRoleBinding(tenant string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   initialClusterRoleBindingName,
			Tenant: tenant,
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.UserKind, Name: "admin"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     initialClusterRoleName,
		},
	}
}

func initialClusterRole(tenant string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   initialClusterRoleName,
			Tenant: tenant,
		},
		Rules: []rbacv1.PolicyRule{initialClusterRoleRules()},
	}
}

var alwaysReady = func() bool { return true }
