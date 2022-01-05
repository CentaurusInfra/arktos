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
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/core"
)

const (
	REBOOT   = "reboot"
	SNAPSHOT = "snapshot"
	RESTORE  = "restore"
)

var POD_JSON_STRING_TEMPLATE string
var supportedActions = []string{REBOOT, SNAPSHOT, RESTORE}

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

type ServerType struct {
	Name            string
	ImageRef        string
	Flavor          string
	Min_count       string
	Max_count       string
	Networks        []Network
	Security_groups []SecurityGroup
	Key_name        string
	Metadata        MetadataType
	User_data       string
}

// VM creation request in Openstack
type OpenstackRequest struct {
	Server ServerType
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

type ArktosRebootParams struct {
	DelayInSeconds int `json:"delayInSeconds"`
}

// Reboot action support
//
type ArktosReboot struct {
	ApiVersion   string             `json:"apiVersion"`
	Kind         string             `json:"kind"`
	Operation    string             `json:"operation"`
	RebootParams ArktosRebootParams `json:"rebootParams"`
}

// Snapshot action support
//
type ArktosSnapshotParams struct {
	SnapshotName string `json:"snapshotName"`
}
type ArktosSnapshot struct {
	ApiVersion     string               `json:"apiVersion"`
	Kind           string               `json:"kind"`
	Operation      string               `json:"operation"`
	SnapshotParams ArktosSnapshotParams `json:"snapshotParams"`
}

// Openstack createImage action match to the Arktos snapshot action
type OpenstackCreateImage struct {
	Name     string
	Metadata MetadataType
}

type OpenstackCreateImageRequest struct {
	Snapshot OpenstackCreateImage
}

// snapshot creation response in Openstack
type OpenstackCreateImageResponse struct {
	ImageId string
}

func (o *OpenstackCreateImageResponse) GetObjectKind() schema.ObjectKind {
	return schema.OpenstackObjectKind
}

func (o *OpenstackCreateImageResponse) DeepCopyObject() runtime.Object {
	return o
}

// Restore action support
//
type ArktosRestoreParams struct {
	SnapshotID string `json:"snapshotID"`
}
type ArktosRestore struct {
	ApiVersion    string              `json:"apiVersion"`
	Kind          string              `json:"kind"`
	Operation     string              `json:"operation"`
	RestoreParams ArktosRestoreParams `json:"restoreParams"`
}

// Openstack rebuild action match to the Arktos restore action
// Openstack rebuild struct has much optional field which are not applicable to Arktos restore action
// slim down to key info
type OpenstackRebuild struct {
	ImageRef string
	Metadata MetadataType
}

type OpenstackRebuildRequest struct {
	Restore OpenstackRebuild
}

// snapshot creation response in Openstack
type OpenstackRebuildResponse struct {
	ServerId  string
	ImageId   string
	CreatedAt string
}

func (o *OpenstackRebuildResponse) GetObjectKind() schema.ObjectKind {
	return schema.OpenstackObjectKind
}

func (o *OpenstackRebuildResponse) DeepCopyObject() runtime.Object {
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

	cpu, mem := GetRequestedResource(obj.Server.Flavor)
	imageUrl := GetVmImageUrl(obj.Server.ImageRef)

	podJson := fmt.Sprintf(POD_JSON_STRING_TEMPLATE, obj.Server.Name, imageUrl, obj.Server.Name, cpu, mem, cpu, mem)
	klog.V(6).Infof("pod json: %s", podJson)
	return []byte(podJson), nil
}

// Convert the action request to Arktos action request body
func ConvertActionFromOpenstackRequest(body []byte) ([]byte, error) {
	op := getActionOperation(body)
	klog.V(6).Infof("Convert %s Action", op)

	switch op {
	case REBOOT:
		obj := ArktosReboot{"v1", "CustomAction", op, ArktosRebootParams{10}}
		return json.Marshal(obj)
	case SNAPSHOT:
		o := OpenstackCreateImageRequest{}
		err := json.Unmarshal(body, &o)
		if err != nil {
			return nil, fmt.Errorf("invalid snapshot request. error %v", err)
		}
		obj := ArktosSnapshot{"v1", "CustomAction", op, ArktosSnapshotParams{o.Snapshot.Name}}
		return json.Marshal(obj)
	case RESTORE:
		o := OpenstackRebuildRequest{}
		err := json.Unmarshal(body, &o)
		if err != nil {
			return nil, fmt.Errorf("invalid restore request. error %v", err)
		}
		obj := ArktosRestore{"v1", "CustomAction", op, ArktosRestoreParams{o.Restore.ImageRef}}
		return json.Marshal(obj)
	default:
		return nil, fmt.Errorf("unsupported action: %s", op)
	}
}

func getActionOperation(body []byte) string {
	for _, action := range supportedActions {
		pattern := fmt.Sprintf(`"%s" *:`, action)
		match, _ := regexp.Match(pattern, body)
		if match {
			return action
		}
	}

	return "unknownAction"
}

func ConvertActionToOpenstackResponse(obj runtime.Object) runtime.Object {
	o := obj.(*v1.Status)
	klog.V(6).Infof("Convert Arktos object: %v", o)

	// for action types reboot, start, stop, simply return empty response since Openstack
	if strings.Contains(o.Message, REBOOT) {
		return nil
	}

	if strings.Contains(o.Message, SNAPSHOT) {
		s := OpenstackCreateImageResponse{ImageId: o.Details.Name}
		return &s
	}

	if strings.Contains(o.Message, RESTORE) {
		s := OpenstackRebuildResponse{ImageId: o.Details.Name}
		return &s
	}

	return o
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

// the suffix of URL path is the action of the VM
// Arktos only supports reboot, stp[, start, snapshot, restore for the current release
func IsActionRequest(path string) bool {
	return strings.HasSuffix(path, "action")
}
