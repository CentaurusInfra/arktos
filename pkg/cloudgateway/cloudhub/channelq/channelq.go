package channelq

import (
	"k8s.io/klog"
	"sync"

	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"k8s.io/kubernetes/pkg/cloudgateway/common/modules"
)

// ChannelMessageQueue is the channel implementation of MessageQueue
type ChannelMessageQueue struct {
	queuePool sync.Map
	storePool sync.Map

	listQueuePool sync.Map
	listStorePool sync.Map
}

// NewChannelMessageQueue initializes a new ChannelMessageQueue
func NewChannelMessageQueue() *ChannelMessageQueue {
	return &ChannelMessageQueue{
	}
}

// DispatchMessage gets the message from the cloud, extracts the
// node id from it, gets the message associated with the node
// and pushes the message to the queue
func (q *ChannelMessageQueue) DispatchMessage() {
	for {
		select {
		case <-beehiveContext.Done():
			klog.Warning("Cloudhub channel eventqueue dispatch message loop stoped")
			return
		default:
		}
		msg, err := beehiveContext.Receive(modules.HubGroup)
		if err != nil {
			klog.Info("receive not Message format message")
			continue
		}
		// TODO(ZongbaoLiu): handle message
		klog.Info("receive not Message format message: %v", &msg)

	}
}
