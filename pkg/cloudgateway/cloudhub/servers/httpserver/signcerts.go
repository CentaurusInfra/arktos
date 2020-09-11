package httpserver

import (
	"crypto/x509"
	"net"

	certutil "k8s.io/client-go/util/cert"
	hubconfig "k8s.io/kubernetes/pkg/cloudgateway/cloudhub/config"
)

// SignCerts creates server's certificate and key
func SignCerts() ([]byte, []byte, error) {
	cfg := &certutil.Config{
		CommonName:   "Arktos",
		Organization: []string{"Arktos"},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		AltNames: certutil.AltNames{
			IPs: getIps(hubconfig.Config.AdvertiseAddress),
		},
	}

	certDER, keyDER, err := NewCloudGatewayCertDERandKey(cfg)
	if err != nil {
		return nil, nil, err
	}

	return certDER, keyDER, nil
}

func getIps(advertiseAddress []string) (Ips []net.IP) {
	for _, addr := range advertiseAddress {
		Ips = append(Ips, net.ParseIP(addr))
	}
	return
}
