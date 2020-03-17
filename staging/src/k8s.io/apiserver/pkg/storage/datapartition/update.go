package datapartition

import (
	"github.com/grafov/bcast"
	"sync"
)

var datapartitionUpdateChGrp *bcast.Group
var mux sync.Mutex

func GetDataPartitionUpdateChGrp() *bcast.Group {
	if datapartitionUpdateChGrp != nil {
		return datapartitionUpdateChGrp
	}

	mux.Lock()
	if datapartitionUpdateChGrp != nil {
		return datapartitionUpdateChGrp
	}

	if datapartitionUpdateChGrp == nil {
		datapartitionUpdateChGrp = bcast.NewGroup()
		go datapartitionUpdateChGrp.Broadcast(0)
	}
	mux.Unlock()

	return datapartitionUpdateChGrp
}
