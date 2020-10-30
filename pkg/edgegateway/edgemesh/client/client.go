package client

import (
	"flag"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/edgegateway/edgemesh/config"
)

func NewClient() *websocket.Conn {
	var addr = flag.String("addr", config.Config.Server, "websocket client address")
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/stream"}
	var requestHeader http.Header
	requestHeader = make(map[string][]string)
	requestHeader.Add("tap", config.Config.TapName)
	c, _, err := websocket.DefaultDialer.Dial(u.String(), requestHeader)
	if err != nil {
		klog.Errorf("failed to get websocket connection, error: %v", err)
		return nil
	}
	klog.Infof("success to get websocket connection")
	return c
}
