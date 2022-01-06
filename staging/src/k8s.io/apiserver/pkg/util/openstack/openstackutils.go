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

package openstack

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/core"
)

var POD_JSON_STRING_TEMPLATE string

func init() {
	t, err := ioutil.ReadFile("/openstackRequestTemplate.json")
	if err != nil {
		klog.Errorf("error reading template file. error : %v", err)
		return
	}

	POD_JSON_STRING_TEMPLATE = string(t)

	initFlavorsCache()
	initImagesCache()

	fl := ListFalvors()
	klog.Infof("debug: flavors: %v", fl)

	il := ListImages()
	klog.Infof("debug: images: %v", il)
}

type Network struct {
	Uuid     string `json:"uuid"`
	Port     string `json:"port"`
	Fixed_ip string `json:"fixed_ip"`
}

type MetadataType struct {
	key   string
	value string
}

type SecurityGroup struct {
	name string
}

type LinkType struct {
	Link string
	Rel  string
}

type ServerType struct {
	Name            string    `json:"name"`
	ImageRef        string `json:"imageRef"`
	Flavor          string `json:"flavorRef"`
	Min_count       int `json:"min_count"`
	Max_count       int `json:"max_count"`
	Networks        []Network `json:"networks"`
	Security_groups []SecurityGroup `json:"security_groups"`
	Key_name        string `json:"key_name"`
	Metadata        MetadataType `json:"metadata"`
	User_data       string `json:"user_data"`
}

// VM creation request in Openstack
type OpenstackRequest struct {
	Server ServerType `json:"server"`
}

// VM creation response in Openstack
type OpenstackResponse struct {
	Id              string
	Links           []LinkType
	Security_groups []SecurityGroup
}

func (o *OpenstackResponse) GetObjectKind() schema.ObjectKind {
	return schema.OpenstackObjectKind
}

func (o *OpenstackResponse) DeepCopyObject() runtime.Object {
	return o
}

// Convert Openstack request to kubernetes pod request body
// Revisit this for dynamically generate the json request
// TODO: fix hard coded flavor and imageRefs with config maps
// TODO: post the initial support, consider push down this logic to the create handler to support other media types than Json
func ConvertToOpenstackRequest(body []byte) ([]byte, error) {

	obj := OpenstackRequest{}

	err := json.Unmarshal(body, &obj)

	if err != nil {
		klog.Errorf("error unmarshal request. error %v", err)
		return nil, err
	}

	klog.Infof("debug: request struct: %v", obj)

	flavor, err := GetFalvor(obj.Server.Flavor)
	if err != nil {
		return nil, err
	}

	image, err := GetImage(obj.Server.ImageRef)
	if err != nil {
		return nil, err
	}

	podJson := fmt.Sprintf(POD_JSON_STRING_TEMPLATE, obj.Server.Name, image.ImageRef, obj.Server.Name, flavor.Vcpus, flavor.MemoryMb, flavor.Vcpus, flavor.MemoryMb)
	klog.Infof("debug: pod json: %s", podJson)
	return []byte(podJson), nil
}

// Convert kubernetes pod response to Openstack response body
func ConvertToOpenstackResponse(obj runtime.Object) runtime.Object {
	pod := obj.(*core.Pod)
	osObj := &OpenstackResponse{}
	osObj.Id = pod.Name

	osObj.Links = []LinkType{{pod.GetSelfLink(), ""}}
	//osObj.SecurityGroups = nil

	return osObj
}

func IsOpenstackRequest(req *http.Request) bool {
	return req.Header.Get("openstack") == "true"
}

// TODO: Get the tenant for the request from the request Token
func GetTenantFromRequest(r *http.Request) string {
	return "system"
}

// TODO: Get the namespace, maps to the Openstack projct, from the Openstack token
func GetNamespaceFromRequest(r *http.Request) string {
	return "kube-system"
}
