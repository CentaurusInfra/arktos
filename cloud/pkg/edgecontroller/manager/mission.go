package manager

import (
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/kubeedge/kubeedge/cloud/pkg/edgecontroller/config"
)

// MissionManager manage all events of rule by SharedInformer
type MissionManager struct {
	events chan watch.Event
}

// Events return the channel save events from watch secret change
func (rem *MissionManager) Events() chan watch.Event {
	return rem.events
}

// NewMissionManager create MissionManager by SharedIndexInformer
func NewMissionManager(si cache.SharedIndexInformer) (*MissionManager, error) {
	events := make(chan watch.Event, config.Config.Buffer.MissionsEvent)
	rh := NewCommonResourceEventHandler(events)
	si.AddEventHandler(rh)

	return &MissionManager{events: events}, nil
}
