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
	"fmt"
	"github.com/grafov/bcast"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"strconv"
	"sync"
)

var instance *DataPartitionConfigManager
var checkInstanceHandler = checkInstanceExistence

type DataPartitionConfigManager struct {
	partitionListerSynced cache.InformerSynced
	partitionLister       corelisters.DataPartitionConfigLister

	isDataPartitionInitialized bool
	DataPartitionConfig        v1.DataPartitionConfig
	rev                        int64
	ServiceGroupId             string
	updateChGrp                *bcast.Group

	mux           sync.Mutex
	notifyHandler func(newDp *v1.DataPartitionConfig)
}

func GetDataPartitionConfigManager() *DataPartitionConfigManager {
	return instance
}

func checkInstanceExistence() {
	if instance != nil {
		klog.Fatalf("Unexpected reference to data partition manager - initialized")
	}
}

func NewDataPartitionConfigManager(serviceGroupId string, dpInformer coreinformers.DataPartitionConfigInformer) *DataPartitionConfigManager {
	checkInstanceHandler()

	manager := &DataPartitionConfigManager{
		ServiceGroupId:             serviceGroupId,
		isDataPartitionInitialized: false,
		updateChGrp:                GetDataPartitionUpdateChGrp(),
	}
	manager.notifyHandler = manager.notifyDataPartitionChanges

	dpInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    manager.addDataPartition,
		UpdateFunc: manager.updateDataPartition,
		DeleteFunc: manager.deleteDataPartition,
	})

	manager.partitionLister = dpInformer.Lister()
	manager.partitionListerSynced = dpInformer.Informer().HasSynced
	err := manager.syncDataPartition()
	if err != nil {
		klog.Fatalf("Unable to get data partitions from registry. Error %v", err)
	}

	instance = manager
	return instance
}

func (m *DataPartitionConfigManager) Run(stopCh <-chan struct{}) {
	klog.Infof("Starting data partition manager. service group id %v", m.ServiceGroupId)
	defer klog.Infof("Shutting down data partition manager. service group id %v", m.ServiceGroupId)

	if !cache.WaitForCacheSync(stopCh, m.partitionListerSynced) {
		klog.Infof("Data partitions NOT synced %v. service group id %v", m.partitionListerSynced, m.ServiceGroupId)
		return
	}

	klog.Infof("Caches are synced for data partition configs. service group id %s", m.ServiceGroupId)
	<-stopCh
	m.updateChGrp.Close()
}

func (m *DataPartitionConfigManager) syncDataPartition() error {
	m.mux.Lock()
	klog.V(4).Infof("mux acquired syncDataPartition. service group id %v", m.ServiceGroupId)
	defer func() {
		m.mux.Unlock()
		klog.V(4).Infof("mux released syncDataPartition. service group id %v", m.ServiceGroupId)
	}()

	paritions, err := m.partitionLister.List(labels.Everything())
	if err != nil {
		klog.Fatalf("Error in getting data partition configurations: %v", err)
	}
	klog.V(3).Infof("Api server data partitions all [%#v]", paritions)

	for _, partition := range paritions {
		if partition.ServiceGroupId == m.ServiceGroupId {
			m.DataPartitionConfig = *partition
			m.isDataPartitionInitialized = true
			klog.Infof("Current api server data partition [%#v]", m.DataPartitionConfig)
			break
		}
	}
	if !m.isDataPartitionInitialized {
		klog.V(3).Infof("Api server data partition not configured for api server group %s. Default to take all data", m.ServiceGroupId)
	}
	return nil
}

func (m *DataPartitionConfigManager) addDataPartition(obj interface{}) {
	dp := obj.(*v1.DataPartitionConfig)
	if dp.DeletionTimestamp != nil {
		return
	}
	klog.V(3).Infof("Received event for NEW data partition %+v.", dp)

	rev, err := strconv.ParseInt(dp.ResourceVersion, 10, 64)
	if err != nil {
		klog.Errorf("Got invalid resource version %s for data partition %v.", dp.ResourceVersion, dp)
		return
	}

	if dp.ServiceGroupId != "" && dp.ServiceGroupId == m.ServiceGroupId {
		if m.isDataPartitionInitialized {
			if m.DataPartitionConfig.ResourceVersion != dp.ResourceVersion {
				klog.Fatalf("Unexpected multiple data partitions for same service group id: %s. Existing data partition [%+v], new data partition [%+v]", m.ServiceGroupId, m.DataPartitionConfig, dp)
			}
			return
		}

		m.mux.Lock()
		klog.V(4).Infof("mux acquired addDataPartition. service group id %s", m.ServiceGroupId)
		defer func() {
			m.mux.Unlock()
			klog.V(4).Infof("mux released addDataPartition. service group id %s", m.ServiceGroupId)
		}()

		m.notifyHandler(dp)
		m.DataPartitionConfig = *dp
		m.isDataPartitionInitialized = true
		m.rev = rev
	}
}

func (m *DataPartitionConfigManager) updateDataPartition(old, cur interface{}) {
	curDp := cur.(*v1.DataPartitionConfig)
	oldDp := old.(*v1.DataPartitionConfig)

	if curDp.ResourceVersion == oldDp.ResourceVersion {
		return
	}

	oldRev, _ := strconv.ParseInt(oldDp.ResourceVersion, 10, 64)
	newRev, err := strconv.ParseInt(curDp.ResourceVersion, 10, 64)
	if err != nil {
		klog.Errorf("Got invalid resource version %s for data partition %v.", curDp.ResourceVersion, curDp)
		return
	}

	if newRev <= oldRev {
		klog.V(3).Infof("Got staled data partition %+v in UpdateFunc. Existing Version %s, new instance version %s.", curDp, oldDp.ResourceVersion, curDp.ResourceVersion)
		return
	}

	if curDp.ServiceGroupId != "" && curDp.ServiceGroupId == m.ServiceGroupId {
		m.mux.Lock()
		klog.V(4).Infof("mux acquired updateDataPartition. service group id %s", m.ServiceGroupId)
		defer func() {
			m.mux.Unlock()
			klog.V(4).Infof("mux released updateDataPartition. service group id %s", m.ServiceGroupId)
		}()

		if m.isDataPartitionInitialized && !isDataPartitionEqual(m.DataPartitionConfig, curDp) {
			m.notifyHandler(curDp)
		}

		m.DataPartitionConfig = *curDp
		m.isDataPartitionInitialized = true
		m.rev = newRev
	}
}

func (m *DataPartitionConfigManager) deleteDataPartition(obj interface{}) {
	dp, ok := obj.(*v1.DataPartitionConfig)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v.", obj))
			return
		}
		dp, ok = tombstone.Obj.(*v1.DataPartitionConfig)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a data partition %#v.", obj))
			return
		}
	}

	klog.V(3).Infof("Received delete event for data partition %+v", dp)
	if dp.ServiceGroupId != m.ServiceGroupId {
		return
	}
}

func (m *DataPartitionConfigManager) notifyDataPartitionChanges(newDp *v1.DataPartitionConfig) {
	go func() {
		m.updateChGrp.Send(*newDp)
	}()
}

func isDataPartitionEqual(dp1 v1.DataPartitionConfig, dp2 *v1.DataPartitionConfig) bool {
	if dp1.IsRangeEndValid != dp2.IsRangeEndValid || dp1.RangeEnd != dp2.RangeEnd || dp1.RangeStart != dp2.RangeStart || dp1.IsRangeStartValid != dp2.IsRangeStartValid {
		return false
	}

	return true
}
