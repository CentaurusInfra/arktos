package constants

const (
	DefaultConfigDir  = "/etc/cloudgateway/config/"
	DefaultConfigFile = "cloudgateway.yaml"
	DefaultCAFile     = "/etc/cloudgateway/ca/rootCA.crt"
	DefaultCAKeyFile  = "/etc/cloudgateway/ca/rootCA.key"
	DefaultCertFile   = "/etc/cloudgateway/certs/server.crt"
	DefaultKeyFile    = "/etc/cloudgateway/certs/server.key"

	DefaultCAURL   = "/ca.crt"
	DefaultCertURL = "/edge.crt"

	// KubeAPIConfig
	DefaultKubeConfig	= "/root/.kube/config"
)
