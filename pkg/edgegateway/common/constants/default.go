package constants

const (
	DefaultConfigDir  = "/etc/edgegateway/config/"
	DefaultConfigFile = "edgegateway.yaml"
	DefaultCAFile     = "/etc/edgegateway/ca/rootCA.crt"
	DefaultCertFile   = "/etc/edgegateway/certs/server.crt"
	DefaultKeyFile    = "/etc/edgegateway/certs/server.key"

	DefaultCAURL   = "/ca.crt"
	DefaultCertURL = "/edge.crt"

	DefaultSiteID = "default-edge-site"
	LocalIP       = "127.0.0.1"

	ResponseOperation = "response"
)
