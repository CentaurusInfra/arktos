/*
Copyright 2016 The Kubernetes Authors.
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

package secret

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/apimachinery/pkg/api/meta"
	"time"

	"k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	corev1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/kubelet/kubeclientmanager"
	"k8s.io/kubernetes/pkg/kubelet/util/manager"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
)

// Manager interface provides methods for Kubelet to manage secrets.
type Manager interface {
	// Get secret by secret namespace and name.
	GetSecret(tenant, namespace, name string) (*v1.Secret, error)

	// WARNING: Register/UnregisterPod functions should be efficient,
	// i.e. should not block on network operations.

	// RegisterPod registers all secrets from a given pod.
	RegisterPod(pod *v1.Pod)

	// UnregisterPod unregisters secrets from a given pod that are not
	// used by any other registered pod.
	UnregisterPod(pod *v1.Pod)
}

// simpleSecretManager implements SecretManager interfaces with
// simple operations to apiserver.
type simpleSecretManager struct {
	kubeClients []clientset.Interface
}

// NewSimpleSecretManager creates a new SecretManager instance.
func NewSimpleSecretManager(kubeClients []clientset.Interface) Manager {
	return &simpleSecretManager{kubeClients: kubeClients}
}

func (s *simpleSecretManager) GetSecret(tenant, namespace, name string) (*v1.Secret, error) {
	tenantPartitionClient := kubeclientmanager.ClientManager.GetTPClient(s.kubeClients, tenant)
	return tenantPartitionClient.CoreV1().SecretsWithMultiTenancy(namespace, tenant).Get(name, metav1.GetOptions{})
}

func (s *simpleSecretManager) RegisterPod(pod *v1.Pod) {
}

func (s *simpleSecretManager) UnregisterPod(pod *v1.Pod) {
}

// secretManager keeps a store with secrets necessary
// for registered pods. Different implementations of the store
// may result in different semantics for freshness of secrets
// (e.g. ttl-based implementation vs watch-based implementation).
type secretManager struct {
	manager manager.Manager
}

func (s *secretManager) GetSecret(tenant, namespace, name string) (*v1.Secret, error) {
	object, err := s.manager.GetObject(tenant, namespace, name)
	if err != nil {
		return nil, err
	}
	if secret, ok := object.(*v1.Secret); ok {
		return secret, nil
	}
	return nil, fmt.Errorf("unexpected object type: %v", object)
}

func (s *secretManager) RegisterPod(pod *v1.Pod) {
	s.manager.RegisterPod(pod)
}

func (s *secretManager) UnregisterPod(pod *v1.Pod) {
	s.manager.UnregisterPod(pod)
}

func getSecretNames(pod *v1.Pod) sets.String {
	result := sets.NewString()
	podutil.VisitPodSecretNames(pod, func(name string) bool {
		result.Insert(name)
		return true
	})
	return result
}

const (
	defaultTTL = time.Minute
)

// NewCachingSecretManager creates a manager that keeps a cache of all secrets
// necessary for registered pods.
// It implements the following logic:
// - whenever a pod is created or updated, the cached versions of all secrets
//   are invalidated
// - every GetObject() call tries to fetch the value from local cache; if it is
//   not there, invalidated or too old, we fetch it from apiserver and refresh the
//   value in cache; otherwise it is just fetched from cache
func NewCachingSecretManager(kubeClients []clientset.Interface, getTTL manager.GetObjectTTLFunc) Manager {
	getSecret := func(tenant, namespace, name string, opts metav1.GetOptions) (runtime.Object, error) {
		tenantPartitionClient := kubeclientmanager.ClientManager.GetTPClient(kubeClients, tenant)
		return tenantPartitionClient.CoreV1().SecretsWithMultiTenancy(namespace, tenant).Get(name, opts)
	}
	secretStore := manager.NewObjectStore(getSecret, clock.RealClock{}, getTTL, defaultTTL)
	return &secretManager{
		manager: manager.NewCacheBasedManager(secretStore, getSecretNames),
	}
}

// NewWatchingSecretManager creates a manager that keeps a cache of all secrets
// necessary for registered pods.
// It implements the following logic:
// - whenever a pod is created or updated, we start individual watches for all
//   referenced objects that aren't referenced from other registered pods
// - every GetObject() returns a value from local cache propagated via watches
func NewWatchingSecretManager(kubeClients []clientset.Interface) Manager {
	listSecret := func(tenant, namespace string, opts metav1.ListOptions) (runtime.Object, error) {
		tenantPartitionClient := kubeclientmanager.ClientManager.GetTPClient(kubeClients, tenant)
		return tenantPartitionClient.CoreV1().SecretsWithMultiTenancy(namespace, tenant).List(opts)
	}
	watchSecret := func(tenant, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
		tenantPartitionClient := kubeclientmanager.ClientManager.GetTPClient(kubeClients, tenant)
		return tenantPartitionClient.CoreV1().SecretsWithMultiTenancy(namespace, tenant).Watch(opts)
	}
	newSecret := func() runtime.Object {
		return &v1.Secret{}
	}
	gr := corev1.Resource("secret")
	return &secretManager{
		manager: manager.NewWatchBasedManager(listSecret, watchSecret, newSecret, gr, getSecretNames),
	}
}

type byHostSecretManager struct {
	kubeClients []clientset.Interface
	hostName    string
	stores      []cache.Store
}

// ensure this is the same as cache.MetaNamespaceKeyFunc
func (s *byHostSecretManager) key(tenant, namespace, name string) string {
	result := name
	if len(namespace) > 0 {
		result = namespace + "/" + result
	} else {
		result = metav1.NamespaceDefault + "/" + result
	}
	if len(tenant) > 0 && tenant != metav1.TenantSystem {
		result = tenant + "/" + result
	} else {
		result = metav1.TenantSystem + "/" + result
	}
	return result
}

func (s *byHostSecretManager) GetSecret(tenant, namespace, name string) (*v1.Secret, error) {
	key := s.key(tenant, namespace, name)
	klog.V(2).Infof("get secret: %s", key)
	for _, store := range s.stores {
		klog.V(6).Infof("store keys: [%v]", store.ListKeys())
		object, _, err := store.GetByKey(key)
		if err != nil {
			return nil, err
		}
		if object, ok := object.(*v1.Secret); ok {
			return object, nil
		}
		return nil, fmt.Errorf("unexpected object type: %v", object)

	}
	return nil, fmt.Errorf("secret not found: %s-%s-%s", tenant, namespace, name)
}

func (s *byHostSecretManager) RegisterPod(pod *v1.Pod) {
}

func (s *byHostSecretManager) UnregisterPod(pod *v1.Pod) {
}

func MetaNamespaceKeyFunc(obj interface{}) (string, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return "", fmt.Errorf("object has no meta: %v", err)
	}

	metaKey := meta.GetName()
	if len(meta.GetNamespace()) > 0 {
		metaKey = meta.GetNamespace() + "/" + metaKey
	} else {
		metaKey = metav1.NamespaceDefault + "/" + metaKey
	}

	if len(meta.GetTenant()) > 0 {
		metaKey = meta.GetTenant() + "/" + metaKey
	} else {
		metaKey = metav1.TenantSystem + "/" + metaKey
	}

	return metaKey, nil
}

func NewByHostWatchingSecretManager(kubeClients []clientset.Interface, hostName string) Manager {

	klog.Infof("create secret manager for host: %s", hostName)
	stores := make([]cache.Store, len(kubeClients))

	for i, tenantPartitionClient := range kubeClients {
		listFunc := func(options metav1.ListOptions) (runtime.Object, error) {
			options.LabelSelector=hostName
			return tenantPartitionClient.CoreV1().SecretsWithMultiTenancy(core.NamespaceAll, core.TenantAll).List(options)
		}
		watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector=hostName
			return tenantPartitionClient.CoreV1().SecretsWithMultiTenancy(core.NamespaceAll, core.TenantAll).Watch(options)
		}
		stores[i] = cache.NewStore(MetaNamespaceKeyFunc)
		r := cache.NewReflector(&cache.ListWatch{ListFunc: listFunc, WatchFunc: watchFunc}, &v1.Secret{}, stores[i], 0)
		go r.Run(wait.NeverStop)
	}

	return &byHostSecretManager{kubeClients: kubeClients, hostName: hostName, stores: stores}
}
