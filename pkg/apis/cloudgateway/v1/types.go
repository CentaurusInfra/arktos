package v1

import (
	"time"

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

// CloudGatewayConfig indicates the config of cloudGateway which get from cloudGateway config file
type CloudGatewayConfig struct {
	metav1.TypeMeta
	// +Required
	KubeAPIConfig *KubeAPIConfig `json:"kubeAPIConfig,omitempty"`
	// Modules indicates cloudGateway modules config
	// +Required
	Modules *Modules `json:"modules,omitempty"`
}

// KubeAPIConfig indicates the configuration for interacting with arktos server
type KubeAPIConfig struct {
	Master     string `json:"master"`
	KubeConfig string `json:"kubeConfig"`
}

// Modules indicates the modules of CloudGateway will be use
type Modules struct {
	// CloudHub indicates CloudHub module config
	CloudHub *CloudHub `json:"cloudHub,omitempty"`
	// Controller indicates Controller module config
	Controller *Controller `json:"controller,omitempty"`
}

// Controller indicates the config of Controller module.
type Controller struct {
	// Enable indicates whether Controller is enabled, if set ot false
	// skip checking other Controller configs.
	Enable bool `json:"enable,omitempty"`
}

// CloudHub indicates the config of CloudHub module.
// CloudHub is a web socket or quic server responsible for watching changes at the cloud side,
// caching and sending messages to EdgeHub.
type CloudHub struct {
	// Enable indicates whether CloudHub is enabled, if set to false (for debugging etc.),
	// skip checking other CloudHub configs.
	// default true
	Enable bool `json:"enable,omitempty"`
	// KeepaliveInterval indicates keep-alive interval (second)
	// default 30
	KeepaliveInterval int32 `json:"keepaliveInterval,omitempty"`
	// NodeLimit indicates node limit
	// default 1000
	NodeLimit int32 `json:"nodeLimit,omitempty"`
	// TLSCAFile indicates ca file path
	// default "/etc/kubeedge/ca/rootCA.crt"
	TLSCAFile string `json:"tlsCAFile,omitempty"`
	// TLSCAKeyFile indicates caKey file path
	// default "/etc/kubeedge/ca/rootCA.key"
	TLSCAKeyFile string `json:"tlsCAKeyFile,omitempty"`
	// TLSPrivateKeyFile indicates key file path
	// default "/etc/kubeedge/certs/server.crt"
	TLSCertFile string `json:"tlsCertFile,omitempty"`
	// TLSPrivateKeyFile indicates key file path
	// default "/etc/kubeedge/certs/server.key"
	TLSPrivateKeyFile string `json:"tlsPrivateKeyFile,omitempty"`
	// WriteTimeout indicates write time (second)
	// default 30
	WriteTimeout int32 `json:"writeTimeout,omitempty"`
	// Quic indicates quic server info
	Quic *CloudHubQUIC `json:"quic,omitempty"`
	// WebSocket indicates websocket server info
	// +Required
	WebSocket *CloudHubWebSocket `json:"websocket,omitempty"`
	// HTTPS indicates https server info
	// +Required
	HTTPS *CloudHubHTTPS `json:"https,omitempty"`
	// AdvertiseAddress sets the IP address for the CloudGateway to advertise.
	AdvertiseAddress []string `json:"advertiseAddress,omitempty"`
	// EdgeCertSigningDuration indicates the validity period of edge certificate
	// default 365d
	EdgeCertSigningDuration time.Duration `json:"edgeCertSigningDuration,omitempty"`
}

// CloudHubQUIC indicates the quic server config
type CloudHubQUIC struct {
	// Enable indicates whether enable quic protocol
	// default false
	Enable bool `json:"enable,omitempty"`
	// Address set server ip address
	// default 0.0.0.0
	Address string `json:"address,omitempty"`
	// Port set open port for quic server
	// default 10001
	Port uint32 `json:"port,omitempty"`
	// MaxIncomingStreams set the max incoming stream for quic server
	// default 10000
	MaxIncomingStreams int32 `json:"maxIncomingStreams,omitempty"`
}

// CloudHubWebSocket indicates the websocket config of CloudHub
type CloudHubWebSocket struct {
	// Enable indicates whether enable websocket protocol
	// default true
	Enable bool `json:"enable,omitempty"`
	// Address indicates server ip address
	// default 0.0.0.0
	Address string `json:"address,omitempty"`
	// Port indicates the open port for websocket server
	// default 10000
	Port uint32 `json:"port,omitempty"`
}

// CloudHubHttps indicates the http config of CloudHub
type CloudHubHTTPS struct {
	// Enable indicates whether enable Https protocol
	// default true
	Enable bool `json:"enable,omitempty"`
	// Address indicates server ip address
	// default 0.0.0.0
	Address string `json:"address,omitempty"`
	// Port indicates the open port for HTTPS server
	// default 10002
	Port uint32 `json:"port,omitempty"`
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
