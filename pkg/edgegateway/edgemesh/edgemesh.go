package edgemesh

import (
	"github.com/gorilla/websocket"
	"github.com/kubeedge/beehive/pkg/core"
	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/edgegateway/v1"
	"k8s.io/kubernetes/pkg/edgegateway/common/modules"
	"k8s.io/kubernetes/pkg/edgegateway/edgemesh/client"
	"k8s.io/kubernetes/pkg/edgegateway/edgemesh/config"
	"k8s.io/kubernetes/pkg/edgegateway/edgemesh/proxy"
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
	return modules.MeshGroup
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

	proxy.Init()
	// set iptables and route
	for {
		select {
		case <-beehiveContext.Done():
			klog.Warning("edgeMesh stop")
			return
		default:
		}
		msg, err := beehiveContext.Receive(modules.EdgeMeshModuleName)
		if err != nil {
			klog.Warningf("%s receive message error: %v", modules.EdgeMeshModuleName, err)
			continue
		}
		proxy.MeshHandler(msg)
	}
}
