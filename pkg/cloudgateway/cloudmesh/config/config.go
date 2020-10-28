package config

import (
	"sync"

	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
)

var Config Configure
var once sync.Once

type Configure struct {
	Address string
	Port    uint32
}

func InitConfigure(cm *v1.CloudMesh) {
	once.Do(func() {
		Config = Configure{
			Address: cm.Address,
			Port:    cm.Port,
		}
	})
}
