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

package apiserver

import (
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/endpoints/handlers/negotiation"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"

	apifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apiregistrationapi "k8s.io/kube-aggregator/pkg/apis/apiregistration"
	apiregistrationv1api "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1beta1api "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	listers "k8s.io/kube-aggregator/pkg/client/listers/apiregistration/internalversion"
)

// apisHandler serves the `/apis` endpoint.
// This is registered as a filter so that it never collides with any explicitly registered endpoints
type apisHandler struct {
	codecs serializer.CodecFactory
	lister listers.APIServiceLister
}

var discoveryGroup = metav1.APIGroup{
	Name: apiregistrationapi.GroupName,
	Versions: []metav1.GroupVersionForDiscovery{
		{
			GroupVersion: apiregistrationv1api.SchemeGroupVersion.String(),
			Version:      apiregistrationv1api.SchemeGroupVersion.Version,
		},
		{
			GroupVersion: apiregistrationv1beta1api.SchemeGroupVersion.String(),
			Version:      apiregistrationv1beta1api.SchemeGroupVersion.Version,
		},
	},
	PreferredVersion: metav1.GroupVersionForDiscovery{
		GroupVersion: apiregistrationv1api.SchemeGroupVersion.String(),
		Version:      apiregistrationv1api.SchemeGroupVersion.Version,
	},
}

// getApiServices return the list of APIService for the tenant in the request,
// including the native APIServices, which is under the system tenant
// and the APIServices corresponding to the CRDs created by the tenant in the request
func getApiServices(lister listers.APIServiceLister, req *http.Request) ([]*apiregistrationapi.APIService, error) {
	// Note: there is an issue here, the CRD created by the system tenant will be visible to all the tenants.
	// TODO: this issue will be addressed in the following work of CRD sharing
	// tracking issue: https://github.com/futurewei-cloud/arktos/issues/292
	systemApiServices, err := lister.APIServicesWithMultiTenancy(metav1.TenantSystem).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	apisTenant, err := apifilters.GetTenantFromQueryThenContext(req)
	if err != nil {
		return nil, err
	}

	if apisTenant == metav1.TenantSystem {
		return systemApiServices, nil
	}

	tenantApiServices, err := lister.APIServicesWithMultiTenancy(apisTenant).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	apiServices := append(systemApiServices, tenantApiServices...)

	return apiServices, nil
}

func (r *apisHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	discoveryGroupList := &metav1.APIGroupList{
		// always add OUR api group to the list first.  Since we'll never have a registered APIService for it
		// and since this is the crux of the API, having this first will give our names priority.  It's good to be king.
		Groups: []metav1.APIGroup{discoveryGroup},
	}

	apiServices, err := getApiServices(r.lister, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiServicesByGroup := apiregistrationapi.SortedByGroupAndVersion(apiServices)
	for _, apiGroupServers := range apiServicesByGroup {
		// skip the legacy group
		if len(apiGroupServers[0].Spec.Group) == 0 {
			continue
		}
		discoveryGroup := convertToDiscoveryAPIGroup(apiGroupServers)
		if discoveryGroup != nil {
			discoveryGroupList.Groups = append(discoveryGroupList.Groups, *discoveryGroup)
		}
	}

	responsewriters.WriteObjectNegotiated(r.codecs, negotiation.DefaultEndpointRestrictions, schema.GroupVersion{}, w, req, http.StatusOK, discoveryGroupList)
}

// convertToDiscoveryAPIGroup takes apiservices in a single group and returns a discovery compatible object.
// if none of the services are available, it will return nil.
func convertToDiscoveryAPIGroup(apiServices []*apiregistrationapi.APIService) *metav1.APIGroup {
	apiServicesByGroup := apiregistrationapi.SortedByGroupAndVersion(apiServices)[0]

	var discoveryGroup *metav1.APIGroup

	for _, apiService := range apiServicesByGroup {
		// the first APIService which is valid becomes the default
		if discoveryGroup == nil {
			discoveryGroup = &metav1.APIGroup{
				Name: apiService.Spec.Group,
				PreferredVersion: metav1.GroupVersionForDiscovery{
					GroupVersion: apiService.Spec.Group + "/" + apiService.Spec.Version,
					Version:      apiService.Spec.Version,
				},
			}
		}

		discoveryGroup.Versions = append(discoveryGroup.Versions,
			metav1.GroupVersionForDiscovery{
				GroupVersion: apiService.Spec.Group + "/" + apiService.Spec.Version,
				Version:      apiService.Spec.Version,
			},
		)
	}

	return discoveryGroup
}

// apiGroupHandler serves the `/apis/<group>` endpoint.
type apiGroupHandler struct {
	codecs    serializer.CodecFactory
	groupName string

	lister listers.APIServiceLister

	delegate http.Handler
}

func (r *apiGroupHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	apiServices, err := getApiServices(r.lister, req)
	if statusErr, ok := err.(*apierrors.StatusError); ok && err != nil {
		responsewriters.WriteRawJSON(int(statusErr.Status().Code), statusErr.Status(), w)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiServicesForGroup := []*apiregistrationapi.APIService{}
	for _, apiService := range apiServices {
		if apiService.Spec.Group == r.groupName {
			apiServicesForGroup = append(apiServicesForGroup, apiService)
		}
	}

	if len(apiServicesForGroup) == 0 {
		r.delegate.ServeHTTP(w, req)
		return
	}

	discoveryGroup := convertToDiscoveryAPIGroup(apiServicesForGroup)
	if discoveryGroup == nil {
		http.Error(w, "", http.StatusNotFound)
		return
	}
	responsewriters.WriteObjectNegotiated(r.codecs, negotiation.DefaultEndpointRestrictions, schema.GroupVersion{}, w, req, http.StatusOK, discoveryGroup)
}