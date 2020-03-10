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

	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// WithTenantInfo set the tenant in the requestInfo based on the authentication result.
func WithTenantInfo(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		requestInfo, exists := request.RequestInfoFrom(ctx)
		if !exists {
			responsewriters.InternalError(w, req, errors.New("no request info found for request"))
			return
		}

		tenantInRequestor := ""
		requestor, exists := request.UserFrom(ctx)
		if !exists || requestor.GetTenant() == "" {
			// if we hit here and found that there is no requestor info in the request,
			// it means that the authenticator allows it in, it is mostly we are in a dev environment
			tenantInRequestor = "system"
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
	if userTenant == "" {
		return "", fmt.Errorf("The user Tenant is null")
	}

	if userTenant == "system" {
		// for system user, we don't touch the tenant value in the objectMeta
		return objectTenant, nil
	}

	if objectTenant != "" && objectTenant != userTenant {
		return "", fmt.Errorf("User under tenant %s is not allowed to access object under tenant %s.", userTenant, objectTenant)
	}

	return userTenant, nil
}
