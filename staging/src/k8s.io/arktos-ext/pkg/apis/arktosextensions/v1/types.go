/*
Copyright 2020 Authors of Arktos.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=create,get,list,watch,updateStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Network specifies a network boundary
type Network struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines desired state of network
	// +optional
	Spec NetworkSpec `json:"spec"`

	// Status is the actual state of network
	// +optional
	Status NetworkStatus `json:"status"`
}

// NetworkSpec is a description of Network
type NetworkSpec struct {
	// Type is the network type
	Type string `json:"type"`
	// VPCID is vpc identifier specific to network provider
	// +optional
	VPCID string `json:"vpcID,omitempty"`
	// Service specifies service related properties
	Service NetworkService `json: "service,omitempty"`
}

// NetworkService defines service related information of the network
type NetworkService struct {
	// IPAM is the IPM type of services
	IPAM NetworkServiceIPAM `json:"ipam,omitempty"`
	// CIDRS specifies ranges of service VIP
	CIDRS []string `json:"cidrs"`
}

// NetworkServiceIPAM describes the IPAM type of services
type NetworkServiceIPAM string

// NetworkPhase describes the lifecycle phase of Network
type NetworkPhase string

const (
	// Valid values of network lifecycle phase
	// NetworkPending means the network accepted by the system, but
	// has not been ready for use.
	NetworkPending NetworkPhase = "Pending"
	// NetworkFailed means for some reason the network is in error state
	// and cannot be used properly any more.
	NetworkFailed NetworkPhase = "Failed"
	// NetworkReady means the network is in good shape, ready to manage networks.
	NetworkReady NetworkPhase = "Ready"
	// NetworkTerminating means the network is in the middle of termination.
	NetworkTerminating NetworkPhase = "Terminating"
	// NetworkUnknown means the state of network cannot be decided due to
	// communication problems.
	NetworkUnknown NetworkPhase = "Unknown"

	// valid values of network service IPAM
	// IPAMArktos is the IPAM that Arktos assigns VIP for service
	IPAMArktos NetworkServiceIPAM = "Arktos"
	// IPAMKubernetes is the IPAM that Kubernetes assigns VIP for service
	IPAMKubernetes NetworkServiceIPAM = "Kubernetes"
	// IPAMExternal is the IPAM that external network controller assigns VIP of service
	IPAMExternal NetworkServiceIPAM = "External"

	// various network-related label & annotations
	NetworkLabel = "arktos.futurewei.com/network"
	// network related annotation keys
	NetworkReadiness = "arktos.futurewei.com/network-readiness"

	// NetworkDefault is the default network name
	NetworkDefault = "default"
)

// NetworkStatus is the status for Network resource
type NetworkStatus struct {
	// Phase is the current lifecycle phase of Network.
	// +optional
	Phase NetworkPhase `json:"phase,omitempty"`
	// Message is the human readable information of the current phase.
	// +optional
	Message string `json:"message,omitempty"`
	// DNSServiceIP is IP address of the DNS service of the network
	// +optional
	DNSServiceIP string `json:"dnsServiceIP,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkList is a list of Network objects.
type NetworkList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta

	// Items is the list of Network objects in the list
	Items []Network
}
