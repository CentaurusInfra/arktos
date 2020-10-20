package cloudmesh

import (
	"github.com/kubeedge/beehive/pkg/core"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
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
	core.Register(newCloudMesh(cm.Enable))
}

func (cm *cloudMesh) Name() string {
	return modules.CloudMeshModuleName
}

func (cm *cloudMesh) Group() string {
	return modules.CloudMeshGroup
}

func (cm *cloudMesh) Enable() bool {
	return cm.enable
}

func (cm *cloudMesh) Start() {

}
