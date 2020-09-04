package config

import (
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	"sync"
)

var once sync.Once

func InitConfigure(c *v1.Controller) {
	once.Do(func() {
		// #TODO(nkwangjun): add controller config here
		return
	})
}
