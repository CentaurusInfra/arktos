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

package datapartition

import (
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"sync"
	"testing"
)

// DataPartitionConfigManager is singleton - make sure each test runs sequentially
var testLock sync.Mutex
var notifyTimes int

const (
	serviceGroupId1 = "1"
	serviceGroupId2 = "2"
)

func TestGetDataPartitionConfigManager(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()

	// test nil instance
	instance = nil
	handler := GetDataPartitionConfigManager()
	assert.Nil(t, handler)

	// test singleton
	instance = &DataPartitionConfigManager{}
	handler = GetDataPartitionConfigManager()
	assert.NotNil(t, handler)
	assert.Equal(t, instance, handler)
}

func setupInformer(client *fake.Clientset, stop chan struct{}) informers.SharedInformerFactory {
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	informerFactory.Start(stop)
	informerFactory.WaitForCacheSync(stop)
	return informerFactory
}

func mockNotifyHandler(newDp *v1.DataPartitionConfig) {
	notifyTimes++
}

func createDataPartition(serviceGroupId, rv string) *v1.DataPartitionConfig {
	dataPartitionConfig1 := &v1.DataPartitionConfig{
		ObjectMeta: k8s_io_apimachinery_pkg_apis_meta_v1.ObjectMeta{
			Name:            "partition-1",
			ResourceVersion: rv,
		},
		RangeStart:        "a",
		IsRangeStartValid: false,
		RangeEnd:          "m",
		IsRangeEndValid:   true,
		ServiceGroupId:    serviceGroupId,
	}

	return dataPartitionConfig1
}

func TestNewDataPartitionConfigManager(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()
	instance = nil

	stop := make(chan struct{})
	defer close(stop)
	client := fake.NewSimpleClientset()
	informer := setupInformer(client, stop)

	dataPartitionConfigManager := NewDataPartitionConfigManager(serviceGroupId1, informer.Core().V1().DataPartitionConfigs())
	assert.NotNil(t, dataPartitionConfigManager)

	// When there is no data partition data entry, default taking all data - initialized as false
	assert.False(t, dataPartitionConfigManager.isDataPartitionInitialized)
	assert.Equal(t, serviceGroupId1, dataPartitionConfigManager.ServiceGroupId)
	assert.Equal(t, false, dataPartitionConfigManager.DataPartitionConfig.IsRangeStartValid)
	assert.Equal(t, false, dataPartitionConfigManager.DataPartitionConfig.IsRangeEndValid)
	assert.NotNil(t, dataPartitionConfigManager.updateChGrp)
}

func TestAddDataPartition(t *testing.T) {
	// test normal add event
	manager := &DataPartitionConfigManager{
		ServiceGroupId: serviceGroupId1,
	}
	manager.notifyHandler = mockNotifyHandler
	dataPartitionConfig1 := createDataPartition(serviceGroupId1, "100")

	oldNotifiedTimes := notifyTimes
	manager.addDataPartition(dataPartitionConfig1)
	assert.Equal(t, oldNotifiedTimes+1, notifyTimes)
	assert.True(t, manager.isDataPartitionInitialized)
	assert.Equal(t, serviceGroupId1, manager.ServiceGroupId)
	assert.Equal(t, dataPartitionConfig1.RangeStart, manager.DataPartitionConfig.RangeStart)
	assert.Equal(t, dataPartitionConfig1.RangeEnd, manager.DataPartitionConfig.RangeEnd)
	assert.Equal(t, dataPartitionConfig1.IsRangeStartValid, manager.DataPartitionConfig.IsRangeStartValid)
	assert.Equal(t, dataPartitionConfig1.IsRangeEndValid, manager.DataPartitionConfig.IsRangeEndValid)

	// test resend add event - ignore, no notification
	oldNotifiedTimes = notifyTimes
	manager.addDataPartition(dataPartitionConfig1)
	assert.Equal(t, oldNotifiedTimes, notifyTimes)
}

func TestUpdateDataPartition(t *testing.T) {
	// test ignore older revision
	manager := &DataPartitionConfigManager{
		ServiceGroupId: serviceGroupId1,
	}
	manager.notifyHandler = mockNotifyHandler
	dataPartitionConfig1 := createDataPartition(serviceGroupId1, "100")
	dataPartitionConfig2 := createDataPartition(serviceGroupId1, "99")
	oldNotifiedTimes := notifyTimes
	manager.updateDataPartition(dataPartitionConfig1, dataPartitionConfig2)
	assert.Equal(t, oldNotifiedTimes, notifyTimes)

	// test ignore different service group id
	manager.addDataPartition(dataPartitionConfig1)
	oldNotifiedTimes = notifyTimes
	dataPartitionConfig3 := createDataPartition(serviceGroupId2, "101")
	manager.updateDataPartition(dataPartitionConfig1, dataPartitionConfig3)
	assert.Equal(t, oldNotifiedTimes, notifyTimes)

	// test update
	dataPartitionConfig1_1 := createDataPartition(serviceGroupId1, "101")
	dataPartitionConfig1_1.IsRangeEndValid = !dataPartitionConfig1.IsRangeEndValid
	oldNotifiedTimes = notifyTimes
	manager.updateDataPartition(dataPartitionConfig1, dataPartitionConfig1_1)
	assert.Equal(t, oldNotifiedTimes+1, notifyTimes)
	assert.Equal(t, dataPartitionConfig1_1.IsRangeEndValid, manager.DataPartitionConfig.IsRangeEndValid)
}
