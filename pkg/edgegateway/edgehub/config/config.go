package config

import (
	"strings"
	"sync"

	v1 "k8s.io/kubernetes/pkg/apis/edgegateway/v1"
)

var Config Configure
var once sync.Once

type Configure struct {
	v1.EdgeHub
	WebSocketURL string
	NodeName     string
}

func InitConfigure(eh *v1.EdgeHub, nodeName string) {
	once.Do(func() {
		Config = Configure{
			EdgeHub:      *eh,
			WebSocketURL: strings.Join([]string{"wss:/", eh.WebSocket.Server, eh.ProjectID, nodeName, "events"}, "/"),
			NodeName:     nodeName,
		}
	})
}
