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

package apiserverupdate

import (
	"github.com/grafov/bcast"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sync"
)

/*
Start up and reset process:
. Client (Workload controller manager, scheduler, kubelet, kubectl, etc.) has initial KubeConfig file in local,
	which has one ip address (or dns) to allow it connect to one api server
. Client use local KubeConfig file to start and connect to one api server, during the start up process, the following
	components are initialized in one process:
	ClientConfig, Clientsets, watch connections, API Server Config Manager, Clientset update watcher.

Those components interact as below:

. API Server config manager: list/watch all api server endpoints from storage, send update notification to ClientConfig,
	and start clientset update watcher.
. ClientConfig: upon update message received from API Server config manager, update its KubeConfig definition; afterwards,
	notify clientsets that are created from the clientConfig.
. ClientSets: create clientsets from new config. Notify clienset update watcher when the updates was completed
. Clientset update watcher: when the last clientset is updated, send notification to ListAndWatch to reset the watchers

*/

var apiServerConfigUpdateChGrp *bcast.Group
var muxCreateApiServerConfigUpdateChGrp sync.Mutex

var apiServerMap map[string]v1.EndpointSubset
var muxUpdateServerMap sync.RWMutex

func GetAPIServerConfigUpdateChGrp() *bcast.Group {
	if apiServerConfigUpdateChGrp != nil {
		return apiServerConfigUpdateChGrp
	}

	muxCreateApiServerConfigUpdateChGrp.Lock()
	defer muxCreateApiServerConfigUpdateChGrp.Unlock()
	if apiServerConfigUpdateChGrp != nil {
		return apiServerConfigUpdateChGrp
	}

	apiServerConfigUpdateChGrp = bcast.NewGroup()
	go apiServerConfigUpdateChGrp.Broadcast(0)

	return apiServerConfigUpdateChGrp
}

func GetAPIServerConfig() map[string]v1.EndpointSubset {
	muxUpdateServerMap.RLock()
	copyApiServerMap := make(map[string]v1.EndpointSubset, len(apiServerMap))
	for k, v := range apiServerMap {
		copyApiServerMap[k] = *v.DeepCopy()
	}

	muxUpdateServerMap.RUnlock()
	return copyApiServerMap
}

func SetAPIServerConfig(c map[string]v1.EndpointSubset) {
	klog.V(2).Infof("Update APIServer Config from [%+v] to [%+v]", apiServerMap, c)
	muxUpdateServerMap.Lock()
	// map is passing as reference. Needs to copy manually
	apiServerMap = make(map[string]v1.EndpointSubset)
	for k, v := range c {
		apiServerMap[k] = *v.DeepCopy()
	}
	muxUpdateServerMap.Unlock()
}

var clientsetUpdateChGrp *bcast.Group
var muxClientSetUpdateChGrp sync.Mutex

func WatchClientSetUpdate() *bcast.Member {
	if clientsetUpdateChGrp != nil {
		return clientsetUpdateChGrp.Join()
	}

	muxClientSetUpdateChGrp.Lock()
	defer muxClientSetUpdateChGrp.Unlock()
	if clientsetUpdateChGrp != nil {
		return clientsetUpdateChGrp.Join()
	}

	clientsetUpdateChGrp = bcast.NewGroup()
	go clientsetUpdateChGrp.Broadcast(0)
	return clientsetUpdateChGrp.Join()
}

var muxCreateClientSetWatcher sync.Mutex
var clientSetWatcher *ClientSetsWatcher

func GetClientSetsWatcher() *ClientSetsWatcher {
	if clientSetWatcher != nil {
		return clientSetWatcher
	}

	muxCreateClientSetWatcher.Lock()
	defer muxCreateClientSetWatcher.Unlock()
	if clientSetWatcher != nil {
		return clientSetWatcher
	}

	clientSetWatcher = &ClientSetsWatcher{
		watcherCount: 0,
		waitingCount: 0,
	}

	return clientSetWatcher
}

type ClientSetsWatcher struct {
	watcherCount    int
	waitingCount    int
	mux             sync.Mutex
	muxStartWaiting sync.Mutex
}

func (w *ClientSetsWatcher) AddWatcher() {
	w.mux.Lock()
	w.watcherCount++
	klog.V(6).Infof("ClientSetsWatcher: Current watcher for clientset updates %v", w.watcherCount)
	w.mux.Unlock()
}

func (w *ClientSetsWatcher) StartWaitingForComplete() {
	w.muxStartWaiting.Lock()
	muxUpdateServerMap.Lock()
	klog.Infof("ClientSetsWatcher: Started waiting for clientset update complete. current watcher %d. muxStartWaiting and muxUpdateServerMap are locked", w.watcherCount)
	w.waitingCount = w.watcherCount
}

func (w *ClientSetsWatcher) NotifyDone() {
	w.mux.Lock()
	defer w.mux.Unlock()
	if w.waitingCount == 1 {
		w.waitingCount--
		// waiting done
		muxUpdateServerMap.Unlock()
		w.muxStartWaiting.Unlock()
		clientsetUpdateChGrp.Send("all clientset update done")
		klog.V(3).Info("ClientSetsWatcher: Sent complete message after all clientset update was done. muxStartWaiting and muxUpdateServerMap are unlocked")
		return
	}

	w.waitingCount--
	klog.V(6).Infof("ClientSetsWatcher: current waitingCount %v", w.waitingCount)
}
