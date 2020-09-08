package clients

import (
	"fmt"
	"time"

	"k8s.io/kubernetes/pkg/edgegateway/edgehub/clients/quicclient"
	"k8s.io/kubernetes/pkg/edgegateway/edgehub/clients/wsclient"
	"k8s.io/kubernetes/pkg/edgegateway/edgehub/config"
)

//GetClient returns an Adapter object with new web socket
func GetClient() (Adapter, error) {
	config := config.Config
	switch {
	case config.WebSocket.Enable:
		websocketConf := wsclient.WebSocketConfig{
			URL:              config.WebSocketURL,
			CertFilePath:     config.TLSCertFile,
			KeyFilePath:      config.TLSPrivateKeyFile,
			HandshakeTimeout: time.Duration(config.WebSocket.HandshakeTimeout) * time.Second,
			ReadDeadline:     time.Duration(config.WebSocket.ReadDeadline) * time.Second,
			WriteDeadline:    time.Duration(config.WebSocket.WriteDeadline) * time.Second,
			ProjectID:        config.ProjectID,
			NodeID:           config.Hostname,
		}
		return wsclient.NewWebSocketClient(&websocketConf), nil
	case config.Quic.Enable:
		quicConfig := quicclient.QuicConfig{
			Addr:             config.Quic.Server,
			CaFilePath:       config.TLSCAFile,
			CertFilePath:     config.TLSCertFile,
			KeyFilePath:      config.TLSPrivateKeyFile,
			HandshakeTimeout: time.Duration(config.Quic.HandshakeTimeout) * time.Second,
			ReadDeadline:     time.Duration(config.Quic.ReadDeadline) * time.Second,
			WriteDeadline:    time.Duration(config.Quic.WriteDeadline) * time.Second,
			ProjectID:        config.ProjectID,
			NodeID:           config.Hostname,
		}
		return quicclient.NewQuicClient(&quicConfig), nil
	}

	return nil, fmt.Errorf("websocket and Quic are both disabled")
}
