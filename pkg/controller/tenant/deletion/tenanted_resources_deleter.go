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

package deletion

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/klog"

	"k8s.io/api/core/v1"
	crdregistry "k8s.io/apiextensions-apiserver/pkg/registry/customresourcedefinition"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
)

// TenantedResourcesDeleterInterface is the interface to delete a tenant with all resources in it.
type TenantedResourcesDeleterInterface interface {
	Delete(tenantName string) error
}

// NewTenantedResourcesDeleter returns a new TenantedResourcesDeleter.
func NewTenantedResourcesDeleter(kubeClient clientset.Interface,
	metadataClient metadata.Interface,
	discoverResourcesFn func() ([]*metav1.APIResourceList, error),
	finalizerToken v1.FinalizerName) TenantedResourcesDeleterInterface {
	d := &tenantedResourcesDeleter{
		kubeClient:          kubeClient,
		metadataClient:      metadataClient,
		discoverResourcesFn: discoverResourcesFn,
		finalizerToken:      finalizerToken,
		opCache: &operationNotSupportedCache{
			m: make(map[operationKey]bool),
		},
	}
	d.initOpCache()
	return d
}

var _ TenantedResourcesDeleterInterface = &tenantedResourcesDeleter{}

// tenantedResourcesDeleter is used to delete all resources in a given tenant.
type tenantedResourcesDeleter struct {
	// kubeClient to get the resources
	kubeClient clientset.Interface
	// metadata client to list and delete all tenanted resources.
	metadataClient metadata.Interface

	// Cache of what operations are not supported on each group version resource.
	opCache *operationNotSupportedCache

	discoverResourcesFn func() ([]*metav1.APIResourceList, error)
	// The finalizer token that should be removed from the tenant
	// when all resources in that tenant have been deleted.
	finalizerToken v1.FinalizerName
}

// Delete deletes all resources in the given tenant.
// Before deleting resources:
// * It ensures that deletion timestamp is set on the
//   tenant (does nothing if deletion timestamp is missing).
// * Verifies that the tenant is in the "terminating" phase
//   (updates the tenant phase if it is not yet marked terminating)
// After deleting the resources:
// * It removes finalizer token from the given tenant.
//
// Returns an error if any of those steps fail.
// Returns ResourcesRemainingError if it deleted some resources but needs
// to wait for them to go away.
// Caller is expected to keep calling this until it succeeds.
func (d *tenantedResourcesDeleter) Delete(tenantName string) error {
	// Multiple controllers may edit a tenant during termination
	// first get the latest state of the tenant before proceeding
	// if the tenant was deleted already, don't do anything
	tenant, err := d.kubeClient.CoreV1().Tenants().Get(tenantName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if tenant.DeletionTimestamp == nil {
		return nil
	}

	klog.V(5).Infof("tenant controller - sync Tenant - tenant: %s, finalizerToken: %s", tenant.Name, d.finalizerToken)

	// ensure that the status is up to date on the tenant
	// if we get a not found error, we assume the tenant is truly gone
	tenant, err = d.retryOnConflictError(tenant, d.updateTenantStatusFunc)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	// the latest view of the tenant asserts that tenant is no longer deleting..
	if tenant.DeletionTimestamp.IsZero() {
		return nil
	}

	// Delete the tenant if it is already finalized.
	if finalized(tenant) {
		return d.deleteTenant(tenant)
	}

	// there may still be content for us to remove
	estimate, err := d.deleteAllContent(tenant, *tenant.DeletionTimestamp)
	if err != nil {
		return err
	}
	if estimate > 0 {
		return &ResourcesRemainingError{estimate}
	}

	// we have removed content, so mark it finalized by us
	tenant, err = d.retryOnConflictError(tenant, d.finalizeTenant)
	if err != nil {
		// in normal practice, this should not be possible, but if a deployment is running
		// two controllers to do tenant deletion that share a common finalizer token it's
		// possible that a not found could occur since the other controller would have finished the delete.
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Check if we can delete now.
	if finalized(tenant) {
		return d.deleteTenant(tenant)
	}

	return nil
}

func (d *tenantedResourcesDeleter) initOpCache() {
	// pre-fill opCache with the discovery info
	resources, err := d.discoverResourcesFn()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get all supported resources from server: %v", err))
	}
	deletableGroupVersionResources := []schema.GroupVersionResource{}
	for _, rl := range resources {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			klog.Errorf("Failed to parse GroupVersion %q, skipping: %v", rl.GroupVersion, err)
			continue
		}

		for _, r := range rl.APIResources {
			gvr := schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: r.Name}
			verbs := sets.NewString([]string(r.Verbs)...)

			if !verbs.Has("delete") {
				klog.V(6).Infof("Skipping resource %v because it cannot be deleted.", gvr)
			}

			if r.Tenanted != true || r.Namespaced == true {
				klog.V(6).Infof("Skipping resource %v because it is not tenant-scoped.", gvr)
			}

			for _, op := range []operation{operationList, operationDeleteCollection} {
				if !verbs.Has(string(op)) {
					d.opCache.setNotSupported(operationKey{operation: op, gvr: gvr})
				}
			}
			deletableGroupVersionResources = append(deletableGroupVersionResources, gvr)
		}
	}
}

// Deletes the given tenant.
func (d *tenantedResourcesDeleter) deleteTenant(tenant *v1.Tenant) error {
	var opts *metav1.DeleteOptions
	uid := tenant.UID
	if len(uid) > 0 {
		opts = &metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}
	}
	err := d.kubeClient.CoreV1().Tenants().Delete(tenant.Name, opts)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

// ResourcesRemainingError is used to inform the caller that all resources are not yet fully removed from the tenant.
type ResourcesRemainingError struct {
	Estimate int64
}

func (e *ResourcesRemainingError) Error() string {
	return fmt.Sprintf("some content remains in the tenant, estimate %d seconds before it is removed", e.Estimate)
}

// operation is used for caching if an operation is supported on a metadata client.
type operation string

const (
	operationDeleteCollection operation = "deletecollection"
	operationList             operation = "list"
	// assume a default estimate for finalizers to complete when found on items pending deletion.
	finalizerEstimateSeconds int64 = int64(15)
)

// operationKey is an entry in a cache.
type operationKey struct {
	operation operation
	gvr       schema.GroupVersionResource
}

// operationNotSupportedCache is a simple cache to remember if an operation is not supported for a resource.
// if the operationKey maps to true, it means the operation is not supported.
type operationNotSupportedCache struct {
	lock sync.RWMutex
	m    map[operationKey]bool
}

// isSupported returns true if the operation is supported
func (o *operationNotSupportedCache) isSupported(key operationKey) bool {
	o.lock.RLock()
	defer o.lock.RUnlock()
	return !o.m[key]
}

func (o *operationNotSupportedCache) setNotSupported(key operationKey) {
	o.lock.Lock()
	defer o.lock.Unlock()
	o.m[key] = true
}

// updateTenantFunc is a function that makes an update to a tenant
type updateTenantFunc func(tenant *v1.Tenant) (*v1.Tenant, error)

// retryOnConflictError retries the specified fn if there was a conflict error
// it will return an error if the UID for an object changes across retry operations.
// TODO RetryOnConflict should be a generic concept in client code
func (d *tenantedResourcesDeleter) retryOnConflictError(tenant *v1.Tenant, fn updateTenantFunc) (result *v1.Tenant, err error) {
	latestTenant := tenant
	for {
		result, err = fn(latestTenant)
		if err == nil {
			return result, nil
		}
		if !errors.IsConflict(err) {
			return nil, err
		}
		prevTenant := latestTenant
		latestTenant, err = d.kubeClient.CoreV1().Tenants().Get(latestTenant.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if prevTenant.UID != latestTenant.UID {
			return nil, fmt.Errorf("tenant uid has changed across retries")
		}
	}
}

// updateTenantStatusFunc will verify that the status of the tenant is correct
func (d *tenantedResourcesDeleter) updateTenantStatusFunc(tenant *v1.Tenant) (*v1.Tenant, error) {
	if tenant.DeletionTimestamp.IsZero() || tenant.Status.Phase == v1.TenantTerminating {
		return tenant, nil
	}
	newTenant := v1.Tenant{}
	newTenant.ObjectMeta = tenant.ObjectMeta
	newTenant.Status = tenant.Status
	newTenant.Status.Phase = v1.TenantTerminating
	return d.kubeClient.CoreV1().Tenants().UpdateStatus(&newTenant)
}

// finalized returns true if the tenant.Spec.Finalizers is an empty list
func finalized(tenant *v1.Tenant) bool {
	return len(tenant.Spec.Finalizers) == 0
}

// finalizeTenant removes the specified finalizerToken and finalizes the tenant
func (d *tenantedResourcesDeleter) finalizeTenant(tenant *v1.Tenant) (*v1.Tenant, error) {
	tenantFinalize := v1.Tenant{}
	tenantFinalize.ObjectMeta = tenant.ObjectMeta
	tenantFinalize.Spec = tenant.Spec
	finalizerSet := sets.NewString()
	for i := range tenant.Spec.Finalizers {
		if tenant.Spec.Finalizers[i] != d.finalizerToken {
			finalizerSet.Insert(string(tenant.Spec.Finalizers[i]))
		}
	}
	tenantFinalize.Spec.Finalizers = make([]v1.FinalizerName, 0, len(finalizerSet))
	for _, value := range finalizerSet.List() {
		tenantFinalize.Spec.Finalizers = append(tenantFinalize.Spec.Finalizers, v1.FinalizerName(value))
	}
	tenant, err := d.kubeClient.CoreV1().Tenants().Finalize(&tenantFinalize)
	if err != nil {
		// it was removed already, so life is good
		if errors.IsNotFound(err) {
			return tenant, nil
		}
	}
	return tenant, err
}

// deleteCollection is a helper function that will delete the collection of resources
// it returns true if the operation was supported on the server.
// it returns an error if the operation was supported on the server but was unable to complete.
func (d *tenantedResourcesDeleter) deleteCollection(gvr schema.GroupVersionResource, tenant string) (bool, error) {
	klog.V(5).Infof("tenant controller - deleteCollection - tenant: %s, gvr: %v", tenant, gvr)

	key := operationKey{operation: operationDeleteCollection, gvr: gvr}
	if !d.opCache.isSupported(key) {
		klog.V(5).Infof("tenant controller - deleteCollection ignored since not supported - tenant %s, gvr: %v", tenant, gvr)
		return false, nil
	}

	// tenant controller does not want the garbage collector to insert the orphan finalizer since it calls
	// resource deletions generically.  it will ensure all resources in the tenant are purged prior to releasing
	// tenant itself.
	background := metav1.DeletePropagationBackground
	opts := &metav1.DeleteOptions{PropagationPolicy: &background}
	err := d.metadataClient.Resource(gvr).NamespaceWithMultiTenancy(metav1.NamespaceAll, tenant).DeleteCollection(opts, metav1.ListOptions{})

	if err == nil {
		return true, nil
	}

	if errors.IsMethodNotSupported(err) || errors.IsNotFound(err) {
		klog.V(5).Infof("tenant controller - deleteCollection not supported - tenant: %s, gvr: %v", tenant, gvr)
		d.opCache.setNotSupported(key)
		return false, nil
	}

	klog.V(5).Infof("tenant controller - deleteCollection unexpected error - tenant: %s, gvr: %v, error: %v", tenant, gvr, err)
	return true, err
}

// listCollection will list the items in the specified tenant
// it returns the following:
//  the list of items in the collection (if found)
//  a boolean if the operation is supported
//  an error if the operation is supported but could not be completed.
func (d *tenantedResourcesDeleter) listCollection(gvr schema.GroupVersionResource, tenant string) (*metav1.PartialObjectMetadataList, bool, error) {
	klog.V(5).Infof("tenant controller - listCollection - tenant: %s, gvr: %v", tenant, gvr)

	key := operationKey{operation: operationList, gvr: gvr}
	if !d.opCache.isSupported(key) {
		klog.V(5).Infof("tenant controller - listCollection ignored since not supported - tenant: %s, gvr: %v", tenant, gvr)
		return nil, false, nil
	}

	partialList, err := d.metadataClient.Resource(gvr).NamespaceWithMultiTenancy(metav1.NamespaceAll, tenant).List(metav1.ListOptions{})
	if err == nil {
		newItems := []metav1.PartialObjectMetadata{}
		for _, item := range partialList.Items {
			if crdregistry.IsSystemForcedCrd(item) {
				continue
			}

			newItems = append(newItems, item)
		}
		partialList.Items = newItems
		return partialList, true, nil
	}

	if errors.IsMethodNotSupported(err) || errors.IsNotFound(err) {
		klog.V(5).Infof("tenant controller - listCollection not supported - tenant: %s, gvr: %v", tenant, gvr)
		d.opCache.setNotSupported(key)
		return nil, false, nil
	}

	return nil, true, err
}

// deleteEachItem is a helper function that will list the collection of resources and delete each item 1 by 1.
func (d *tenantedResourcesDeleter) deleteEachItem(gvr schema.GroupVersionResource, tenant string) error {
	klog.V(5).Infof("tenant controller - deleteEachItem -tenant: %s, gvr: %v", tenant, gvr)

	partialList, listSupported, err := d.listCollection(gvr, tenant)
	if err != nil {
		return err
	}
	if !listSupported {
		return nil
	}
	for _, item := range partialList.Items {
		background := metav1.DeletePropagationBackground
		opts := &metav1.DeleteOptions{PropagationPolicy: &background}
		if err = d.metadataClient.Resource(gvr).NamespaceWithMultiTenancy(metav1.NamespaceAll, tenant).Delete(item.GetName(), opts); err != nil && !errors.IsNotFound(err) && !errors.IsMethodNotSupported(err) {
			return err
		}
	}
	return nil
}

// deleteAllContentForGroupVersionResource will use the metadata client to delete each resource identified in gvr.
// It returns an estimate of the time remaining before the remaining resources are deleted.
// If estimate > 0, not all resources are guaranteed to be gone.
func (d *tenantedResourcesDeleter) deleteAllContentForGroupVersionResource(
	gvr schema.GroupVersionResource, tenant string,
	tenantDeletedAt metav1.Time) (int64, error) {
	klog.V(5).Infof("tenant controller - deleteAllContentForGroupVersionResource - tenant: %s, gvr: %v", tenant, gvr)

	// estimate how long it will take for the resource to be deleted (needed for objects that support graceful delete)
	estimate, err := d.estimateGracefulTermination(gvr, tenant, tenantDeletedAt)
	if err != nil {
		klog.V(5).Infof("tenant controller - deleteAllContentForGroupVersionResource - unable to estimate - tenant: %s, gvr: %v, err: %v", tenant, gvr, err)
		return estimate, err
	}
	klog.V(5).Infof("tenant controller - deleteAllContentForGroupVersionResource - estimate - tenant: %s, gvr: %v, estimate: %v", tenant, gvr, estimate)

	// first try to delete the entire collection
	deleteCollectionSupported, err := d.deleteCollection(gvr, tenant)
	if err != nil {
		return estimate, err
	}

	// delete collection was not supported, so we list and delete each item...
	if !deleteCollectionSupported {
		err = d.deleteEachItem(gvr, tenant)
		if err != nil {
			return estimate, err
		}
	}

	// verify there are no more remaining items
	// it is not an error condition for there to be remaining items if local estimate is non-zero
	klog.V(5).Infof("tenant controller - deleteAllContentForGroupVersionResource - checking for no more items in tenant: %s, gvr: %v", tenant, gvr)
	partialList, listSupported, err := d.listCollection(gvr, tenant)
	if err != nil {
		klog.V(5).Infof("tenant controller - deleteAllContentForGroupVersionResource - error verifying no items in tenant: %s, gvr: %v, err: %v", tenant, gvr, err)
		return estimate, err
	}
	if !listSupported {
		return estimate, nil
	}
	klog.V(5).Infof("tenant controller - deleteAllContentForGroupVersionResource - items remaining - tenant: %s, gvr: %v, items: %v", tenant, gvr, len(partialList.Items))
	if len(partialList.Items) != 0 && estimate == int64(0) {
		// if any item has a finalizer, we treat that as a normal condition, and use a default estimation to allow for GC to complete.
		for _, item := range partialList.Items {
			if len(item.GetFinalizers()) > 0 {
				klog.V(5).Infof("tenant controller - deleteAllContentForGroupVersionResource - items remaining with finalizers - tenant: %s, gvr: %v, finalizers: %v", tenant, gvr, item.GetFinalizers())
				return finalizerEstimateSeconds, nil
			}
		}
		// nothing reported a finalizer, so something was unexpected as it should have been deleted.
		return estimate, fmt.Errorf("unexpected items still remain in tenant: %s for gvr: %v", tenant, gvr)
	}
	return estimate, nil
}

// deleteAllContent will use the metadata client to delete each resource identified in groupVersionResources.
// It returns an estimate of the time remaining before the remaining resources are deleted.
// If estimate > 0, not all resources are guaranteed to be gone.
func (d *tenantedResourcesDeleter) deleteAllContent(tenant *v1.Tenant, tenantDeletedAt metav1.Time) (int64, error) {
	var errs []error
	estimate := int64(0)
	klog.V(4).Infof("tenant controller - deleteAllContent - tenant: %s", tenant.Name)

	resources, err := d.discoverResourcesFn()
	if err != nil {
		// discovery errors are not fatal.  We often have some set of resources we can operate against even if we don't have a complete list
		errs = append(errs, err)
	}

	deletableResources := discovery.FilteredBy(discovery.SupportsAllVerbs{Verbs: []string{"delete"}}, resources)
	groupVersionResources, err := discovery.GroupVersionResources(deletableResources)
	if err != nil {
		// discovery errors are not fatal.  We often have some set of resources we can operate against even if we don't have a complete list
		errs = append(errs, err)
	}
	for gvr := range groupVersionResources {
		gvrEstimate, err := d.deleteAllContentForGroupVersionResource(gvr, tenant.Name, tenantDeletedAt)
		if err != nil {
			// If there is an error, hold on to it but proceed with all the remaining
			// groupVersionResources.
			errs = append(errs, err)
		}
		if gvrEstimate > estimate {
			estimate = gvrEstimate
		}
	}
	if len(errs) > 0 {
		return estimate, utilerrors.NewAggregate(errs)
	}
	klog.V(4).Infof("tenant controller - deleteAllContent - tenant: %s, estimate: %v", tenant.Name, estimate)
	return estimate, nil
}

// estimateGrracefulTermination will estimate the graceful termination required for the specific entity in the tenant
func (d *tenantedResourcesDeleter) estimateGracefulTermination(gvr schema.GroupVersionResource, tenant string, tenantDeletedAt metav1.Time) (int64, error) {
	groupResource := gvr.GroupResource()
	klog.V(5).Infof("tenant controller - estimateGracefulTermination - group %s, resource: %s", groupResource.Group, groupResource.Resource)
	estimate := int64(0)
	var err error
	switch groupResource {
	// the tenant-scoped resource that consumes the most time in deletion is namespaces.
	// ignoring the time cost of deleting the other resources
	case schema.GroupResource{Group: "", Resource: "namespaces"}:
		estimate, err = d.estimateGracefulTerminationForNamespaces(tenant)
	}
	if err != nil {
		return estimate, err
	}
	// determine if the estimate is greater than the deletion timestamp
	duration := time.Since(tenantDeletedAt.Time)
	allowedEstimate := time.Duration(estimate) * time.Second
	if duration >= allowedEstimate {
		estimate = int64(0)
	}
	return estimate, nil
}

// estimateGracefulTerminationForNamespaces determines the graceful termination period for namespaces in the tenant
func (d *tenantedResourcesDeleter) estimateGracefulTerminationForNamespaces(tenant string) (int64, error) {
	klog.V(5).Infof("tenant controller - estimateGracefulTerminationForNamespaces - tenant: %s", tenant)
	estimate := int64(0)
	nsList, err := d.kubeClient.CoreV1().NamespacesWithMultiTenancy(tenant).List(metav1.ListOptions{})
	if err != nil {
		return int64(0), fmt.Errorf("unexpected: failed to get the namespace list. Cannot estimate grace period seconds for namespaces")
	}

	// the namespaced-scoped resource that consumes the most time in deletion is namespaces.
	// ignoring the time cost of deleting the other resources
	for _, namespace := range nsList.Items {
		nsEstimate, err := d.estimateGracefulTerminationForPods(namespace.Name, tenant)
		if err == nil && nsEstimate > estimate {
			estimate = nsEstimate
		}
	}

	return estimate, nil
}

// estimateGracefulTerminationForPods determines the graceful termination period for pods in a given tenant/namespace
func (d *tenantedResourcesDeleter) estimateGracefulTerminationForPods(ns string, tenant string) (int64, error) {
	klog.V(5).Infof("tenant controller - estimateGracefulTerminationForPods - tenant: %s, namespace %s", tenant, ns)
	estimate := int64(0)
	items, err := d.kubeClient.CoreV1().PodsWithMultiTenancy(ns, tenant).List(metav1.ListOptions{})
	if err != nil {
		return estimate, err
	}
	for i := range items.Items {
		pod := items.Items[i]
		// filter out terminal pods
		phase := pod.Status.Phase
		if v1.PodSucceeded == phase || v1.PodFailed == phase {
			continue
		}
		if pod.Spec.TerminationGracePeriodSeconds != nil {
			grace := *pod.Spec.TerminationGracePeriodSeconds
			if grace > estimate {
				estimate = grace
			}
		}
	}
	return estimate, nil
}
