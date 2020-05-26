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

package lifecycle

import (
	"fmt"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	kubeadmission "k8s.io/apiserver/pkg/admission/initializer"
	informers "k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
)

// newHandlerForTest returns a configured handler for testing.
func newHandlerForTest(c clientset.Interface) (*Lifecycle, informers.SharedInformerFactory, error) {
	return newHandlerForTestWithClock(c, clock.RealClock{})
}

// newHandlerForTestWithClock returns a configured handler for testing.
func newHandlerForTestWithClock(c clientset.Interface, cacheClock clock.Clock) (*Lifecycle, informers.SharedInformerFactory, error) {
	f := informers.NewSharedInformerFactory(c, 5*time.Minute)
	handler, err := newLifecycleWithClock(sets.NewString(metav1.NamespaceDefault, metav1.NamespaceSystem),
		sets.NewString(metav1.TenantSystem, metav1.TenantSystem),
		cacheClock)
	if err != nil {
		return nil, f, err
	}
	pluginInitializer := kubeadmission.New(c, f, nil)
	pluginInitializer.Initialize(handler)
	err = admission.ValidateInitialization(handler)
	return handler, f, err
}

// newMockClientForTest creates a mock client that returns a client configured for the specified list of namespaces with the specified phase.
func newMockClientForTest(namespaces map[string]v1.NamespacePhase, tenants map[string]v1.TenantPhase) *fake.Clientset {
	mockClient := &fake.Clientset{}
	mockClient.AddReactor("list", "namespaces", func(action core.Action) (bool, runtime.Object, error) {
		namespaceList := &v1.NamespaceList{
			ListMeta: metav1.ListMeta{
				ResourceVersion: fmt.Sprintf("%d", len(namespaces)),
			},
		}
		index := 0
		for fullName, phase := range namespaces {
			parts := strings.Split(fullName, "/")
			if len(parts) != 2 {
				panic("Invalid namespace full name format: " + fullName)
			}

			namespaceList.Items = append(namespaceList.Items, v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Tenant:          parts[0],
					Name:            parts[1],
					ResourceVersion: fmt.Sprintf("%d", index),
				},
				Status: v1.NamespaceStatus{
					Phase: phase,
				},
			})
			index++
		}
		return true, namespaceList, nil
	})

	mockClient.AddReactor("list", "tenants", func(action core.Action) (bool, runtime.Object, error) {
		tenantList := &v1.TenantList{
			ListMeta: metav1.ListMeta{
				ResourceVersion: fmt.Sprintf("%d", len(tenants)),
			},
		}
		index := 0
		for name, phase := range tenants {
			tenantList.Items = append(tenantList.Items, v1.Tenant{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					ResourceVersion: fmt.Sprintf("%d", index),
				},
				Status: v1.TenantStatus{
					Phase: phase,
				},
			})
			index++
		}
		return true, tenantList, nil
	})

	return mockClient
}

// newPod returns a new pod for the specified namespace
func newPod(namespace string, tenant string) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "123", Namespace: namespace, Tenant: tenant},
		Spec: v1.PodSpec{
			Volumes:    []v1.Volume{{Name: "vol"}},
			Containers: []v1.Container{{Name: "ctr", Image: "image"}},
		},
	}
}

func TestAccessReviewCheckOnMissingNamespace(t *testing.T) {
	namespace := "test/test"
	mockClient := newMockClientForTest(map[string]v1.NamespacePhase{}, map[string]v1.TenantPhase{})
	mockClient.AddReactor("get", "namespaces", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("nope, out of luck")
	})
	handler, informerFactory, err := newHandlerForTest(mockClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	err = handler.Admit(admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{Group: "authorization.k8s.io", Version: "v1", Kind: "LocalSubjectAccesReview"}, metav1.TenantSystem, namespace, "", schema.GroupVersionResource{Group: "authorization.k8s.io", Version: "v1", Resource: "localsubjectaccessreviews"}, "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if err != nil {
		t.Error(err)
	}
}

// TestAdmissionNamespaceDoesNotExist verifies pod is not admitted if namespace does not exist.
func TestAdmissionNamespaceDoesNotExist(t *testing.T) {
	testAdmissionNamespaceDoesNotExist(t, metav1.TenantSystem)
}

func TestAdmissionNamespaceDoesNotExistWithMultiTenancy(t *testing.T) {
	testAdmissionNamespaceDoesNotExist(t, "test-te")
}

func testAdmissionNamespaceDoesNotExist(t *testing.T, tenant string) {
	namespace := "test"
	namespacePhases := map[string]v1.NamespacePhase{}
	tenantPhases := map[string]v1.TenantPhase{}
	mockClient := newMockClientForTest(namespacePhases, tenantPhases)
	mockClient.AddReactor("get", "namespaces", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("nope, out of luck")
	})
	handler, informerFactory, err := newHandlerForTest(mockClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	pod := newPod(namespace, tenant)
	err = handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if err == nil {
		actions := ""
		for _, action := range mockClient.Actions() {
			actions = actions + action.GetVerb() + ":" + action.GetResource().Resource + ":" + action.GetSubresource() + ", "
		}
		t.Errorf("expected error returned from admission handler: %v", actions)
	}

	// verify create operations in the namespace cause an error
	err = handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if err == nil {
		t.Errorf("Expected error rejecting creates in a namespace when it is missing")
	}

	// verify update operations in the namespace cause an error
	err = handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil), nil)
	if err == nil {
		t.Errorf("Expected error rejecting updates in a namespace when it is missing")
	}

	// verify delete operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil), nil)
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}
}

// TestAdmissionNamespaceActive verifies a resource is admitted when the namespace is active.
func TestAdmissionNamespaceActive(t *testing.T) {
	testAdmissionNamespaceActive(t, metav1.TenantSystem)
}

func TestAdmissionNamespaceActiveWithMultiTenancy(t *testing.T) {
	testAdmissionNamespaceActive(t, "test-te")
}
func testAdmissionNamespaceActive(t *testing.T, tenant string) {
	namespace := "test"
	namespacePhases := map[string]v1.NamespacePhase{tenant + "/" + namespace: v1.NamespaceActive}
	tenantPhases := map[string]v1.TenantPhase{tenant: v1.TenantActive}
	mockClient := newMockClientForTest(namespacePhases, tenantPhases)

	handler, informerFactory, err := newHandlerForTest(mockClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	pod := newPod(namespace, tenant)
	err = handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if err != nil {
		t.Errorf("unexpected error returned from admission handler")
	}
}

// TestAdmissionNamespaceTerminating verifies a resource is not created when the namespace is active.
func TestAdmissionNamespaceTerminating(t *testing.T) {
	testAdmissionNamespaceTerminating(t, metav1.TenantSystem)
}

func TestAdmissionNamespaceTerminatingWithMultiTenancy(t *testing.T) {
	testAdmissionNamespaceTerminating(t, "test-te")
}

func testAdmissionNamespaceTerminating(t *testing.T, tenant string) {
	namespace := "test"
	namespacePhases := map[string]v1.NamespacePhase{tenant + "/" + namespace: v1.NamespaceTerminating}
	tenantPhases := map[string]v1.TenantPhase{tenant: v1.TenantActive}
	mockClient := newMockClientForTest(namespacePhases, tenantPhases)

	handler, informerFactory, err := newHandlerForTest(mockClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	pod := newPod(namespace, tenant)
	// verify create operations in the namespace cause an error
	err = handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if err == nil {
		t.Errorf("Expected error rejecting creates in a namespace when it is terminating")
	}

	// verify update operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil), nil)
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// verify delete operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil), nil)
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// verify delete of namespace default can never proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, nil, v1.SchemeGroupVersion.WithKind("Namespace").GroupKind().WithVersion("version"), tenant, "", metav1.NamespaceDefault, v1.Resource("namespaces").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil), nil)
	if err == nil {
		t.Errorf("Expected an error that this namespace can never be deleted")
	}

	// verify delete of namespace other than default can proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, nil, v1.SchemeGroupVersion.WithKind("Namespace").GroupKind().WithVersion("version"), tenant, "", "other", v1.Resource("namespaces").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil), nil)
	if err != nil {
		t.Errorf("Did not expect an error %v", err)
	}
}

func TestAdmissionTenantTerminating(t *testing.T) {
	namespace := "test"
	tenant := "test-te"
	namespacePhases := map[string]v1.NamespacePhase{tenant + "/" + namespace: v1.NamespaceActive}
	tenantPhases := map[string]v1.TenantPhase{tenant: v1.TenantTerminating}
	mockClient := newMockClientForTest(namespacePhases, tenantPhases)

	handler, informerFactory, err := newHandlerForTest(mockClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	pod := newPod(namespace, tenant)
	// verify create operations cause an error
	err = handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if err == nil {
		t.Errorf("Expected error rejecting creates when the tenant is terminating")
	}

	// verify update operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil), nil)
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// verify delete operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil), nil)
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// verify delete of namespace default can never proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, nil, v1.SchemeGroupVersion.WithKind("Namespace").GroupKind().WithVersion("version"), tenant, "", metav1.NamespaceDefault, v1.Resource("namespaces").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil), nil)
	if err == nil {
		t.Errorf("Expected an error that this namespace can never be deleted")
	}

	// verify delete of namespace other than default can proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, nil, v1.SchemeGroupVersion.WithKind("Namespace").GroupKind().WithVersion("version"), tenant, "", "other", v1.Resource("namespaces").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil), nil)
	if err != nil {
		t.Errorf("Did not expect an error %v", err)
	}
}

// TestAdmissionNamespaceForceLiveLookup verifies live lookups are done after deleting a namespace
func TestAdmissionNamespaceForceLiveLookup(t *testing.T) {
	testAdmissionNamespaceForceLiveLookup(t, metav1.TenantSystem)
}

func TestAdmissionNamespaceForceLiveLookupWithMultiTenancy(t *testing.T) {
	testAdmissionNamespaceForceLiveLookup(t, "test-te")
}

func testAdmissionNamespaceForceLiveLookup(t *testing.T, tenant string) {
	namespace := "test"
	getCalls := int64(0)
	namespacePhases := map[string]v1.NamespacePhase{tenant + "/" + namespace: v1.NamespaceActive}
	tenantPhases := map[string]v1.TenantPhase{tenant: v1.TenantActive}
	mockClient := newMockClientForTest(namespacePhases, tenantPhases)
	mockClient.AddReactor("get", "namespaces", func(action core.Action) (bool, runtime.Object, error) {
		getCalls++
		return true, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}, Status: v1.NamespaceStatus{Phase: namespacePhases[namespace]}}, nil
	})

	fakeClock := clock.NewFakeClock(time.Now())

	handler, informerFactory, err := newHandlerForTestWithClock(mockClient, fakeClock)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	pod := newPod(namespace, tenant)
	// verify create operations in the namespace is allowed
	err = handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if err != nil {
		t.Errorf("Unexpected error rejecting creates in an active namespace")
	}
	if getCalls != 0 {
		t.Errorf("Expected no live lookups of the namespace, got %d", getCalls)
	}
	getCalls = 0

	// verify delete of namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, nil, v1.SchemeGroupVersion.WithKind("Namespace").GroupKind().WithVersion("version"), tenant, namespace, namespace, v1.Resource("namespaces").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil), nil)
	if err != nil {
		t.Errorf("Expected namespace deletion to be allowed")
	}
	if getCalls != 0 {
		t.Errorf("Expected no live lookups of the namespace, got %d", getCalls)
	}
	getCalls = 0

	// simulate the phase changing
	namespacePhases[namespace] = v1.NamespaceTerminating

	// verify create operations in the namespace cause an error
	err = handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if err == nil {
		t.Errorf("Expected error rejecting creates in a namespace right after deleting it")
	}
	if getCalls != 1 {
		t.Errorf("Expected a live lookup of the namespace at t=0, got %d", getCalls)
	}
	getCalls = 0

	// Ensure the live lookup is still forced up to forceLiveLookupTTL
	fakeClock.Step(forceLiveLookupTTL)

	// verify create operations in the namespace cause an error
	err = handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if err == nil {
		t.Errorf("Expected error rejecting creates in a namespace right after deleting it")
	}
	if getCalls != 1 {
		t.Errorf("Expected a live lookup of the namespace at t=forceLiveLookupTTL, got %d", getCalls)
	}
	getCalls = 0

	// Ensure the live lookup expires
	fakeClock.Step(time.Millisecond)

	// verify create operations in the namespace don't force a live lookup after the timeout
	handler.Admit(admission.NewAttributesRecord(&pod, nil, v1.SchemeGroupVersion.WithKind("Pod").GroupKind().WithVersion("version"), pod.Tenant, pod.Namespace, pod.Name, v1.Resource("pods").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil), nil)
	if getCalls != 0 {
		t.Errorf("Expected no live lookup of the namespace at t=forceLiveLookupTTL+1ms, got %d", getCalls)
	}
	getCalls = 0
}
