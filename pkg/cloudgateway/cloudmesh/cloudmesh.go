package cloudmesh

import (
	"github.com/kubeedge/beehive/pkg/core"
	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudmesh/config"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudmesh/proxy"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudmesh/server"
	"k8s.io/kubernetes/pkg/cloudgateway/common/modules"
)

type cloudMesh struct {
	enable bool
}

func newCloudMesh(enable bool) *cloudMesh {
	return &cloudMesh{
		enable: enable,
	}
}

func Register(cm *v1.CloudMesh) {
	config.InitConfigure(cm)
	core.Register(newCloudMesh(cm.Enable))
}

func (cm *cloudMesh) Name() string {
	return modules.CloudMeshModuleName
}

func (cm *cloudMesh) Group() string {
	return modules.MeshGroup
}

func (cm *cloudMesh) Enable() bool {
	return cm.enable
}

func (cm *cloudMesh) Start() {
	// start cloudMesh
	go server.StartCloudMesh()

	proxy.Init()
	// set iptables and route
	for {
		select {
		case <-beehiveContext.Done():
			klog.Warning("cloudMesh stop")
			return
		default:
		}
		msg, err := beehiveContext.Receive(modules.CloudMeshModuleName)
		if err != nil {
			klog.Warningf("%s receive message error: %v", modules.CloudMeshModuleName, err)
			continue
		}
		proxy.MeshHandler(msg)
	}
}
