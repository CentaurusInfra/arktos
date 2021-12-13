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
}

type Network struct {
	Uuid     string
	Port     string
	Fixed_ip string
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

// VM creation request in Openstack
type OpenstackRequest struct {
	Name            string
	Flavor          string
	Networks        []Network
	Key_name        string
	Metadata        MetadataType
	Security_groups []SecurityGroup
	User_data       string
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
func ConvertToOpenstackRequest(body []byte) []byte {
	cpu, mem := GetRequestedResource("m1.tiny")
	imageUrl := GetVmImageUrl("ImageRefID")

	podJson := fmt.Sprintf(POD_JSON_STRING_TEMPLATE, imageUrl, cpu, mem, cpu, mem)
	return []byte(podJson)
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

// TODO: add config map to cache a few OS images to simulate Openstack image resitry
func GetVmImageUrl(imageRef string) string {
	return "download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img"
}

// get the requested resource, CPU, RAM, from the Openstack flavor of VM
// TODO: add config map for most used Openstack flavors
//       consider to use resource structure instead of returning individual resources
func GetRequestedResource(flavor string) (int, int) {
	return 1, 2
}

// TODO: Get the tenant for the request from the request Token
func GetTenantFromRequest(r *http.Request) string {
	return "system"
}

// TODO: Get the namespace, maps to the Openstack projct, from the Openstack token
func GetNamespaceFromRequest(r *http.Request) string {
	return "kube-system"
}
