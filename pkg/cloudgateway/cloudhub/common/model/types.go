package model

import (
	// Mapping value of json to struct member
	_ "encoding/json"
)

const (
	OpKeepalive = "keepalive"
	ResNode     = "node"
)

// HubInfo saves identifier information for edge hub
type HubInfo struct {
	ProjectID string
	NodeID    string
}
