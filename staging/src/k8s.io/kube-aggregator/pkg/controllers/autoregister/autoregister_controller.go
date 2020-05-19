/*
Copyright 2017 The Kubernetes Authors.
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

package autoregister

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"k8s.io/klog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/kube-aggregator/pkg/apis/apiregistration"
	apiregistrationclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/internalclientset/typed/apiregistration/internalversion"
	informers "k8s.io/kube-aggregator/pkg/client/informers/internalversion/apiregistration/internalversion"
	listers "k8s.io/kube-aggregator/pkg/client/listers/apiregistration/internalversion"
	"k8s.io/kube-aggregator/pkg/controllers"
)

const (
	// AutoRegisterManagedLabel is a label attached to the APIService that identifies how the APIService wants to be synced.
	AutoRegisterManagedLabel = "kube-aggregator.kubernetes.io/automanaged"

	// manageOnStart is a value for the AutoRegisterManagedLabel that indicates the APIService wants to be synced one time when the controller starts.
	manageOnStart = "onstart"
	// manageContinuously is a value for the AutoRegisterManagedLabel that indicates the APIService wants to be synced continuously.
	manageContinuously = "true"
)

// AutoAPIServiceRegistration is an interface which callers can re-declare locally and properly cast to for
// adding and removing APIServices
type AutoAPIServiceRegistration interface {
	// AddAPIServiceToSyncOnStart adds an API service to sync on start.
	AddAPIServiceToSyncOnStart(in *apiregistration.APIService)
	// AddAPIServiceToSync adds an API service to sync continuously.
	AddAPIServiceToSync(in *apiregistration.APIService)
	// RemoveAPIServiceToSync removes an API service to auto-register.
	RemoveAPIServiceToSync(tenantedApiServiceName string)
}

// autoRegisterController is used to keep a particular set of APIServices present in the API.  It is useful
// for cases where you want to auto-register APIs like TPRs or groups from the core kube-apiserver
type autoRegisterController struct {
	apiServiceLister listers.APIServiceLister
	apiServiceSynced cache.InformerSynced
	apiServiceClient apiregistrationclient.APIServicesGetter

	apiServicesToSyncLock sync.RWMutex
	apiServicesToSync     map[string]*apiregistration.APIService

	syncHandler func(apiServiceName string) error

	// track which services we have synced
	syncedSuccessfullyLock *sync.RWMutex
	syncedSuccessfully     map[string]bool

	// remember names of services that existed when we started
	apiServicesAtStart map[string]bool

	// queue is where incoming work is placed to de-dup and to allow "easy" rate limited requeues on errors
	queue workqueue.RateLimitingInterface
}

// NewAutoRegisterController creates a new autoRegisterController.
func NewAutoRegisterController(apiServiceInformer informers.APIServiceInformer, apiServiceClient apiregistrationclient.APIServicesGetter) *autoRegisterController {
	c := &autoRegisterController{
		apiServiceLister:  apiServiceInformer.Lister(),
		apiServiceSynced:  apiServiceInformer.Informer().HasSynced,
		apiServiceClient:  apiServiceClient,
		apiServicesToSync: map[string]*apiregistration.APIService{},

		apiServicesAtStart: map[string]bool{},

		syncedSuccessfullyLock: &sync.RWMutex{},
		syncedSuccessfully:     map[string]bool{},

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "autoregister"),
	}
	c.syncHandler = c.checkAPIService

	apiServiceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cast := obj.(*apiregistration.APIService)
			c.queue.Add(NameWithMultiTenancy(cast))
		},
		UpdateFunc: func(_, obj interface{}) {
			cast := obj.(*apiregistration.APIService)
			c.queue.Add(NameWithMultiTenancy(cast))
		},
		DeleteFunc: func(obj interface{}) {
			cast, ok := obj.(*apiregistration.APIService)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					klog.V(2).Infof("Couldn't get object from tombstone %#v", obj)
					return
				}
				cast, ok = tombstone.Obj.(*apiregistration.APIService)
				if !ok {
					klog.V(2).Infof("Tombstone contained unexpected object: %#v", obj)
					return
				}
			}
			c.queue.Add(NameWithMultiTenancy(cast))
		},
	})

	return c
}

// Run starts the autoregister controller in a loop which syncs API services until stopCh is closed.
func (c *autoRegisterController) Run(threadiness int, stopCh <-chan struct{}) {
	// don't let panics crash the process
	defer utilruntime.HandleCrash()
	// make sure the work queue is shutdown which will trigger workers to end
	defer c.queue.ShutDown()

	klog.Infof("Starting autoregister controller")
	defer klog.Infof("Shutting down autoregister controller")

	// wait for your secondary caches to fill before starting your work
	if !controllers.WaitForCacheSync("autoregister", stopCh, c.apiServiceSynced) {
		return
	}

	// record APIService objects that existed when we started
	if services, err := c.apiServiceLister.List(labels.Everything()); err == nil {
		for _, service := range services {
			c.apiServicesAtStart[service.Name] = true
		}
	}

	// start up your worker threads based on threadiness.  Some controllers have multiple kinds of workers
	for i := 0; i < threadiness; i++ {
		// runWorker will loop until "something bad" happens.  The .Until will then rekick the worker
		// after one second
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	// wait until we're told to stop
	<-stopCh
}

func (c *autoRegisterController) runWorker() {
	// hot loop until we're told to stop.  processNextWorkItem will automatically wait until there's work
	// available, so we don't worry about secondary waits
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false when it's time to quit.
func (c *autoRegisterController) processNextWorkItem() bool {
	// pull the next work item from queue.  It should be a key we use to lookup something in a cache
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// you always have to indicate to the queue that you've completed a piece of work
	defer c.queue.Done(key)

	// do your work on the key.  This method will contains your "do stuff" logic
	err := c.syncHandler(key.(string))
	if err == nil {
		// if you had no error, tell the queue to stop tracking history for your key.  This will
		// reset things like failure counts for per-item rate limiting
		c.queue.Forget(key)
		return true
	}

	// there was a failure so be sure to report it.  This method allows for pluggable error handling
	// which can be used for things like cluster-monitoring
	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", key, err))
	// since we failed, we should requeue the item to work on later.  This method will add a backoff
	// to avoid hotlooping on particular items (they're probably still not going to work right away)
	// and overall controller protection (everything I've done is broken, this controller needs to
	// calm down or it can starve other useful work) cases.
	c.queue.AddRateLimited(key)

	return true
}

// checkAPIService syncs the current APIService against a list of desired APIService objects
//
//                                                 | A. desired: not found | B. desired: sync on start | C. desired: sync always
// ------------------------------------------------|-----------------------|---------------------------|------------------------
// 1. current: lookup error                        | error                 | error                     | error
// 2. current: not found                           | -                     | create once               | create
// 3. current: no sync                             | -                     | -                         | -
// 4. current: sync on start, not present at start | -                     | -                         | -
// 5. current: sync on start, present at start     | delete once           | update once               | update once
// 6. current: sync always                         | delete                | update once               | update
func (c *autoRegisterController) checkAPIService(tenantedApiServiceName string) (err error) {
	desired := c.GetAPIServiceToSync(tenantedApiServiceName)
	tenant, serviceName := ResolveNameWithMultiTenancy(tenantedApiServiceName)
	curr, err := c.apiServiceLister.APIServicesWithMultiTenancy(tenant).Get(serviceName)

	// if we've never synced this service successfully, record a successful sync.
	hasSynced := c.hasSyncedSuccessfully(tenantedApiServiceName)
	if !hasSynced {
		defer func() {
			if err == nil {
				c.setSyncedSuccessfully(tenantedApiServiceName)
			}
		}()
	}

	switch {
	// we had a real error, just return it (1A,1B,1C)
	case err != nil && !apierrors.IsNotFound(err):
		return err

	// we don't have an entry and we don't want one (2A)
	case apierrors.IsNotFound(err) && desired == nil:
		return nil

	// the local object only wants to sync on start and has already synced (2B,5B,6B "once" enforcement)
	case isAutomanagedOnStart(desired) && hasSynced:
		return nil

	// we don't have an entry and we do want one (2B,2C)
	case apierrors.IsNotFound(err) && desired != nil:
		_, err := c.apiServiceClient.APIServicesWithMultiTenancy(tenant).Create(desired)
		if apierrors.IsAlreadyExists(err) {
			// created in the meantime, we'll get called again
			return nil
		}
		return err

	// we aren't trying to manage this APIService (3A,3B,3C)
	case !isAutomanaged(curr):
		return nil

	// the remote object only wants to sync on start, but was added after we started (4A,4B,4C)
	case isAutomanagedOnStart(curr) && !c.apiServicesAtStart[tenantedApiServiceName]:
		return nil

	// the remote object only wants to sync on start and has already synced (5A,5B,5C "once" enforcement)
	case isAutomanagedOnStart(curr) && hasSynced:
		return nil

	// we have a spurious APIService that we're managing, delete it (5A,6A)
	case desired == nil:
		opts := &metav1.DeleteOptions{Preconditions: metav1.NewUIDPreconditions(string(curr.UID))}
		err := c.apiServiceClient.APIServicesWithMultiTenancy(tenant).Delete(curr.Name, opts)
		if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
			// deleted or changed in the meantime, we'll get called again
			return nil
		}
		return err

	// if the specs already match, nothing for us to do
	case reflect.DeepEqual(curr.Spec, desired.Spec):
		return nil
	}

	// we have an entry and we have a desired, now we deconflict.  Only a few fields matter. (5B,5C,6B,6C)
	apiService := curr.DeepCopy()
	apiService.Spec = desired.Spec
	_, err = c.apiServiceClient.APIServicesWithMultiTenancy(tenant).Update(apiService)
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		// deleted or changed in the meantime, we'll get called again
		return nil
	}
	return err
}

// GetAPIServiceToSync gets a single API service to sync.
func (c *autoRegisterController) GetAPIServiceToSync(tenantedApiServiceName string) *apiregistration.APIService {
	c.apiServicesToSyncLock.RLock()
	defer c.apiServicesToSyncLock.RUnlock()

	return c.apiServicesToSync[tenantedApiServiceName]
}

// AddAPIServiceToSyncOnStart registers an API service to sync only when the controller starts.
func (c *autoRegisterController) AddAPIServiceToSyncOnStart(in *apiregistration.APIService) {
	c.addAPIServiceToSync(in, manageOnStart)
}

// AddAPIServiceToSync registers an API service to sync continuously.
func (c *autoRegisterController) AddAPIServiceToSync(in *apiregistration.APIService) {
	c.addAPIServiceToSync(in, manageContinuously)
}

func NameWithMultiTenancy(apiService *apiregistration.APIService) string {
	return apiService.Tenant + "/" + apiService.Name
}

func GroupWithMultiTenancy(apiService *apiregistration.APIService) string {
	return apiService.Tenant + "/" + apiService.Spec.Group
}

func ResolveNameWithMultiTenancy(tenantedApiServiceName string) (string, string) {
	// note that tenantedApiServiceName always contains a "/"
	i := strings.Index(tenantedApiServiceName, "/")
	return tenantedApiServiceName[:i], tenantedApiServiceName[i+1:]
}

func (c *autoRegisterController) addAPIServiceToSync(in *apiregistration.APIService, syncType string) {
	c.apiServicesToSyncLock.Lock()
	defer c.apiServicesToSyncLock.Unlock()

	apiService := in.DeepCopy()
	if apiService.Labels == nil {
		apiService.Labels = map[string]string{}
	}
	apiService.Labels[AutoRegisterManagedLabel] = syncType

	tenantedApiServiceName := NameWithMultiTenancy(apiService)
	c.apiServicesToSync[tenantedApiServiceName] = apiService
	c.queue.Add(tenantedApiServiceName)
}

// RemoveAPIServiceToSync deletes a registered APIService.
func (c *autoRegisterController) RemoveAPIServiceToSync(tenantedApiServiceName string) {
	c.apiServicesToSyncLock.Lock()
	defer c.apiServicesToSyncLock.Unlock()

	delete(c.apiServicesToSync, tenantedApiServiceName)
	c.queue.Add(tenantedApiServiceName)
}

func (c *autoRegisterController) hasSyncedSuccessfully(tenantedApiServiceName string) bool {
	c.syncedSuccessfullyLock.RLock()
	defer c.syncedSuccessfullyLock.RUnlock()
	return c.syncedSuccessfully[tenantedApiServiceName]
}

func (c *autoRegisterController) setSyncedSuccessfully(tenantedApiServiceName string) {
	c.syncedSuccessfullyLock.Lock()
	defer c.syncedSuccessfullyLock.Unlock()
	c.syncedSuccessfully[tenantedApiServiceName] = true
}

func automanagedType(service *apiregistration.APIService) string {
	if service == nil {
		return ""
	}
	return service.Labels[AutoRegisterManagedLabel]
}

func isAutomanagedOnStart(service *apiregistration.APIService) bool {
	return automanagedType(service) == manageOnStart
}

func isAutomanaged(service *apiregistration.APIService) bool {
	managedType := automanagedType(service)
	return managedType == manageOnStart || managedType == manageContinuously
}
