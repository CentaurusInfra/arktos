package httpserver

import hubconfig "k8s.io/kubernetes/pkg/cloudgateway/cloudhub/config"

func UpdateConfig(ca, caKey, cert, key []byte) {
	if ca != nil {
		hubconfig.Config.Ca = ca
	}
	if caKey != nil {
		hubconfig.Config.CaKey = caKey
	}
	if cert != nil {
		hubconfig.Config.Cert = cert
	}
	if key != nil {
		hubconfig.Config.Key = key
	}
}
