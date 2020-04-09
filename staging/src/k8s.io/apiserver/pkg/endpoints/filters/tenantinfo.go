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

package filters

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// WithTenantInfo set the tenant in the requestInfo for short-path requests based on the user info from the authentication result.
func WithTenantInfo(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		newReq, err := NormalizeTenant(req)
		if err != nil {
			responsewriters.InternalError(w, req, err)
			return
		}

		handler.ServeHTTP(w, newReq)
	})
}

// NormalizeObjectTenant sets the tenant in request info based on the user tenant.
func NormalizeTenant(req *http.Request) (*http.Request, error) {

	ctx := req.Context()

	requestor, exists := request.UserFrom(ctx)
	if !exists {
		return nil, errors.New("The user info is missing.")
	}

	tenantInRequestor := requestor.GetTenant()
	if tenantInRequestor == metav1.TenantNone {
		return nil, errors.New("The tenant in the user info is empty.")
	}

	requestInfo, exists := request.RequestInfoFrom(ctx)
	if !exists {
		return nil, errors.New("The request info is missing.")
	}

	//fmt.Printf("\n ~~~~~~~~~~~ user %v %v ", requestor.GetTenant(), requestor.GetName())
	//fmt.Printf("\n ~~~~~~~~~~~ request %v %v", requestInfo.Verb, requestInfo.Path)

	// for a reqeust from a regular user, if the tenant in the object is empty, use the tenant from user info
	// this is what we call "shor-path", which allows users to use traditional Kubernets API in the multi-tenancy Arktos
	if requestInfo.Tenant == metav1.TenantNone && tenantInRequestor != metav1.TenantSystem {
		requestInfo.Tenant = tenantInRequestor
	}

	if strings.HasPrefix(requestInfo.Path, "/apis/samplecontroller.k8s.io") {
		//if requestInfo.Path == metav1.TenantDefault && (tenantInRequestor != metav1.TenantSystem && tenantInRequestor != metav1.TenantDefault) {
		fmt.Printf("\n ~~~~~~~~~~~ user %v %v ", requestor.GetTenant(), requestor.GetName())
		fmt.Printf("\n ~~~~~~~~~~~ request %v %v", requestInfo.Verb, requestInfo.Path)
		//	requestInfo.Tenant = tenantInRequestor
	}

	//fmt.Printf("\n ~~~~~~~~~~~ full request %#v", requestInfo)

	req = req.WithContext(request.WithRequestInfo(ctx, requestInfo))

	return req, nil
}
