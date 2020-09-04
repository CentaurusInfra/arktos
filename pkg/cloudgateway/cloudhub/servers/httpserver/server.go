/*
Copyright 2020 The KubeEdge Authors.

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

package httpserver

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog"
	hubconfig "k8s.io/kubernetes/pkg/cloudgateway/cloudhub/config"
	"k8s.io/kubernetes/pkg/cloudgateway/common/constants"
)

const (
	certificateBlockType = "CERTIFICATE"
	// NodeName is for the clearer log
	NodeName = "NodeName"
)

// StartHTTPServer starts the http service
func StartHTTPServer() {
	router := mux.NewRouter()
	router.HandleFunc(constants.DefaultCertURL, edgeGatewayClientCert).Methods("GET")
	router.HandleFunc(constants.DefaultCAURL, getCA).Methods("GET")

	addr := fmt.Sprintf("%s:%d", hubconfig.Config.HTTPS.Address, hubconfig.Config.HTTPS.Port)

	cert, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: certificateBlockType, Bytes: hubconfig.Config.Cert}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: hubconfig.Config.Key}))

	if err != nil {
		klog.Fatal(err)
	}

	server := &http.Server{
		Addr:    addr,
		Handler: router,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.RequestClientCert,
		},
	}
	klog.Fatal(server.ListenAndServeTLS("", ""))
}

// getCA returns the caCertDER
func getCA(w http.ResponseWriter, r *http.Request) {
	caCertDER := hubconfig.Config.Ca
	w.Write(caCertDER)
}

// edgeGatewayClientCert will verify the certificate of EdgeGateway or token then create EdgeGatewayCert and return it
func edgeGatewayClientCert(w http.ResponseWriter, r *http.Request) {
	if cert := r.TLS.PeerCertificates; len(cert) > 0 {
		if err := verifyCert(cert[0]); err != nil {
			klog.Errorf("failed to sign the certificate for edge site: %s, failed to verify the certificate", r.Header.Get(NodeName))
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(err.Error()))
		} else {
			signEdgeCert(w, r)
		}
		return
	}
	if verifyAuthorization(w, r) {
		signEdgeCert(w, r)
	} else {
		klog.Errorf("failed to sign the certificate for edge site: %s, invalid token", r.Header.Get(NodeName))
	}
}

// verifyCert verifies the edge certificate by CA certificate when edge certificates rotate.
func verifyCert(cert *x509.Certificate) error {
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: certificateBlockType, Bytes: hubconfig.Config.Ca}))
	if !ok {
		return fmt.Errorf("failed to parse root certificate")
	}
	opts := x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if _, err := cert.Verify(opts); err != nil {
		return fmt.Errorf("failed to verify edge certificate: %v", err)
	}
	return nil
}

// verifyAuthorization verifies the token from EdgeGateway CSR
func verifyAuthorization(w http.ResponseWriter, r *http.Request) bool {
	authorizationHeader := r.Header.Get("authorization")
	if authorizationHeader == "" {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Invalid authorization token"))
		return false
	}
	bearerToken := strings.Split(authorizationHeader, " ")
	if len(bearerToken) != 2 {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Invalid authorization token"))
		return false
	}
	token, err := jwt.Parse(bearerToken[1], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("there was an error")
		}
		caKey := hubconfig.Config.CaKey
		return caKey, nil
	})
	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Invalid authorization token"))
			return false
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid authorization token"))
		return false
	}
	if !token.Valid {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Invalid authorization token"))
		return false
	}
	return true
}

// signEdgeCert signs the CSR from EdgeGateway
func signEdgeCert(w http.ResponseWriter, r *http.Request) {
	csrContent, err := ioutil.ReadAll(r.Body)
	if err != nil {
		klog.Errorf("fail to read file when signing the cert for edge site:%s! error:%v", r.Header.Get(NodeName), err)
	}
	csr, err := x509.ParseCertificateRequest(csrContent)
	if err != nil {
		klog.Errorf("fail to ParseCertificateRequest of edge site: %s! error:%v", r.Header.Get(NodeName), err)
	}
	subject := csr.Subject
	clientCertDER, err := signCerts(subject, csr.PublicKey)
	if err != nil {
		klog.Errorf("fail to signCerts for edge site:%s! error:%v", r.Header.Get(NodeName), err)
	}

	w.Write(clientCertDER)
}

// signCerts will create a certificate for EdgeGateway
func signCerts(subInfo pkix.Name, pbKey crypto.PublicKey) ([]byte, error) {
	cfgs := &certutil.Config{
		CommonName:   subInfo.CommonName,
		Organization: subInfo.Organization,
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientKey := pbKey

	ca := hubconfig.Config.Ca
	caCert, err := x509.ParseCertificate(ca)
	if err != nil {
		return nil, fmt.Errorf("unable to ParseCertificate: %v", err)
	}

	caKeyDER := hubconfig.Config.CaKey
	caKey, err := x509.ParseECPrivateKey(caKeyDER)
	if err != nil {
		return nil, fmt.Errorf("unable to ParseECPrivateKey: %v", err)
	}

	edgeCertSigningDuration := hubconfig.Config.CloudHub.EdgeCertSigningDuration
	certDER, err := NewCertFromCa(cfgs, caCert, clientKey, caKey, edgeCertSigningDuration) //crypto.Signer(caKey)
	if err != nil {
		return nil, fmt.Errorf("unable to NewCertFromCa: %v", err)
	}

	return certDER, err
}

// PrepareAllCerts check whether the certificates exist in the local directory, generate if they don't exist
func PrepareAllCerts() error {
	// Check whether the ca exists in the local directory
	if hubconfig.Config.Ca == nil && hubconfig.Config.CaKey == nil {
		klog.Info("Ca and CaKey don't exist in local directory, and will be created by CloudGateway")
		caDER, caKey, err := NewCertificateAuthorityDer()
		if err != nil {
			klog.Errorf("failed to create Certificate Authority, error: %v", err)
			return err
		}

		caKeyDER, err := x509.MarshalECPrivateKey(caKey.(*ecdsa.PrivateKey))
		if err != nil {
			klog.Errorf("failed to convert an EC private key to SEC 1, ASN.1 DER form, error: %v", err)
			return err
		}

		UpdateConfig(caDER, caKeyDER, nil, nil)
	}

	// Check whether the CloudGateway certificates exist in the local directory
	if hubconfig.Config.Key == nil && hubconfig.Config.Cert == nil {
		klog.Infof("CloudGatewayCert and key don't exist in local directory, and will be signed by CA")
		certDER, keyDER, err := SignCerts()
		if err != nil {
			klog.Errorf("failed to sign a certificate, error: %v", err)
			return err
		}

		UpdateConfig(nil, nil, certDER, keyDER)
	}
	return nil
}
