package handler

import (
	"sync"
	"time"

	"github.com/kubeedge/viaduct/pkg/conn"
	"github.com/kubeedge/viaduct/pkg/mux"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudhub/channelq"
	hubio "k8s.io/kubernetes/pkg/cloudgateway/cloudhub/common/io"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudhub/common/model"
	hubconfig "k8s.io/kubernetes/pkg/cloudgateway/cloudhub/config"
)

// ExitCode exit code
type ExitCode int

// MessageHandle processes messages between cloud and edge
type MessageHandle struct {
	KeepaliveInterval int
	WriteTimeout      int
	Nodes             sync.Map
	nodeConns         sync.Map
	nodeLocks         sync.Map
	nodeRegistered    sync.Map
	MessageQueue      *channelq.ChannelMessageQueue
	Handlers          []HandleFunc
	NodeLimit         int
	KeepaliveChannel  sync.Map
	MessageAcks       sync.Map
}

type HandleFunc func(info *model.HubInfo, exitServe chan ExitCode)

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
			NodeLimit:         int(hubconfig.Config.NodeLimit),
		}

		CloudhubHandler.Handlers = []HandleFunc{
			CloudhubHandler.KeepaliveCheckLoop,
			// TODO(ZongbaoLiu):
			// CloudhubHandler.MessageWriteLoop,
			// CloudhubHandler.ListMessageWriteLoop,
		}

		CloudhubHandler.initServerEntries()
	})
}

// initServerEntries register handler func
func (mh *MessageHandle) initServerEntries() {
	mux.Entry(mux.NewPattern("*").Op("*"), mh.HandleServer)
}

// HandleServer handle all the request from node
func (mh *MessageHandle) HandleServer(container *mux.MessageContainer, writer mux.ResponseWriter) {

}

// OnRegister register node on first connection
func (mh *MessageHandle) OnRegister(connection conn.Connection) {

}

// KeepaliveCheckLoop checks whether the edge node is still alive
func (mh *MessageHandle) KeepaliveCheckLoop(info *model.HubInfo, stopServe chan ExitCode) {
	keepaliveTicker := time.NewTimer(time.Duration(mh.KeepaliveInterval) * time.Second)
	nodeKeepaliveChannel, _ := mh.KeepaliveChannel.Load(info.NodeID)

	for {
		select {
		case _, ok := <-nodeKeepaliveChannel.(chan struct{}):
			if !ok {
				klog.Warningf("Stop keepalive check for node: %s", info.NodeID)
				return
			}
			klog.Infof("Node %s is still alive", info.NodeID)
			keepaliveTicker.Reset(time.Duration(mh.KeepaliveInterval) * time.Second)
		case <-keepaliveTicker.C:
			if conn, ok := mh.nodeConns.Load(info.NodeID); ok {
				klog.Warningf("Timeout to receive heart beat from edge node %s for project %s", info.NodeID, info.ProjectID)
				conn.(hubio.CloudHubIO).Close()
				mh.nodeConns.Delete(info.NodeID)
			}
		}
	}
}
