package edgeservice

import (
	"github.com/kubeedge/beehive/pkg/core"
	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/edgegateway/v1"
	"k8s.io/kubernetes/pkg/edgegateway/common/modules"
	"k8s.io/kubernetes/pkg/edgegateway/edgeservice/httpservice"
)

type edgeService struct {
	enable bool
}

func newEdgeService(enable bool) *edgeService {
	return &edgeService{
		enable: enable,
	}
}

func Register(s *v1.EdgeService) {
	core.Register(newEdgeService(s.Enable))
}

func (s *edgeService) Name() string {
	return modules.EdgeServiceModuleName
}

func (s *edgeService) Group() string {
	return modules.EdgeServiceGroup
}

func (s *edgeService) Enable() bool {
	return s.enable
}

func (s *edgeService) Start() {

	go httpservice.StartHttpServer()

	for {
		select {
		case <-beehiveContext.Done():
			klog.Warning("edgeService stop")
			return
		default:
		}
		msg, err := beehiveContext.Receive(modules.EdgeServiceModuleName)
		if err != nil {
			klog.Warningf("%s receive message error: %v", modules.EdgeServiceModuleName, err)
			continue
		}
		if msg.GetSource() != modules.CloudServiceModuleName {
			continue
		}
		klog.Infof("%s receive a message with ID %s", modules.EdgeServiceModuleName, msg.GetID())

		go messageRouter(msg)
	}
}
