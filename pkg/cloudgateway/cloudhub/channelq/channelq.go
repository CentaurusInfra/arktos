package channelq

import (
	"fmt"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"strings"
	"sync"

	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	beehiveModel "github.com/kubeedge/beehive/pkg/core/model"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudhub/common/model"
	"k8s.io/kubernetes/pkg/cloudgateway/common/modules"
)

// ChannelMessageQueue is the channel implementation of MessageQueue
type ChannelMessageQueue struct {
	queuePool sync.Map
	storePool sync.Map
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
		nodeID, err := GetNodeID(&msg)
		if nodeID == "" || err != nil {
			klog.Warning("node id is not found in the message, don't send this message to edge")
			continue
		}

		q.addMessageToQueue(nodeID, &msg)
	}
}

func (q *ChannelMessageQueue) addMessageToQueue(nodeId string, msg *beehiveModel.Message) {
	nodeQueue := q.GetNodeQueue(nodeId)
	nodeStore := q.GetNodeStore(nodeId)

	messageKey, _ := getMsgKey(msg)
	nodeStore.Add(msg)
	nodeQueue.Add(messageKey)
}

func getMsgKey(obj interface{}) (string, error) {
	msg := obj.(*beehiveModel.Message)

	return msg.Header.ID, nil
}

func (q *ChannelMessageQueue) GetNodeQueue(nodeID string) workqueue.RateLimitingInterface {
	queue, ok := q.queuePool.Load(nodeID)
	if !ok {
		klog.Warningf("nodeQueue for edge node %s not found and created now", nodeID)
		nodeQueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), nodeID)
		q.queuePool.Store(nodeID, nodeQueue)
		return nodeQueue
	}
	return queue.(workqueue.RateLimitingInterface)
}

func (q *ChannelMessageQueue) GetNodeStore(nodeID string) cache.Store {
	store, ok := q.storePool.Load(nodeID)
	if !ok {
		klog.Warningf("nodeStore for edge node %s not found and created now", nodeID)
		nodeStore := cache.NewStore(getMsgKey)
		q.storePool.Store(nodeID, nodeStore)
		return nodeStore
	}
	return store.(cache.Store)
}

// Publish sends edge message to Controllers
func (q *ChannelMessageQueue) Publish(msg *beehiveModel.Message) error {
	// TODO(liuzongbao): send edge message to controllers
	return nil
}

// GetNodeID get nodeID from resource of message
func GetNodeID(msg *beehiveModel.Message) (string, error) {
	resource := msg.GetResource()
	res := strings.Split(resource, "/")
	for index, value := range res {
		if value == model.ResNode && index+1 < len(res) && res[index+1] != "" {
			return res[index+1], nil
		}
	}
	return "", fmt.Errorf("no nodeID in Message.Router.Resource: %s", resource)
}

// Connect allocates the queues and stores for given node
func (q *ChannelMessageQueue) Connect(info *model.HubInfo) {
	_, queueExist := q.queuePool.Load(info.NodeID)
	_, storeExit := q.storePool.Load(info.NodeID)

	if queueExist && storeExit {
		klog.Infof("Message queue and store for edge node %s are already exist", info.NodeID)
		return
	}

	if !queueExist {
		nodeQueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), info.NodeID)
		q.queuePool.Store(info.NodeID, nodeQueue)
	}
	if !storeExit {
		nodeStore := cache.NewStore(getMsgKey)
		q.storePool.Store(info.NodeID, nodeStore)
	}
}

// Close closes queues and stores for given node
func (q *ChannelMessageQueue) Close(info *model.HubInfo) {
	_, queueExist := q.queuePool.Load(info.NodeID)
	_, storeExist := q.storePool.Load(info.NodeID)

	if !queueExist && !storeExist {
		klog.Warningf("rChannel for edge node %s is already removed", info.NodeID)
		return
	}

	if queueExist {
		q.queuePool.Delete(info.NodeID)
	}
	if storeExist {
		q.storePool.Delete(info.NodeID)
	}
}
