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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// WithTenantInfo set the tenant in the requestInfo for short-path requests based on the user info from the authentication result.
func WithTenantInfo(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		requestor, exists := request.UserFrom(ctx)
		if !exists {
			responsewriters.InternalError(w, req, errors.New("The user info is missing."))
			return
		}

		tenantInRequestor := requestor.GetTenant()
		if tenantInRequestor == metav1.TenantNone {
			responsewriters.InternalError(w, req, errors.New(fmt.Sprintf("The tenant in the user info of %s is empty. ", requestor.GetName())))
			return
		}

		requestInfo, exists := request.RequestInfoFrom(ctx)
		if !exists {
			responsewriters.InternalError(w, req, errors.New("The request info is missing."))
			return
		}

		requestInfo.Tenant = normalizeTenant(tenantInRequestor, requestInfo.Tenant)
		req = req.WithContext(request.WithRequestInfo(ctx, requestInfo))

		handler.ServeHTTP(w, req)
	})
}

// normalizeObjectTenant decides what the object tenant should be based on what the user tenant is.
func normalizeTenant(userTenant, objectTenant string) string {
	// for a reqeust from a regular user, if the tenant in the object is empty, use the tenant from user info
	// this is what we call "shor-path", which allows users to use traditional Kubernets API in the multi-tenancy Arktos
	if objectTenant == metav1.TenantNone && userTenant != metav1.TenantSystem {
		return userTenant
	}

	// in the other cases, we continue to use the tenant in the request info
	return objectTenant
}
