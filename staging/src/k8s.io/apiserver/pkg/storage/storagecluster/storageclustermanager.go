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
	"fmt"
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

var instance *StorageClusterManager
var checkInstanceHandler = checkInstanceExistence

type StorageClusterManager struct {
	storageClusterListerSynced cache.InformerSynced
	storageClusterLister       corelisters.StorageClusterLister

	backendClusters map[string]*v1.StorageCluster

	mux sync.Mutex
	rev int64
}

func GetStorageClusterManager() *StorageClusterManager {
	return instance
}

func checkInstanceExistence() {
	if instance != nil {
		klog.Fatalf("Unexpected reference to storage cluster manager - initialized")
	}
}

func NewStorageClusterManager(scInformer coreinformers.StorageClusterInformer) *StorageClusterManager {
	checkInstanceHandler()

	manager := &StorageClusterManager{
		rev:             int64(0),
		backendClusters: make(map[string]*v1.StorageCluster),
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

	instance = manager
	return instance
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

	for _, cluster := range clusters {
		s.backendClusters[cluster.StorageClusterId] = cluster
		newRev, err := strconv.ParseInt(cluster.ResourceVersion, 10, 64)
		if err != nil {
			klog.Errorf("Got invalid resource version %s for storage cluster %v.", cluster.ResourceVersion, cluster)
		} else if s.rev < newRev {
			s.rev = newRev
		}
	}

	for clusterId := range s.backendClusters {
		s.updateBackendStorageConfig(clusterId, "ADD")
	}

	return nil
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

	if s.rev >= rev {
		// Storage cluster data shall be in system ETCD cluster only.
		klog.V(3).Infof("Received duplicated storage cluster add event. Ignore [%v]. Current rev %v.", c, s.rev)
		return
	}
	existedCluster, isOK := s.backendClusters[c.StorageClusterId]
	if isOK {
		klog.Errorf("Storage cluster id %s already used in %s. Skipping now", c.StorageClusterId, existedCluster.Name)
		return
	}

	s.mux.Lock()
	klog.V(4).Infof("mux acquired addCluster.")
	s.backendClusters[c.StorageClusterId] = c
	s.updateBackendStorageConfig(c.StorageClusterId, "ADD")

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

	if newRev <= oldRev {
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
		if curCluster.ServiceAddress != oldCluster.ServiceAddress {
			go s.updateBackendStorageConfig(curCluster.StorageClusterId, "UPDATE")
		}

		s.backendClusters[curCluster.StorageClusterId] = curCluster
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
	if rev <= s.rev {
		klog.V(3).Infof("Received duplicated storage cluster delete event. Ignore [%v]. Current rev %v.", cluster, s.rev)
	}

	_, isOK := s.backendClusters[cluster.StorageClusterId]
	if isOK {
		s.mux.Lock()
		klog.V(4).Infof("mux acquired deleteCluster.")
		defer func() {
			s.mux.Unlock()
			klog.V(4).Infof("mux released deleteCluster.")
		}()

		s.updateBackendStorageConfig(cluster.StorageClusterId, "DELETE")
		delete(s.backendClusters, cluster.StorageClusterId)
		s.rev = rev
	} else {
		klog.Errorf("Trying to delete cluster [%+v] but not in current storage cluster config [%+v]", cluster, s.backendClusters)
	}
}

func (s *StorageClusterManager) updateBackendStorageConfig(storageClusterId string, action string) {
	updateCh := GetStorageClusterUpdateCh()

	addresses := []string{s.backendClusters[storageClusterId].ServiceAddress}

	storageClusterAction := StorageClusterAction{
		StorageClusterId: storageClusterId,
		ServerAddresses:  addresses,
		Action:           action,
	}
	updateCh <- storageClusterAction
	klog.Infof("Sent storage cluster action. [%+v]", storageClusterAction)
}
