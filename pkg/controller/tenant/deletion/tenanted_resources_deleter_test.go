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

package deletion

import (
	"fmt"
	"k8s.io/client-go/metadata"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	restclient "k8s.io/client-go/rest"

	api "k8s.io/kubernetes/pkg/apis/core"
)

func TestFinalized(t *testing.T) {
	testTenant := &v1.Tenant{
		Spec: v1.TenantSpec{
			Finalizers: []v1.FinalizerName{"a", "b"},
		},
	}
	if finalized(testTenant) {
		t.Errorf("Unexpected result, tenant is not finalized")
	}
	testTenant.Spec.Finalizers = []v1.FinalizerName{}
	if !finalized(testTenant) {
		t.Errorf("Expected object to be finalized")
	}
}

func TestFinalizeTenantFunc(t *testing.T) {
	mockClient := &fake.Clientset{}
	testTenant := &v1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "1",
		},
		Spec: v1.TenantSpec{
			Finalizers: []v1.FinalizerName{v1.FinalizerArktos, "other"},
		},
	}
	d := tenantedResourcesDeleter{
		kubeClient:     mockClient,
		finalizerToken: v1.FinalizerArktos,
	}
	d.finalizeTenant(testTenant)
	actions := mockClient.Actions()
	if len(actions) != 1 {
		t.Errorf("Expected 1 mock client action, but got %v", len(actions))
	}
	if !actions[0].Matches("create", "tenants") || actions[0].GetSubresource() != "finalize" {
		t.Errorf("Expected finalize-tenant action %v", actions[0])
	}
	finalizers := actions[0].(core.CreateAction).GetObject().(*v1.Tenant).Spec.Finalizers
	if len(finalizers) != 1 {
		t.Errorf("There should be a single finalizer remaining")
	}
	if "other" != string(finalizers[0]) {
		t.Errorf("Unexpected finalizer value, %v", finalizers[0])
	}
}

func testSyncTenantThatIsTerminating(t *testing.T, versions *metav1.APIVersions) {
	now := metav1.Now()
	tenantName := "test"
	testTenantPendingFinalize := &v1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              tenantName,
			ResourceVersion:   "1",
			DeletionTimestamp: &now,
		},
		Spec: v1.TenantSpec{
			Finalizers: []v1.FinalizerName{v1.FinalizerArktos},
		},
		Status: v1.TenantStatus{
			Phase: v1.TenantTerminating,
		},
	}
	testTenantFinalizeComplete := &v1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              tenantName,
			ResourceVersion:   "1",
			DeletionTimestamp: &now,
		},
		Spec: v1.TenantSpec{},
		Status: v1.TenantStatus{
			Phase: v1.TenantTerminating,
		},
	}

	// when doing a delete all of content, we will do a GET of a collection, and DELETE of a collection by default
	metadataClientActionSet := sets.NewString()
	resources := testResources()
	groupVersionResources, _ := discovery.GroupVersionResources(resources)
	for groupVersionResource := range groupVersionResources {
		var urlPath string

		urlPath = path.Join([]string{
			dynamic.LegacyAPIPathResolverFunc(schema.GroupVersionKind{Group: groupVersionResource.Group, Version: groupVersionResource.Version}),
			groupVersionResource.Group,
			groupVersionResource.Version,
			"tenants",
			tenantName,
			groupVersionResource.Resource,
		}...)

		metadataClientActionSet.Insert((&fakeAction{method: "GET", path: urlPath}).String())
		metadataClientActionSet.Insert((&fakeAction{method: "DELETE", path: urlPath}).String())
	}

	scenarios := map[string]struct {
		testTenant              *v1.Tenant
		kubeClientActionSet     sets.String
		metadataClientActionSet sets.String
		gvrError                error
	}{
		"pending-finalize": {
			testTenant: testTenantPendingFinalize,
			kubeClientActionSet: sets.NewString(
				strings.Join([]string{"get", "tenants", ""}, "-"),
				strings.Join([]string{"create", "tenants", "finalize"}, "-"),
				strings.Join([]string{"list", "namespaces", ""}, "-"),
				strings.Join([]string{"delete", "tenants", ""}, "-"),
			),
			metadataClientActionSet: metadataClientActionSet,
		},
		"complete-finalize": {
			testTenant: testTenantFinalizeComplete,
			kubeClientActionSet: sets.NewString(
				strings.Join([]string{"get", "tenants", ""}, "-"),
				strings.Join([]string{"delete", "tenants", ""}, "-"),
			),
			metadataClientActionSet: sets.NewString(),
		},
		"groupVersionResourceErr": {
			testTenant: testTenantFinalizeComplete,
			kubeClientActionSet: sets.NewString(
				strings.Join([]string{"get", "tenants", ""}, "-"),
				strings.Join([]string{"delete", "tenants", ""}, "-"),
			),
			metadataClientActionSet: sets.NewString(),
			gvrError:                fmt.Errorf("test error"),
		},
	}

	for scenario, testInput := range scenarios {
		testHandler := &fakeActionHandler{statusCode: 200}
		srv, clientConfig := testServerAndClientConfig(testHandler.ServeHTTP)
		defer srv.Close()

		mockClient := fake.NewSimpleClientset(testInput.testTenant)
		metadataClient, err := metadata.NewForConfig(clientConfig)
		if err != nil {
			t.Fatal(err)
		}

		fn := func() ([]*metav1.APIResourceList, error) {
			return resources, nil
		}
		d := NewTenantedResourcesDeleter(mockClient, metadataClient, fn, v1.FinalizerArktos)
		if err := d.Delete(testInput.testTenant.Name); err != nil {
			t.Errorf("scenario %s - Unexpected error when synching tenant %v", scenario, err)
		}

		// validate traffic from kube client
		actionSet := sets.NewString()
		for _, action := range mockClient.Actions() {
			actionSet.Insert(strings.Join([]string{action.GetVerb(), action.GetResource().Resource, action.GetSubresource()}, "-"))
		}
		if !actionSet.Equal(testInput.kubeClientActionSet) {
			t.Errorf("scenario %s - mock client expected actions:\n%v\n but got:\n%v\nDifference:\n%v", scenario,
				testInput.kubeClientActionSet, actionSet, testInput.kubeClientActionSet.Difference(actionSet))
		}

		// validate traffic from metadata client
		actionSet = sets.NewString()
		for _, action := range testHandler.actions {
			actionSet.Insert(action.String())
		}
		if !actionSet.Equal(testInput.metadataClientActionSet) {
			t.Errorf("scenario %s - metadata client expected actions:\n%v\n but got:\n%v\nDifference:\n%v", scenario,
				testInput.metadataClientActionSet, actionSet, testInput.metadataClientActionSet.Difference(actionSet))
		}
	}
}

func TestSyncTenantThatIsTerminatingNonExperimental(t *testing.T) {
	testSyncTenantThatIsTerminating(t, &metav1.APIVersions{})
}

func TestSyncTenantThatIsTerminatingV1(t *testing.T) {
	testSyncTenantThatIsTerminating(t, &metav1.APIVersions{Versions: []string{"policy/v1beta1"}})
}

func TestRetryOnConflictError(t *testing.T) {
	mockClient := &fake.Clientset{}
	numTries := 0
	retryOnce := func(tenant *v1.Tenant) (*v1.Tenant, error) {
		numTries++
		if numTries <= 1 {
			return tenant, errors.NewConflict(api.Resource("tenants"), tenant.Name, fmt.Errorf("ERROR"))
		}
		return tenant, nil
	}
	tenant := &v1.Tenant{}
	d := tenantedResourcesDeleter{
		kubeClient: mockClient,
	}
	_, err := d.retryOnConflictError(tenant, retryOnce)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if numTries != 2 {
		t.Errorf("Expected %v, but got %v", 2, numTries)
	}
}

func TestSyncTenantThatIsActive(t *testing.T) {
	mockClient := &fake.Clientset{}
	testTenant := &v1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "1",
		},
		Spec: v1.TenantSpec{
			Finalizers: []v1.FinalizerName{v1.FinalizerArktos},
		},
		Status: v1.TenantStatus{
			Phase: v1.TenantActive,
		},
	}
	fn := func() ([]*metav1.APIResourceList, error) {
		return testResources(), nil
	}
	d := NewTenantedResourcesDeleter(mockClient, nil, fn, v1.FinalizerArktos)
	err := d.Delete(testTenant.Name)
	if err != nil {
		t.Errorf("Unexpected error when synching tenant %v", err)
	}
	if len(mockClient.Actions()) != 1 {
		t.Errorf("Expected only one action from controller, but got: %d %v", len(mockClient.Actions()), mockClient.Actions())
	}
	action := mockClient.Actions()[0]
	if !action.Matches("get", "tenants") {
		t.Errorf("Expected get tenants, got: %v", action)
	}
}

// testServerAndClientConfig returns a server that listens and a config that can reference it
func testServerAndClientConfig(handler func(http.ResponseWriter, *http.Request)) (*httptest.Server, *restclient.Config) {
	srv := httptest.NewServer(http.HandlerFunc(handler))
	kubeConfig := &restclient.KubeConfig{
		Host: srv.URL,
	}
	configs := restclient.NewAggregatedConfig(kubeConfig)
	return srv, configs
}

// fakeAction records information about requests to aid in testing.
type fakeAction struct {
	method string
	path   string
}

// String returns method=path to aid in testing
func (f *fakeAction) String() string {
	return strings.Join([]string{f.method, f.path}, "=")
}

// fakeActionHandler holds a list of fakeActions received
type fakeActionHandler struct {
	// statusCode returned by this handler
	statusCode int

	lock    sync.Mutex
	actions []fakeAction
}

// ServeHTTP logs the action that occurred and always returns the associated status code
func (f *fakeActionHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.actions = append(f.actions, fakeAction{method: request.Method, path: request.URL.Path})
	response.Header().Set("Content-Type", runtime.ContentTypeJSON)
	response.WriteHeader(f.statusCode)
	response.Write([]byte("{\"apiVersion\": \"v1\", \"kind\": \"List\",\"items\":null}"))
}

// testResources returns a mocked up set of resources across different api groups for testing tenant controller.
func testResources() []*metav1.APIResourceList {
	results := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "namespaces",
					Tenanted:   true,
					Namespaced: false,
					Kind:       "Namespace",
					Verbs:      []string{"get", "list", "delete", "deletecollection", "create", "update"},
				},
				{
					Name:       "persistentvolumes",
					Tenanted:   true,
					Namespaced: false,
					Kind:       "PersistentVolume",
					Verbs:      []string{"get", "list", "delete", "deletecollection", "create", "update"},
				},
			},
		},
		{
			GroupVersion: "policy/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Name:     "podsecuritypolicies",
					Tenanted: true,
					Kind:     "PodSecurityPolicy",
					Verbs:    []string{"get", "list", "delete", "deletecollection", "create", "update"},
				},
			},
		},
	}
	return results
}
