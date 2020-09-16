package httpservice

import "net/http"

// HTTPRequest is HTTP request structure
type HTTPRequest struct {
	Header http.Header `json:"header"`
	Body   []byte      `json:"body"`
}

// HTTPResponse is HTTP request's response structure
type HTTPResponse struct {
	Header     http.Header `json:"header"`
	StatusCode int         `json:"status_code"`
	Body       []byte      `json:"body"`
}
