package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// EdgeGatewayConfig indicates the EdgeGateway config which read from EdgeGateway config file
type EdgeGatewayConfig struct {
	metav1.TypeMeta
	// Modules indicates EdgeGateway modules config
	// +Required
	Modules *Modules `json:"modules,omitempty"`
}

// Modules indicates the modules which EdgeGateway will be used
type Modules struct {
	// EdgeHub indicates edgeHub module config
	// +Required
	EdgeHub *EdgeHub `json:"edgeHub,omitempty"`
	// EdgeService indicates EdgeService module config
	EdgeService *EdgeService `json:"edgeService,omitempty"`
	// EdgeMesh indicates EdgeMesh module config
	EdgeMesh *EdgeMesh `json:"edgeMesh,omitempty"`
}

// EdgeMesh indicates the config of EdgeMesh module.
type EdgeMesh struct {
	// Enable indicates whether EdgeMesh is enabled, if set ot false
	// skip checking other EdgeMesh configs.
	Enable bool `json:"enable,omitempty"`
	// Server indicates websocket server address (ip:port)
	// +Required
	Server string `json:"server,omitempty"`
	// TapInterface set tap interface for EdgeMesh
	// default tap0
	TapInterface string `json:"tapInterface,omitempty"`
}

// EdgeService indicates the config of EdgeService module.
type EdgeService struct {
	// Enable indicates whether EdgeService is enabled, if set ot false
	// skip checking other EdgeService configs.
	Enable bool `json:"enable,omitempty"`
}

// EdgeHub indicates the EdgeHub module config
type EdgeHub struct {
	// Enable indicates whether EdgeHub is enabled,
	// if set to false (for debugging etc.), skip checking other EdgeHub configs.
	// default true
	Enable bool `json:"enable,omitempty"`
	// Heartbeat indicates heart beat (second)
	// default 15
	Heartbeat int32 `json:"heartbeat,omitempty"`
	// TLSCAFile set ca file path
	// default "/etc/edgegateway/ca/rootCA.crt"
	TLSCAFile string `json:"tlsCaFile,omitempty"`
	// TLSCertFile indicates the file containing x509 Certificate for HTTPS
	// default "/etc/edgegateway/certs/edge.crt"
	TLSCertFile string `json:"tlsCertFile,omitempty"`
	// TLSPrivateKeyFile indicates the file containing x509 private key matching tlsCertFile
	// default "/etc/edgegateway/certs/edge.key"
	TLSPrivateKeyFile string `json:"tlsPrivateKeyFile,omitempty"`
	// SiteID indicates siteID
	// default "default-edge-site"
	SiteID string `json:"siteID,omitempty"`
	// Quic indicates quic config for EdgeHub module
	// Optional if websocket is configured
	Quic *EdgeHubQUIC `json:"quic,omitempty"`
	// WebSocket indicates websocket config for EdgeHub module
	// Optional if quic is configured
	WebSocket *EdgeHubWebSocket `json:"websocket,omitempty"`
	// HTTPServer indicates the server for edge to apply for the certificate.
	HTTPServer string `json:"httpServer,omitempty"`
}

// EdgeHubQUIC indicates the quic client config
type EdgeHubQUIC struct {
	// Enable indicates whether enable this protocol
	// default true
	Enable bool `json:"enable,omitempty"`
	// HandshakeTimeout indicates hand shake timeout (second)
	// default 30
	HandshakeTimeout int32 `json:"handshakeTimeout,omitempty"`
	// ReadDeadline indicates read dead line (second)
	// default 15
	ReadDeadline int32 `json:"readDeadline,omitempty"`
	// Server indicates quic server address (ip:port)
	// +Required
	Server string `json:"server,omitempty"`
	// WriteDeadline indicates write dead line (second)
	// default 15
	WriteDeadline int32 `json:"writeDeadline,omitempty"`
}

// EdgeHubWebSocket indicates the websocket client config
type EdgeHubWebSocket struct {
	// Enable indicates whether enable this protocol
	// default true
	Enable bool `json:"enable,omitempty"`
	// HandshakeTimeout indicates handshake timeout (second)
	// default  30
	HandshakeTimeout int32 `json:"handshakeTimeout,omitempty"`
	// ReadDeadline indicates read dead line (second)
	// default 15
	ReadDeadline int32 `json:"readDeadline,omitempty"`
	// Server indicates websocket server address (ip:port)
	// +Required
	Server string `json:"server,omitempty"`
	// WriteDeadline indicates write dead line (second)
	// default 15
	WriteDeadline int32 `json:"writeDeadline,omitempty"`
}
