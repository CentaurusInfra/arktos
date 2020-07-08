/*
Copyright 2020 Authors of Arktos.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storagecluster

import (
	"github.com/grafov/bcast"
	"sync"
)

var storageClusterUpdateChgrp *bcast.Group
var muxCreateStorageUpdateCh sync.Mutex

type StorageClusterAction struct {
	StorageClusterId uint8
	ServerAddresses  []string
	// Action can be
	// ADD - added a new storage cluster
	/// UPDATE - update existing cluster address
	//  DELETE - delete a storage cluster
	Action string
}

func getStorageClusterUpdateChgrp() *bcast.Group {
	if storageClusterUpdateChgrp != nil {
		return storageClusterUpdateChgrp
	}

	muxCreateStorageUpdateCh.Lock()
	defer muxCreateStorageUpdateCh.Unlock()
	if storageClusterUpdateChgrp != nil {
		return storageClusterUpdateChgrp
	}

	storageClusterUpdateChgrp = bcast.NewGroup()
	go storageClusterUpdateChgrp.Broadcast(0)
	return storageClusterUpdateChgrp
}

func WatchStorageClusterUpdate() *bcast.Member {
	return getStorageClusterUpdateChgrp().Join()
}

func SendStorageClusterUpdate(action StorageClusterAction) {
	getStorageClusterUpdateChgrp().Send(action)
}
