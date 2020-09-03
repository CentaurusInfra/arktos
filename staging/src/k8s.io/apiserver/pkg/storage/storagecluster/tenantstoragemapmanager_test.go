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
	"sync"
	"testing"
)

// TenantStorageManager is singleton - make sure each test runs sequentially
var testTSLock sync.Mutex

func TestGetTenantStorageManager(t *testing.T) {
	testTSLock.Lock()
	defer testTSLock.Unlock()

	// test nil instance
	instanceTenantMapManager = nil
	handler := GetTenantStorageManager()
	assert.Nil(t, handler)

	// test singleton
	instanceTenantMapManager = &TenantStorageMapManager{}
	handler = GetTenantStorageManager()
	assert.NotNil(t, handler)
	assert.Equal(t, instanceTenantMapManager, handler)
}

func TestNewTenantStorageMapManager(t *testing.T) {
	testTSLock.Lock()
	defer testTSLock.Unlock()
	instanceTenantMapManager = nil

	stop := make(chan struct{})
	defer close(stop)
	informer := setupInformer(stop)

	tsMapManager := NewTenantStorageMapManager(informer.Core().V1().Tenants())
	assert.NotNil(t, tsMapManager)
	assert.NotNil(t, tsMapManager.tenantToClusterMap)
}

func createTenant(name, rv, clusterId string) *v1.Tenant {
	tenant := &v1.Tenant{
		ObjectMeta: k8s_io_apimachinery_pkg_apis_meta_v1.ObjectMeta{
			Name:            name,
			ResourceVersion: rv,
		},
		Spec: v1.TenantSpec{
			StorageClusterId: clusterId,
		},
	}

	return tenant
}

func TestAddTenant(t *testing.T) {
	manager := &TenantStorageMapManager{tenantToClusterMap: make(map[string]*tenantStorage)}
	tenant1 := createTenant("t1", "1", "1")
	manager.addTenant(tenant1)
	assert.NotNil(t, manager.tenantToClusterMap)
	assert.Equal(t, int64(1), manager.rev)
	clusterIdRead := manager.GetClusterIdFromTenant(tenant1.Name)
	assert.Equal(t, uint8(1), clusterIdRead)

	tenant2 := createTenant("t2", "100", "2")
	manager.addTenant(tenant2)
	assert.Equal(t, int64(100), manager.rev)
	clusterIdRead = manager.GetClusterIdFromTenant(tenant2.Name)
	assert.Equal(t, uint8(2), clusterIdRead)

	// got old event
	tenant1_old := createTenant(tenant1.Name, "99", "2")
	manager.addTenant(tenant1_old)
	clusterIdRead = manager.GetClusterIdFromTenant(tenant1.Name)
	assert.Equal(t, int64(100), manager.rev)
	assert.Equal(t, uint8(1), clusterIdRead)

	// got new event but is add existing tenant
	tenant1_new := createTenant(tenant1.Name, "200", "2")
	manager.addTenant(tenant1_new)
	clusterIdRead = manager.GetClusterIdFromTenant(tenant1.Name)
	assert.Equal(t, int64(100), manager.rev)
	assert.Equal(t, uint8(1), clusterIdRead)
}

func TestUpdateTenant(t *testing.T) {
	manager := &TenantStorageMapManager{tenantToClusterMap: make(map[string]*tenantStorage)}
	tenant1 := createTenant("t1", "1", "1")
	manager.addTenant(tenant1)
	assert.NotNil(t, manager.tenantToClusterMap)
	assert.Equal(t, int64(1), manager.rev)
	clusterIdRead := manager.GetClusterIdFromTenant(tenant1.Name)
	assert.Equal(t, uint8(1), clusterIdRead)

	tenant1_new := createTenant(tenant1.Name, "100", "2")
	manager.updateTenant(tenant1, tenant1_new)
	assert.Equal(t, int64(100), manager.rev)
	clusterIdRead = manager.GetClusterIdFromTenant(tenant1.Name)
	assert.Equal(t, uint8(2), clusterIdRead)

	// update tenant name and cluster id together
	tenant1_new2 := createTenant("t100", "105", "3")
	manager.updateTenant(tenant1_new, tenant1_new2)
	assert.Equal(t, int64(105), manager.rev)
	clusterIdRead = manager.GetClusterIdFromTenant(tenant1_new2.Name)
	assert.Equal(t, uint8(3), clusterIdRead)
}

func TestDeleteTenant(t *testing.T) {
	manager := &TenantStorageMapManager{tenantToClusterMap: make(map[string]*tenantStorage)}
	tenant1 := createTenant("t1", "1", "1")
	manager.addTenant(tenant1)
	assert.NotNil(t, manager.tenantToClusterMap)
	assert.Equal(t, int64(1), manager.rev)
	clusterIdRead := manager.GetClusterIdFromTenant(tenant1.Name)
	assert.Equal(t, uint8(1), clusterIdRead)

	// delete tenant not in map
	tenant2 := createTenant("t2", "100", "1")
	manager.deleteTenant(tenant2)
	assert.NotNil(t, manager.tenantToClusterMap)
	clusterIdRead = manager.GetClusterIdFromTenant(tenant1.Name)
	assert.Equal(t, uint8(1), clusterIdRead)

	// tenant not in map will go to cluster 0
	clusterIdRead = manager.GetClusterIdFromTenant(tenant2.Name)
	assert.Equal(t, uint8(0), clusterIdRead)

	// delete tenant in map
	tenant1_del := createTenant("t1", "110", "1")
	manager.deleteTenant(tenant1_del)
	assert.NotNil(t, manager.tenantToClusterMap)
	clusterIdRead = manager.GetClusterIdFromTenant(tenant1.Name)
	assert.Equal(t, uint8(0), clusterIdRead)
}
