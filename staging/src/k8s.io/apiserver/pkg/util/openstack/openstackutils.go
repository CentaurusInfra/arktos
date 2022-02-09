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

package openstack

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/core"
)

const (
	REBOOT   = "reboot"
	SNAPSHOT = "snapshot"
	RESTORE  = "restore"
)

var ERROR_UNKNOWN_ACTION = fmt.Errorf("unknown action")
var supportedActions = []string{REBOOT, SNAPSHOT, RESTORE}

// init function initialize the VM flavor and image cache
func init() {
	initFlavorsCache()
	initImagesCache()

	fl := ListFalvors()
	klog.V(6).Infof("built-in flavors: %v", fl)

	il := ListImages()
	klog.V(6).Infof("built-in images: %v", il)
}

// Convert Openstack request to kubernetes pod request body
// Revisit this for dynamically generate the json request
// TODO: post the initial support, consider push down this logic to the create handler to support other media types than Json
func ConvertServerFromOpenstackRequest(body []byte) ([]byte, error) {

	obj := OpenstackServerRequest{}

	err := json.Unmarshal(body, &obj)

	if err != nil {
		klog.Errorf("error unmarshal request. error %v", err)
		return nil, err
	}

	flavor, err := GetFalvor(obj.Server.Flavor)
	if err != nil {
		return nil, err
	}

	image, err := GetImage(obj.Server.ImageRef)
	if err != nil {
		return nil, err
	}

	var ret []byte

	if IsBatchCreationRequest(obj) {
		replicas := obj.Min_count
		ret, err = constructReplicasetRequestBody(replicas, obj.Server, image.ImageRef, flavor.Vcpus, flavor.MemoryMb)
	} else {
		ret, err = constructVmPodRequestBody(obj.Server, image.ImageRef, flavor.Vcpus, flavor.MemoryMb)
	}
	if err != nil {
		klog.Errorf("failed to construct request body. error: %v", err)
		return nil, err
	}

	return ret, nil
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
		return nil, ERROR_UNKNOWN_ACTION
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
	o := obj.(*metav1.Status)
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
		s := OpenstackRebuildResponse{ImageId: o.Details.Name, CreatedAt: time.Now().String()}
		return &s
	}

	return o
}

// Convert kubernetes pod response to Openstack response body
func ConvertToOpenstackResponse(obj runtime.Object) runtime.Object {
	typeStr := reflect.TypeOf(obj).String()

	switch typeStr {
	case "*apps.ReplicaSet":
		rs := obj.(*apps.ReplicaSet)
		osObj := &OpenstackBatchResponse{}
		osObj.Reservation_Id = rs.Name
		return osObj
	case "*core.PodList":
		pl := obj.(*core.PodList)
		osObj := &OpenstackServerListResponse{}
		osObj.Servers = make([]OpenstackResponse, len(pl.Items))
		for i, pod := range pl.Items {
			osObj.Servers[i].Id = pod.Name
			osObj.Servers[i].Links = []LinkType{{convertPodSelfLink(pod.GetSelfLink()), ""}}
			i++
		}

		return osObj
	case "*core.Pod":
		pod := obj.(*core.Pod)
		osObj := &OpenstackResponse{}
		osObj.Id = pod.Name
		osObj.Links = []LinkType{{convertPodSelfLink(pod.GetSelfLink()), ""}}
		osObj.Image = &ImageType{Name: pod.Spec.VirtualMachine.Image}
		osObj.Tenant = pod.Tenant
		if pod.Status.VirtualMachineStatus != nil {
			osObj.Status = string(pod.Status.VirtualMachineStatus.State)
			osObj.OS_EXT_STS_Power_state = string(pod.Status.VirtualMachineStatus.PowerState)
		} else {
			osObj.Status = "statusUnknown"
			osObj.OS_EXT_STS_Power_state = string(core.NoState)
		}

		if pod.Status.StartTime != nil {
			osObj.CreatedAt = pod.Status.StartTime.String()
		}

		osObj.AccessIpV4 = pod.Status.PodIP

		osObj.Flavor = &FlavorType{Vcpus: int(pod.Spec.VirtualMachine.Resources.Requests.Cpu().Value()),
			MemoryMb: int(pod.Spec.VirtualMachine.Resources.Requests.Memory().Value() / 1024 / 1024)} // display in Mi

		osObj.HostId = pod.Spec.Hostname
		if v, found := pod.Annotations["mizar.com/vpc"]; found {
			osObj.Vpc = v
		}
		if v, found := pod.Annotations["mizar.com/subnet"]; found {
			osObj.Subnet = v
		}

		return osObj
	default:
		klog.Errorf("Unsupported response type: %s", typeStr)
		return nil
	}

}

// convertPodSelfLink converts the self link from Arktos: /api/v1/namespaces/kube-system/pods/{vmname}
//                       to Openstack: /servers/{vmid}
// currently vmname is used as vmid for Arktos 130 release
func convertPodSelfLink(selfLink string) string {
	if selfLink == "" {
		return selfLink
	}

	elements := strings.Split(selfLink, "pods/")

	if len(elements) != 2 {
		return selfLink
	}

	return fmt.Sprintf("/servers/%s", elements[1])
}

func IsOpenstackRequest(req *http.Request) bool {
	return req.Header.Get("openstack") == "true"
}

// TODO: Get the tenant for the request from the request Token
func GetTenantFromRequest(r *http.Request) string {
	return metav1.TenantSystem
}

// TODO: Get the namespace, maps to the Openstack projct, from the Openstack token
func GetNamespaceFromRequest(r *http.Request) string {
	return metav1.NamespaceSystem
}

// The suffix of URL path is the action of the VM
// Arktos only supports reboot, stp[, start, snapshot, restore for the current release
func IsActionRequest(path string) bool {
	return strings.HasSuffix(path, "action")
}

// Internally the OpenStackServerRequest struct is shared with both batch request and non-batch request.
// For non-batch requests, which create VM in bare Arktos PODs, only Server object is set from users
// so the min-count is 0 as default int value
//
// Any non-zero possitive numbers which are set by the user request body and will be considerred as a batch request
func IsBatchCreationRequest(r OpenstackServerRequest) bool {
	return r.Min_count > 0
}
