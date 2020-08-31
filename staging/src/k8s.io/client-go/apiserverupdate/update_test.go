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
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog"
	"sync"
	"testing"
	"time"
)

const (
	masterIP1 = "192.168.1.1"
	masterIP2 = "192.168.1.2"

	serviceGroupId1 = "1"
	serviceGroupId2 = "2"
)

func TestGetAPIServerConfigUpdateChGrp(t *testing.T) {
	// get existing apiServerConfigUpdateChGrp
	apiServerConfigUpdateChGrp = bcast.NewGroup()
	go apiServerConfigUpdateChGrp.Broadcast(0)
	chGrp := GetAPIServerConfigUpdateChGrp()
	assert.NotNil(t, chGrp)
	assert.Equal(t, apiServerConfigUpdateChGrp, chGrp)

	// when apiServerConfigUpdateChGrp is nil, create new one
	apiServerConfigUpdateChGrp.Close()
	apiServerConfigUpdateChGrp = nil
	chGrp = GetAPIServerConfigUpdateChGrp()
	assert.NotNil(t, chGrp)
	assert.Equal(t, apiServerConfigUpdateChGrp, chGrp)

	// tear down
	apiServerConfigUpdateChGrp.Close()
	apiServerConfigUpdateChGrp = nil
}

func TestSetAPIServerConfig(t *testing.T) {
	t.Log("1. Test update argument later won't affect APIServerConfig in global")
	ssMap1 := make(map[string]v1.EndpointSubset)
	ssMap1[serviceGroupId1] = v1.EndpointSubset{
		Addresses:      []v1.EndpointAddress{{IP: masterIP1}},
		ServiceGroupId: serviceGroupId1,
	}

	SetAPIServerConfig(ssMap1)

	ssMap1[serviceGroupId1].Addresses[0].Hostname = "123"
	ssMap2 := GetAPIServerConfig()
	assert.NotNil(t, ssMap2)
	ss2, isOK := ssMap2[serviceGroupId1]
	assert.True(t, isOK)
	assert.NotNil(t, ss2)
	assert.Equal(t, serviceGroupId1, ss2.ServiceGroupId)
	assert.Equal(t, 1, len(ss2.Addresses))
	assert.Equal(t, masterIP1, ss2.Addresses[0].IP)
	assert.Equal(t, "", ss2.Addresses[0].Hostname)
}

func setAndReadAPIServerConfig(wg *sync.WaitGroup, epMap map[string]v1.EndpointSubset) {
	wg.Add(1)
	SetAPIServerConfig(epMap)
	readEPMap := GetAPIServerConfig()
	for sg, ss := range readEPMap {
		time.Sleep(10 * time.Millisecond)
		klog.V(6).Infof("Make sure read server group %s and endpoints %v", sg, ss)
	}
	wg.Done()
}

func TestConcurrentReadWriteAPIServerConfig(t *testing.T) {
	epMap1 := make(map[string]v1.EndpointSubset)
	epMap2 := make(map[string]v1.EndpointSubset)

	epMap1[serviceGroupId1] = v1.EndpointSubset{
		Addresses:      []v1.EndpointAddress{{IP: masterIP1}},
		ServiceGroupId: serviceGroupId1,
	}

	epMap2[serviceGroupId1] = v1.EndpointSubset{
		Addresses:      []v1.EndpointAddress{{IP: masterIP1}},
		ServiceGroupId: serviceGroupId1,
	}

	epMap2[serviceGroupId2] = v1.EndpointSubset{
		Addresses:      []v1.EndpointAddress{{IP: masterIP2}},
		ServiceGroupId: serviceGroupId2,
	}

	var wg sync.WaitGroup
	for j := 0; j < 10; j++ {
		for i := 0; i < 5000; i++ {
			go setAndReadAPIServerConfig(&wg, epMap1)
			go setAndReadAPIServerConfig(&wg, epMap2)
		}
		wg.Wait()
	}

	// final test
	readEPMap := GetAPIServerConfig()
	assert.NotNil(t, readEPMap)
	assert.True(t, len(readEPMap) <= 2)
	assert.Equal(t, 1, len(readEPMap[serviceGroupId1].Addresses))
	assert.Equal(t, masterIP1, readEPMap[serviceGroupId1].Addresses[0].IP)
}

func TestGetClientSetsWatcher(t *testing.T) {
	// test get existing clientSetWatcher
	clientSetWatcher = &ClientSetsWatcher{}
	cw := GetClientSetsWatcher()
	assert.NotNil(t, cw)
	assert.Equal(t, clientSetWatcher, cw)

	// when clientSetWatcher is nil, create new one
	clientSetWatcher = nil
	cw = GetClientSetsWatcher()
	assert.NotNil(t, cw)
	assert.Equal(t, clientSetWatcher, cw)

	// tear down
	clientSetWatcher = nil
}

func TestClientSetWatcherNotification(t *testing.T) {
	for j := 0; j < 10; j++ {
		csWatcher := &ClientSetsWatcher{}

		rand.Seed(time.Now().UnixNano())
		watcherCount := rand.IntnRange(10, 5000)
		for i := 0; i < watcherCount; i++ {
			WatchClientSetUpdate()
			csWatcher.AddWatcher()
		}

		csWatcher.StartWaitingForComplete()
		for i := 0; i < watcherCount; i++ {
			go csWatcher.NotifyDone()
		}

		tick := time.NewTicker(1 * time.Second)
		after := time.NewTimer(1 * time.Minute).C

	waitChannel:
		for {
			select {
			case <-tick.C:
				if csWatcher.waitingCount == 0 {
					break waitChannel
				}
			case <-after:
				break waitChannel
			}
		}

		assert.Equal(t, 0, csWatcher.waitingCount)
		// make sure all locks are released
		csWatcher.mux.Lock()
		csWatcher.mux.Unlock()
		csWatcher.muxStartWaiting.Lock()
		csWatcher.muxStartWaiting.Unlock()
		muxUpdateServerMap.Lock()
		muxUpdateServerMap.Unlock()
	}
}
