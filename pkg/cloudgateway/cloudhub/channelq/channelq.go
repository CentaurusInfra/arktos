package channelq

import (
	"fmt"
	"strings"
	"sync"

	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	beehiveModel "github.com/kubeedge/beehive/pkg/core/model"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/cloudgateway/common/constants"
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
// site id from it, gets the message associated with the site
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
		siteID, err := GetSiteID(&msg)
		if siteID == "" || err != nil {
			klog.Warning("site id is not found in the message, don't send this message to edge")
			continue
		}

		q.addMessageToQueue(siteID, &msg)
	}
}

func (q *ChannelMessageQueue) addMessageToQueue(siteId string, msg *beehiveModel.Message) {
	siteQueue := q.GetSiteQueue(siteId)
	siteStore := q.GetSiteStore(siteId)

	messageKey, _ := getMsgKey(msg)
	siteStore.Add(msg)
	siteQueue.Add(messageKey)
}

func getMsgKey(obj interface{}) (string, error) {
	msg := obj.(*beehiveModel.Message)

	return msg.Header.ID, nil
}

func (q *ChannelMessageQueue) GetSiteQueue(siteID string) workqueue.RateLimitingInterface {
	queue, ok := q.queuePool.Load(siteID)
	if !ok {
		klog.Warningf("siteQueue for edge site %s not found and created now", siteID)
		siteQueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), siteID)
		q.queuePool.Store(siteID, siteQueue)
		return siteQueue
	}
	return queue.(workqueue.RateLimitingInterface)
}

func (q *ChannelMessageQueue) GetSiteStore(siteID string) cache.Store {
	store, ok := q.storePool.Load(siteID)
	if !ok {
		klog.Warningf("siteStore for edge site %s not found and created now", siteID)
		siteStore := cache.NewStore(getMsgKey)
		q.storePool.Store(siteID, siteStore)
		return siteStore
	}
	return store.(cache.Store)
}

// Publish sends edge message to Controllers
func (q *ChannelMessageQueue) Publish(msg *beehiveModel.Message) {
	switch msg.GetGroup() {
	case modules.CloudServiceGroup:
		beehiveContext.SendToGroup(modules.CloudServiceGroup, *msg)
	default:
		klog.Warningf("message %s does not belong to any group, it will be discarded", msg.GetID())
	}
	return
}

// GetSiteID get siteID from resource of message
func GetSiteID(msg *beehiveModel.Message) (string, error) {
	resource := msg.GetResource()
	res := strings.Split(resource, "/")
	for index, value := range res {
		if value == constants.ResSite && index+1 < len(res) && res[index+1] != "" {
			return res[index+1], nil
		}
	}
	return "", fmt.Errorf("no siteID in Message.Router.Resource: %s", resource)
}

// Connect allocates the queues and stores for given site
func (q *ChannelMessageQueue) Connect(siteID string) {
	_, queueExist := q.queuePool.Load(siteID)
	_, storeExit := q.storePool.Load(siteID)

	if queueExist && storeExit {
		klog.Infof("Message queue and store for edge site %s are already exist", siteID)
		return
	}

	if !queueExist {
		siteQueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), siteID)
		q.queuePool.Store(siteID, siteQueue)
	}
	if !storeExit {
		siteStore := cache.NewStore(getMsgKey)
		q.storePool.Store(siteID, siteStore)
	}
}

// Close closes queues and stores for given site
func (q *ChannelMessageQueue) Close(siteID string) {
	_, queueExist := q.queuePool.Load(siteID)
	_, storeExist := q.storePool.Load(siteID)

	if !queueExist && !storeExist {
		klog.Warningf("rChannel for edge site %s is already removed", siteID)
		return
	}

	if queueExist {
		q.queuePool.Delete(siteID)
	}
	if storeExist {
		q.storePool.Delete(siteID)
	}
}
