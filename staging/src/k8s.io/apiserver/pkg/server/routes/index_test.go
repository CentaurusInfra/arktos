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

package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type fakePathProvider struct{}

func (p fakePathProvider) ListedPaths() []string {
	return []string{
		"aaaa",
		"bbbb?tenant=john",
		"bbbb?tenant=alice",
		"cccc?tenant=john",
		"dddd?tenant=alice",
	}
}

func getTenantedUserInfo(tenant string) user.Info {
	return &user.DefaultInfo{Name: "fake-user", Tenant: tenant}
}

type indexTestCase struct {
	Name          string
	tenant        string
	expectedPaths []string
}

func run(t *testing.T, testCase indexTestCase) {
	handler := IndexLister{StatusCode: http.StatusOK, PathProvider: fakePathProvider{}}

	req, _ := http.NewRequest("GET", "", nil)
	req.RemoteAddr = "127.0.0.1"
	ctx := req.Context()
	ctx = request.WithUser(ctx, getTenantedUserInfo(testCase.tenant))
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Test Case %q: unexpected response code %v, expected 200", testCase.Name, w.Code)
	}

	resp := w.Result()
	var resultPaths metav1.RootPaths
	if err := json.NewDecoder(resp.Body).Decode(&resultPaths); err != nil {
		t.Errorf("Test Case %q: failed to decode the response body JSON: %v", testCase.Name, err)
	}

	if len(resultPaths.Paths) != len(testCase.expectedPaths) {
		t.Errorf("Test Case %q: expected path %v, got %v", testCase.Name, testCase.expectedPaths, resultPaths.Paths)
		return
	}

	for i := range testCase.expectedPaths {
		if resultPaths.Paths[i] != testCase.expectedPaths[i] {
			t.Errorf("Test Case %q: expected path %v, got %v", testCase.Name, testCase.expectedPaths, resultPaths.Paths)
			return
		}
	}

}

func TestListPaths(t *testing.T) {
	testCases := []indexTestCase{
		{
			Name:   "system tenant root paths",
			tenant: metav1.TenantSystem,
			expectedPaths: []string{
				"aaaa",
			},
		},
		{
			Name:   "tenant alice root paths",
			tenant: "alice",
			expectedPaths: []string{
				"aaaa",
				"bbbb",
				"dddd",
			},
		},
		{
			Name:   "tenant john root paths",
			tenant: "john",
			expectedPaths: []string{
				"aaaa",
				"bbbb",
				"cccc",
			},
		},
	}

	for _, testCase := range testCases {
		run(t, testCase)
	}
}
