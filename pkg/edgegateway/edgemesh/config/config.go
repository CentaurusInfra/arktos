package config

import (
	"os"
	"sync"

	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/edgegateway/v1"
	"k8s.io/kubernetes/pkg/edgegateway/edgemesh/taptun"
)

var Config Configure
var once sync.Once

type Configure struct {
	Server       string
	TapName      string
	TapInterface *taptun.Interface
}

func InitConfigure(em *v1.EdgeMesh) {
	once.Do(func() {
		tapInterface, err := taptun.OpenTAP(em.TapInterface)
		if err != nil {
			klog.Errorf("open tap failed", err)
			os.Exit(1)
		}
		Config = Configure{
			Server:       em.Server,
			TapName:      em.TapInterface,
			TapInterface: tapInterface,
		}
	})
}
