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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

// poc, tenant name -> api server id
var Tenant2api map[string]int

func GetTPClient(kubeClients []clientset.Interface, tenant string) clientset.Interface {
	var client clientset.Interface
	pick, ok := Tenant2api[tenant]
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
