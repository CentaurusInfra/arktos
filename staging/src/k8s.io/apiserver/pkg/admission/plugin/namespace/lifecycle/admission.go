/*
Copyright 2015 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

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

package lifecycle

import (
	"fmt"
	"io"
	"time"

	"k8s.io/klog"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilcache "k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
)

const (
	// PluginName indicates the name of admission plug-in
	PluginName = "NamespaceLifecycle"
	// how long a namespace or a tenant stays in the force live lookup cache before expiration.
	forceLiveLookupTTL = 30 * time.Second
	// how long to wait for a missing tenant namespace before re-checking the cache (and then doing a live lookup)
	// this accomplishes two things:
	// 1. It allows a watch-fed cache time to observe a tenant/namespace creation event
	// 2. It allows time for a tenant/namespace creation to distribute to members of a storage cluster,
	//    so the live lookup has a better chance of succeeding even if it isn't performed against the leader.
	missingTenantNamespaceWait = 50 * time.Millisecond
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return NewLifecycle(
			sets.NewString(metav1.NamespaceDefault, metav1.NamespaceSystem, metav1.NamespacePublic),
			sets.NewString(metav1.TenantSystem, metav1.TenantSystem),
		)
	})
}

// Lifecycle is an implementation of admission.Interface.
// It enforces life-cycle constraints around a Namespace depending on its Phase
type Lifecycle struct {
	*admission.Handler
	client             kubernetes.Interface
	immortalNamespaces sets.String
	namespaceLister    corelisters.NamespaceLister
	// forceLiveNamespaceLookupCache holds a list of entries for namespaces that we have a strong reason to believe are stale in our local cache.
	// if a namespace is in this cache, then we will ignore our local state and always fetch latest from api server.
	forceLiveNamespaceLookupCache *utilcache.LRUExpireCache
	tenantLister                  corelisters.TenantLister
	immortalTenants               sets.String
	// forceLiveTenantLookupCache holds a list of entries for tenants that we have a strong reason to believe are stale in our local cache.
	// if a tenant is in this cache, then we will ignore our local state and always fetch latest from api server.
	forceLiveTenantLookupCache *utilcache.LRUExpireCache
}

var _ = initializer.WantsExternalKubeInformerFactory(&Lifecycle{})
var _ = initializer.WantsExternalKubeClientSet(&Lifecycle{})

func (l *Lifecycle) tenantLifecycleAdmit(a admission.Attributes) error {
	if a.GetTenant() == metav1.TenantNone || a.GetTenant() == metav1.TenantSystem {
		return nil
	}

	var (
		exists bool
		err    error
	)

	tenant, err := l.tenantLister.Get(a.GetTenant())
	if err != nil {
		if !errors.IsNotFound(err) {
			return errors.NewInternalError(err)
		}
	} else {
		exists = true
	}

	if !exists && a.GetOperation() == admission.Create {
		// give the cache time to observe the tenant before rejecting a create.
		// this helps when creating a tenant and immediately creating objects within it.
		time.Sleep(missingTenantNamespaceWait)
		tenant, err = l.tenantLister.Get(a.GetTenant())
		switch {
		case errors.IsNotFound(err):
			// no-op
		case err != nil:
			return errors.NewInternalError(err)
		default:
			exists = true
		}
		if exists {
			klog.V(4).Infof("found tenant %s in cache after waiting", a.GetTenant())
		}
	}

	// forceLiveTenantLookup if true will skip looking at local cache state and instead always make a live call to server.
	forceLiveTenantLookup := false
	if _, ok := l.forceLiveTenantLookupCache.Get(a.GetTenant()); ok {
		// we think the tenant was marked for deletion, but our current local cache says otherwise, we will force a live lookup.
		forceLiveTenantLookup = exists && tenant.Status.Phase == v1.TenantActive
	}

	// refuse to operate on non-existent tenants
	if !exists || forceLiveTenantLookup {
		// as a last resort, make a call directly to storage
		tenant, err = l.client.CoreV1().Tenants().Get(a.GetTenant(), metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
			return err
		case err != nil:
			return errors.NewInternalError(err)
		}
		klog.V(4).Infof("found tenant %s via storage lookup", a.GetTenant())
	}

	// ensure that we're not trying to create objects in terminating tenants
	if a.GetOperation() == admission.Create {
		if tenant.Status.Phase != v1.TenantTerminating {
			return nil
		}

		// TODO: This should probably not be a 403
		return admission.NewForbidden(a, fmt.Errorf("unable to create new content in tenant %s because it is being terminated", a.GetTenant()))
	}

	return nil
}

func (l *Lifecycle) namespaceLifecycleAdmit(a admission.Attributes) error {
	if len(a.GetNamespace()) == 0 {
		return nil
	}
	var (
		exists bool
		err    error
	)

	namespace, err := l.namespaceLister.NamespacesWithMultiTenancy(a.GetTenant()).Get(a.GetNamespace())
	if err != nil {
		if !errors.IsNotFound(err) {
			return errors.NewInternalError(err)
		}
	} else {
		exists = true
	}

	if !exists && a.GetOperation() == admission.Create {
		// give the cache time to observe the namespace before rejecting a create.
		// this helps when creating a namespace and immediately creating objects within it.
		time.Sleep(missingTenantNamespaceWait)
		namespace, err = l.namespaceLister.NamespacesWithMultiTenancy(a.GetTenant()).Get(a.GetNamespace())
		switch {
		case errors.IsNotFound(err):
			// no-op
		case err != nil:
			return errors.NewInternalError(err)
		default:
			exists = true
		}
		if exists {
			klog.V(4).Infof("found namespace %s/%s in cache after waiting", a.GetTenant(), a.GetNamespace())
		}
	}

	// forceLiveNamespaceLookup if true will skip looking at local cache state and instead always make a live call to server.
	forceLiveNamespaceLookup := false
	if _, ok := l.forceLiveNamespaceLookupCache.Get(a.GetTenant() + "/" + a.GetNamespace()); ok {
		// we think the namespace was marked for deletion, but our current local cache says otherwise, we will force a live lookup.
		forceLiveNamespaceLookup = exists && namespace.Status.Phase == v1.NamespaceActive
	}

	// refuse to operate on non-existent namespaces
	if !exists || forceLiveNamespaceLookup {
		// as a last resort, make a call directly to storage
		namespace, err = l.client.CoreV1().NamespacesWithMultiTenancy(a.GetTenant()).Get(a.GetNamespace(), metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
			return err
		case err != nil:
			return errors.NewInternalError(err)
		}
		klog.V(4).Infof("found %s via storage lookup", a.GetNamespace())
	}

	// ensure that we're not trying to create objects in terminating namespaces
	if a.GetOperation() == admission.Create {
		if namespace.Status.Phase != v1.NamespaceTerminating {
			return nil
		}

		// TODO: This should probably not be a 403
		return admission.NewForbidden(a, fmt.Errorf("unable to create new content in namespace %s/%s because it is being terminated", a.GetTenant(), a.GetNamespace()))
	}

	return nil
}

// Admit makes an admission decision based on the request attributes
func (l *Lifecycle) Admit(a admission.Attributes, o admission.ObjectInterfaces) error {

	if a.GetKind().GroupKind() == v1.SchemeGroupVersion.WithKind("Tenant").GroupKind() {
		// if a tenant is deleted, we want to prevent all further creates into it
		// while it is undergoing termination.  to reduce incidences where the cache
		// is slow to update, we add the tenant into a force live lookup list to ensure
		// we are not looking at stale state.
		if a.GetOperation() == admission.Delete {
			// prevent deletion of immortal tenants
			if l.immortalTenants.Has(a.GetName()) {
				return errors.NewForbidden(a.GetResource().GroupResource(), a.GetName(), fmt.Errorf("this tenant may not be deleted"))
			}
			l.forceLiveTenantLookupCache.Add(a.GetName(), true, forceLiveLookupTTL)
		}
		// allow all operations to tenants
		return nil
	}

	// always allow cluster-scoped resources if it is not "tenant"
	if len(a.GetTenant()) == 0 {
		return nil
	}

	if a.GetKind().GroupKind() == v1.SchemeGroupVersion.WithKind("Namespace").GroupKind() {
		// if a namespace is deleted, we want to prevent all further creates into it
		// while it is undergoing termination.  to reduce incidences where the cache
		// is slow to update, we add the namespace into a force live lookup list to ensure
		// we are not looking at stale state.
		if a.GetOperation() == admission.Delete {
			if l.immortalNamespaces.Has(a.GetName()) {
				return errors.NewForbidden(a.GetResource().GroupResource(), a.GetName(), fmt.Errorf("this namespace may not be deleted"))
			}
			l.forceLiveNamespaceLookupCache.Add(a.GetTenant()+"/"+a.GetName(), true, forceLiveLookupTTL)
		}
		// allow all operations to namespaces
		return nil
	}

	// always allow deletion of other resources
	if a.GetOperation() == admission.Delete {
		return nil
	}

	// always allow access review checks.  Returning status about the namespace would be leaking information
	if isAccessReview(a) {
		return nil
	}

	// we need to wait for our caches to warm
	if !l.WaitForReady() {
		return admission.NewForbidden(a, fmt.Errorf("not yet ready to handle request"))
	}

	// check the tenant is not terminating
	if err := l.tenantLifecycleAdmit(a); err != nil {
		return err
	}

	// check the namespace is not terminating
	if err := l.namespaceLifecycleAdmit(a); err != nil {
		return err
	}

	return nil
}

// NewLifecycle creates a new namespace Lifecycle admission control handler
func NewLifecycle(immortalNamespaces sets.String, immortalTenants sets.String) (*Lifecycle, error) {
	return newLifecycleWithClock(immortalNamespaces, immortalTenants, clock.RealClock{})
}

func newLifecycleWithClock(immortalNamespaces sets.String, immortalTenants sets.String, clock utilcache.Clock) (*Lifecycle, error) {
	forceLiveNamespaceLookupCache := utilcache.NewLRUExpireCacheWithClock(100, clock)
	forceLiveTenantLookupCache := utilcache.NewLRUExpireCacheWithClock(100, clock)
	return &Lifecycle{
		Handler:                       admission.NewHandler(admission.Create, admission.Update, admission.Delete),
		immortalNamespaces:            immortalNamespaces,
		forceLiveNamespaceLookupCache: forceLiveNamespaceLookupCache,
		immortalTenants:               immortalTenants,
		forceLiveTenantLookupCache:    forceLiveTenantLookupCache,
	}, nil
}

// SetExternalKubeInformerFactory implements the WantsExternalKubeInformerFactory interface.
func (l *Lifecycle) SetExternalKubeInformerFactory(f informers.SharedInformerFactory) {
	tenantInformer := f.Core().V1().Tenants()
	l.tenantLister = tenantInformer.Lister()
	l.SetReadyFunc(tenantInformer.Informer().HasSynced)

	namespaceInformer := f.Core().V1().Namespaces()
	l.namespaceLister = namespaceInformer.Lister()
	l.SetReadyFunc(namespaceInformer.Informer().HasSynced)
}

// SetExternalKubeClientSet implements the WantsExternalKubeClientSet interface.
func (l *Lifecycle) SetExternalKubeClientSet(client kubernetes.Interface) {
	l.client = client
}

// ValidateInitialization implements the InitializationValidator interface.
func (l *Lifecycle) ValidateInitialization() error {
	if l.tenantLister == nil {
		return fmt.Errorf("missing tenantLister")
	}
	if l.namespaceLister == nil {
		return fmt.Errorf("missing namespaceLister")
	}
	if l.client == nil {
		return fmt.Errorf("missing client")
	}
	return nil
}

// accessReviewResources are resources which give a view into permissions in a namespace.  Users must be allowed to create these
// resources because returning "not found" errors allows someone to search for the "people I'm going to fire in 2017" namespace.
var accessReviewResources = map[schema.GroupResource]bool{
	{Group: "authorization.k8s.io", Resource: "localsubjectaccessreviews"}: true,
}

func isAccessReview(a admission.Attributes) bool {
	return accessReviewResources[a.GetResource().GroupResource()]
}
