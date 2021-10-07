/*
Copyright 2020 Authors of Arktos.
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

package kubeclientmanager

import (
	"strconv"
	"strings"
	"sync"

	"k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
)

/*
This manager keeps a map between tenant name and its corresponding tenant partition apiserver id.
This map is to be used together with kubeTPClients in the Kubelet struct to obtain the correct kubeclient
*/
type KubeClientManager struct {
	tenant2api     map[string]int // tenant name -> tenant partition apiserver id
	tenant2apiLock sync.RWMutex
}

var ClientManager *KubeClientManager
var kubeclientManagerOnce sync.Once

func NewKubeClientManager() {
	kubeclientManagerOnce.Do(
		newKubeClientManagerFunc(),
	)
}

func newKubeClientManagerFunc() func() {
	return func() {
		ClientManager = &KubeClientManager{
			tenant2api: make(map[string]int),
		}
		klog.Infof("kubeclient manager initialized %v", ClientManager)
	}
}

func (manager *KubeClientManager) RegisterTenantSourceServer(source string, ref *v1.Pod) {
	if len(ref.Tenant) == 0 || !strings.HasPrefix(source, kubetypes.ApiserverSource) {
		klog.Warningf("unable to register tenant source : tenant='%s', source='%s'", ref.Tenant, source)
		return
	}

	manager.tenant2apiLock.Lock()
	defer manager.tenant2apiLock.Unlock()

	key := strings.ToLower(ref.Tenant)
	if source == kubetypes.ApiserverSource {
		klog.Infof("source is '%s', will only use kube client #0", kubetypes.ApiserverSource)
		manager.tenant2api[key] = 0
		return
	}

	clientId, err := strconv.Atoi(source[len(kubetypes.ApiserverSource):])
	if err != nil {
		klog.Errorf("unable to get a tenant partition id, Err: %s", err)
		return
	}

	if _, ok := manager.tenant2api[key]; !ok {
		manager.tenant2api[key] = clientId
		klog.Infof("added %d to the map, map has %+v", clientId, manager.tenant2api)
	}
}

func (manager *KubeClientManager) GetTPClient(kubeClients []clientset.Interface, tenant string) clientset.Interface {
	if kubeClients == nil || len(kubeClients) == 0 {
		klog.Errorf("invalid kubeClients : %v", kubeClients)
		return nil
	}
	pick := manager.PickClient(tenant)
	klog.Infof("using client #%v for tenant '%s'", pick, tenant)
	return kubeClients[pick]
}

func (manager *KubeClientManager) PickClient(tenant string) int {
	manager.tenant2apiLock.RLock()
	defer manager.tenant2apiLock.RUnlock()
	pick, ok := manager.tenant2api[strings.ToLower(tenant)]
	if !ok {
		klog.Warningf("no registered client for tenant %s, defaulted to client #0", tenant)
		pick = 0
	}
	return pick
}
