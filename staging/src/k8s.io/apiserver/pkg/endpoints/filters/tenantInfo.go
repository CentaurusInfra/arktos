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

// WithTenantInfo set the tenant in the requestInfo based on the user info from the authentication result.
func WithTenantInfo(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		requestInfo, exists := request.RequestInfoFrom(ctx)
		tenantInRequestor := ""
		requestor, exists := request.UserFrom(ctx)
		if !exists || requestor.GetTenant() == metav1.TenantNone {
			responsewriters.InternalError(w, req, errors.New("The user tenant for the request cannot be identfied."))
			return
		} else {
			tenantInRequestor = requestor.GetTenant()
		}

		tenant, err := normalizeTenant(tenantInRequestor, requestInfo.Tenant)
		if err != nil {
			responsewriters.InternalError(w, req, err)
			return
		}

		requestInfo.Tenant = tenant
		req = req.WithContext(request.WithRequestInfo(ctx, requestInfo))

		handler.ServeHTTP(w, req)
	})
}

// normalizeObjectTenant returns the tenant of the object based on what the user tenant is.
func normalizeTenant(userTenant, objectTenant string) (string, error) {
	if userTenant == metav1.TenantNone {
		return "", fmt.Errorf("The user Tenant is null")
	}

	// for a reqeust from a regular user, if the tenant in the object is empty, use the tenant from user info
	// this is what we call "shor-path", which allows users to use traditional Kubernets API in the multi-tenancy Arktos
	if objectTenant == metav1.TenantNone && userTenant != metav1.TenantSystem {
		return userTenant, nil
	}

	// in the other cases, we continue to use the tenant in the request info
	return objectTenant, nil
}
