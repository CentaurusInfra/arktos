package cloudservice

import (
	"github.com/kubeedge/beehive/pkg/core"
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
}
