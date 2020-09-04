package model

import (
	// Mapping value of json to struct member
	_ "encoding/json"
)

// HubInfo saves identifier information for edge hub
type HubInfo struct {
	ProjectID string
	NodeID    string
}
