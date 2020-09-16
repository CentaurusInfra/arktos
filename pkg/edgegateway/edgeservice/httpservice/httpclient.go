package httpservice

import (
	"bytes"
	"net/http"
	"time"
)

func SendWithHTTP(method, url string, body []byte) (*http.Response, error){
	var client = new(http.Client)
	client.Timeout = time.Second * 10

	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
