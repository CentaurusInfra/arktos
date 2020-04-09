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

package discovery

import (
	"errors"
	"net/http"

	restful "github.com/emicklei/go-restful"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/handlers/negotiation"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type APIResourceLister interface {
	ListAPIResources(filter func(metav1.APIResource) bool) []metav1.APIResource
}

type APIResourceListerFunc func(filter func(metav1.APIResource) bool) []metav1.APIResource

func (f APIResourceListerFunc) ListAPIResources(filter func(metav1.APIResource) bool) []metav1.APIResource {
	return f(filter)
}

// APIVersionHandler creates a webservice serving the supported resources for the version
// E.g., such a web service will be registered at /apis/extensions/v1beta1.
type APIVersionHandler struct {
	serializer runtime.NegotiatedSerializer

	groupVersion      schema.GroupVersion
	apiResourceLister APIResourceLister
}

func NewAPIVersionHandler(serializer runtime.NegotiatedSerializer, groupVersion schema.GroupVersion, apiResourceLister APIResourceLister) *APIVersionHandler {
	if keepUnversioned(groupVersion.Group) {
		// Because in release 1.1, /apis/extensions returns response with empty
		// APIVersion, we use stripVersionNegotiatedSerializer to keep the
		// response backwards compatible.
		serializer = stripVersionNegotiatedSerializer{serializer}
	}

	return &APIVersionHandler{
		serializer:        serializer,
		groupVersion:      groupVersion,
		apiResourceLister: apiResourceLister,
	}
}

func (s *APIVersionHandler) AddToWebService(ws *restful.WebService) {
	mediaTypes, _ := negotiation.MediaTypesForSerializer(s.serializer)
	ws.Route(ws.GET("/").To(s.handle).
		Doc("get available resources").
		Operation("getAPIResources").
		Produces(mediaTypes...).
		Consumes(mediaTypes...).
		Writes(metav1.APIResourceList{}))
}

// handle returns a handler which will return the api.VersionAndVersion of the group.
func (s *APIVersionHandler) handle(req *restful.Request, resp *restful.Response) {
	s.ServeHTTP(resp.ResponseWriter, req.Request)
}

func (s *APIVersionHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var filterFunc func(metav1.APIResource) bool
	ctx := req.Context()
	requestor, exists := request.UserFrom(ctx)

	if !exists {
		responsewriters.InternalError(w, req, errors.New("The user info is missing."))
		return
	}

	tenant := requestor.GetTenant()

	if tenant == metav1.TenantNone {
		filterFunc = nil
		// workaround
		/*
			responsewriters.InternalError(w, req, errors.New(fmt.Sprintf("The tenant in the user info %s is missing.", requestor.GetName()))
			return*/
	}

	if tenant == metav1.TenantSystem {
		filterFunc = nil
	} else {
		// for regular tenants, we only return the resources that are tenant-scoped
		filterFunc = func(resource metav1.APIResource) bool {
			return resource.Tenanted
		}
	}

	resources := s.apiResourceLister.ListAPIResources(filterFunc)
	result := []metav1.APIResource{}
	for _, resource := range resources {
		if resource.Tenant == "" || tenant == "system" || (resource.Tenant != "" && resource.Tenant == tenant) {
			result = append(result, resource)
		}
	}

	responsewriters.WriteObjectNegotiated(s.serializer, negotiation.DefaultEndpointRestrictions, schema.GroupVersion{}, w, req, http.StatusOK,
		&metav1.APIResourceList{GroupVersion: s.groupVersion.String(), APIResources: result})
}
