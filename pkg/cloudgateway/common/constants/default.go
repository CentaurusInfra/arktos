package constants

const (
	DefaultConfigDir  = "/etc/cloudgateway/config/"
	DefaultConfigFile = "cloudgateway.yaml"

	DefaultCAURL   = "/ca.crt"
	DefaultCertURL = "/edge.crt"

	// KubeAPIConfig
	DefaultKubeConfig = "/root/.kube/config"

	ResponseOperation = "response"
	OpKeepalive       = "keepalive"
	ResSite           = "site"

	ServiceClient        = "client"
	ServiceServer        = "server"
	DefaultCloudSiteName = "cloud"
	Insert               = "insert"
	Delete               = "delete"
)
