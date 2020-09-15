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
}

func InitConfigure(eh *v1.EdgeHub) {
	once.Do(func() {
		Config = Configure{
			EdgeHub:      *eh,
			WebSocketURL: strings.Join([]string{"wss:/", eh.WebSocket.Server, eh.SiteID, "events"}, "/"),
		}
	})
}
