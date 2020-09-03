package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EGatewayPhase string

// These are the valid statuses of EGateway.
const (
	// EGatewayPending means the EGateway has been created/added by the system, but not configured.
	EGatewayPending EGatewayPhase = "Pending"
	// EGatewayRunning means the EGateway has been configured and has Arktos components running.
	EGatewayRunning EGatewayPhase = "Running"
	// EGatewayTerminated means the EGateway has been removed from the cluster.
	EGatewayTerminated EGatewayPhase = "Terminated"
)

// EGatewayStatus is information about the current status of a EGa.
type EGatewayStatus struct {
	// +optional
	Phase EGatewayPhase
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ESite describe the edge site resource definition
type ESite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// list type
type ESiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ESite `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EGateway describe the edge gateway definition
type EGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Ip address of the gateway
	Ip string

	// Virtual presence ip address cidr of this gateway
	// +optional
	VirtualPresenceIPcidr string

	// ESiteName associated to the gateway
	ESiteName string

	// EGatewayStatus describes the current status of a EGateway
	// +optional
	Status EGatewayStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// list type
type EGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EGateway `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualPresence describe the Virtual Presence of the service
type VirtualPresence struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Ip address of the virtual presence
	VirtualIp string

	// Associated service name
	EServiceName string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// list type
type VirtualPresenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []VirtualPresence `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EService describe the Service exposed in the site
type EService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Protocol of the service
	Protocol string

	// Port of the service use
	Port int32

	// Ip of the service
	Ip string

	// Associated site name
	ESiteName string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// list type
type EServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EService `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EServer describe the server in the site
type EServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// EServerName of the server
	EServerName string

	// Ip of the server
	Ip string

	// Associated site name
	ESiteName string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// list type
type EServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EServer `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EPolicy describe the access policy of the service
type EPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Allowed server names of this policy
	AllowedServers []string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// list type
type EPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EPolicy `json:"items"`
}

type ServiceExposePhase string

// These are the valid statuses of ServiceExpose.
const (
	// ServiceExposePending means the ServiceExpose has been created/added by the system, but not configured.
	ServiceExposePending ServiceExposePhase = "Pending"
	// ServiceExposeActive means the ServiceExpose has been configured and has Arktos components running.
	ServiceExposeActive ServiceExposePhase = "Active"
)

type ServiceExposeStatus struct {
	phase ServiceExposePhase
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceExpose describe how to expose the service to other site
type ServiceExpose struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Associated service name will be exposed
	EServiceName string

	// Dns Name of the service will be exposed
	DnsName string

	// ESite name list
	ESites []string

	// EPolicys name list of the service will be allowed to access
	EPolicys []string

	// ServiceExposeStatus describes the current status of a ServiceExpose
	// +optional
	Status ServiceExposeStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// list type
type ServiceExposeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ServiceExpose `json:"items"`
}
