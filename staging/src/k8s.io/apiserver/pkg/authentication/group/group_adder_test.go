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

package group

import (
	"net/http"
	"reflect"
	"testing"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	testTenant = "test-tenant"
)

func TestGroupAdder(t *testing.T) {
	adder := authenticator.Request(
		NewGroupAdder(
			authenticator.RequestFunc(func(req *http.Request) (*authenticator.Response, bool, error) {
				return &authenticator.Response{User: &user.DefaultInfo{Tenant: testTenant, Name: "user", Groups: []string{"original"}}}, true, nil
			}),
			[]string{"added"},
		),
	)

	r, _, _ := adder.AuthenticateRequest(nil)
	if want := []string{"original", "added"}; !reflect.DeepEqual(r.User.GetGroups(), want) {
		t.Errorf("Unexpected groups\ngot:\t%#v\nwant:\t%#v", r.User.GetGroups(), want)
	}
}

func TestAuthenticatedGroupAdder(t *testing.T) {
	tests := []struct {
		name         string
		inputUser    user.Info
		expectedUser user.Info
	}{
		{
			name: "add",
			inputUser: &user.DefaultInfo{
				Name:   "user",
				Groups: []string{"some-group"},
				Tenant: testTenant,
			},
			expectedUser: &user.DefaultInfo{
				Name:   "user",
				Groups: []string{"some-group", user.AllAuthenticated},
				Tenant: testTenant,
			},
		},
		{
			name: "don't double add",
			inputUser: &user.DefaultInfo{
				Name:   "user",
				Groups: []string{user.AllAuthenticated, "some-group"},
				Tenant: testTenant,
			},
			expectedUser: &user.DefaultInfo{
				Name:   "user",
				Groups: []string{user.AllAuthenticated, "some-group"},
				Tenant: testTenant,
			},
		},
		{
			name: "don't add for anon",
			inputUser: &user.DefaultInfo{
				Name:   user.Anonymous,
				Groups: []string{"some-group"},
				Tenant: testTenant,
			},
			expectedUser: &user.DefaultInfo{
				Name:   user.Anonymous,
				Groups: []string{"some-group"},
				Tenant: testTenant,
			},
		},
		{
			name: "don't add for unauthenticated group",
			inputUser: &user.DefaultInfo{
				Name:   "user",
				Groups: []string{user.AllUnauthenticated, "some-group"},
				Tenant: testTenant,
			},
			expectedUser: &user.DefaultInfo{
				Name:   "user",
				Groups: []string{user.AllUnauthenticated, "some-group"},
				Tenant: testTenant,
			},
		},
	}

	for _, test := range tests {
		adder := authenticator.Request(
			NewAuthenticatedGroupAdder(
				authenticator.RequestFunc(func(req *http.Request) (*authenticator.Response, bool, error) {
					return &authenticator.Response{User: test.inputUser}, true, nil
				}),
			),
		)

		r, _, _ := adder.AuthenticateRequest(nil)
		if !reflect.DeepEqual(r.User, test.expectedUser) {
			t.Errorf("Unexpected user\ngot:\t%#v\nwant:\t%#v", r.User, test.expectedUser)
		}
	}

}
