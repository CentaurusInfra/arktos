package edgehub

import (
	"sync"
	"time"

	"github.com/kubeedge/beehive/pkg/core"
	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/edgegateway/v1"
	"k8s.io/kubernetes/pkg/edgegateway/common/modules"
	"k8s.io/kubernetes/pkg/edgegateway/edgehub/clients"
	"k8s.io/kubernetes/pkg/edgegateway/edgehub/config"
)

// EdgeHub defines edgehub object structure
type EdgeHub struct {
	chClient      clients.Adapter
	reconnectChan chan struct{}
	keeperLock    sync.RWMutex
	enable        bool
}

func newEdgeHub(enable bool) *EdgeHub {
	return &EdgeHub{
		reconnectChan: make(chan struct{}),
		enable:        enable,
	}
}

// Register register edgehub
func Register(eh *v1.EdgeHub) {
	config.InitConfigure(eh)
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
	for {
		select {
		case <-beehiveContext.Done():
			klog.Warning("EdgeHub stop")
			return
		default:
		}
		err := eh.initial()
		if err != nil {
			klog.Fatalf("failed to init controller: %v", err)
			return
		}
		err = eh.chClient.Init()
		if err != nil {
			klog.Errorf("connection err, try again after a minute: %v", err)
			time.Sleep(time.Minute)
			continue
		}

		// send heartbeat to cloudhub
		go eh.keepalive()

		// stop websocket connection
		<-eh.reconnectChan
		eh.chClient.Uninit()

		time.Sleep(time.Duration(config.Config.Heartbeat) * time.Second * 2)

		// clean channel
	clean:
		for {
			select {
			case <-eh.reconnectChan:
			default:
				break clean
			}
		}
	}
}
