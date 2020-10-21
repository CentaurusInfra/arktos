package edgemesh

import (
	"github.com/gorilla/websocket"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/edgegateway/edgemesh/config"
)

// send edge site message stream to cloud
func (em *edgeMesh) upstream() {
	buf := make([]byte, 65536)
	for {
		n, err := config.Config.TapInterface.Read(buf)
		if err != nil {
			klog.Errorf("failed to read tap0, error: %v", err)
			return
		}
		err = em.client.WriteMessage(websocket.BinaryMessage, buf[:n])
		if err != nil {
			klog.Errorf("failed to send message stream to cloud by websocket, error: %v", err)
			return
		}
		klog.Infof("success to send message stream to cloud")
	}
}

// send cloud message stream to edge service
func (em *edgeMesh) downstream() {

	for {
		_, buf, err := em.client.ReadMessage()
		if err != nil {
			klog.Errorf("read message error: %v", err)
			return
		}
		_, err = config.Config.TapInterface.Write(buf)
		if err != nil {
			klog.Errorf("failed to write message to tap0, error: %v", err)
			continue
		}
		klog.Infof("write message to tap0")
	}
}
