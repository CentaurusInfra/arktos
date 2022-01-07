/*
Copyright 2018 The Kubernetes Authors.
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

package manager

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	corev1 "k8s.io/kubernetes/pkg/apis/core/v1"

	"github.com/stretchr/testify/assert"
)

func listSecret(fakeClient clientset.Interface) listObjectFunc {
	return func(tenant, namespace string, _ int, opts metav1.ListOptions) (runtime.Object, error) {
		return fakeClient.CoreV1().SecretsWithMultiTenancy(namespace, tenant).List(opts)
	}
}

func watchSecret(fakeClient clientset.Interface) watchObjectFunc {
	return func(tenant, namespace string, _ int, opts metav1.ListOptions) (watch.Interface, error) {
		return fakeClient.CoreV1().SecretsWithMultiTenancy(namespace, tenant).Watch(opts)
	}
}

func newSecretCache(fakeClient clientset.Interface) *objectCache {
	return &objectCache{
		listObject:    listSecret(fakeClient),
		watchObject:   watchSecret(fakeClient),
		newObject:     func() runtime.Object { return &v1.Secret{} },
		groupResource: corev1.Resource("secret"),
		items:         make(map[objectKey]*objectCacheItem),
	}
}

func TestSecretCache(t *testing.T) {
	testSecretCache(t, metav1.TenantSystem)
}

func TestSecretCacheWithMultiTenancy(t *testing.T) {
	testSecretCache(t, "test-te")
}

func testSecretCache(t *testing.T, tenant string) {
	fakeClient := &fake.Clientset{}

	listReactor := func(a core.Action) (bool, runtime.Object, error) {
		result := &v1.SecretList{
			ListMeta: metav1.ListMeta{
				ResourceVersion: "123",
			},
		}
		return true, result, nil
	}
	fakeClient.AddReactor("list", "secrets", listReactor)
	fakeWatch := watch.NewFake()
	fakeClient.AddWatchReactor("secrets", core.DefaultWatchReactor(fakeWatch, nil))

	store := newSecretCache(fakeClient)

	store.AddReference(tenant, "ns", "name", 0)
	_, err := store.Get(tenant, "ns", "name", 0)
	if !apierrors.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got: %v", err)
	}

	// Eventually we should be able to read added secret.
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "name", Namespace: "ns", Tenant: tenant, ResourceVersion: "125"},
	}
	fakeWatch.Add(secret)
	getFn := func() (bool, error) {
		object, err := store.Get(tenant, "ns", "name", 0)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		secret := object.(*v1.Secret)
		if secret == nil || secret.Name != "name" || secret.Namespace != "ns" || secret.Tenant != tenant {
			return false, fmt.Errorf("unexpected secret: %v", secret)
		}
		return true, nil
	}
	if err := wait.PollImmediate(10*time.Millisecond, time.Second, getFn); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Eventually we should observer secret deletion.
	fakeWatch.Delete(secret)
	getFn = func() (bool, error) {
		_, err := store.Get(tenant, "ns", "name", 0)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	}
	if err := wait.PollImmediate(10*time.Millisecond, time.Second, getFn); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	store.DeleteReference(tenant, "ns", "name", 0)
	_, err = store.Get(tenant, "ns", "name", 0)
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSecretCacheMultipleRegistrations(t *testing.T) {
	testSecretCacheMultipleRegistrations(t, metav1.TenantSystem)
}

func TestSecretCacheMultipleRegistrationsWithMultiTenancy(t *testing.T) {
	testSecretCacheMultipleRegistrations(t, "test-te")
}

func testSecretCacheMultipleRegistrations(t *testing.T, tenant string) {
	fakeClient := &fake.Clientset{}

	listReactor := func(a core.Action) (bool, runtime.Object, error) {
		result := &v1.SecretList{
			ListMeta: metav1.ListMeta{
				ResourceVersion: "123",
			},
		}
		return true, result, nil
	}
	fakeClient.AddReactor("list", "secrets", listReactor)
	fakeWatch := watch.NewFake()
	fakeClient.AddWatchReactor("secrets", core.DefaultWatchReactor(fakeWatch, nil))

	store := newSecretCache(fakeClient)

	store.AddReference(tenant, "ns", "name", 0)
	// This should trigger List and Watch actions eventually.
	actionsFn := func() (bool, error) {
		actions := fakeClient.Actions()
		if len(actions) > 2 {
			return false, fmt.Errorf("too many actions: %v", actions)
		}
		if len(actions) < 2 {
			return false, nil
		}
		if actions[0].GetVerb() != "list" || actions[1].GetVerb() != "watch" {
			return false, fmt.Errorf("unexpected actions: %v", actions)
		}
		return true, nil
	}
	if err := wait.PollImmediate(10*time.Millisecond, time.Second, actionsFn); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Next registrations shouldn't trigger any new actions.
	for i := 0; i < 20; i++ {
		store.AddReference(tenant, "ns", "name", 0)
		store.DeleteReference(tenant, "ns", "name", 0)
	}
	actions := fakeClient.Actions()
	assert.Equal(t, 2, len(actions), "unexpected actions: %#v", actions)

	// Final delete also doesn't trigger any action.
	store.DeleteReference(tenant, "ns", "name", 0)
	_, err := store.Get(tenant, "ns", "name", 0)
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("unexpected error: %v", err)
	}
	actions = fakeClient.Actions()
	assert.Equal(t, 2, len(actions), "unexpected actions: %#v", actions)
}
