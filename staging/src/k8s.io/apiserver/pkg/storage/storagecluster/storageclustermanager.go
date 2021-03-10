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
	"errors"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/diff"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"strconv"
	"sync"
)

var instanceStorageClusterManager *StorageClusterManager
var checkInstanceHandler = checkStorageClusterInstanceExistence

type StorageClusterManager struct {
	storageClusterListerSynced cache.InformerSynced
	storageClusterLister       corelisters.StorageClusterLister

	backendClusters map[uint8]*v1.StorageCluster

	mux sync.Mutex
	rev int64
}

func GetStorageClusterManager() *StorageClusterManager {
	return instanceStorageClusterManager
}

func checkStorageClusterInstanceExistence() {
	if instanceStorageClusterManager != nil {
		klog.Fatalf("Unexpected reference to storage cluster manager - initialized")
	}
}

func NewStorageClusterManager(scInformer coreinformers.StorageClusterInformer) *StorageClusterManager {
	checkStorageClusterInstanceExistence()

	manager := &StorageClusterManager{
		rev:             int64(0),
		backendClusters: make(map[uint8]*v1.StorageCluster),
	}

	scInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    manager.addCluster,
		UpdateFunc: manager.updateCluster,
		DeleteFunc: manager.deleteCluster,
	})

	manager.storageClusterLister = scInformer.Lister()
	manager.storageClusterListerSynced = scInformer.Informer().HasSynced
	err := manager.syncClusters()
	if err != nil {
		klog.Fatalf("Unable to get storage clusters from registry. Error %v", err)
	}

	instanceStorageClusterManager = manager
	return instanceStorageClusterManager
}

func (s *StorageClusterManager) Run(stopCh <-chan struct{}) {
	klog.Infof("Starting storage cluster manager.")
	defer klog.Infof("Shutting down storage cluster manager.")

	if !cache.WaitForCacheSync(stopCh, s.storageClusterListerSynced) {
		klog.Infof("Storage cluster NOT synced %v.", s.storageClusterListerSynced)
		return
	}

	klog.Infof("Caches are synced for storage cluster configs.")
	<-stopCh
}

func (s *StorageClusterManager) syncClusters() error {
	s.mux.Lock()
	klog.V(4).Infof("mux acquired syncClusters.")
	defer func() {
		s.mux.Unlock()
		klog.V(4).Infof("mux released syncClusters.")
	}()

	clusters, err := s.storageClusterLister.List(labels.Everything())
	if err != nil {
		klog.Fatalf("Error in getting storage cluster configurations: %v", err)
	}
	klog.V(3).Infof("Backend storage clusters all [%#v]", clusters)

	aggErr := []error{}
	for _, cluster := range clusters {
		clusterId, err := diff.GetClusterIdFromString(cluster.StorageClusterId)
		if err != nil {
			klog.Fatalf("Invalid storage cluster id %v", cluster.StorageClusterId)
		}

		s.backendClusters[clusterId] = cluster
		newRev, err := strconv.ParseInt(cluster.ResourceVersion, 10, 64)
		if err != nil {
			aggErr = append(aggErr, errors.New(fmt.Sprintf("Got invalid resource version %s for storage cluster %v.", cluster.ResourceVersion, cluster)))
		} else if s.rev < newRev {
			s.rev = newRev
		}
	}

	for clusterId := range s.backendClusters {
		s.updateBackendStorageConfig(clusterId, "ADD")
	}

	return utilerrors.NewAggregate(aggErr)
}

func (s *StorageClusterManager) addCluster(obj interface{}) {
	c := obj.(*v1.StorageCluster)
	if c.DeletionTimestamp != nil {
		return
	}

	klog.Infof("Received event for NEW storage cluster %+v", c)

	rev, err := strconv.ParseInt(c.ResourceVersion, 10, 64)
	if err != nil {
		klog.Errorf("Got invalid resource version %s for storage cluster %v.", c.ResourceVersion, c)
		return
	}

	if diff.RevisionIsNewer(uint64(s.rev), uint64(rev)) {
		klog.V(3).Infof("Received duplicated storage cluster add event. Ignore [%v]. Current rev %v.", c, s.rev)
		return
	}
	clusterId, err := diff.GetClusterIdFromString(c.StorageClusterId)
	if err != nil {
		klog.Fatalf("Invalid storage cluster id %v", c.StorageClusterId)
	}

	existedCluster, isOK := s.backendClusters[clusterId]
	if isOK {
		klog.Errorf("Storage cluster id %s already used in %s. Skipping now", c.StorageClusterId, existedCluster.Name)
		return
	}

	s.mux.Lock()
	klog.V(4).Infof("mux acquired addCluster.")
	s.backendClusters[clusterId] = c
	s.updateBackendStorageConfig(clusterId, "ADD")

	s.rev = rev
	s.mux.Unlock()
	klog.V(4).Infof("mux released addCluster.")
}

func (s *StorageClusterManager) updateCluster(old, cur interface{}) {
	curCluster := cur.(*v1.StorageCluster)
	oldCluster := old.(*v1.StorageCluster)

	if curCluster.ResourceVersion == oldCluster.ResourceVersion {
		return
	}

	oldRev, _ := strconv.ParseInt(oldCluster.ResourceVersion, 10, 64)
	newRev, err := strconv.ParseInt(curCluster.ResourceVersion, 10, 64)
	if err != nil {
		klog.Errorf("Got invalid resource version %s for storage cluster %v.", curCluster.ResourceVersion, curCluster)
		return
	}

	if diff.RevisionIsNewer(uint64(oldRev), uint64(newRev)) {
		klog.V(2).Infof("Got staled storage cluster %+v in UpdateFunc. Existing Version %s, new instance version %s.", curCluster, oldCluster.ResourceVersion, curCluster.ResourceVersion)
		return
	}

	if curCluster.StorageClusterId != "" && curCluster.StorageClusterId == oldCluster.StorageClusterId {
		klog.Infof("Received event for update storage cluster. New cluster: [%+v]. Old cluster [%+v]", curCluster, oldCluster)

		// Just update record, no need to notify watcher
		s.mux.Lock()
		klog.V(4).Infof("mux acquired updateCluster.")
		defer func() {
			s.mux.Unlock()
			klog.V(4).Infof("mux released updateCluster.")
		}()

		clusterId, err := diff.GetClusterIdFromString(curCluster.StorageClusterId)
		if err != nil {
			klog.Fatalf("Invalid storage cluster id %v", clusterId)
		}

		if curCluster.ServiceAddress != oldCluster.ServiceAddress {
			go s.updateBackendStorageConfig(clusterId, "UPDATE")
		}

		s.backendClusters[clusterId] = curCluster
		s.rev = newRev

	} else {
		klog.Errorf("Got unexpected storage cluster update. Skipped updating. New cluster: [%+v]. Old cluster [%+v]", curCluster, oldCluster)
	}
}

func (s *StorageClusterManager) deleteCluster(obj interface{}) {
	cluster, ok := obj.(*v1.StorageCluster)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v.", obj))
			return
		}
		cluster, ok = tombstone.Obj.(*v1.StorageCluster)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a storage cluster %#v.", obj))
			return
		}
	}

	klog.Infof("Received delete event for storage cluster [%+v]", cluster)
	rev, err := strconv.ParseInt(cluster.ResourceVersion, 10, 64)
	if err != nil {
		klog.Errorf("Got invalid resource version %s for storage cluster %v.", cluster.ResourceVersion, cluster)
		return
	}
	if diff.RevisionIsNewer(uint64(s.rev), uint64(rev)) {
		klog.V(3).Infof("Received duplicated storage cluster delete event. Ignore [%v]. Current rev %v.", cluster, s.rev)
	}

	clusterId, err := diff.GetClusterIdFromString(cluster.StorageClusterId)
	if err != nil {
		klog.Fatalf("Invalid storage cluster id %v", clusterId)
	}

	_, isOK := s.backendClusters[clusterId]
	if isOK {
		s.mux.Lock()
		klog.V(4).Infof("mux acquired deleteCluster.")
		defer func() {
			s.mux.Unlock()
			klog.V(4).Infof("mux released deleteCluster.")
		}()

		s.updateBackendStorageConfig(clusterId, "DELETE")
		delete(s.backendClusters, clusterId)
		s.rev = rev
	} else {
		klog.Errorf("Trying to delete cluster [%+v] but not in current storage cluster config [%+v]", cluster, s.backendClusters)
	}
}

func (s *StorageClusterManager) updateBackendStorageConfig(storageClusterId uint8, action string) {
	addresses := []string{s.backendClusters[storageClusterId].ServiceAddress}

	storageClusterAction := StorageClusterAction{
		StorageClusterId: storageClusterId,
		ServerAddresses:  addresses,
		Action:           action,
	}
	SendStorageClusterUpdate(storageClusterAction)
	klog.V(3).Infof("Sent storage cluster action. [%+v]", storageClusterAction)
}
