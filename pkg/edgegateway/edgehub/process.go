package edgehub

import (
	"fmt"
	"time"

	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"github.com/kubeedge/beehive/pkg/core/model"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/edgegateway/common/constants"
	"k8s.io/kubernetes/pkg/edgegateway/common/modules"
	"k8s.io/kubernetes/pkg/edgegateway/edgehub/clients"
	"k8s.io/kubernetes/pkg/edgegateway/edgehub/config"
)

var groupMap = map[string]string{
	"edgeService": modules.EdgeServiceGroup,
}

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
		msg.BuildRouter(modules.EdgeHubModuleName, "", "", "keepalive").FillBody("")
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

func (eh *EdgeHub) dispatch(message model.Message) error {
	// handle response message
	if message.GetOperation() == constants.ResponseOperation {
		beehiveContext.SendResp(message)
		return nil
	}

	md, ok := groupMap[message.GetGroup()]
	if !ok {
		klog.Warningf("msg_group not found")
		return fmt.Errorf("msg_group not found")
	}
	beehiveContext.SendToGroup(md, message)
	return nil
}

// distribute message from cloudhub to other modules
func (eh *EdgeHub) routeToEdge() {
	for {
		select {
		case <-beehiveContext.Done():
			klog.Warning("EdgeHub routeToEdge stop")
			return
		default:
		}
		msg, err := eh.chClient.Receive()
		if err != nil {
			klog.Errorf("websocket read error: %v", err)
			eh.reconnectChan <- struct{}{}
			return
		}

		klog.Infof("received message from cloudhub: %+v", msg)
		err = eh.dispatch(msg)
		if err != nil {
			klog.Errorf("failed to dispatch message: %v", err)
		}
	}
}

// send message to cloudhub
func (eh *EdgeHub) routeToCloud() {
	for {
		select {
		case <-beehiveContext.Done():
			klog.Warning("EdgeHub routeToCloud stop")
			return
		default:
		}
		msg, err := beehiveContext.Receive(modules.EdgeHubModuleName)
		if err != nil {
			klog.Errorf("failed to receive message from edge: %v", err)
			time.Sleep(time.Second)
			continue
		}

		// post message to cloudhub
		err = eh.sendToCloud(msg)
		if err != nil {
			klog.Errorf("failed to send message to cloud: %v", err)
			eh.reconnectChan <- struct{}{}
		}
	}
}
