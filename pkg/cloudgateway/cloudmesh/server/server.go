package server

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudmesh/config"
)

func StartCloudMesh() {
	serverAddr := fmt.Sprintf("%s:%d", config.Config.Address, config.Config.Port)
	var addr = flag.String("addr", serverAddr, "websocket server address")
	http.HandleFunc("/stream", StreamHandler())
	klog.Fatal(http.ListenAndServe(*addr, nil))
}

func StreamHandler() func(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{}
	handleFunc := func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			klog.Errorf("upgrade: %v", err)
			return
		}

		defer conn.Close()
		go downstream(conn)
		upstream(conn)
	}
	return handleFunc
}

// send edge site message stream to cloud service
func upstream(connection *websocket.Conn) {
	buf := make([]byte, 65536)
	for {
		n, err := config.Config.TapInterface.Read(buf)
		if err != nil {
			klog.Errorf("failed to read tap0, error: %v", err)
			return
		}
		err = connection.WriteMessage(websocket.BinaryMessage, buf[:n])
		if err != nil {
			klog.Errorf("failed to send message stream to edge site by websocket, error: %v", err)
			return
		}
	}
}

// send cloud message stream to edge site
func downstream(connection *websocket.Conn) {
	for {
		_, b, err := connection.ReadMessage()
		if err != nil {
			klog.Errorf("read message error: %v", err)
			return
		}
		_, err = config.Config.TapInterface.Write(b)
		if err != nil {
			klog.Errorf("failed to write message to tap0, error: %v", err)
			continue
		}
		klog.Infof("write message to tap0")
	}
}
