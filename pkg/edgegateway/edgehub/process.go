package edgehub

import (
	"fmt"
	"time"

	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"github.com/kubeedge/beehive/pkg/core/model"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/edgegateway/common/modules"
	"k8s.io/kubernetes/pkg/edgegateway/edgehub/clients"
	"k8s.io/kubernetes/pkg/edgegateway/edgehub/config"
)

// initializes a client to connect cloudhub
func (eh *EdgeHub) initial() (err error) {
	cloudHubClient, err := clients.GetClient()
	if err != nil {
		return err
	}
	eh.chClient = cloudHubClient
	return nil
}

// send message to cloudhub
func (eh *EdgeHub) sendToCloud(message model.Message) error {
	eh.keeperLock.Lock()
	err := eh.chClient.Send(message)
	eh.keeperLock.Unlock()
	if err != nil {
		klog.Errorf("failed to send message: %v", err)
		return fmt.Errorf("failed to send message, error: %v", err)
	}
	return nil
}

// send heartbeat to cloudhub
func (eh *EdgeHub) keepalive() {
	for {
		select {
		case <-beehiveContext.Done():
			klog.Warning("EdgeHub KeepAlive stop")
			return
		default:
		}
		msg := model.NewMessage("")
		msg.BuildRouter(modules.EdgeHubModuleName, "", "", "").FillBody("")
		msg.FillBody("")

		// send message to cloudhub
		err := eh.sendToCloud(*msg)
		if err != nil {
			klog.Errorf("websocket write error : %v", err)
			eh.reconnectChan <- struct{}{}
			return
		}

		time.Sleep(time.Duration(config.Config.Heartbeat) * time.Second)
	}
}
