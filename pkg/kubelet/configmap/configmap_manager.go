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

package configmap

import (
	"fmt"
	"k8s.io/klog"
	"time"

	"k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	corev1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/kubelet/util/manager"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
)

// Manager interface provides methods for Kubelet to manage ConfigMap.
type Manager interface {
	// Get configmap by configmap namespace and name.
	GetConfigMap(tenant, namespace, name string) (*v1.ConfigMap, error)

	// WARNING: Register/UnregisterPod functions should be efficient,
	// i.e. should not block on network operations.

	// RegisterPod registers all configmaps from a given pod.
	RegisterPod(pod *v1.Pod)

	// UnregisterPod unregisters configmaps from a given pod that are not
	// used by any other registered pod.
	UnregisterPod(pod *v1.Pod)
}

// simpleConfigMapManager implements ConfigMap Manager interface with
// simple operations to apiserver.
type simpleConfigMapManager struct {
	kubeClients []clientset.Interface
}

// NewSimpleConfigMapManager creates a new ConfigMapManager instance.
func NewSimpleConfigMapManager(kubeClients []clientset.Interface) Manager {
	return &simpleConfigMapManager{kubeClients: kubeClients}
}

func (s *simpleConfigMapManager) GetConfigMap(tenant, namespace, name string) (*v1.ConfigMap, error) {
	client := getTPClient(s.kubeClients, tenant)
	return client.CoreV1().ConfigMapsWithMultiTenancy(namespace, tenant).Get(name, metav1.GetOptions{})
}

func (s *simpleConfigMapManager) RegisterPod(pod *v1.Pod) {
}

func (s *simpleConfigMapManager) UnregisterPod(pod *v1.Pod) {
}

// configMapManager keeps a cache of all configmaps necessary
// for registered pods. Different implementation of the store
// may result in different semantics for freshness of configmaps
// (e.g. ttl-based implementation vs watch-based implementation).
type configMapManager struct {
	manager manager.Manager
}

func (c *configMapManager) GetConfigMap(tenant, namespace, name string) (*v1.ConfigMap, error) {
	object, err := c.manager.GetObject(tenant, namespace, name)
	if err != nil {
		return nil, err
	}
	if configmap, ok := object.(*v1.ConfigMap); ok {
		return configmap, nil
	}
	return nil, fmt.Errorf("unexpected object type: %v", object)
}

func (c *configMapManager) RegisterPod(pod *v1.Pod) {
	c.manager.RegisterPod(pod)
}

func (c *configMapManager) UnregisterPod(pod *v1.Pod) {
	c.manager.UnregisterPod(pod)
}

func getConfigMapNames(pod *v1.Pod) sets.String {
	result := sets.NewString()
	podutil.VisitPodConfigmapNames(pod, func(name string) bool {
		result.Insert(name)
		return true
	})
	return result
}

const (
	defaultTTL = time.Minute
)

func getTPClient(kubeClients []clientset.Interface, tenant string) clientset.Interface {
	var client clientset.Interface
	pick := 0
	if len(kubeClients)==1 || tenant[0] <= 'm' {
		client = kubeClients[0]
	} else {
		client = kubeClients[1]
		pick = 1
	}
	klog.Infof("tenant %s using client # %d", tenant, pick)
	return client
}

// NewCachingConfigMapManager creates a manager that keeps a cache of all configmaps
// necessary for registered pods.
// It implement the following logic:
// - whenever a pod is create or updated, the cached versions of all configmaps
//   are invalidated
// - every GetObject() call tries to fetch the value from local cache; if it is
//   not there, invalidated or too old, we fetch it from apiserver and refresh the
//   value in cache; otherwise it is just fetched from cache
func NewCachingConfigMapManager(kubeClient []clientset.Interface, getTTL manager.GetObjectTTLFunc) Manager {
	getConfigMap := func(tenant, namespace, name string, opts metav1.GetOptions) (runtime.Object, error) {
		client := getTPClient(kubeClient, tenant)
		return client.CoreV1().ConfigMapsWithMultiTenancy(namespace, tenant).Get(name, opts)
	}
	configMapStore := manager.NewObjectStore(getConfigMap, clock.RealClock{}, getTTL, defaultTTL)
	return &configMapManager{
		manager: manager.NewCacheBasedManager(configMapStore, getConfigMapNames),
	}
}

// NewWatchingConfigMapManager creates a manager that keeps a cache of all configmaps
// necessary for registered pods.
// It implements the following logic:
// - whenever a pod is created or updated, we start individual watches for all
//   referenced objects that aren't referenced from other registered pods
// - every GetObject() returns a value from local cache propagated via watches
func NewWatchingConfigMapManager(kubeClients []clientset.Interface) Manager {
	listConfigMap := func(tenant, namespace string, opts metav1.ListOptions) (runtime.Object, error) {
		client := getTPClient(kubeClients, tenant)
		return client.CoreV1().ConfigMapsWithMultiTenancy(namespace, tenant).List(opts)
	}
	watchConfigMap := func(tenant, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
		client := getTPClient(kubeClients, tenant)
		return client.CoreV1().ConfigMapsWithMultiTenancy(namespace, tenant).Watch(opts)
	}
	newConfigMap := func() runtime.Object {
		return &v1.ConfigMap{}
	}
	gr := corev1.Resource("configmap")
	return &configMapManager{
		manager: manager.NewWatchBasedManager(listConfigMap, watchConfigMap, newConfigMap, gr, getConfigMapNames),
	}
}
