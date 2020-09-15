package certificate

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io/ioutil"

	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/edgegateway/v1"
	"k8s.io/kubernetes/pkg/edgegateway/common/constants"
	"k8s.io/kubernetes/pkg/edgegateway/edgehub/common/certutil"
	"k8s.io/kubernetes/pkg/edgegateway/edgehub/common/http"
)

type CertManager struct {
	CR     *x509.CertificateRequest
	SiteID string
	// the location to save certificate
	caFile   string
	certFile string
	keyFile  string
	// the url to get certificate from cloud
	caURL   string
	certURL string
}

func NewCertManager(edgehub v1.EdgeHub) CertManager {
	certReq := &x509.CertificateRequest{
		Subject: pkix.Name{
			Organization: []string{"Arktos"},
			CommonName:   "arktos.io",
		},
	}
	return CertManager{
		CR:       certReq,
		SiteID:   edgehub.SiteID,
		caFile:   edgehub.TLSCAFile,
		certFile: edgehub.TLSCertFile,
		keyFile:  edgehub.TLSPrivateKeyFile,
		caURL:    edgehub.HTTPServer + constants.DefaultCAURL,
		certURL:  edgehub.HTTPServer + constants.DefaultCertURL,
	}
}

// start the CertManager
func (cm *CertManager) Start() {
	err := cm.checkCerts()
	if err != nil {
		err = cm.getCerts()
		if err != nil {
			klog.Fatalf("failed to get edge cert from cloud, err: %v", err)
		}
	}
}

// check if certificates exist
func (cm *CertManager) checkCerts() error {
	_, err := ioutil.ReadFile(cm.caFile)
	if err != nil {
		return fmt.Errorf("unable to find ca certificate, err: %v", err)
	}
	_, err = tls.LoadX509KeyPair(cm.certFile, cm.keyFile)
	if err != nil {
		return fmt.Errorf("unable to load certificate data, err: %v", err)
	}
	return nil
}

// get certificate from cloud
func (cm *CertManager) getCerts() error {
	// get the ca.crt
	caCert, err := GetCACert(cm.caURL)
	if err != nil {
		return fmt.Errorf("failed to get CA certificate, err: %v", err)
	}
	// save the ca.crt to file
	ca, err := x509.ParseCertificate(caCert)
	if err != nil {
		return fmt.Errorf("failed to parse the CA certificate, error: %v", err)
	}
	if err = certutil.WriteCert(cm.caFile, ca); err != nil {
		return fmt.Errorf("failed to save the CA certificate to file: %s, error: %v", cm.caFile, err)
	}

	// get the edge.crt
	pk, edgeCert, err := cm.GetEdgeCert(cm.certURL)
	if err != nil {
		return fmt.Errorf("failed to get edge certificate from the cloudgateway, error: %v", err)
	}
	// save the edge.crt to the file
	cert, err := x509.ParseCertificate(edgeCert)
	if err != nil {
		return fmt.Errorf("failed to parse the edge certificate, error: %v", err)
	}
	if err = certutil.WriteKeyAndCert(cm.keyFile, cm.certFile, pk, cert); err != nil {
		return fmt.Errorf("failed to save the edge key and certificate to file: %s, error: %v", cm.certFile, err)
	}

	return nil
}

// GetCACert gets the cloudGateway CA certificate
func GetCACert(url string) ([]byte, error) {
	client := http.NewHTTPClient()
	res, err := http.SendRequest(client, url, nil, "GET", "")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	caCert, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return caCert, nil
}

// GetEdgeCert applies for the certificate from cloudGateway
func (cm *CertManager) GetEdgeCert(url string) (*ecdsa.PrivateKey, []byte, error) {
	pk, csr, err := cm.getCSR()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CSR: %v", err)
	}

	client := http.NewHTTPClient()
	if err != nil {
		return nil, nil, fmt.Errorf("falied to create http client: %v", err)
	}

	res, err := http.SendRequest(client, url, bytes.NewReader(csr), "GET", cm.SiteID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send http request: %v", err)
	}
	defer res.Body.Close()

	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, nil, err
	}
	if res.StatusCode != 200 {
		return nil, nil, fmt.Errorf(string(content))
	}

	return pk, content, nil
}

func (cm *CertManager) getCSR() (*ecdsa.PrivateKey, []byte, error) {
	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, cm.CR, pk)
	if err != nil {
		return nil, nil, err
	}

	return pk, csr, nil
}
