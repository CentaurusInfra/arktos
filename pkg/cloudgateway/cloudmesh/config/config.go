package config

import (
	"os"
	"sync"

	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudmesh/taptun"
)

var Config Configure
var once sync.Once

type Configure struct {
	Address      string
	Port         uint32
	TapInterface *taptun.Interface
}

func InitConfigure(cm *v1.CloudMesh) {
	once.Do(func() {
		tapInterface, err := taptun.OpenTAP(cm.TapInterface)
		if err != nil {
			klog.Errorf("open tap failed", err)
			os.Exit(1)
		}
		Config = Configure{
			Address:      cm.Address,
			Port:         cm.Port,
			TapInterface: tapInterface,
		}
	})
}
