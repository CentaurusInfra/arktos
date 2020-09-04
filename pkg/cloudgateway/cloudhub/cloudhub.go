package cloudhub

import (
	"github.com/kubeedge/beehive/pkg/core"
	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudhub/channelq"
	hubconfig "k8s.io/kubernetes/pkg/cloudgateway/cloudhub/config"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudhub/servers"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudhub/servers/httpserver"
	"k8s.io/kubernetes/pkg/cloudgateway/common/modules"
)

type cloudHub struct {
	enable bool
}

func newCloudHub(enable bool) *cloudHub {
	return &cloudHub{
		enable: enable,
	}
}

func Register(hub *v1.CloudHub) {
	hubconfig.InitConfigure(hub)
	core.Register(newCloudHub(hub.Enable))
}

func (a *cloudHub) Name() string {
	return modules.CloudHubModuleName
}

func (a *cloudHub) Group() string {
	return modules.HubGroup
}

// Enable indicates whether enable this module
func (a *cloudHub) Enable() bool {
	return a.enable
}

func (a *cloudHub) Start() {

	messageq := channelq.NewChannelMessageQueue()

	// start dispatch message from the cloud to edge site
	go messageq.DispatchMessage()

	// check whether the certificates exist in the local directory, generate if they don't exist
	if err := httpserver.PrepareAllCerts(); err != nil {
		klog.Fatal(err)
	}

	// generate Token
	if err := httpserver.GenerateToken(); err != nil {
		klog.Fatal(err)
	}

	// HttpServer mainly used to issue certificates for the edge
	go httpserver.StartHTTPServer()

	servers.StartCloudHub(messageq)
}
