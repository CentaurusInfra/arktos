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

var instanceTenantMapManager *TenantStorageMapManager

type tenantStorage struct {
	clusterId string
	rev       int64
}

type TenantStorageMapManager struct {
	tenantListerSynced cache.InformerSynced
	tenantLister       corelisters.TenantLister

	tenantToClusterMap map[string]*tenantStorage

	mux sync.RWMutex
	rev int64
}

var GetClusterIdFromTenantHandler = getClusterIdFromTenant

func GetTenantStorageManager() *TenantStorageMapManager {
	return instanceTenantMapManager
}

func checkTenantStorageInstanceExistence() {
	if instanceTenantMapManager != nil {
		klog.Fatalf("Unexpected reference to tenant storage cluster manager - initialized")
	}
}

func NewTenantStorageMapManager(tenantInformer coreinformers.TenantInformer) *TenantStorageMapManager {
	checkTenantStorageInstanceExistence()

	manager := &TenantStorageMapManager{
		tenantToClusterMap: make(map[string]*tenantStorage),
		rev:                int64(0),
	}

	tenantInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    manager.addTenant,
		UpdateFunc: manager.updateTenant,
		DeleteFunc: manager.deleteTenant,
	})

	manager.tenantLister = tenantInformer.Lister()
	manager.tenantListerSynced = tenantInformer.Informer().HasSynced
	err := manager.syncTenants()
	if err != nil {
		klog.Fatalf("Unable to get tenant storage from registry. Error %v", err)
	}

	instanceTenantMapManager = manager
	return instanceTenantMapManager
}

func (ts *TenantStorageMapManager) Run(stopCh <-chan struct{}) {
	klog.Infof("Starting tenant storage map manager.")
	defer klog.Infof("Shutting down tenant storage map manager.")

	if !cache.WaitForCacheSync(stopCh, ts.tenantListerSynced) {
		klog.Infof("Tenant storage map NOT synced %v.", ts.tenantListerSynced)
		return
	}

	klog.Infof("Caches are synced for tenant storage map.")
	<-stopCh
}

func (ts *TenantStorageMapManager) syncTenants() error {
	ts.mux.Lock()
	klog.V(4).Infof("mux acquired syncTenants.")
	defer func() {
		ts.mux.Unlock()
		klog.V(4).Infof("mux released syncTenants.")
	}()

	// Question: do we need to ignore deleted tenants?
	tenants, err := ts.tenantLister.List(labels.Everything())
	if err != nil {
		klog.Fatalf("Error in getting tenant list: %v", err)
	}

	aggErr := []error{}
	for _, tenant := range tenants {
		rev, err := strconv.ParseInt(tenant.ResourceVersion, 10, 64)
		if err != nil {
			aggErr = append(aggErr, errors.New(fmt.Sprintf("Got invalid resource version %s for tenant %s", tenant.ResourceVersion, tenant.Name)))
			rev = int64(-1)
		}

		ts.tenantToClusterMap[tenant.Name] = &tenantStorage{
			clusterId: tenant.Spec.StorageClusterId,
			rev:       rev,
		}
	}

	return utilerrors.NewAggregate(aggErr)
}

func (ts *TenantStorageMapManager) addTenant(obj interface{}) {
	tenant := obj.(*v1.Tenant)
	if tenant.DeletionTimestamp != nil {
		return
	}

	klog.V(3).Infof("Received event for NEW tenant %s with storage cluster %s. Rev %s", tenant.Name, tenant.Spec.StorageClusterId, tenant.ResourceVersion)

	rev, err := strconv.ParseInt(tenant.ResourceVersion, 10, 64)
	if err != nil {
		klog.Errorf("Got invalid resource version %s for tenant %v.", tenant.ResourceVersion, tenant.Name)
		return
	}

	if diff.RevisionIsNewer(uint64(ts.rev), uint64(rev)) {
		klog.V(3).Infof("Received duplicated tenant add event. Ignore [%v]. Current rev %v.", tenant, ts.rev)
		return
	}
	existedTenant, isOK := ts.tenantToClusterMap[tenant.Name]
	if isOK {
		if existedTenant.clusterId == tenant.Spec.StorageClusterId {
			klog.Warningf("Tenant %s storage cluster %s already saved. Skipping now", tenant.Name, existedTenant.clusterId)
			return
		} else {
			klog.Errorf("Tenant %s storage cluster does not match in ADD. Existing cluster id %s, new cluster id %s. No updates.",
				tenant.Name, existedTenant.clusterId, tenant.Spec.StorageClusterId)
			return
		}
		return
	}

	ts.mux.Lock()
	klog.V(4).Infof("mux acquired addTenant.")
	ts.tenantToClusterMap[tenant.Name] = &tenantStorage{
		clusterId: tenant.Spec.StorageClusterId,
		rev:       rev,
	}
	ts.rev = rev
	ts.mux.Unlock()
	klog.V(4).Infof("mux released addTenant.")
}

func (ts *TenantStorageMapManager) updateTenant(old, cur interface{}) {
	curTenant := cur.(*v1.Tenant)
	oldTenant := old.(*v1.Tenant)

	if curTenant.ResourceVersion == oldTenant.ResourceVersion {
		return
	}

	oldRev, _ := strconv.ParseInt(oldTenant.ResourceVersion, 10, 64)
	newRev, err := strconv.ParseInt(curTenant.ResourceVersion, 10, 64)
	if err != nil {
		klog.Errorf("Got invalid resource version %s for tenant %v.", curTenant.ResourceVersion, curTenant)
		return
	}

	if diff.RevisionIsNewer(uint64(oldRev), uint64(newRev)) {
		klog.V(2).Infof("Got staled tenant event %+v in UpdateFunc. Existing Version %s, new instance version %s.", curTenant, oldTenant.ResourceVersion, curTenant.ResourceVersion)
		return
	}

	ts.mux.Lock()
	klog.V(4).Infof("mux acquired updateTenant.")
	defer func() {
		ts.tenantToClusterMap[curTenant.Name] = &tenantStorage{
			clusterId: curTenant.Spec.StorageClusterId,
			rev:       newRev,
		}
		ts.rev = newRev
		ts.mux.Unlock()
		klog.V(4).Infof("mux released updateTenant.")
	}()

	if curTenant.Name != oldTenant.Name {
		klog.Warningf("Tenant name was updated from %s to %s", oldTenant.Name, curTenant.Name)
		delete(ts.tenantToClusterMap, oldTenant.Name)
		return
	}

	existedTenant, isOK := ts.tenantToClusterMap[oldTenant.Name]
	if !isOK {
		klog.Warningf("Received event for update tenant %s not in tenant map", oldTenant.Name)
		return
	}
	if curTenant.Spec.StorageClusterId != existedTenant.clusterId {
		klog.Warningf("Received event for update tenant %s cluster. New cluster %s, old cluster %s. Currently storage moving is not supported.",
			curTenant.Name, curTenant.Spec.StorageClusterId, existedTenant.clusterId)
		return
	}
}

func (ts *TenantStorageMapManager) deleteTenant(obj interface{}) {
	tenant, ok := obj.(*v1.Tenant)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v.", obj))
			return
		}
		tenant, ok = tombstone.Obj.(*v1.Tenant)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a tenant %#v.", obj))
			return
		}
	}

	klog.V(3).Infof("Received delete event for tenant [%+v]", tenant)
	rev, err := strconv.ParseInt(tenant.ResourceVersion, 10, 64)
	if err != nil {
		klog.Errorf("Got invalid resource version %s for tenant %v.", tenant.ResourceVersion, tenant.Name)
		return
	}

	if diff.RevisionIsNewer(uint64(ts.rev), uint64(rev)) {
		klog.Errorf("Got invalid resource version %s for tenant %s.", tenant.ResourceVersion, tenant.Name)
		return
	}
	ts.mux.Lock()
	klog.V(4).Infof("mux acquired deleteTenant.")
	delete(ts.tenantToClusterMap, tenant.Name)

	ts.mux.Unlock()
	klog.V(4).Infof("mux released deleteTenant.")
}

func (ts *TenantStorageMapManager) GetClusterIdFromTenant(tenant string) uint8 {
	ts.mux.RLock()
	tenantStorage, isOK := ts.tenantToClusterMap[tenant]
	ts.mux.RUnlock()
	clusterId := uint8(0)
	var err error
	if isOK {
		clusterId, err = diff.GetClusterIdFromString(tenantStorage.clusterId)
		if err != nil {
			klog.Errorf("Tenant %s storage cluster id is not valid. Use system cluster instead", tenant)
		}
	}

	return clusterId
}

func getClusterIdFromTenant(tenant string) uint8 {
	return GetTenantStorageManager().GetClusterIdFromTenant(tenant)
}
