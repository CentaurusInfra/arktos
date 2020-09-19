package config

import (
	"sync"

	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
)

var Config Configure
var once sync.Once

type Configure struct {
	v1.CloudHub
	KubeAPIConfig *v1.KubeAPIConfig
	Ca            []byte
	CaKey         []byte
	Cert          []byte
	Key           []byte
}

func InitConfigure(hub *v1.CloudHub, kubeAPIConfig *v1.KubeAPIConfig) {
	once.Do(func() {
		Config = Configure{
			CloudHub:      *hub,
			KubeAPIConfig: kubeAPIConfig,
		}
	})
}
