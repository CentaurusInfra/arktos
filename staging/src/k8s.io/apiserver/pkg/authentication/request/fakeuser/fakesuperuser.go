/*
Copyright 2017 The Kubernetes Authors.
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

package fakeuser

import (
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
)

var FakeSuperUserInfo = &user.DefaultInfo{
	Name:   "fake-user",
	Groups: []string{user.SystemPrivilegedGroup, user.AllAuthenticated},
	Tenant: metav1.TenantSystem,
}

// FakeSuperUser implements authenticator.Request to always return a super user.
type FakeSuperUser struct{}

func (FakeSuperUser) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	auds, _ := authenticator.AudiencesFrom(req.Context())
	return &authenticator.Response{
		User:      FakeSuperUserInfo,
		Audiences: auds,
	}, true, nil
}
