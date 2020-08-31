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
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"strconv"
	"sync"
	"testing"
)

// StorageClusterManager is singleton - make sure each test runs sequentially
var testLock sync.Mutex
var notifyTimes int

func setupInformer(stop chan struct{}) informers.SharedInformerFactory {
	client := fake.NewSimpleClientset()

	informerFactory := informers.NewSharedInformerFactory(client, 0)
	informerFactory.Start(stop)
	informerFactory.WaitForCacheSync(stop)
	return informerFactory
}

func TestGetStorageClusterManager(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()

	// test nil instance
	instanceStorageClusterManager = nil
	handler := GetStorageClusterManager()
	assert.Nil(t, handler)

	// test singleton
	instanceStorageClusterManager = &StorageClusterManager{}
	handler = GetStorageClusterManager()
	assert.Equal(t, instanceStorageClusterManager, handler)
}

func TestNewStorageClusterManager(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()

	instanceStorageClusterManager = nil
	stop := make(chan struct{})
	defer close(stop)
	informer := setupInformer(stop)

	storageClusterManager := NewStorageClusterManager(informer.Core().V1().StorageClusters())
	assert.NotNil(t, storageClusterManager)
}

func createStorageCluster(clusterName, clusterId, rv string) *v1.StorageCluster {
	cluster1 := &v1.StorageCluster{
		ObjectMeta: k8s_io_apimachinery_pkg_apis_meta_v1.ObjectMeta{
			Name:            clusterName,
			ResourceVersion: rv,
		},
		StorageClusterId: clusterId,
		ServiceAddress:   "127.0.0.1",
	}

	return cluster1
}

func TestAddCluster(t *testing.T) {
	manager := &StorageClusterManager{
		rev:             int64(0),
		backendClusters: make(map[uint8]*v1.StorageCluster),
	}
	cluster1 := createStorageCluster("cluster-1", "1", "100")
	manager.addCluster(cluster1)
	assert.Equal(t, cluster1.ResourceVersion, strconv.FormatInt(manager.rev, 10))
	assert.NotNil(t, manager.backendClusters)
	clusterRead, isOK := manager.backendClusters[1]
	assert.True(t, isOK)
	assert.Equal(t, cluster1.Name, clusterRead.Name)
	assert.Equal(t, cluster1.StorageClusterId, clusterRead.StorageClusterId)

	// test older event will have no changes
	cluster1_old := createStorageCluster("cluster-01", "1", "99")
	manager.addCluster(cluster1_old)
	assert.Equal(t, cluster1.ResourceVersion, strconv.FormatInt(manager.rev, 10))
	clusterRead, isOK = manager.backendClusters[1]
	assert.True(t, isOK)
	assert.Equal(t, cluster1.Name, clusterRead.Name)
	assert.Equal(t, cluster1.StorageClusterId, clusterRead.StorageClusterId)
}

func TestUpdateCluster(t *testing.T) {
	// test older even will not have effect
	manager := &StorageClusterManager{
		rev:             int64(0),
		backendClusters: make(map[uint8]*v1.StorageCluster),
	}
	cluster1 := createStorageCluster("cluster-1", "1", "100")
	manager.addCluster(cluster1)

	cluster1_old := createStorageCluster("cluster-01", "1", "99")
	manager.updateCluster(cluster1_old, cluster1)
	assert.Equal(t, cluster1.ResourceVersion, strconv.FormatInt(manager.rev, 10))
	clusterRead, isOK := manager.backendClusters[1]
	assert.True(t, isOK)
	assert.Equal(t, cluster1.Name, clusterRead.Name)
	assert.Equal(t, cluster1.StorageClusterId, clusterRead.StorageClusterId)

	// test new event will have effect
	cluster1_new := createStorageCluster("cluster-2", "1", "101")
	manager.updateCluster(cluster1, cluster1_new)
	assert.Equal(t, cluster1_new.ResourceVersion, strconv.FormatInt(manager.rev, 10))
	clusterRead, isOK = manager.backendClusters[1]
	assert.True(t, isOK)
	assert.Equal(t, cluster1_new.Name, clusterRead.Name)
	assert.Equal(t, cluster1_new.StorageClusterId, clusterRead.StorageClusterId)
}

func TestDeleteCluster(t *testing.T) {
	manager := &StorageClusterManager{
		rev:             int64(0),
		backendClusters: make(map[uint8]*v1.StorageCluster),
	}

	// test delete non existing cluster
	cluster1 := createStorageCluster("cluster-1", "1", "100")
	manager.addCluster(cluster1)
	assert.Equal(t, cluster1.ResourceVersion, strconv.FormatInt(manager.rev, 10))
	assert.NotNil(t, manager.backendClusters)
	clusterRead, isOK := manager.backendClusters[1]
	assert.True(t, isOK)
	assert.Equal(t, cluster1.Name, clusterRead.Name)
	assert.Equal(t, cluster1.StorageClusterId, clusterRead.StorageClusterId)

	clusterDel := createStorageCluster("cluster-01", "2", "101")
	manager.deleteCluster(clusterDel)
	assert.Equal(t, cluster1.ResourceVersion, strconv.FormatInt(manager.rev, 10))
	assert.NotNil(t, manager.backendClusters)
	clusterRead, isOK = manager.backendClusters[1]
	assert.True(t, isOK)
	assert.Equal(t, cluster1.Name, clusterRead.Name)
	assert.Equal(t, cluster1.StorageClusterId, clusterRead.StorageClusterId)

	_, isOK = manager.backendClusters[2]
	assert.False(t, isOK)

	// delete existed cluster
	cluster1.ResourceVersion = "110"
	manager.deleteCluster(cluster1)
	assert.Equal(t, cluster1.ResourceVersion, strconv.FormatInt(manager.rev, 10))
	assert.NotNil(t, manager.backendClusters)
	_, isOK = manager.backendClusters[1]
	assert.False(t, isOK)
}
