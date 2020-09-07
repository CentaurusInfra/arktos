package edgehub

import (
	"github.com/kubeedge/beehive/pkg/core"
	v1 "k8s.io/kubernetes/pkg/apis/edgegateway/v1"
	"k8s.io/kubernetes/pkg/edgegateway/common/modules"
	"k8s.io/kubernetes/pkg/edgegateway/edgehub/config"
)

// EdgeHub defines edgehub object structure
type EdgeHub struct {
	enable        bool
}

func newEdgeHub(enable bool) *EdgeHub {
	return &EdgeHub{
		enable: enable,
	}
}

// Register register edgehub
func Register(eh *v1.EdgeHub, nodeName string) {
	config.InitConfigure(eh, nodeName)
	core.Register(newEdgeHub(eh.Enable))
}

// Name returns the name of EdgeHub module
func (eh *EdgeHub) Name() string {
	return modules.EdgeHubModuleName
}

// Group returns EdgeHub group
func (eh *EdgeHub) Group() string {
	return modules.EdgeHubGroup
}

// Enable indicates whether this module is enabled
func (eh *EdgeHub) Enable() bool {
	return eh.enable
}

// Start sets context and starts the controller
func (eh *EdgeHub) Start() {

}
