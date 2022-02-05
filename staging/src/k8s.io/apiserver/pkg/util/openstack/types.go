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
	"strconv"

	v12 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
)

const (
	LABEL_SELECTOR_NAME = "ln"
	OPENSTACK_API       = "openstsckApi"
)

var cpuModelAnnotation = map[string]string{"VirtletCPUModel": "host-model"}

type Openstack_error struct {
	Message   string `json:"message"`
	ErrorCode int    `json:"errorcode"`
	Reason    string `json:"reason"`
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
	Name string `json:"name"`
	// Boot from image is what currently supported in Arktos-vm-runtime, so this is required
	ImageRef string `json:"imageRef"`
	Flavor   string `json:"flavorRef"`
	// +optional
	Networks []Network `json:"networks"`
	// +optional
	Security_groups []SecurityGroup `json:"security_groups"`
	// +optional
	Key_name string `json:"key_name"`
	// +optional
	Metadata MetadataType `json:"metadata"`
	// +optional
	User_data string `json:"user_data"`
	// the compute service host node the server to be created on
	// +optional
	Host string `json:"host"`
}

// VM creation request in Openstack
// non-zero possitive Min or max count indicates batch creation, even if it is one
// Return_Reservation_Id indicates response behavior, true to return the reservationID ( replicset name in Arktos )
// So that the client can list the servers associated with this replicaset
type OpenstackServerRequest struct {
	Server                ServerType `json:"server"`
	Min_count             int        `json:"min_count"`
	Max_count             int        `json:"max_count"`
	Return_Reservation_Id bool       `json:"return_reservation_id"`
}

// VM creation response in Openstack
type OpenstackResponse struct {
	Id                     string          `json:"id,omitempty"`
	Links                  []LinkType      `json:"links,omitempty"`
	Security_groups        []SecurityGroup `json:"reason,omitempty"`
	Flavor                 *FlavorType     `json:"flavor,omitempty"`
	Image                  *ImageType      `json:"image,omitempty"`
	Status                 string          `json:"status,omitempty"`
	Tenant                 string          `json:"tenant,omitempty"`
	CreatedAt              string          `json:"createdAt,omitempty"`
	HostId                 string          `json:"hostid,omitempty"`
	AccessIpV4             string          `json:"accessIpv4,omitempty"`
	OS_EXT_STS_Power_state string          `json:"os_ext_sts_power_state,omitempty"`
	Vpc                    string          `json:"vpc,omitempty"`
	Subnet                 string          `json:"subnet,omitempty"`
}

func (o *OpenstackResponse) GetObjectKind() schema.ObjectKind {
	return schema.OpenstackObjectKind
}

func (o *OpenstackResponse) DeepCopyObject() runtime.Object {
	return o
}

type OpenstackServerListRequest struct {
	Reservation_Id string `json:"reservation_id"`
}

// VM list response
type OpenstackServerListResponse struct {
	Servers []OpenstackResponse
}

func (o *OpenstackServerListResponse) GetObjectKind() schema.ObjectKind {
	return schema.OpenstackObjectKind
}

func (o *OpenstackServerListResponse) DeepCopyObject() runtime.Object {
	return o
}

// VM Batch creation response in Openstack
type OpenstackBatchResponse struct {
	Reservation_Id string `json:"reservation_id"`
}

func (o *OpenstackBatchResponse) GetObjectKind() schema.ObjectKind {
	return schema.OpenstackObjectKind
}

func (o *OpenstackBatchResponse) DeepCopyObject() runtime.Object {
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

// rebuild response in Openstack
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

type batchRequestBody struct {
	ApiVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	MetaData   metav1.ObjectMeta  `json:"metadata"`
	Spec       v12.ReplicaSetSpec `json:"spec"`
}

func constructReplicasetRequestBody(replicas int, server ServerType, imageRef string, vcpu, memInMi int) ([]byte, error) {
	t := batchRequestBody{}
	t.ApiVersion = "apps/v1"
	t.Kind = "ReplicaSet"
	t.MetaData = metav1.ObjectMeta{
		Name:      server.Name,
		Namespace: metav1.NamespaceSystem,
		Tenant:    metav1.TenantSystem,
	}

	i := int32(replicas)
	t.Spec = v12.ReplicaSetSpec{
		Replicas: &i,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{LABEL_SELECTOR_NAME: server.Name},
		},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: cpuModelAnnotation,
				Labels:      map[string]string{LABEL_SELECTOR_NAME: server.Name, OPENSTACK_API: "true"},
			},
			Spec: v1.PodSpec{
				VirtualMachine: constructVMSpec(server, imageRef, vcpu, memInMi),
			},
		},
	}

	b, err := json.Marshal(t)

	if err != nil {
		return nil, err
	}

	return b, nil

}

type vmRequestBody struct {
	ApiVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	MetaData   metav1.ObjectMeta `json:"metadata"`
	Spec       v1.PodSpec        `json:"spec"`
}

func constructVmPodRequestBody(server ServerType, imageRef string, vcpu, memInMi int) ([]byte, error) {
	t := vmRequestBody{}
	t.ApiVersion = "v1"
	t.Kind = "Pod"
	t.MetaData = metav1.ObjectMeta{
		Name:        server.Name,
		Namespace:   metav1.NamespaceSystem,
		Tenant:      metav1.TenantSystem,
		Annotations: cpuModelAnnotation,
		Labels:      map[string]string{OPENSTACK_API: "true"},
	}

	t.Spec = v1.PodSpec{
		VirtualMachine: constructVMSpec(server, imageRef, vcpu, memInMi),
	}

	b, err := json.Marshal(t)

	if err != nil {
		return nil, err
	}

	return b, nil

}

func constructVMSpec(server ServerType, imageRef string, vcpu, memInMi int) *v1.VirtualMachine {
	return &v1.VirtualMachine{
		Image:           imageRef,
		ImagePullPolicy: v1.PullIfNotPresent,
		Name:            server.Name,
		PublicKey:       server.Key_name,
		UserData:        []byte(server.User_data),
		Resources: v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(strconv.Itoa(vcpu)),
				v1.ResourceMemory: resource.MustParse(strconv.Itoa(memInMi) + "Mi"),
			},
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(strconv.Itoa(vcpu)),
				v1.ResourceMemory: resource.MustParse(strconv.Itoa(memInMi) + "Mi"),
			},
		},
	}
}
