/*
Copyright 2021 Authors of Arktos.

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
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apiserver/pkg/server/mux"
	"k8s.io/apiserver/pkg/util/openstack"
	"k8s.io/klog"
)

const (
	REPLICASETS_URL_TEMPLATE = "/apis/apps/v1/tenants/%s/namespaces/%s/replicasets"
	POD_URL_TEMPLATE         = "/api/v1/tenants/%s/namespaces/%s/pods"
	OPENSTACK_SERVERS_PATH   = "/servers"
	OPENSTACK_FLAVORS_PATH   = "/flavors"
	OPENSTACK_IMAGES_PATH    = "/images"

	TARGET_FLAVORS = "flavors"
	TARGET_IMAGES  = "images"
)

type Openstack struct{}

// the url path is /servers/{vmId} OR /servers/{vmId}/{action}
func getElementFromPath(path string) string {
	elements := strings.Split(path, "/")
	if len(elements) < 3 {
		return ""
	}
	return strings.Split(path, "/")[2]
}

func isGetDetail(path string) bool {
	pattern := "/servers/[a-z]+"
	match, _ := regexp.MatchString(pattern, path)

	return match
}

func isBatchCreationRequest(req *http.Request) (bool, error) {
	obj := openstack.OpenstackServerRequest{}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		klog.Errorf("error read request body. error %v", err)
		return false, err
	}

	err = json.Unmarshal(body, &obj)

	if err != nil {
		klog.V(6).Infof("Failed unmarshal request body: %s", string(body))
		klog.Errorf("error unmarshal request: %v", err)
		return false, err
	}

	return openstack.IsBatchCreationRequest(obj), nil
}

// For now, supported filter is the reservation_id when VMs were created in batch
func getReservationIdFromListRequest(req *http.Request) (string, error) {
	obj := openstack.OpenstackServerListRequest{}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		klog.Errorf("error read request body. error %v", err)
		return "", err
	}

	if body == nil || len(body) == 0 {
		return "", nil
	}

	err = json.Unmarshal(body, &obj)

	if err != nil {
		klog.Errorf("error unmarshal request. error %v", err)
		return "", err
	}

	return obj.Reservation_Id, nil
}

func (o Openstack) imageHandler(resp http.ResponseWriter, req *http.Request) {
	o.genericOpenStackRequestHandler(TARGET_IMAGES, resp, req)
}

func (o Openstack) flavorHandler(resp http.ResponseWriter, req *http.Request) {
	o.genericOpenStackRequestHandler(TARGET_FLAVORS, resp, req)
}

func (o Openstack) genericOpenStackRequestHandler(target string, resp http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	klog.V(4).Infof("URL path: %s", path)

	if req.Method != "GET" {
		resp.WriteHeader(http.StatusMethodNotAllowed) // only GET method for this release
		return
	}

	var body []byte
	var err error
	var f interface{}

	getter := func(name string) (interface{}, error) {
		switch target {
		case TARGET_FLAVORS:
			return openstack.GetFalvor(name)
		case TARGET_IMAGES:
			return openstack.GetImage(name)
		default:
			return nil, fmt.Errorf("invalid target %s", target)
		}
	}

	lister := func() interface{} {
		switch target {
		case TARGET_FLAVORS:
			return openstack.ListFalvors()
		case TARGET_IMAGES:
			return openstack.ListImages()
		default:
			return nil
		}
	}

	if strings.HasSuffix(path, target) {
		body, err = json.Marshal(lister())
	} else {
		name := getElementFromPath(path)
		f, err = getter(name)
		if err != nil {
			klog.Infof("Element %s-%s not found", target, name)
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		body, err = json.Marshal(f)
	}

	if err != nil {
		klog.Errorf("failed encoding %s, error %v", target, err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	resp.WriteHeader(http.StatusOK)
	resp.Write(body)
}

func (o Openstack) actionHandler(resp http.ResponseWriter, req *http.Request, redirectUrlPath string) {
	klog.V(3).Infof("Redirect request to %s", redirectUrlPath)
	http.Redirect(resp, req, redirectUrlPath, http.StatusTemporaryRedirect)
}

// TODO: redirect could introduce perf impact to the server, so setup the routes for Openstack requests
//       in the API installation flow at the api server init state will be a better solution
// GET a server : GET - /servers/serverName
// LIST severs:   GET - /servers
// LIST servers:  GET - /servers, with label selector in query
// CREATE a server: POST - /servers
// CREATE batch servers: POST  - /servers
// DELETE a server : DELETE - /servers/serverName
// note that NO batch delete, in openstack
// TODO: for post 130, we should support cases to delete all in batch if this is a valid use case

// actions:
// CREATE an action: POST - /servers/serverName/action
//
func (o Openstack) serverHandler(resp http.ResponseWriter, req *http.Request) {
	klog.V(4).Infof("handle /servers. URL path: %s", req.URL.Path)

	tenant := openstack.GetTenantFromRequest(req)
	namespace := openstack.GetNamespaceFromRequest(req)
	vmId := getElementFromPath(req.URL.Path)
	redirectUrl := ""

	// For now, just check if it is empty string, which indicate badRequest
	validateVmName := func(name string) {
		if name == "" {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	if openstack.IsActionRequest(req.URL.Path) {
		validateVmName(vmId)
		redirectUrl = fmt.Sprintf(POD_URL_TEMPLATE, tenant, namespace)
		redirectUrl += "/" + vmId + "/action"
		o.actionHandler(resp, req, redirectUrl)
		return
	}

	switch req.Method {
	case http.MethodGet:
		redirectUrl = fmt.Sprintf(POD_URL_TEMPLATE, tenant, namespace)
		if isGetDetail(req.URL.Path) {
			validateVmName(vmId)
			redirectUrl += "/" + vmId
		}

		rev_id, err := getReservationIdFromListRequest(req)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		if rev_id != "" {
			redirectUrl += fmt.Sprintf("?labelSelector=%s=true,ln=%s", openstack.OPENSTACK_API, rev_id)
		} else {
			redirectUrl += fmt.Sprintf("?labelSelector=%s=true", openstack.OPENSTACK_API)
		}

	case http.MethodDelete:
		validateVmName(vmId)
		redirectUrl = fmt.Sprintf(POD_URL_TEMPLATE, tenant, namespace)
		redirectUrl += "/" + vmId
	case http.MethodPost:
		isBatchRequest, err := isBatchCreationRequest(req)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		if isBatchRequest {
			redirectUrl = fmt.Sprintf(REPLICASETS_URL_TEMPLATE, tenant, namespace)
		} else {
			redirectUrl = fmt.Sprintf(POD_URL_TEMPLATE, tenant, namespace)
		}
	default:
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	klog.V(3).Infof("Redirect request to %s", redirectUrl)
	http.Redirect(resp, req, redirectUrl, http.StatusTemporaryRedirect)
}

// Install adds the Openstack webservice to the given mux
func (o Openstack) Install(c *mux.PathRecorderMux) {
	c.HandleFunc(OPENSTACK_SERVERS_PATH, o.serverHandler)
	c.HandlePrefix(OPENSTACK_SERVERS_PATH+"/", http.HandlerFunc(o.serverHandler))

	c.HandleFunc(OPENSTACK_FLAVORS_PATH, o.flavorHandler)
	c.HandlePrefix(OPENSTACK_FLAVORS_PATH+"/", http.HandlerFunc(o.flavorHandler))

	c.HandleFunc(OPENSTACK_IMAGES_PATH, o.imageHandler)
	c.HandlePrefix(OPENSTACK_IMAGES_PATH+"/", http.HandlerFunc(o.imageHandler))
}
