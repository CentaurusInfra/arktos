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
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// WithTenantInfo set the tenant in the requestInfo for short-path requests based on the user info from the authentication result.
func WithTenantInfo(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		newReq, err := SetShortPathRequestTenant(req)
		if err != nil {
			responsewriters.InternalError(w, req, err)
			return
		}

		handler.ServeHTTP(w, newReq)
	})
}

// SetShortPathRequestTenant sets the tenant in request info based on the user tenant.
func SetShortPathRequestTenant(req *http.Request) (*http.Request, error) {

	ctx := req.Context()

	requestor, exists := request.UserFrom(ctx)
	if !exists {
		return nil, errors.New("The user info is missing.")
	}

	userTenant := requestor.GetTenant()
	if userTenant == metav1.TenantNone {
		// temporary workaround
		// tracking issue: https://github.com/futurewei-cloud/arktos/issues/102
		userTenant = metav1.TenantSystem
		//When https://github.com/futurewei-cloud/arktos/issues/102 is done, remove the above line
		// and enable the following two lines.
		//responsewriters.InternalError(w, req, errors.New(fmt.Sprintf("The tenant in the user info of %s is empty. ", requestor.GetName())))
		//return
	}

	requestInfo, exists := request.RequestInfoFrom(ctx)
	if !exists {
		return nil, errors.New("The request info is missing.")
	}

	// for a reqeust from a regular user, if the tenant in the object is empty, use the tenant from user info
	// this is what we call "short-path", which allows users to use traditional Kubernets API in the multi-tenancy Arktos
	resourceTenant := requestInfo.Tenant
	if resourceTenant == metav1.TenantNone {
		requestInfo.Tenant = userTenant
	}

	if resourceTenant == metav1.TenantAllExplicit {
		requestInfo.Tenant = metav1.TenantAll
	}

	req = req.WithContext(request.WithRequestInfo(ctx, requestInfo))

	return req, nil
}
