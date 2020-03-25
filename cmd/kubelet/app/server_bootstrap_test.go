/*
Copyright 2016 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

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

package app

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	cfsslconfig "github.com/cloudflare/cfssl/config"
	cfsslsigner "github.com/cloudflare/cfssl/signer"
	cfssllocal "github.com/cloudflare/cfssl/signer/local"

	certapi "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	restclient "k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"
)

// Test_buildClientCertificateManager validates that we can build a local client cert
// manager that will use the bootstrap client until we get a valid cert, then use our
// provided identity on subsequent requests.
func Test_buildClientCertificateManager(t *testing.T) {
	testDir, err := ioutil.TempDir("", "kubeletcert")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.RemoveAll(testDir) }()

	serverPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serverCA, err := certutil.NewSelfSignedCACert(certutil.Config{
		CommonName: "the-test-framework",
	}, serverPrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	server := &csrSimulator{
		t:                t,
		serverPrivateKey: serverPrivateKey,
		serverCA:         serverCA,
	}
	s := httptest.NewServer(server)
	defer s.Close()

	kubeConfig1 := &restclient.KubeConfig{
		UserAgent: "FirstClient",
		Host:      s.URL,
	}
	config1 := restclient.NewAggregatedConfig(kubeConfig1)

	kubeConfig2 := &restclient.KubeConfig{
		UserAgent: "SecondClient",
		Host:      s.URL,
	}
	config2 := restclient.NewAggregatedConfig(kubeConfig2)

	nodeName := types.NodeName("test")
	m, err := buildClientCertificateManager(config1, config2, testDir, nodeName)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	r := m.(rotater)

	// get an expired CSR (simulating historical output)
	server.backdate = 2 * time.Hour
	server.expectUserAgent = "FirstClient"
	ok, err := r.RotateCerts()
	if !ok || err != nil {
		t.Fatalf("unexpected rotation err: %t %v", ok, err)
	}
	if cert := m.Current(); cert != nil {
		t.Fatalf("Unexpected cert, should be expired: %#v", cert)
	}
	fi := getFileInfo(testDir)
	if len(fi) != 2 {
		t.Fatalf("Unexpected directory contents: %#v", fi)
	}

	// if m.Current() == nil, then we try again and get a valid
	// client
	server.backdate = 0
	server.expectUserAgent = "FirstClient"
	if ok, err := r.RotateCerts(); !ok || err != nil {
		t.Fatalf("unexpected rotation err: %t %v", ok, err)
	}
	if cert := m.Current(); cert == nil {
		t.Fatalf("Unexpected cert, should be valid: %#v", cert)
	}
	fi = getFileInfo(testDir)
	if len(fi) != 2 {
		t.Fatalf("Unexpected directory contents: %#v", fi)
	}

	// if m.Current() != nil, then we should use the second client
	server.expectUserAgent = "SecondClient"
	if ok, err := r.RotateCerts(); !ok || err != nil {
		t.Fatalf("unexpected rotation err: %t %v", ok, err)
	}
	if cert := m.Current(); cert == nil {
		t.Fatalf("Unexpected cert, should be valid: %#v", cert)
	}
	fi = getFileInfo(testDir)
	if len(fi) != 2 {
		t.Fatalf("Unexpected directory contents: %#v", fi)
	}
}

func Test_buildClientCertificateManager_populateCertDir(t *testing.T) {
	testDir, err := ioutil.TempDir("", "kubeletcert")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.RemoveAll(testDir) }()

	// when no cert is provided, write nothing to disk
	kubeConfig1 := &restclient.KubeConfig{
		UserAgent: "FirstClient",
		Host:      "http://localhost",
	}
	config1 := restclient.NewAggregatedConfig(kubeConfig1)

	kubeConfig2 := &restclient.KubeConfig{
		UserAgent: "SecondClient",
		Host:      "http://localhost",
	}
	config2 := restclient.NewAggregatedConfig(kubeConfig2)
	nodeName := types.NodeName("test")
	if _, err := buildClientCertificateManager(config1, config2, testDir, nodeName); err != nil {
		t.Fatal(err)
	}
	fi := getFileInfo(testDir)
	if len(fi) != 0 {
		t.Fatalf("Unexpected directory contents: %#v", fi)
	}

	// an invalid cert should be ignored
	kubeConfig2.CertData = []byte("invalid contents")
	kubeConfig2.KeyData = []byte("invalid contents")
	if _, err := buildClientCertificateManager(config1, config2, testDir, nodeName); err == nil {
		t.Fatal("unexpected non error")
	}
	fi = getFileInfo(testDir)
	if len(fi) != 0 {
		t.Fatalf("Unexpected directory contents: %#v", fi)
	}

	// an expired client certificate should be written to disk, because the cert manager can
	// use config1 to refresh it and the cert manager won't return it for clients.
	kubeConfig2.CertData, kubeConfig2.KeyData = genClientCert(t, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	if _, err := buildClientCertificateManager(config1, config2, testDir, nodeName); err != nil {
		t.Fatal(err)
	}
	fi = getFileInfo(testDir)
	if len(fi) != 2 {
		t.Fatalf("Unexpected directory contents: %#v", fi)
	}

	// a valid, non-expired client certificate should be written to disk
	kubeConfig2.CertData, kubeConfig2.KeyData = genClientCert(t, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	if _, err := buildClientCertificateManager(config1, config2, testDir, nodeName); err != nil {
		t.Fatal(err)
	}
	fi = getFileInfo(testDir)
	if len(fi) != 2 {
		t.Fatalf("Unexpected directory contents: %#v", fi)
	}

}

func getFileInfo(dir string) map[string]os.FileInfo {
	fi := make(map[string]os.FileInfo)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if path == dir {
			return nil
		}
		fi[path] = info
		if !info.IsDir() {
			os.Remove(path)
		}
		return nil
	})
	return fi
}

type rotater interface {
	RotateCerts() (bool, error)
}

func getCSR(req *http.Request) (*certapi.CertificateSigningRequest, error) {
	if req.Body == nil {
		return nil, nil
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	csr := &certapi.CertificateSigningRequest{}
	if err := json.Unmarshal(body, csr); err != nil {
		return nil, err
	}
	return csr, nil
}

func mustMarshal(obj interface{}) []byte {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return data
}

type csrSimulator struct {
	t *testing.T

	serverPrivateKey *ecdsa.PrivateKey
	serverCA         *x509.Certificate
	backdate         time.Duration

	expectUserAgent string

	lock sync.Mutex
	csr  *certapi.CertificateSigningRequest
}

func (s *csrSimulator) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.lock.Lock()
	defer s.lock.Unlock()
	t := s.t

	// filter out timeouts as csrSimulator don't support them
	q := req.URL.Query()
	q.Del("timeout")
	q.Del("timeoutSeconds")
	req.URL.RawQuery = q.Encode()

	t.Logf("Request %q %q %q", req.Method, req.URL, req.UserAgent())

	if len(s.expectUserAgent) > 0 && req.UserAgent() != s.expectUserAgent {
		t.Errorf("Unexpected user agent: %s", req.UserAgent())
	}

	switch {
	case req.Method == "POST" && req.URL.Path == "/apis/certificates.k8s.io/v1beta1/certificatesigningrequests":
		csr, err := getCSR(req)
		if err != nil {
			t.Fatal(err)
		}
		if csr.Name == "" {
			csr.Name = "test-csr"
		}

		csr.UID = types.UID("1")
		csr.ResourceVersion = "1"
		data := mustMarshal(csr)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)

		csr = csr.DeepCopy()
		csr.ResourceVersion = "2"
		var usages []string
		for _, usage := range csr.Spec.Usages {
			usages = append(usages, string(usage))
		}
		policy := &cfsslconfig.Signing{
			Default: &cfsslconfig.SigningProfile{
				Usage:        usages,
				Expiry:       time.Hour,
				ExpiryString: time.Hour.String(),
				Backdate:     s.backdate,
			},
		}
		cfs, err := cfssllocal.NewSigner(s.serverPrivateKey, s.serverCA, cfsslsigner.DefaultSigAlgo(s.serverPrivateKey), policy)
		if err != nil {
			t.Fatal(err)
		}
		csr.Status.Certificate, err = cfs.Sign(cfsslsigner.SignRequest{
			Request: string(csr.Spec.Request),
		})
		if err != nil {
			t.Fatal(err)
		}
		csr.Status.Conditions = []certapi.CertificateSigningRequestCondition{
			{Type: certapi.CertificateApproved},
		}
		s.csr = csr

	case req.Method == "GET" && req.URL.Path == "/apis/certificates.k8s.io/v1beta1/certificatesigningrequests" && req.URL.RawQuery == "fieldSelector=metadata.name%3Dtest-csr&limit=500&resourceVersion=0":
		if s.csr == nil {
			t.Fatalf("no csr")
		}
		csr := s.csr.DeepCopy()

		data := mustMarshal(&certapi.CertificateSigningRequestList{
			ListMeta: metav1.ListMeta{
				ResourceVersion: "2",
			},
			Items: []certapi.CertificateSigningRequest{
				*csr,
			},
		})
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)

	case req.Method == "GET" && req.URL.Path == "/apis/certificates.k8s.io/v1beta1/certificatesigningrequests" && req.URL.RawQuery == "fieldSelector=metadata.name%3Dtest-csr&resourceVersion=2&watch=true":
		if s.csr == nil {
			t.Fatalf("no csr")
		}
		csr := s.csr.DeepCopy()

		data := mustMarshal(&metav1.WatchEvent{
			Type: "ADDED",
			Object: runtime.RawExtension{
				Raw: mustMarshal(csr),
			},
		})
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)

	default:
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
	}
}

// genClientCert generates an x509 certificate for testing. Certificate and key
// are returned in PEM encoding.
func genClientCert(t *testing.T, from, to time.Time) ([]byte, []byte) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyRaw, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{Organization: []string{"Acme Co"}},
		NotBefore:    from,
		NotAfter:     to,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	certRaw, err := x509.CreateCertificate(rand.Reader, cert, cert, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certRaw}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyRaw})
}
