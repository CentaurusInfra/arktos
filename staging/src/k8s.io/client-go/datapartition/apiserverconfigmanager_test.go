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
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/apiserverupdate"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"sync"
	"testing"
	"time"
)

const (
	masterIP1 = "192.168.1.1"
	masterIP2 = "192.168.1.2"

	serviceGroupId1 = "1"
	serviceGroupId2 = "2"
)

var callNum_SetApiServerConfigMap int
var callNum_ExternalSetAPIServerConfig int
var callNum_StartWaitForComplete int
var callNum_SendUpdateMessage int

var testLock sync.Mutex

func newEndpoint(endpointName string, rev string, masterIP string, serviceGroupId string) *v1.Endpoints {
	ep := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            endpointName,
			Namespace:       Namespace_System,
			ResourceVersion: rev,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP: masterIP,
					},
				},
				ServiceGroupId: serviceGroupId,
			},
		},
	}

	return ep
}

func mockSetApiServerConfigMap(a *APIServerConfigManager, ep *v1.Endpoints) {
	callNum_SetApiServerConfigMap++
}

func mockExternalSetAPIServerConfig(c map[string]v1.EndpointSubset) bool {
	callNum_ExternalSetAPIServerConfig++
	return true
}

func mockStartWaitForComplete() {
	callNum_StartWaitForComplete++
}

func mockSendUpdateMessage(a *APIServerConfigManager) {
	callNum_SendUpdateMessage++
}

func TestGetAPIServerConfigManagerMock(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()

	if instance == nil {
		newInstance := GetAPIServerConfigManagerMock()
		assert.NotNil(t, newInstance)

		instance2 := GetAPIServerConfigManagerMock()
		assert.NotNil(t, instance2)

		assert.Equal(t, newInstance, instance2)

		assert.Nil(t, instance.kubeClient)
		instance = nil
	}

	SyncApiServerConfigHandler = syncApiServerConfig
}

func TestUpdateEvent(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()

	apiServerConfigManager := GetAPIServerConfigManagerMock()
	setApiServerConfigMapHandler = mockSetApiServerConfigMap
	defer func() {
		setApiServerConfigMapHandler = setApiServerConfigMap
	}()

	t.Log("1. Check only Kubernetes endpoint update event will be picked up")
	oldEp := newEndpoint("foo", "1", masterIP1, serviceGroupId1)
	curEp := newEndpoint("foo", "2", masterIP2, serviceGroupId1)
	callNum_SetApiServerConfigMap = 0
	apiServerConfigManager.updateApiServer(oldEp, curEp)
	assert.Equal(t, 0, callNum_SetApiServerConfigMap)

	t.Log("2.1 Check only event with newer revision will be picked up")
	oldEp = newEndpoint(KubernetesServiceName, "3", masterIP1, serviceGroupId1)
	curEp = newEndpoint(KubernetesServiceName, "2", masterIP2, serviceGroupId1)
	apiServerConfigManager.updateApiServer(oldEp, curEp)
	assert.Equal(t, 0, callNum_SetApiServerConfigMap)

	t.Log("2.2 Check only event with newer revision will be picked up")
	oldEp = newEndpoint(KubernetesServiceName, "3", masterIP1, serviceGroupId1)
	curEp = newEndpoint(KubernetesServiceName, "3", masterIP2, serviceGroupId1)
	apiServerConfigManager.updateApiServer(oldEp, curEp)
	assert.Equal(t, 0, callNum_SetApiServerConfigMap)

	t.Log("3. Check only event with different endpoint value will be picked up")
	oldEp = newEndpoint(KubernetesServiceName, "3", masterIP1, serviceGroupId1)
	curEp = newEndpoint(KubernetesServiceName, "4", masterIP1, serviceGroupId1)
	apiServerConfigManager.updateApiServer(oldEp, curEp)
	assert.Equal(t, 0, callNum_SetApiServerConfigMap)

	t.Log("4. Check event with different endpoint will be picked up")
	oldEp = newEndpoint(KubernetesServiceName, "3", masterIP1, serviceGroupId1)
	curEp = newEndpoint(KubernetesServiceName, "4", masterIP2, serviceGroupId1)
	apiServerConfigManager.updateApiServer(oldEp, curEp)
	assert.Equal(t, 1, callNum_SetApiServerConfigMap)

	t.Logf("5. Check same endpoints update times won't be picked up")
	curEp = newEndpoint(KubernetesServiceName, "4", masterIP2, serviceGroupId1)
	oldEp = curEp
	callNum_SetApiServerConfigMap = 0
	apiServerConfigManager.updateApiServer(oldEp, curEp)
	assert.Equal(t, 0, callNum_SetApiServerConfigMap)
}

func TestSetApiServerConfigMap(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()

	apiServerConfigManager := GetAPIServerConfigManagerMock()
	setAPIServerConfigHandler = mockExternalSetAPIServerConfig
	startWaitForCompleteHandler = mockStartWaitForComplete
	sendUpdateMessageHandler = mockSendUpdateMessage
	defer func() {
		setAPIServerConfigHandler = apiserverupdate.SetAPIServerConfig
		startWaitForCompleteHandler = apiserverupdate.GetClientSetsWatcher().StartWaitingForComplete
	}()

	callNum_ExternalSetAPIServerConfig = 0
	callNum_StartWaitForComplete = 0

	t.Log("1.1 Check 1st update for single config will not call set server config or wait for complete")
	apiServerConfigManager.APIServerMap = make(map[string]v1.EndpointSubset)
	apiServerConfigManager.isApiServerConfigInitialized = false
	apiServerConfigManager.firstUpdateTime = time.Now()
	ep := newEndpoint(KubernetesServiceName, "10", masterIP1, serviceGroupId1)
	setApiServerConfigMap(apiServerConfigManager, ep)
	time.Sleep(100 * time.Millisecond) // wait for go routine to execute
	assert.Equal(t, 0, callNum_ExternalSetAPIServerConfig)
	assert.Equal(t, 0, callNum_StartWaitForComplete)
	assert.Equal(t, true, apiServerConfigManager.isApiServerConfigInitialized)
	assert.Equal(t, 1, len(apiServerConfigManager.APIServerMap))
	epCheck, isOK := apiServerConfigManager.APIServerMap[serviceGroupId1]
	assert.Equal(t, true, isOK)
	assert.Equal(t, 1, len(epCheck.Addresses))
	assert.Equal(t, masterIP1, epCheck.Addresses[0].IP)

	callNum_ExternalSetAPIServerConfig = 0
	callNum_StartWaitForComplete = 0
	callNum_SendUpdateMessage = 0
	t.Log("1.2 Check 1st update for double configs will call set server config and wait for complete")
	apiServerConfigManager.APIServerMap = make(map[string]v1.EndpointSubset)
	apiServerConfigManager.isApiServerConfigInitialized = false
	apiServerConfigManager.firstUpdateTime = time.Now()
	ep.Subsets = append(ep.Subsets, v1.EndpointSubset{
		Addresses:      []v1.EndpointAddress{{IP: masterIP2}},
		ServiceGroupId: serviceGroupId2,
	})
	setApiServerConfigMap(apiServerConfigManager, ep)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, callNum_ExternalSetAPIServerConfig)
	assert.Equal(t, 1, callNum_StartWaitForComplete)
	assert.Equal(t, true, apiServerConfigManager.isApiServerConfigInitialized)
	assert.Equal(t, 2, len(apiServerConfigManager.APIServerMap))
	epCheck, isOK = apiServerConfigManager.APIServerMap[serviceGroupId1]
	assert.Equal(t, true, isOK)
	assert.Equal(t, 1, len(epCheck.Addresses))
	assert.Equal(t, masterIP1, epCheck.Addresses[0].IP)
	epCheck, isOK = apiServerConfigManager.APIServerMap[serviceGroupId2]
	assert.Equal(t, true, isOK)
	assert.Equal(t, 1, len(epCheck.Addresses))
	assert.Equal(t, masterIP2, epCheck.Addresses[0].IP)

	t.Log("2. Check same api server configuration will not call set server config nor wait for complete")
	callNum_ExternalSetAPIServerConfig = 0
	callNum_StartWaitForComplete = 0
	callNum_SendUpdateMessage = 0
	newEp := ep.DeepCopy()
	newEp.ResourceVersion = "11"
	apiServerConfigManager.updateApiServer(ep, newEp)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, callNum_ExternalSetAPIServerConfig)
	assert.Equal(t, 0, callNum_StartWaitForComplete)
	assert.Equal(t, true, apiServerConfigManager.isApiServerConfigInitialized)
	assert.Equal(t, 2, len(apiServerConfigManager.APIServerMap))

	t.Log("3. Check remove one api server group will call set server config and wait for complete")
	newEp.Subsets = ep.Subsets[1:]
	newEp.ResourceVersion = "12"
	apiServerConfigManager.updateApiServer(ep, newEp)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, callNum_ExternalSetAPIServerConfig)
	assert.Equal(t, 1, callNum_StartWaitForComplete)
	assert.Equal(t, true, apiServerConfigManager.isApiServerConfigInitialized)
	assert.Equal(t, 1, len(apiServerConfigManager.APIServerMap))
	epCheck, isOK = apiServerConfigManager.APIServerMap[serviceGroupId2]
	assert.Equal(t, true, isOK)
	assert.Equal(t, 1, len(epCheck.Addresses))
	assert.Equal(t, masterIP2, epCheck.Addresses[0].IP)
}

func TestStartAPIServerConfigManager(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()
	instance = nil
	SyncApiServerConfigHandler = syncApiServerConfig

	client := fake.NewSimpleClientset()
	serviceGroupId := "0"
	ip := "1.2.3.4"
	ep := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: KubernetesServiceName},
		Subsets: []v1.EndpointSubset{
			{
				Addresses:      []v1.EndpointAddress{{IP: ip}},
				Ports:          nil,
				ServiceGroupId: serviceGroupId,
			},
		},
	}

	epCreated, err := client.CoreV1().Endpoints(Namespace_System).Create(ep)
	assert.Nil(t, err)
	assert.NotNil(t, epCreated)

	stopCh := make(chan struct{})
	defer close(stopCh)
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	result, err := StartAPIServerConfigManager(informerFactory.Core().V1().Endpoints(), client, stopCh)
	assert.True(t, result)
	assert.Nil(t, err)
	assert.NotNil(t, instance)

	epMap := instance.GetAPIServerConfig()
	assert.NotNil(t, epMap)
	assert.Equal(t, 1, len(epMap))
	assert.Equal(t, serviceGroupId, epMap[serviceGroupId].ServiceGroupId)
	assert.Equal(t, ip, epMap[serviceGroupId].Addresses[0].IP)
}

func TestStartAPIServerConfigManagerAndInformerFactory(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()
	instance = nil
	SyncApiServerConfigHandler = syncApiServerConfig

	client := fake.NewSimpleClientset()
	serviceGroupId := "0"
	ip := "1.2.3.4"
	ep := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: KubernetesServiceName},
		Subsets: []v1.EndpointSubset{
			{
				Addresses:      []v1.EndpointAddress{{IP: ip}},
				Ports:          nil,
				ServiceGroupId: serviceGroupId,
			},
		},
	}

	epCreated, err := client.CoreV1().Endpoints(Namespace_System).Create(ep)
	assert.Nil(t, err)
	assert.NotNil(t, epCreated)

	stopCh := make(chan struct{})
	defer close(stopCh)
	StartAPIServerConfigManagerAndInformerFactory(client, stopCh)

	epMap := instance.GetAPIServerConfig()
	assert.NotNil(t, epMap)
	assert.Equal(t, 1, len(epMap))
	assert.Equal(t, serviceGroupId, epMap[serviceGroupId].ServiceGroupId)
	assert.Equal(t, ip, epMap[serviceGroupId].Addresses[0].IP)
}
