package config

import (
	"encoding/pem"
	"io/ioutil"
	"sync"

	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
)

var Config Configure
var once sync.Once

type Configure struct {
	v1.CloudHub
	Ca    []byte
	CaKey []byte
	Cert  []byte
	Key   []byte
}

func InitConfigure(hub *v1.CloudHub) {
	once.Do(func() {
		if len(hub.AdvertiseAddress) == 0 {
			klog.Fatal("AdvertiseAddress must be specified!")
		}

		Config = Configure{
			CloudHub:      *hub,
		}

		ca, err := ioutil.ReadFile(hub.TLSCAFile)
		if err == nil {
			block, _ := pem.Decode(ca)
			ca = block.Bytes
			klog.Info("Succeed in loading CA certificate from local directory")
		}

		caKey, err := ioutil.ReadFile(hub.TLSCAKeyFile)
		if err == nil {
			block, _ := pem.Decode(caKey)
			caKey = block.Bytes
			klog.Info("Succeed in loading CA key from local directory")
		}

		if ca != nil && caKey != nil {
			Config.Ca = ca
			Config.CaKey = caKey
		} else if !(ca == nil && caKey == nil) {
			klog.Fatal("Both of ca and caKey should be specified!")
		}

		cert, err := ioutil.ReadFile(hub.TLSCertFile)
		if err == nil {
			block, _ := pem.Decode(cert)
			cert = block.Bytes
			klog.Info("Succeed in loading certificate from local directory")
		}
		key, err := ioutil.ReadFile(hub.TLSPrivateKeyFile)
		if err == nil {
			block, _ := pem.Decode(key)
			key = block.Bytes
			klog.Info("Succeed in loading private key from local directory")
		}

		if cert != nil && key != nil {
			Config.Cert = cert
			Config.Key = key
		} else if !(cert == nil && key == nil) {
			klog.Fatal("Both of cert and key should be specified!")
		}
	})
}
