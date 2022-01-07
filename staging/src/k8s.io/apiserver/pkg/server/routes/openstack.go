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

	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apiserver/pkg/server/mux"
	"k8s.io/apiserver/pkg/util/openstack"
	"k8s.io/klog"
)

const (
	POD_URL_TEMPLATE       = "/api/v1/tenants/%s/namespaces/%s/pods"
	OPENSTACK_SERVERS_PATH = "/servers"
	OPENSTACK_FLAVORS_PATH = "/flavors"
	OPENSTACK_IMAGES_PATH  = "/images"

	TARGET_FLAVORS = "flavors"
	TARGET_IMAGES  = "images"
)

type Openstack struct{}

// the url path is /servers/{vmId}
func getElementFromPath(path string) string {
	return strings.Split(path, "/")[2]
}

func (o Openstack) imageHandler(resp http.ResponseWriter, req *http.Request) {
	o.genericFunc(TARGET_IMAGES, resp, req)
}

func (o Openstack) flavorHandler(resp http.ResponseWriter, req *http.Request) {
	o.genericFunc(TARGET_FLAVORS, resp, req)
}

func (o Openstack) genericFunc(target string, resp http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	klog.V(4).Infof("URL path: %s", path)

	if req.Method != "GET" {
		resp.WriteHeader(405) // method not allowed for the current release
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
			klog.Errorf("Invalid %s %s", target, name)
			resp.WriteHeader(500)
			return
		}
		body, err = json.Marshal(f)
	}

	if err != nil {
		klog.Errorf("failed encoding %s, error %v", target, err)
		resp.WriteHeader(500)
		return
	}
	resp.WriteHeader(200)
	resp.Write(body)
}

// TODO: redirect could introduce perf impact to the server, so setup the routes for Openstack requests
//       in the API installation flow at the api server init state will be a better solution
func (o Openstack) serverHandler(resp http.ResponseWriter, req *http.Request) {
	klog.V(4).Infof("handle /servers. URL path: %s", req.URL.Path)

	tenant := openstack.GetTenantFromRequest(req)
	namespace := openstack.GetNamespaceFromRequest(req)

	redirectUrl := fmt.Sprintf(POD_URL_TEMPLATE, tenant, namespace)

	if openstack.IsActionRequest(req.URL.Path) {
		redirectUrl += "/" + getElementFromPath(req.URL.Path) + "/action"
	} else if req.Method == "GET" || req.Method == "DELETE" {
		//Get the VM ID for redirect
		redirectUrl += "/" + getElementFromPath(req.URL.Path)
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
