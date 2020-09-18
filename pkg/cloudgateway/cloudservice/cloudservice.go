package cloudservice

import (
	"github.com/kubeedge/beehive/pkg/core"
	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudservice/httpservice"
	"k8s.io/kubernetes/pkg/cloudgateway/common/modules"
)

type cloudService struct {
	enable bool
}

func newCloudService(enable bool) *cloudService {
	return &cloudService{
		enable: enable,
	}
}

func Register(s *v1.CloudService) {
	core.Register(newCloudService(s.Enable))
}

func (s *cloudService) Name() string {
	return modules.CloudServiceModuleName
}

func (s *cloudService) Group() string {
	return modules.CloudServiceGroup
}

func (s *cloudService) Enable() bool {
	return s.enable
}

func (s *cloudService) Start() {

	go httpservice.StartHttpServer()

	for {
		select {
		case <-beehiveContext.Done():
			klog.Warning("cloudService stop")
			return
		default:
		}
		msg, err := beehiveContext.Receive(modules.CloudServiceModuleName)
		if err != nil {
			klog.Warningf("%s receive message error: %v", modules.CloudServiceModuleName, err)
			continue
		}
		if msg.GetSource() != modules.EdgeServiceModuleName {
			continue
		}
		klog.Infof("%s receive a message with ID %s", modules.CloudServiceModuleName, msg.GetID())

		go messageRouter(msg)
	}
}
