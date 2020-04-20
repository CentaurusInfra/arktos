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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/apiserverupdate"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/metrics"
	"reflect"
	"strconv"
	"sync"
	"time"
)

const (
	KubernetesServiceName      = "kubernetes"
	Namespace_System           = "default"
	startUpResetDelayInSeconds = 30 * time.Second
)

var instance *APIServerConfigManager

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

func GetAPIServerConfigManager() *APIServerConfigManager {
	return instance
}

func checkInstanceExistence() {
	if instance != nil {
		klog.Fatalf("Unexpected reference to api server config manager - initialized")
	}
}

func NewAPIServerConfigManager(kubeClient clientset.Interface) *APIServerConfigManager {
	manager := &APIServerConfigManager{
		kubeClient:                   kubeClient,
		isApiServerConfigInitialized: false,
		APIServerMap:                 make(map[string]v1.EndpointSubset),
	}

	err := manager.syncApiServerConfig()
	if err != nil {
		klog.Fatalf("Unable to get api server list from registry. Error %v", err)
	}

	instance = manager
	return instance
}

func NewAPIServerConfigManagerWithInformer(epInformer coreinformers.EndpointsInformer, kubeClient clientset.Interface) *APIServerConfigManager {
	checkInstanceExistence()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().EventsWithMultiTenancy("", "")})

	if kubeClient != nil && kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		metrics.RegisterMetricAndTrackRateLimiterUsage("api_server_config_manager", kubeClient.CoreV1().RESTClient().GetRateLimiter())
	}

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
	err := manager.syncApiServerConfig()
	if err != nil {
		klog.Fatalf("Unable to get api server list from registry. Error %v", err)
	}

	instance = manager
	return instance
}

func (a *APIServerConfigManager) Run(stopCh <-chan struct{}) {
	klog.Info("Starting api server config manager")
	defer klog.Info("Shutting down api server config manager")

	if !cache.WaitForCacheSync(stopCh, a.apiserverListerSynced) {
		klog.Info("Api service end points NOT synced %v.")
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

func (a *APIServerConfigManager) syncApiServerConfig() error {
	a.mux.Lock()
	klog.V(4).Info("mux acquired syncApiServerConfig.")
	defer func() {
		a.mux.Unlock()
		klog.V(4).Info("mux released syncApiServerConfig.")
	}()

	apiEndpoints, err := a.kubeClient.CoreV1().Endpoints(Namespace_System).Get(KubernetesServiceName, metav1.GetOptions{})
	if err != nil {
		klog.Fatalf("Error in getting api server endpoints list: %v", err)
	} else if apiEndpoints == nil {
		klog.Fatalf("Cannot get %s endpoints: %v", KubernetesServiceName, err)
	}
	klog.V(3).Infof("Api server end points [%+v]", apiEndpoints.Subsets)

	// TODO - currently assume the first endpoint for each service group id is load balancer,
	// connect to the load balancer of the service group cluster

	a.rev, _ = strconv.Atoi(apiEndpoints.ResourceVersion)
	a.firstUpdateTime = time.Now().Add(startUpResetDelayInSeconds)
	klog.Infof("Set first update api server to time %v", a.firstUpdateTime)
	a.setApiServerConfig(apiEndpoints)
	return nil
}

func (a *APIServerConfigManager) updateApiServer(old, cur interface{}) {
	curEp := cur.(*v1.Endpoints)
	oldEp := old.(*v1.Endpoints)
	if !isApiServerEndpoint(curEp) || !isApiServerEndpoint(oldEp) {
		return
	}

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
		}
	}

	if reflect.DeepEqual(oldEp.Subsets, curEp.Subsets) {
		return
	}

	a.mux.Lock()
	klog.V(4).Infof("mux acquired updateApiServer")
	defer func() {
		a.mux.Unlock()
		klog.V(4).Infof("mux released updateApiServer")
	}()

	a.setApiServerConfig(curEp)
}

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

	klog.Warningf("Received event for deleting % end point", KubernetesServiceName)
	// TODO - how to deal with integration test running in parrallel
}

func (a *APIServerConfigManager) setApiServerConfig(ep *v1.Endpoints) {
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

	if hasUpdate {
		// wait 30 (defined in startUpResetDelayInSeconds) second after start to send update message
		//   - otherwise, update api server config during start up might cause issue
		go func(firstUpdateTime time.Time) {
			now := time.Now()
			if firstUpdateTime.After(now) {
				time.Sleep(firstUpdateTime.Sub(now))
			}

			apiserverupdate.SetAPIServerConfig(a.APIServerMap)

			// No need to reset config if there is only one server as it is already connected
			if !a.isApiServerConfigInitialized {
				a.isApiServerConfigInitialized = true

				if len(a.APIServerMap) == 1 {
					return
				}
			}

			go a.updateChGrp.Send("API server config updated")
			apiserverupdate.GetClientSetsWatcher().StartWaitingForComplete()
		}(a.firstUpdateTime)
	}
}

func isApiServerEndpoint(ep *v1.Endpoints) bool {
	if ep != nil && ep.Name == KubernetesServiceName && ep.Namespace == Namespace_System {
		return true
	}

	return false
}
