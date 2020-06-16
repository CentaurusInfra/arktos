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

package exists

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	genericadmissioninitializer "k8s.io/apiserver/pkg/admission/initializer"
	informers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	api "k8s.io/kubernetes/pkg/apis/core"
)

// newHandlerForTest returns the admission controller configured for testing.
func newHandlerForTest(c kubernetes.Interface) (admission.ValidationInterface, informers.SharedInformerFactory, error) {
	f := informers.NewSharedInformerFactory(c, 5*time.Minute)
	handler := NewExists()
	pluginInitializer := genericadmissioninitializer.New(c, f, nil)
	pluginInitializer.Initialize(handler)
	err := admission.ValidateInitialization(handler)
	return handler, f, err
}

// newMockClientForTest creates a mock client that returns a client configured for the specified list of tenants.
func newMockClientForTest(tenants []string) *fake.Clientset {
	mockClient := &fake.Clientset{}
	mockClient.AddReactor("list", "tenants", func(action core.Action) (bool, runtime.Object, error) {
		tenantList := &corev1.TenantList{
			ListMeta: metav1.ListMeta{
				ResourceVersion: fmt.Sprintf("%d", len(tenants)),
			},
		}
		for i, tenant := range tenants {
			tenantList.Items = append(tenantList.Items, corev1.Tenant{
				ObjectMeta: metav1.ObjectMeta{
					Name:            tenant,
					ResourceVersion: fmt.Sprintf("%d", i),
				},
			})
		}
		return true, tenantList, nil
	})
	return mockClient
}

// newNamespace returns a new namespace for the specified tenant
func newNamespace(tenant string) api.Namespace {
	return api.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "mynamepace", Tenant: tenant},
	}
}

// TestAdmissionTenantExists verifies Tenant is admitted only if tenant exists.
func TestAdmissionTenantExists(t *testing.T) {
	tenant := "test"
	mockClient := newMockClientForTest([]string{tenant})
	handler, informerFactory, err := newHandlerForTest(mockClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	ns := newNamespace(tenant)
	err = handler.Validate(admission.NewAttributesRecord(&ns, nil, api.Kind("Namespace").WithVersion("version"), ns.Tenant, "", ns.Name, api.Resource("namespaces").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if err != nil {
		t.Errorf("unexpected error returned from admission handler")
	}
}

// TestAdmissionTenantDoesNotExist verifies Tenant is not admitted if tenant does not exist.
func TestAdmissionTenantDoesNotExist(t *testing.T) {
	tenant := "test"
	mockClient := newMockClientForTest([]string{})
	mockClient.AddReactor("get", "tenants", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("nope, out of luck")
	})
	handler, informerFactory, err := newHandlerForTest(mockClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	ns := newNamespace(tenant)
	err = handler.Validate(admission.NewAttributesRecord(&ns, nil, api.Kind("Namespace").WithVersion("version"), ns.Tenant, "", ns.Name, api.Resource("namespaces").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if err == nil {
		actions := ""
		for _, action := range mockClient.Actions() {
			actions = actions + action.GetVerb() + ":" + action.GetResource().Resource + ":" + action.GetSubresource() + ", "
		}
		t.Errorf("expected error returned from admission handler: %v", actions)
	}
}
