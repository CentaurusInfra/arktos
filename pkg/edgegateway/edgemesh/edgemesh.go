package edgemesh

import (
	"github.com/gorilla/websocket"
	"github.com/kubeedge/beehive/pkg/core"
	v1 "k8s.io/kubernetes/pkg/apis/edgegateway/v1"
	"k8s.io/kubernetes/pkg/edgegateway/common/modules"
	"k8s.io/kubernetes/pkg/edgegateway/edgemesh/client"
	"k8s.io/kubernetes/pkg/edgegateway/edgemesh/config"
)

type edgeMesh struct {
	client *websocket.Conn
	enable bool
}

func newEdgeMesh(enable bool) *edgeMesh {
	return &edgeMesh{
		enable: enable,
	}
}

func Register(em *v1.EdgeMesh) {
	config.InitConfigure(em)
	core.Register(newEdgeMesh(em.Enable))
}

func (em *edgeMesh) Name() string {
	return modules.EdgeMeshModuleName
}

func (em *edgeMesh) Group() string {
	return modules.EdgeMeshGroup
}

func (em *edgeMesh) Enable() bool {
	return em.enable
}

func (em *edgeMesh) Start() {
	em.client = client.NewClient()
	// send edge site message stream to cloud
	go em.upstream()
	// send cloud message stream to edge service
	go em.downstream()
}
