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
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/grafov/bcast"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/apiserverupdate"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

const (
	KubernetesServiceName      = "kubernetes"
	Namespace_System           = "default"
	startUpResetDelayInSeconds = 30 * time.Second
)

var instance *APIServerConfigManager
var muxCreateInstance sync.Mutex

var SyncApiServerConfigHandler = syncApiServerConfig
var setApiServerConfigMapHandler = setApiServerConfigMap
var setAPIServerConfigHandler = apiserverupdate.SetAPIServerConfig
var sendUpdateMessageHandler = sendUpdateMessage
var startWaitForCompleteHandler = apiserverupdate.GetClientSetsWatcher().StartWaitingForComplete

type APIServerConfigManager struct {
	apiserverListerSynced cache.InformerSynced
	apiserverLister       corelisters.EndpointsLister
	kubeClient            clientset.Interface
	recorder              record.EventRecorder

	isApiServerConfigInitialized bool
	APIServerMap                 map[string]v1.EndpointSubset
	rev                          int

	updateChGrp *bcast.Group

	mux             sync.RWMutex
	firstUpdateTime time.Time
}

// Use to mock APIServerConfigManager in integration tests
func GetAPIServerConfigManagerMock() *APIServerConfigManager {
	if instance != nil {
		return instance
	}

	muxCreateInstance.Lock()
	defer muxCreateInstance.Unlock()
	if instance != nil {
		return instance
	}

	manager := &APIServerConfigManager{
		isApiServerConfigInitialized: true,
	}

	SyncApiServerConfigHandler = mockApiServerConfigSync
	instance = manager
	return instance
}

func mockApiServerConfigSync(a *APIServerConfigManager) error {
	return nil
}

func NewAPIServerConfigManagerWithInformer(epInformer coreinformers.EndpointsInformer, kubeClient clientset.Interface) (*APIServerConfigManager, error) {
	klog.Infof("NewAPIServerConfigManagerWithInformer instance pointer %p", instance)
	if instance != nil {
		return instance, nil
	}

	muxCreateInstance.Lock()
	defer muxCreateInstance.Unlock()
	if instance != nil {
		return instance, nil
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	/*if kubeClient != nil && kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		metrics.RegisterMetricAndTrackRateLimiterUsage("api_server_config_manager", kubeClient.CoreV1().RESTClient().GetRateLimiter())
	}
	*/

	manager := &APIServerConfigManager{
		kubeClient:                   kubeClient,
		isApiServerConfigInitialized: false,
		APIServerMap:                 make(map[string]v1.EndpointSubset),
		updateChGrp:                  apiserverupdate.GetAPIServerConfigUpdateChGrp(),
	}

	// Endpoint for api server should already existed. No need to listen for add event
	epInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: manager.updateApiServer,
		DeleteFunc: manager.deleteApiServer,
	})

	manager.apiserverLister = epInformer.Lister()
	manager.apiserverListerSynced = epInformer.Informer().HasSynced

	stopRetryTime := time.Now().Add(5 * time.Minute)
	for {
		err := SyncApiServerConfigHandler(manager)
		if err == nil {
			break
		}
		if time.Now().After(stopRetryTime) {
			return nil, err
		}
		time.Sleep(10 * time.Second)
	}

	SyncApiServerConfigHandler = syncApiServerConfig

	instance = manager
	return instance, nil
}

func (a *APIServerConfigManager) Run(stopCh <-chan struct{}) {
	klog.Info("Starting api server config manager")
	defer klog.Info("Shutting down api server config manager")

	if a.kubeClient == nil {
		return
	}
	if !cache.WaitForCacheSync(stopCh, a.apiserverListerSynced) {
		klog.Info("Api service end points NOT synced.")
		return
	}

	klog.Infof("Caches are synced for api server end points. [%+v]", a.APIServerMap)
	<-stopCh
}

// TODO - check updating from others
func (a *APIServerConfigManager) GetAPIServerConfig() map[string]v1.EndpointSubset {
	a.mux.RLock()
	defer a.mux.RUnlock()
	return a.APIServerMap
}

func syncApiServerConfig(a *APIServerConfigManager) error {
	apiEndpoints, err := a.kubeClient.CoreV1().Endpoints(Namespace_System).Get(KubernetesServiceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Error in getting api server endpoints list: %v", err)
		return err
	} else if apiEndpoints == nil {
		klog.Fatalf("Cannot get %s endpoints: %v", KubernetesServiceName, err)
	}
	klog.V(3).Infof("Api server end points [%+v]", apiEndpoints.Subsets)

	// TODO - currently assume the first endpoint for each service group id is load balancer,
	// connect to the load balancer of the service group cluster

	a.rev, _ = strconv.Atoi(apiEndpoints.ResourceVersion)
	a.firstUpdateTime = time.Now().Add(startUpResetDelayInSeconds)
	klog.Infof("Set first update api server to time %v", a.firstUpdateTime)
	setApiServerConfigMapHandler(a, apiEndpoints)
	return nil
}

func (a *APIServerConfigManager) updateApiServer(old, cur interface{}) {
	curEp := cur.(*v1.Endpoints)
	oldEp := old.(*v1.Endpoints)
	if !isApiServerEndpoint(curEp) || !isApiServerEndpoint(oldEp) {
		return
	}

	// compare objs received in events
	if curEp.ResourceVersion == oldEp.ResourceVersion {
		return
	} else {
		oldRev, _ := strconv.Atoi(oldEp.ResourceVersion)
		newRev, err := strconv.Atoi(curEp.ResourceVersion)
		if err != nil {
			klog.Errorf("Got invalid resource version %s for endpoint %v", curEp.ResourceVersion, KubernetesServiceName)
			return
		}

		if newRev < oldRev {
			klog.V(3).Infof("Got staled endpoint %s in updateFunc. Existing version %s, new instance version %v", KubernetesServiceName, oldEp.ResourceVersion, curEp.ResourceVersion)
			return
		}
	}

	if reflect.DeepEqual(oldEp.Subsets, curEp.Subsets) {
		return
	}

	setApiServerConfigMapHandler(a, curEp)
}

// It's ok not to test here since Kubernetes endpoints should never be deleted
func (a *APIServerConfigManager) deleteApiServer(obj interface{}) {
	ep, ok := obj.(*v1.Endpoints)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v.", obj))
			return
		}
		ep, ok = tombstone.Obj.(*v1.Endpoints)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not an end point instance %#v.", obj))
			return
		}
	}
	if !isApiServerEndpoint(ep) {
		return
	}

	klog.Warningf("Received event for deleting %s end point", KubernetesServiceName)
	// TODO - how to deal with integration test running in parrallel
}

func setApiServerConfigMap(a *APIServerConfigManager, ep *v1.Endpoints) {
	a.mux.Lock()
	klog.V(4).Info("mux acquired setApiServerConfigMap.")

	hasUpdate := false

	existingServiceGroupIds := make(map[string]bool)
	// add new or update existing service groups
	for _, ss := range ep.Subsets {
		_, isOK := a.APIServerMap[ss.ServiceGroupId]
		if isOK && !reflect.DeepEqual(a.APIServerMap[ss.ServiceGroupId], ss) {
			klog.V(4).Infof("Got additional api service endpoints [%+v] for service group %s", ss, ss.ServiceGroupId)
		}

		a.APIServerMap[ss.ServiceGroupId] = ss
		existingServiceGroupIds[ss.ServiceGroupId] = true
		hasUpdate = true
	}
	// remove staled service groups
	for oldServiceGroupId := range a.APIServerMap {
		if ss, isOK := existingServiceGroupIds[oldServiceGroupId]; !isOK {
			klog.V(4).Infof("Removed staled service group %s with endpoints [%+v]", oldServiceGroupId, ss)
			delete(a.APIServerMap, oldServiceGroupId)
			hasUpdate = true
		}
	}

	a.mux.Unlock()
	klog.V(4).Info("mux released setApiServerConfigMap.")

	if hasUpdate {
		// wait 30 (defined in startUpResetDelayInSeconds) second after start to send update message
		//   - otherwise, update api server config during start up might cause issue
		go func(a *APIServerConfigManager) {
			now := time.Now()
			if a.firstUpdateTime.After(now) {
				time.Sleep(a.firstUpdateTime.Sub(now))
			}

			a.mux.Lock()
			klog.V(4).Info("mux acquired setApiServerConfigMap.")
			defer func() {
				a.mux.Unlock()
				klog.V(4).Info("mux released setApiServerConfigMap.")
			}()

			// No need to reset config if there is only one server as it is already connected
			if !a.isApiServerConfigInitialized {
				a.isApiServerConfigInitialized = true

				if len(a.APIServerMap) == 1 {
					return
				}
			}

			isUpdated := setAPIServerConfigHandler(a.APIServerMap)
			if !isUpdated {
				return
			}

			startWaitForCompleteHandler()
			sendUpdateMessageHandler(a)
		}(a)
	}
}

func sendUpdateMessage(a *APIServerConfigManager) {
	go a.updateChGrp.Send("API server config updated")
}

func isApiServerEndpoint(ep *v1.Endpoints) bool {
	if ep != nil && ep.Name == KubernetesServiceName && ep.Namespace == Namespace_System {
		return true
	}

	return false
}

func StartAPIServerConfigManager(endpointsInformer coreinformers.EndpointsInformer, client clientset.Interface, stopCh <-chan struct{}) (bool, error) {
	apiServerConfigManager, err := NewAPIServerConfigManagerWithInformer(
		endpointsInformer, client)
	if err != nil {
		// TODO - check whether need to add retry
		klog.Fatalf("Error getting api server endpoints. Error %v", err)
	}
	go apiServerConfigManager.Run(stopCh)

	return true, nil
}

func StartAPIServerConfigManagerAndInformerFactory(client clientset.Interface, stopCh <-chan struct{}) {
	fieldSelector := fields.Set{"metadata.name": types.KubernetesServiceName,
		"metadata.namespace": v1.NamespaceDefault,
		"metadata.tenant":    v1.TenantSystem}.AsSelector().String()
	informerFactory := informers.NewSharedInformerFactoryWithOptions(client, 10*time.Minute,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fieldSelector
		}))
	for {
		apiServerConfigManager, err := NewAPIServerConfigManagerWithInformer(
			informerFactory.Core().V1().Endpoints(), client)
		if err == nil {
			go apiServerConfigManager.Run(stopCh)
			informerFactory.Start(stopCh)
			return
		} else {
			klog.Errorf("Error creating api server config manager. %v. Retry after 10 seconds", err)
			time.Sleep(10 * time.Second)
		}
	}
}
