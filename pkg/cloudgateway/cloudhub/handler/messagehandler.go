package handler

import (
	"fmt"
	"strings"
	"sync"
	"time"

	beehiveModel "github.com/kubeedge/beehive/pkg/core/model"
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

const nodeStop ExitCode = iota

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
			CloudhubHandler.MessageWriteLoop,
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
	nodeID := container.Header.Get("node_id")

	if mh.GetNodeCount() >= mh.NodeLimit {
		klog.Errorf("Fail to serve node %s, reach node limit", nodeID)
		return
	}

	// receive heartbeat from edge
	if container.Message.GetOperation() == model.OpKeepalive {
		klog.Infof("Keepalive message received from node: %s", nodeID)

		nodeKeepalive, ok := mh.KeepaliveChannel.Load(nodeID)
		if !ok {
			klog.Errorf("Failed to load node : %s", nodeID)
			return
		}
		nodeKeepalive.(chan struct{}) <- struct{}{}
		return
	}

	// handle message from edge
	err := mh.PubToController(nodeID, container.Message)
	if err != nil {
		klog.Errorf("")
	}
}

// GetNodeCount returns the number of connected Nodes
func (mh *MessageHandle) GetNodeCount() int {
	var num int
	iter := func(key, value interface{}) bool {
		num++
		return true
	}
	mh.Nodes.Range(iter)
	return num
}

func (mh *MessageHandle) PubToController(nodeID string, msg *beehiveModel.Message) error {
	msg.SetResourceOperation(fmt.Sprintf("node/%s/%s", nodeID, msg.GetResource()), msg.GetOperation())
	klog.Infof("receive message from node %s, %s, content: %s", nodeID, dumpMessageMetadata(msg), msg.GetContent())
	err := mh.MessageQueue.Publish(msg)
	if err != nil {
		klog.Errorf("failed to publish message for node %s, %s, reason: %s", nodeID, dumpMessageMetadata(msg), err.Error())
	}
	return nil
}

// OnRegister register node on first connection
func (mh *MessageHandle) OnRegister(connection conn.Connection) {
	nodeID := connection.ConnectionState().Headers.Get("node_id")
	projectID := connection.ConnectionState().Headers.Get("project_id")

	if _, ok := mh.KeepaliveChannel.Load(nodeID); !ok {
		mh.KeepaliveChannel.Store(nodeID, make(chan struct{}, 1))
	}

	io := &hubio.JSONIO{Connection: connection}

	if _, ok := mh.nodeRegistered.Load(nodeID); ok {
		if conn, exist := mh.nodeConns.Load(nodeID); exist {
			conn.(hubio.CloudHubIO).Close()
		}
		mh.nodeConns.Store(nodeID, io)
		return
	}
	mh.nodeConns.Store(nodeID, io)
	go mh.ServeConn(&model.HubInfo{ProjectID: projectID, NodeID: nodeID})
}

// ServeConn starts serving the incoming connection
func (mh *MessageHandle) ServeConn(info *model.HubInfo) {
	err := mh.RegisterNode(info)
	if err != nil {
		klog.Errorf("fail to register node %s, reason %s", info.NodeID, err.Error())
		return
	}

	klog.Infof("edge node %s for project %s connected", info.NodeID, info.ProjectID)
	exitServe := make(chan ExitCode, 3)

	for _, handle := range mh.Handlers {
		go handle(info, exitServe)
	}

	code := <-exitServe
	mh.UnregisterNode(info, code)
}

// RegisterNode register node in cloudhub for the incoming connection
func (mh *MessageHandle) RegisterNode(info *model.HubInfo) error {
	mh.MessageQueue.Connect(info)

	mh.nodeLocks.Store(info.NodeID, &sync.Mutex{})
	mh.Nodes.Store(info.NodeID, true)
	mh.nodeRegistered.Store(info.NodeID, true)
	return nil
}

// UnregisterNode unregister node in cloudhub
func (mh *MessageHandle) UnregisterNode(info *model.HubInfo, code ExitCode) {
	if conn, exist := mh.nodeConns.Load(info.NodeID); exist {
		conn.(hubio.CloudHubIO).Close()
	}

	mh.nodeLocks.Delete(info.NodeID)
	mh.nodeConns.Delete(info.NodeID)
	mh.nodeRegistered.Delete(info.NodeID)
	nodeKeepalive, ok := mh.KeepaliveChannel.Load(info.NodeID)
	if !ok {
		klog.Errorf("fail to load node %s", info.NodeID)
	} else {
		close(nodeKeepalive.(chan struct{}))
		mh.KeepaliveChannel.Delete(info.NodeID)
	}

	mh.Nodes.Delete(info.NodeID)

	// delete the nodeQueue and nodeStore when node stopped
	if code == nodeStop {
		mh.MessageQueue.Close(info)
	}
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
				klog.Warningf("Timeout to receive heart beat from edge node %s", info.NodeID)
				conn.(hubio.CloudHubIO).Close()
				mh.nodeConns.Delete(info.NodeID)
			}
		}
	}
}

func dumpMessageMetadata(msg *beehiveModel.Message) string {
	return fmt.Sprintf("id: %s, parent_id: %s, group: %s, source: %s, resource: %s, operation: %s",
		msg.GetID(), msg.GetParentID(), msg.GetGroup(), msg.GetSource(), msg.GetResource(), msg.GetOperation())
}

// MessageWriteLoop processes all write request, send message to edge from cloud
func (mh *MessageHandle) MessageWriteLoop(info *model.HubInfo, stopServe chan ExitCode) {
	nodeQueue := mh.MessageQueue.GetNodeQueue(info.NodeID)
	nodeStore := mh.MessageQueue.GetNodeStore(info.NodeID)

	for {
		key, quit := nodeQueue.Get()
		if quit {
			klog.Errorf("nodeQueue for node %s has shutdown", info.NodeID)
			return
		}
		obj, exist, _ := nodeStore.GetByKey(key.(string))
		if !exist {
			klog.Errorf("nodeStore for node %s doesn't exist", info.NodeID)
			continue
		}
		msg := obj.(*beehiveModel.Message)
		klog.V(4).Infof("event to send for node %s, %s, content %s", info.NodeID, dumpMessageMetadata(msg), msg.Content)

		// remove node/nodeID from resource of message, then send to edge
		trimMessage(msg)
		conn, ok := mh.nodeConns.Load(info.NodeID)
		if !ok {
			continue
		}
		mh.send(conn.(hubio.CloudHubIO), info.NodeID, msg)

		// delete successfully sent events from the queue/store
		nodeStore.Delete(msg)
		nodeQueue.Forget(key.(string))
		nodeQueue.Done(key)
	}
}

func (mh *MessageHandle) send(hi hubio.CloudHubIO, nodeID string, msg *beehiveModel.Message) error {
	err := mh.hubIoWrite(hi, nodeID, msg)
	if err != nil {
		return err
	}
	return nil
}

func (mh *MessageHandle) hubIoWrite(hi hubio.CloudHubIO, nodeID string, msg *beehiveModel.Message) error {
	value, ok := mh.nodeLocks.Load(nodeID)
	if !ok {
		return fmt.Errorf("node disconnected")
	}
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	return hi.WriteData(msg)
}

func trimMessage(msg *beehiveModel.Message) {
	resource := msg.GetResource()
	if strings.HasPrefix(resource, model.ResNode) {
		tokens := strings.Split(resource, "/")
		if len(tokens) < 3 {
			klog.Warningf("event resource %s starts with node but length less than 3", resource)
		} else {
			msg.SetResourceOperation(strings.Join(tokens[2:], "/"), msg.GetOperation())
		}
	}
}
