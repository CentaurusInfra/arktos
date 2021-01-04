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

package util

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
	"strconv"
	"strings"
	"sync"
)

// poc, tenant name -> api server id
var Tenant2api map[string]int

var Tenant2apiLock sync.RWMutex

func RegisterTenantSourceServer(source string, ref *v1.Pod) {
	if len(ref.Tenant) == 0 || !strings.HasPrefix(source, kubetypes.ApiserverSource) {
		return
	}
	key := strings.ToLower(ref.Tenant)
	clientId, err := strconv.Atoi(source[(len(source) - 1):])
	if err != nil {
		klog.Errorf("Failed to get apiserver id. Err: %s", err)
		return
	}
	Tenant2apiLock.Lock()
	defer Tenant2apiLock.Unlock()
	if _, ok := Tenant2api[key]; !ok {
		Tenant2api[key] = clientId
	}
}

func GetTPClient(kubeClients []clientset.Interface, tenant string) clientset.Interface {
	Tenant2apiLock.RLock()
	defer Tenant2apiLock.RUnlock()
	var client clientset.Interface
	pick, ok := Tenant2api[strings.ToLower(tenant)]
	if len(kubeClients) == 1 || !ok { // !ok to force a retry
		client = kubeClients[0]
	} else {
		client = kubeClients[pick]
	}
	klog.V(4).Infof("tenant %s using client # %d", tenant, pick)
	return client
}

// FromApiserverCache modifies <opts> so that the GET request will
// be served from apiserver cache instead of from etcd.
func FromApiserverCache(opts *metav1.GetOptions) {
	opts.ResourceVersion = "0"
}
