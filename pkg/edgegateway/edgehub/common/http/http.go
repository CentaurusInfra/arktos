package http

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"time"

	"k8s.io/klog"
)

var (
	connectTimeout            = 30 * time.Second
	keepaliveTimeout          = 30 * time.Second
	responseReadTimeout       = 300 * time.Second
	maxIdleConnectionsPerHost = 3
)

// NewHTTPClient create new client
func NewHTTPClient() *http.Client {
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   connectTimeout,
			KeepAlive: keepaliveTimeout,
		}).Dial,
		MaxIdleConnsPerHost:   maxIdleConnectionsPerHost,
		ResponseHeaderTimeout: responseReadTimeout,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}
	klog.Infof("tlsConfig InsecureSkipVerify true")
	return &http.Client{Transport: transport}
}

// SendRequest create and send a HTTP request, and return the resp info
func SendRequest(client *http.Client, urlStr string, body io.Reader, method string) (*http.Response, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
