package server

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudmesh/config"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudmesh/taptun"
)

var tap map[string]*taptun.Interface

func StartCloudMesh() {
	tap = make(map[string]*taptun.Interface)
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

		tapName := r.Header.Get("tap")
		_, ok := tap[tapName]
		if !ok {
			tapInterface := getTapInterface(tapName)
			tap[tapName] = tapInterface
		}

		defer conn.Close()
		go downstream(conn, tapName)
		upstream(conn, tapName)
	}
	return handleFunc
}

// send cloud message stream to edge site
func downstream(connection *websocket.Conn, tapName string) {
	buf := make([]byte, 65536)
	tapInterface := tap[tapName]
	for {
		n, err := tapInterface.Read(buf)
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

// send edge site message stream to cloud service
func upstream(connection *websocket.Conn, tapName string) {
	tapInterface := tap[tapName]
	for {
		_, b, err := connection.ReadMessage()
		if err != nil {
			klog.Errorf("read message error: %v", err)
			return
		}
		_, err = tapInterface.Write(b)
		if err != nil {
			klog.Errorf("failed to write message to tap0, error: %v", err)
			continue
		}
		klog.Infof("write message to tap0")
	}
}

// get tap interface
func getTapInterface(tapName string) *taptun.Interface {
	tapInterface, err := taptun.OpenTAP(tapName)
	if err != nil {
		klog.Errorf("open tap failed", err)
		os.Exit(1)
	}
	return tapInterface
}
