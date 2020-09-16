package handler

import (
	"fmt"
	"strings"
	"sync"
	"time"

	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	beehiveModel "github.com/kubeedge/beehive/pkg/core/model"
	"github.com/kubeedge/viaduct/pkg/conn"
	"github.com/kubeedge/viaduct/pkg/mux"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudhub/channelq"
	hubio "k8s.io/kubernetes/pkg/cloudgateway/cloudhub/common/io"
	hubconfig "k8s.io/kubernetes/pkg/cloudgateway/cloudhub/config"
	"k8s.io/kubernetes/pkg/cloudgateway/common/constants"
)

// ExitCode exit code
type ExitCode int

const siteStop ExitCode = iota

// MessageHandle processes messages between cloud and edge
type MessageHandle struct {
	KeepaliveInterval int
	WriteTimeout      int
	Sites             sync.Map
	siteConns         sync.Map
	siteLocks         sync.Map
	siteRegistered    sync.Map
	MessageQueue      *channelq.ChannelMessageQueue
	Handlers          []HandleFunc
	SiteLimit         int
	KeepaliveChannel  sync.Map
}

type HandleFunc func(siteID string, exitServe chan ExitCode)

var once sync.Once

// CloudhubHandler the shared handler for both websocket and quic servers
var CloudhubHandler *MessageHandle

// InitHandler create a handler for websocket and quic servers
func InitHandler(eventq *channelq.ChannelMessageQueue) {
	once.Do(func() {
		CloudhubHandler = &MessageHandle{
			KeepaliveInterval: int(hubconfig.Config.KeepaliveInterval),
			WriteTimeout:      int(hubconfig.Config.WriteTimeout),
			MessageQueue:      eventq,
			SiteLimit:         int(hubconfig.Config.SiteLimit),
		}

		CloudhubHandler.Handlers = []HandleFunc{
			CloudhubHandler.KeepaliveCheckLoop,
			CloudhubHandler.MessageWriteLoop,
		}

		CloudhubHandler.initServerEntries()
	})
}

// initServerEntries register handler func
func (mh *MessageHandle) initServerEntries() {
	mux.Entry(mux.NewPattern("*").Op("*"), mh.HandleServer)
}

// HandleServer handle all the request from site
func (mh *MessageHandle) HandleServer(container *mux.MessageContainer, writer mux.ResponseWriter) {
	siteID := container.Header.Get("siteID")

	if mh.GetSiteCount() >= mh.SiteLimit {
		klog.Errorf("Fail to serve site %s, reach site limit", siteID)
		return
	}

	// receive heartbeat from edge
	if container.Message.GetOperation() == constants.OpKeepalive {
		klog.Infof("Keepalive message received from site: %s", siteID)

		siteKeepalive, ok := mh.KeepaliveChannel.Load(siteID)
		if !ok {
			klog.Errorf("Failed to load site : %s", siteID)
			return
		}
		siteKeepalive.(chan struct{}) <- struct{}{}
		return
	}

	// handle response message
	if container.Message.GetOperation() == constants.ResponseOperation {
		beehiveContext.SendResp(*container.Message)
		return
	}

	// handle message from edge
	err := mh.PubToController(siteID, container.Message)
	if err != nil {
		klog.Errorf("")
	}
}

// GetSiteCount returns the number of connected sites
func (mh *MessageHandle) GetSiteCount() int {
	var num int
	iter := func(key, value interface{}) bool {
		num++
		return true
	}
	mh.Sites.Range(iter)
	return num
}

func (mh *MessageHandle) PubToController(siteID string, msg *beehiveModel.Message) error {
	msg.SetResourceOperation(fmt.Sprintf("site/%s/%s", siteID, msg.GetResource()), msg.GetOperation())
	klog.Infof("receive message from site %s, %s, content: %s", siteID, dumpMessageMetadata(msg), msg.GetContent())
	err := mh.MessageQueue.Publish(msg)
	if err != nil {
		klog.Errorf("failed to publish message for site %s, %s, reason: %s", siteID, dumpMessageMetadata(msg), err.Error())
	}
	return nil
}

// OnRegister register site on first connection
func (mh *MessageHandle) OnRegister(connection conn.Connection) {
	siteID := connection.ConnectionState().Headers.Get("siteID")

	if _, ok := mh.KeepaliveChannel.Load(siteID); !ok {
		mh.KeepaliveChannel.Store(siteID, make(chan struct{}, 1))
	}

	io := &hubio.JSONIO{Connection: connection}

	if _, ok := mh.siteRegistered.Load(siteID); ok {
		if conn, exist := mh.siteConns.Load(siteID); exist {
			conn.(hubio.CloudHubIO).Close()
		}
		mh.siteConns.Store(siteID, io)
		return
	}
	mh.siteConns.Store(siteID, io)
	go mh.ServeConn(siteID)
}

// ServeConn starts serving the incoming connection
func (mh *MessageHandle) ServeConn(siteID string) {
	err := mh.RegisterSite(siteID)
	if err != nil {
		klog.Errorf("fail to register site %s, reason %s", siteID, err.Error())
		return
	}

	klog.Infof("edge site %s connected", siteID)
	exitServe := make(chan ExitCode, 3)

	for _, handle := range mh.Handlers {
		go handle(siteID, exitServe)
	}

	code := <-exitServe
	mh.UnregisterSite(siteID, code)
}

// RegisterSite register site in cloudhub for the incoming connection
func (mh *MessageHandle) RegisterSite(siteID string) error {
	mh.MessageQueue.Connect(siteID)

	mh.siteLocks.Store(siteID, &sync.Mutex{})
	mh.Sites.Store(siteID, true)
	mh.siteRegistered.Store(siteID, true)
	return nil
}

// UnregisterSite unregister site in cloudhub
func (mh *MessageHandle) UnregisterSite(siteID string, code ExitCode) {
	if conn, exist := mh.siteConns.Load(siteID); exist {
		conn.(hubio.CloudHubIO).Close()
	}

	mh.siteLocks.Delete(siteID)
	mh.siteConns.Delete(siteID)
	mh.siteRegistered.Delete(siteID)
	siteKeepalive, ok := mh.KeepaliveChannel.Load(siteID)
	if !ok {
		klog.Errorf("fail to load site %s", siteID)
	} else {
		close(siteKeepalive.(chan struct{}))
		mh.KeepaliveChannel.Delete(siteID)
	}

	mh.Sites.Delete(siteID)

	// delete the siteQueue and siteStore when site stopped
	if code == siteStop {
		mh.MessageQueue.Close(siteID)
	}
}

// KeepaliveCheckLoop checks whether the edge site is still alive
func (mh *MessageHandle) KeepaliveCheckLoop(siteID string, stopServe chan ExitCode) {
	keepaliveTicker := time.NewTimer(time.Duration(mh.KeepaliveInterval) * time.Second)
	siteKeepaliveChannel, _ := mh.KeepaliveChannel.Load(siteID)

	for {
		select {
		case _, ok := <-siteKeepaliveChannel.(chan struct{}):
			if !ok {
				klog.Warningf("Stop keepalive check for site: %s", siteID)
				return
			}
			klog.Infof("site %s is still alive", siteID)
			keepaliveTicker.Reset(time.Duration(mh.KeepaliveInterval) * time.Second)
		case <-keepaliveTicker.C:
			if conn, ok := mh.siteConns.Load(siteID); ok {
				klog.Warningf("Timeout to receive heart beat from edge site %s", siteID)
				conn.(hubio.CloudHubIO).Close()
				mh.siteConns.Delete(siteID)
			}
		}
	}
}

func dumpMessageMetadata(msg *beehiveModel.Message) string {
	return fmt.Sprintf("id: %s, parent_id: %s, group: %s, source: %s, resource: %s, operation: %s",
		msg.GetID(), msg.GetParentID(), msg.GetGroup(), msg.GetSource(), msg.GetResource(), msg.GetOperation())
}

// MessageWriteLoop processes all write request, send message to edge from cloud
func (mh *MessageHandle) MessageWriteLoop(siteID string, stopServe chan ExitCode) {
	siteQueue := mh.MessageQueue.GetSiteQueue(siteID)
	siteStore := mh.MessageQueue.GetSiteStore(siteID)

	for {
		key, quit := siteQueue.Get()
		if quit {
			klog.Errorf("siteQueue for site %s has shutdown", siteID)
			return
		}
		obj, exist, _ := siteStore.GetByKey(key.(string))
		if !exist {
			klog.Errorf("siteStore for site %s doesn't exist", siteID)
			continue
		}
		msg := obj.(*beehiveModel.Message)
		klog.V(4).Infof("event to send for site %s, %s, content %s", siteID, dumpMessageMetadata(msg), msg.Content)

		// remove site/siteID from resource of message, then send to edge
		trimMessage(msg)
		conn, ok := mh.siteConns.Load(siteID)
		if !ok {
			continue
		}
		mh.send(conn.(hubio.CloudHubIO), siteID, msg)

		// delete successfully sent events from the queue/store
		siteStore.Delete(msg)
		siteQueue.Forget(key.(string))
		siteQueue.Done(key)
	}
}

func (mh *MessageHandle) send(hi hubio.CloudHubIO, siteID string, msg *beehiveModel.Message) error {
	err := mh.hubIoWrite(hi, siteID, msg)
	if err != nil {
		return err
	}
	return nil
}

func (mh *MessageHandle) hubIoWrite(hi hubio.CloudHubIO, siteID string, msg *beehiveModel.Message) error {
	value, ok := mh.siteLocks.Load(siteID)
	if !ok {
		return fmt.Errorf("site disconnected")
	}
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	return hi.WriteData(msg)
}

func trimMessage(msg *beehiveModel.Message) {
	resource := msg.GetResource()
	if strings.HasPrefix(resource, constants.ResSite) {
		tokens := strings.Split(resource, "/")
		if len(tokens) < 3 {
			klog.Warningf("event resource %s starts with site but length less than 3", resource)
		} else {
			msg.SetResourceOperation(strings.Join(tokens[2:], "/"), msg.GetOperation())
		}
	}
}
