/*
Copyright 2021 The Arktos Authors.

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

// TODO: reconsider this to add this to the api extension
package routes

import (
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apiserver/pkg/server/mux"
	"k8s.io/apiserver/pkg/util/openstack"
	"k8s.io/klog"
)

const (
	POD_URL_TEMPLATE       = "/api/v1/tenants/%s/namespaces/%s/pods"
	OPENSTACK_SERVERS_PATH = "/servers"
)

type Openstack struct{}

// the url path is /servers/{vmId}
func getVmFromPath(path string) string {
	return strings.Split(path, "/")[2]
}

// TODO: redirect could introduce perf impact to the server, so setup the routes for Openstack requests
//       in the API installation flow at the api server init state will be a better solution
func (o Openstack) serverHandler(resp http.ResponseWriter, req *http.Request) {
	klog.V(4).Infof("handle /servers")

	tenant := openstack.GetTenantFromRequest(req)
	namespace := openstack.GetNamespaceFromRequest(req)

	redirectUrl := fmt.Sprintf(POD_URL_TEMPLATE, tenant, namespace)

	if openstack.IsActionRequest(req.URL.Path) {
		redirectUrl += "/" + getVmFromPath(req.URL.Path) + "/action"
	} else if req.Method == "GET" || req.Method == "DELETE" {
		//Get the VM ID for redirect
		redirectUrl += "/" + getVmFromPath(req.URL.Path)
	}

	klog.V(3).Infof("Redirect request to %s", redirectUrl)
	http.Redirect(resp, req, redirectUrl, http.StatusTemporaryRedirect)
}

// Install adds the Openstack webservice to the given mux
func (o Openstack) Install(c *mux.PathRecorderMux) {
	c.HandleFunc(OPENSTACK_SERVERS_PATH, o.serverHandler)
	c.HandlePrefix(OPENSTACK_SERVERS_PATH+"/", http.HandlerFunc(o.serverHandler))
}
