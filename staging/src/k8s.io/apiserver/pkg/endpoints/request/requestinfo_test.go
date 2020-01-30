/*
Copyright 2016 The Kubernetes Authors.
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

package request

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// This test verifies that the legacy urls before the introduction of multi-tenancy still work.
// Validations are added to check the tenant value is set as expected for the legacy urls.
// This test should be removed when we no longer support the legacy urls..
func TestGetAPIRequestInfo(t *testing.T) {
	namespaceAll := metav1.NamespaceAll
	successCases := []struct {
		method              string
		url                 string
		expectedVerb        string
		expectedAPIPrefix   string
		expectedAPIGroup    string
		expectedAPIVersion  string
		expectedNamespace   string
		expectedResource    string
		expectedSubresource string
		expectedName        string
		expectedParts       []string
	}{
		// resource paths
		{"GET", "/api/v1/namespaces", "list", "api", "", "v1", "", "namespaces", "", "", []string{"namespaces"}},
		{"GET", "/api/v1/namespaces/other", "get", "api", "", "v1", "other", "namespaces", "", "other", []string{"namespaces", "other"}},

		{"GET", "/api/v1/namespaces/other/pods", "list", "api", "", "v1", "other", "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/namespaces/other/pods/foo", "get", "api", "", "v1", "other", "pods", "", "foo", []string{"pods", "foo"}},
		{"HEAD", "/api/v1/namespaces/other/pods/foo", "get", "api", "", "v1", "other", "pods", "", "foo", []string{"pods", "foo"}},
		{"GET", "/api/v1/pods", "list", "api", "", "v1", namespaceAll, "pods", "", "", []string{"pods"}},
		{"HEAD", "/api/v1/pods", "list", "api", "", "v1", namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/namespaces/other/pods/foo", "get", "api", "", "v1", "other", "pods", "", "foo", []string{"pods", "foo"}},
		{"GET", "/api/v1/namespaces/other/pods", "list", "api", "", "v1", "other", "pods", "", "", []string{"pods"}},

		// special verbs
		{"GET", "/api/v1/proxy/namespaces/other/pods/foo", "proxy", "api", "", "v1", "other", "pods", "", "foo", []string{"pods", "foo"}},
		{"GET", "/api/v1/proxy/namespaces/other/pods/foo/subpath/not/a/subresource", "proxy", "api", "", "v1", "other", "pods", "", "foo", []string{"pods", "foo", "subpath", "not", "a", "subresource"}},
		{"GET", "/api/v1/watch/pods", "watch", "api", "", "v1", namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/pods?watch=true", "watch", "api", "", "v1", namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/pods?watch=false", "list", "api", "", "v1", namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/watch/namespaces/other/pods", "watch", "api", "", "v1", "other", "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/namespaces/other/pods?watch=1", "watch", "api", "", "v1", "other", "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/namespaces/other/pods?watch=0", "list", "api", "", "v1", "other", "pods", "", "", []string{"pods"}},

		// subresource identification
		{"GET", "/api/v1/namespaces/other/pods/foo/status", "get", "api", "", "v1", "other", "pods", "status", "foo", []string{"pods", "foo", "status"}},
		{"GET", "/api/v1/namespaces/other/pods/foo/proxy/subpath", "get", "api", "", "v1", "other", "pods", "proxy", "foo", []string{"pods", "foo", "proxy", "subpath"}},
		{"PUT", "/api/v1/namespaces/other/finalize", "update", "api", "", "v1", "other", "namespaces", "finalize", "other", []string{"namespaces", "other", "finalize"}},
		{"PUT", "/api/v1/namespaces/other/status", "update", "api", "", "v1", "other", "namespaces", "status", "other", []string{"namespaces", "other", "status"}},

		// verb identification
		{"PATCH", "/api/v1/namespaces/other/pods/foo", "patch", "api", "", "v1", "other", "pods", "", "foo", []string{"pods", "foo"}},
		{"DELETE", "/api/v1/namespaces/other/pods/foo", "delete", "api", "", "v1", "other", "pods", "", "foo", []string{"pods", "foo"}},
		{"POST", "/api/v1/namespaces/other/pods", "create", "api", "", "v1", "other", "pods", "", "", []string{"pods"}},

		// deletecollection verb identification
		{"DELETE", "/api/v1/nodes", "deletecollection", "api", "", "v1", "", "nodes", "", "", []string{"nodes"}},
		{"DELETE", "/api/v1/namespaces/other/pods", "deletecollection", "api", "", "v1", "other", "pods", "", "", []string{"pods"}},
		{"DELETE", "/apis/extensions/v1/namespaces/other/pods", "deletecollection", "api", "extensions", "v1", "other", "pods", "", "", []string{"pods"}},

		// api group identification
		{"POST", "/apis/extensions/v1/namespaces/other/pods", "create", "api", "extensions", "v1", "other", "pods", "", "", []string{"pods"}},

		// api version identification
		{"POST", "/apis/extensions/v1beta3/namespaces/other/pods", "create", "api", "extensions", "v1beta3", "other", "pods", "", "", []string{"pods"}},
	}

	resolver := newTestRequestInfoResolver()

	for _, successCase := range successCases {
		req, _ := http.NewRequest(successCase.method, successCase.url, nil)

		apiRequestInfo, err := resolver.NewRequestInfo(req)
		if err != nil {
			t.Errorf("Unexpected error for url: %s %v", successCase.url, err)
		}
		if !apiRequestInfo.IsResourceRequest {
			t.Errorf("Expected resource request")
		}
		if successCase.expectedVerb != apiRequestInfo.Verb {
			t.Errorf("Unexpected verb for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedVerb, apiRequestInfo.Verb)
		}
		if successCase.expectedAPIVersion != apiRequestInfo.APIVersion {
			t.Errorf("Unexpected apiVersion for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedAPIVersion, apiRequestInfo.APIVersion)
		}
		if successCase.expectedNamespace != apiRequestInfo.Namespace {
			t.Errorf("Unexpected namespace for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedNamespace, apiRequestInfo.Namespace)
		}
		if successCase.expectedResource != apiRequestInfo.Resource {
			t.Errorf("Unexpected resource for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedResource, apiRequestInfo.Resource)
		}
		if successCase.expectedSubresource != apiRequestInfo.Subresource {
			t.Errorf("Unexpected resource for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedSubresource, apiRequestInfo.Subresource)
		}
		if successCase.expectedName != apiRequestInfo.Name {
			t.Errorf("Unexpected name for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedName, apiRequestInfo.Name)
		}
		if !reflect.DeepEqual(successCase.expectedParts, apiRequestInfo.Parts) {
			t.Errorf("Unexpected parts for url: %s, expected: %v, actual: %v", successCase.url, successCase.expectedParts, apiRequestInfo.Parts)
		}
		// Here we check the tenant field is set as expected for those legacy urls
		if metav1.TenantNone != apiRequestInfo.Tenant {
			t.Errorf("Unexpected tenant for url: %s, expected: %s, actual: %s", successCase.url, metav1.TenantNone, apiRequestInfo.Tenant)
		}

	}

	errorCases := map[string]string{
		"no resource path":            "/",
		"just apiversion":             "/api/version/",
		"just prefix, group, version": "/apis/group/version/",
		"apiversion with no resource": "/api/version/",
		"bad prefix":                  "/badprefix/version/resource",
		"missing api group":           "/apis/version/resource",
	}
	for k, v := range errorCases {
		req, err := http.NewRequest("GET", v, nil)
		if err != nil {
			t.Errorf("Unexpected error %v", err)
		}
		apiRequestInfo, err := resolver.NewRequestInfo(req)
		if err != nil {
			t.Errorf("%s: Unexpected error %v", k, err)
		}
		if apiRequestInfo.IsResourceRequest {
			t.Errorf("%s: expected non-resource request", k)
		}
	}
}

// Here we test the multi-tenancy aware api urls.
func TestGetMultiTenancyAPIRequestInfo(t *testing.T) {
	namespaceAll := metav1.NamespaceAll
	tenantAll := metav1.TenantAll
	successCases := []struct {
		method              string
		url                 string
		expectedVerb        string
		expectedAPIPrefix   string
		expectedAPIGroup    string
		expectedAPIVersion  string
		expectedTenant      string
		expectedNamespace   string
		expectedResource    string
		expectedSubresource string
		expectedName        string
		expectedParts       []string
	}{

		// resource paths
		{"GET", "/api/v1/tenants", "list", "api", "", "v1", "", "", "tenants", "", "", []string{"tenants"}},
		{"GET", "/api/v1/tenants/fake_te", "get", "api", "", "v1", "", "", "tenants", "", "fake_te", []string{"tenants", "fake_te"}},
		{"GET", "/api/v1/tenants/fake_te/namespaces", "list", "api", "", "v1", "fake_te", "", "namespaces", "", "", []string{"namespaces"}},
		{"GET", "/api/v1/tenants/fake_te/namespaces/fake_ns", "get", "api", "", "v1", "fake_te", "fake_ns", "namespaces", "", "fake_ns", []string{"namespaces", "fake_ns"}},

		{"GET", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods", "list", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods/foo", "get", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "foo", []string{"pods", "foo"}},
		{"HEAD", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods/foo", "get", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "foo", []string{"pods", "foo"}},
		{"GET", "/api/v1/tenants/fake_te/pods", "list", "api", "", "v1", "fake_te", namespaceAll, "pods", "", "", []string{"pods"}},
		{"HEAD", "/api/v1/tenants/fake_te/pods", "list", "api", "", "v1", "fake_te", namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/pods", "list", "api", "", "v1", tenantAll, namespaceAll, "pods", "", "", []string{"pods"}},
		{"HEAD", "/api/v1/pods", "list", "api", "", "v1", tenantAll, namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods/foo", "get", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "foo", []string{"pods", "foo"}},
		{"GET", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods", "list", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/namespaces", "list", "api", "", "v1", tenantAll, namespaceAll, "namespaces", "", "", []string{"namespaces"}},

		// special verbs
		{"GET", "/api/v1/proxy/tenants/fake_te/namespaces/fake_ns/pods/foo", "proxy", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "foo", []string{"pods", "foo"}},
		{"GET", "/api/v1/proxy/tenants/fake_te/namespaces/fake_ns/pods/foo/subpath/not/a/subresource", "proxy", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "foo", []string{"pods", "foo", "subpath", "not", "a", "subresource"}},
		{"GET", "/api/v1/watch/tenants/fake_te/pods", "watch", "api", "", "v1", "fake_te", namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/tenants/fake_te/pods?watch=true", "watch", "api", "", "v1", "fake_te", namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/tenants/fake_te/pods?watch=false", "list", "api", "", "v1", "fake_te", namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/watch/pods", "watch", "api", "", "v1", tenantAll, namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/watch/namespaces", "watch", "api", "", "v1", tenantAll, namespaceAll, "namespaces", "", "", []string{"namespaces"}},
		{"GET", "/api/v1/pods?watch=true", "watch", "api", "", "v1", tenantAll, namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/pods?watch=false", "list", "api", "", "v1", tenantAll, namespaceAll, "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/watch/tenants/fake_te/namespaces/fake_ns/pods", "watch", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods?watch=1", "watch", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "", []string{"pods"}},
		{"GET", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods?watch=0", "list", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "", []string{"pods"}},

		// subresource identification
		{"GET", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods/foo/status", "get", "api", "", "v1", "fake_te", "fake_ns", "pods", "status", "foo", []string{"pods", "foo", "status"}},
		{"GET", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods/foo/proxy/subpath", "get", "api", "", "v1", "fake_te", "fake_ns", "pods", "proxy", "foo", []string{"pods", "foo", "proxy", "subpath"}},
		{"PUT", "/api/v1/tenants/fake_te/namespaces/fake_ns/finalize", "update", "api", "", "v1", "fake_te", "fake_ns", "namespaces", "finalize", "fake_ns", []string{"namespaces", "fake_ns", "finalize"}},
		{"PUT", "/api/v1/tenants/fake_te/namespaces/fake_ns/status", "update", "api", "", "v1", "fake_te", "fake_ns", "namespaces", "status", "fake_ns", []string{"namespaces", "fake_ns", "status"}},
		{"PUT", "/api/v1/tenants/fake_te/finalize", "update", "api", "", "v1", "", "", "tenants", "finalize", "fake_te", []string{"tenants", "fake_te", "finalize"}},
		{"PUT", "/api/v1/tenants/fake_te/status", "update", "api", "", "v1", "", "", "tenants", "status", "fake_te", []string{"tenants", "fake_te", "status"}},
		{"PUT", "/api/v1/namespaces", "update", "api", "", "v1", tenantAll, namespaceAll, "namespaces", "", "", []string{"namespaces"}},

		// verb identification
		{"PATCH", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods/foo", "patch", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "foo", []string{"pods", "foo"}},
		{"DELETE", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods/foo", "delete", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "foo", []string{"pods", "foo"}},
		{"POST", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods", "create", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "", []string{"pods"}},
		{"PATCH", "/api/v1/namespaces", "patch", "api", "", "v1", tenantAll, namespaceAll, "namespaces", "", "", []string{"namespaces"}},
		{"POST", "/api/v1/namespaces", "create", "api", "", "v1", tenantAll, namespaceAll, "namespaces", "", "", []string{"namespaces"}},

		// deletecollection verb identification
		{"DELETE", "/api/v1/nodes", "deletecollection", "api", "", "v1", "", "", "nodes", "", "", []string{"nodes"}},
		{"DELETE", "/api/v1/tenants", "deletecollection", "api", "", "v1", "", "", "tenants", "", "", []string{"tenants"}},
		{"DELETE", "/api/v1/tenants/fake_te/namespaces", "deletecollection", "api", "", "v1", "fake_te", "", "namespaces", "", "", []string{"namespaces"}},
		{"DELETE", "/api/v1/tenants/fake_te/namespaces/fake_ns/pods", "deletecollection", "api", "", "v1", "fake_te", "fake_ns", "pods", "", "", []string{"pods"}},
		{"DELETE", "/apis/extensions/v1/tenants/fake_te/namespaces/fake_ns/pods", "deletecollection", "api", "extensions", "v1", "fake_te", "fake_ns", "pods", "", "", []string{"pods"}},
		{"DELETE", "/api/v1/namespaces", "deletecollection", "api", "", "v1", tenantAll, namespaceAll, "namespaces", "", "", []string{"namespaces"}},

		// api group identification
		{"POST", "/apis/extensions/v1/tenants/fake_te/namespaces/fake_ns/pods", "create", "api", "extensions", "v1", "fake_te", "fake_ns", "pods", "", "", []string{"pods"}},

		// api version identification
		{"POST", "/apis/extensions/v1beta3/tenants/fake_te/namespaces/fake_ns/pods", "create", "api", "extensions", "v1beta3", "fake_te", "fake_ns", "pods", "", "", []string{"pods"}},
	}

	resolver := newTestRequestInfoResolver()

	for _, successCase := range successCases {
		req, _ := http.NewRequest(successCase.method, successCase.url, nil)

		apiRequestInfo, err := resolver.NewRequestInfo(req)
		if err != nil {
			t.Errorf("Unexpected error for url: %s %v", successCase.url, err)
		}
		if !apiRequestInfo.IsResourceRequest {
			t.Errorf("Expected resource request")
		}
		if successCase.expectedVerb != apiRequestInfo.Verb {
			t.Errorf("Unexpected verb for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedVerb, apiRequestInfo.Verb)
		}
		if successCase.expectedAPIVersion != apiRequestInfo.APIVersion {
			t.Errorf("Unexpected apiVersion for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedAPIVersion, apiRequestInfo.APIVersion)
		}
		if successCase.expectedTenant != apiRequestInfo.Tenant {
			t.Errorf("Unexpected tenant for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedTenant, apiRequestInfo.Tenant)
		}
		if successCase.expectedNamespace != apiRequestInfo.Namespace {
			t.Errorf("Unexpected namespace for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedNamespace, apiRequestInfo.Namespace)
		}
		if successCase.expectedResource != apiRequestInfo.Resource {
			t.Errorf("Unexpected resource for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedResource, apiRequestInfo.Resource)
		}
		if successCase.expectedSubresource != apiRequestInfo.Subresource {
			t.Errorf("Unexpected resource for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedSubresource, apiRequestInfo.Subresource)
		}
		if successCase.expectedName != apiRequestInfo.Name {
			t.Errorf("Unexpected name for url: %s, expected: %s, actual: %s", successCase.url, successCase.expectedName, apiRequestInfo.Name)
		}
		if !reflect.DeepEqual(successCase.expectedParts, apiRequestInfo.Parts) {
			t.Errorf("Unexpected parts for url: %s, expected: %v, actual: %v", successCase.url, successCase.expectedParts, apiRequestInfo.Parts)
		}
	}

	errorCases := map[string]string{
		"no resource path":            "/",
		"just apiversion":             "/api/version/",
		"just prefix, group, version": "/apis/group/version/",
		"apiversion with no resource": "/api/version/",
		"bad prefix":                  "/badprefix/version/resource",
		"missing api group":           "/apis/version/resource",
	}
	for k, v := range errorCases {
		req, err := http.NewRequest("GET", v, nil)
		if err != nil {
			t.Errorf("Unexpected error %v", err)
		}
		apiRequestInfo, err := resolver.NewRequestInfo(req)
		if err != nil {
			t.Errorf("%s: Unexpected error %v", k, err)
		}
		if apiRequestInfo.IsResourceRequest {
			t.Errorf("%s: expected non-resource request", k)
		}
	}
}

func TestGetNonAPIRequestInfo(t *testing.T) {
	tests := map[string]struct {
		url      string
		expected bool
	}{
		"simple groupless":  {"/api/version/resource", true},
		"simple group":      {"/apis/group/version/resource/name/subresource", true},
		"more steps":        {"/api/version/resource/name/subresource", true},
		"group list":        {"/apis/batch/v1/job", true},
		"group get":         {"/apis/batch/v1/job/foo", true},
		"group subresource": {"/apis/batch/v1/job/foo/scale", true},

		"bad root":                     {"/not-api/version/resource", false},
		"group without enough steps":   {"/apis/extensions/v1beta1", false},
		"group without enough steps 2": {"/apis/extensions/v1beta1/", false},
		"not enough steps":             {"/api/version", false},
		"one step":                     {"/api", false},
		"zero step":                    {"/", false},
		"empty":                        {"", false},
	}

	resolver := newTestRequestInfoResolver()

	for testName, tc := range tests {
		req, _ := http.NewRequest("GET", tc.url, nil)

		apiRequestInfo, err := resolver.NewRequestInfo(req)
		if err != nil {
			t.Errorf("%s: Unexpected error %v", testName, err)
		}
		if e, a := tc.expected, apiRequestInfo.IsResourceRequest; e != a {
			t.Errorf("%s: expected %v, actual %v", testName, e, a)
		}
	}
}

func newTestRequestInfoResolver() *RequestInfoFactory {
	return &RequestInfoFactory{
		APIPrefixes:          sets.NewString("api", "apis"),
		GrouplessAPIPrefixes: sets.NewString("api"),
	}
}

func TestFieldSelectorParsing(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expectedName string
		expectedErr  error
		expectedVerb string
	}{
		{
			name:         "no selector",
			url:          "/apis/group/version/resource",
			expectedVerb: "list",
		},
		{
			name:         "metadata.name selector",
			url:          "/apis/group/version/resource?fieldSelector=metadata.name=name1",
			expectedName: "name1",
			expectedVerb: "list",
		},
		{
			name:         "metadata.name selector with watch",
			url:          "/apis/group/version/resource?watch=true&fieldSelector=metadata.name=name1",
			expectedName: "name1",
			expectedVerb: "watch",
		},
		{
			name:         "random selector",
			url:          "/apis/group/version/resource?fieldSelector=foo=bar",
			expectedName: "",
			expectedVerb: "list",
		},
		{
			name:         "invalid selector with metadata.name",
			url:          "/apis/group/version/resource?fieldSelector=metadata.name=name1,foo",
			expectedName: "",
			expectedErr:  fmt.Errorf("invalid selector"),
			expectedVerb: "list",
		},
		{
			name:         "invalid selector with metadata.name with watch",
			url:          "/apis/group/version/resource?fieldSelector=metadata.name=name1,foo&watch=true",
			expectedName: "",
			expectedErr:  fmt.Errorf("invalid selector"),
			expectedVerb: "watch",
		},
		{
			name:         "invalid selector with metadata.name with watch false",
			url:          "/apis/group/version/resource?fieldSelector=metadata.name=name1,foo&watch=false",
			expectedName: "",
			expectedErr:  fmt.Errorf("invalid selector"),
			expectedVerb: "list",
		},
	}

	resolver := newTestRequestInfoResolver()

	for _, tc := range tests {
		req, _ := http.NewRequest("GET", tc.url, nil)

		apiRequestInfo, err := resolver.NewRequestInfo(req)
		if err != nil {
			if tc.expectedErr == nil || !strings.Contains(err.Error(), tc.expectedErr.Error()) {
				t.Errorf("%s: Unexpected error %v", tc.name, err)
			}
		}
		if e, a := tc.expectedName, apiRequestInfo.Name; e != a {
			t.Errorf("%s: expected %v, actual %v", tc.name, e, a)
		}
		if e, a := tc.expectedVerb, apiRequestInfo.Verb; e != a {
			t.Errorf("%s: expected verb %v, actual %v", tc.name, e, a)
		}
	}
}
