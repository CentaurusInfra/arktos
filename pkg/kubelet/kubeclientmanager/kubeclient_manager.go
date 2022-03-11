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
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
)

/*
This manager keeps a map between pod UID and its corresponding tenant partition apiserver id.
This map is to be used together with kubeTPClients in the Kubelet struct to obtain the correct kubeclient
*/
type KubeClientManager struct {
	puid2api     map[types.UID]int // pod UID -> apiserver id
	puid2apiLock sync.RWMutex
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
			puid2api: make(map[types.UID]int),
		}
		klog.V(2).Infof("kubeclient manager initialized %v", ClientManager)
	}
}

func (manager *KubeClientManager) RegisterPodSourceServer(source string, ref *v1.Pod) {
	if len(ref.Tenant) == 0 || !strings.HasPrefix(source, kubetypes.ApiserverSource) {
		klog.Warningf("unable to register source: pod uid='%s', tenant='%s', source='%s', ", ref.UID, ref.Tenant, source)
		return
	}

	if len(ref.UID) == 0 {
		klog.Warningf("unable to register source for pod with empty UID: tenant='%s', namespace='%s', name='%s', source='%s'", ref.Tenant, ref.Namespace, ref.Name, source)
		return
	}

	manager.puid2apiLock.Lock()
	defer manager.puid2apiLock.Unlock()

	if source == kubetypes.ApiserverSource {
		klog.V(6).Infof("source is '%s', will only use kube client #0", kubetypes.ApiserverSource)
		manager.puid2api[ref.UID] = 0
		return
	}

	clientId, err := strconv.Atoi(source[len(kubetypes.ApiserverSource):])
	if err != nil {
		klog.Errorf("unable to get a tenant partition id, Err: %s", err)
		return
	}

	if _, ok := manager.puid2api[ref.UID]; !ok {
		manager.puid2api[ref.UID] = clientId
		klog.V(6).Infof("added %d to the map, map has %+v", clientId, manager.puid2api)
	} else {
		// todo: consider more stringent error handling
		if clientId == manager.puid2api[ref.UID] {
			klog.Warningf("pod uid=%s already registered in source map", ref.UID)
		} else {
			klog.Errorf("pod uid=%s source tracking conflicts: in source map %d, to set %d", ref.UID, manager.puid2api[ref.UID], clientId)
		}
	}
}

func (manager *KubeClientManager) UnregisterPodSourceServer(ref *v1.Pod) {
	if len(ref.UID) == 0 {
		klog.Warningf("unable to unregister source for pod with empty UID: tenant='%s'", ref.Tenant)
		return
	}

	manager.puid2apiLock.Lock()
	defer manager.puid2apiLock.Unlock()

	delete(manager.puid2api, ref.UID)
}

func (manager *KubeClientManager) GetTPClient(kubeClients []clientset.Interface, podUID types.UID) clientset.Interface {
	if kubeClients == nil || len(kubeClients) == 0 {
		klog.Errorf("invalid kubeClients : %v", kubeClients)
		return nil
	}
	// todo: calling param w/ use pod uid only
	pick := manager.PickClient(podUID)
	klog.V(6).Infof("using client #%v for pod UID '%s'", pick, podUID)
	return kubeClients[pick]
}

func (manager *KubeClientManager) PickClient(podUID types.UID) int {
	manager.puid2apiLock.RLock()
	defer manager.puid2apiLock.RUnlock()
	pick, ok := manager.puid2api[podUID]
	if !ok {
		klog.Warningf("no registered client for pod UID %s, defaulted to client #0", podUID)
		pick = 0
	}
	return pick
}
