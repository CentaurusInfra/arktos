/*
Copyright 2019 The Kubernetes Authors.
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

package kubelet

import (
	"encoding/json"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubeapiservertesting "k8s.io/kubernetes/cmd/kube-apiserver/app/testing"
	"k8s.io/kubernetes/test/integration/framework"
	"testing"
)

func createClientSet(t *testing.T) (*kubernetes.Clientset, *kubeapiservertesting.TestServer, *kubeapiservertesting.TestServer, error) {
	instanceOptions := &kubeapiservertesting.TestServerInstanceOptions{DisableStorageCleanup: true}
	sharedEtcd := framework.SharedEtcd()

	server1 := kubeapiservertesting.StartTestServerOrDie(t, instanceOptions, nil, sharedEtcd)
	server2 := kubeapiservertesting.StartTestServerOrDie(t, instanceOptions, nil, sharedEtcd)
	var configs []*rest.KubeConfig

	for _, config := range server1.ClientConfig.GetAllConfigs() {
		configs = append(configs, config)
	}

	for _, config := range server2.ClientConfig.GetAllConfigs() {
		configs = append(configs, config)
	}
	client, err := kubernetes.NewForConfig(rest.NewAggregatedConfig(configs...))
	return client, server1, server2, err
}

func TestCreating(t *testing.T) {
	testNamespace := "test-creating"

	client, server1, server2, err := createClientSet(t)
	defer server1.TearDownFn()
	defer server2.TearDownFn()
	if err != nil {
		fmt.Printf("The err is %v", err)
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := client.CoreV1().Namespaces().Create((&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}})); err != nil {
		t.Fatal(err)
	}
	var res *v1.Namespace
	if res, err = client.CoreV1().Namespaces().Get(testNamespace, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatalf("The namespace %s failed to create.", testNamespace)
	}
}

func TestListing(t *testing.T) {
	testNamespace := "test-listing"

	client, server1, server2, err := createClientSet(t)
	defer server1.TearDownFn()
	defer server2.TearDownFn()
	if err != nil {
		fmt.Printf("The err is %v", err)
		t.Fatalf("unexpected error: %v", err)
	}
	var list *v1.NamespaceList
	if list, err = client.CoreV1().Namespaces().List(metav1.ListOptions{}); err != nil {
		t.Fatal(err)
	}
	itemLen := len(list.Items)

	if _, err := client.CoreV1().Namespaces().Create((&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}})); err != nil {
		t.Fatal(err)
	}

	if list, err = client.CoreV1().Namespaces().List(metav1.ListOptions{}); err != nil {
		t.Fatal(err)
	}

	if itemLen != len(list.Items)-1 {
		t.Fatalf("The list is not increased after the namespace %s failed to create.", testNamespace)
	}
}

func TestDeleting(t *testing.T) {
	testNamespace := "test-deleting"

	client, server1, server2, err := createClientSet(t)
	defer server1.TearDownFn()
	defer server2.TearDownFn()
	if err != nil {
		fmt.Printf("The err is %v", err)
		t.Fatalf("unexpected error: %v", err)
	}
	var list *v1.NamespaceList
	if list, err = client.CoreV1().Namespaces().List(metav1.ListOptions{}); err != nil {
		t.Fatal(err)
	}
	itemLen := len(list.Items)

	if _, err := client.CoreV1().Namespaces().Create((&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}})); err != nil {
		t.Fatal(err)
	}

	if list, err = client.CoreV1().Namespaces().List(metav1.ListOptions{}); err != nil {
		t.Fatal(err)
	}

	if itemLen != len(list.Items)-1 {
		t.Fatalf("The list is not increased after the namespace %s failed to create.", testNamespace)
	}

	if err = client.CoreV1().Namespaces().Delete(testNamespace, &metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}

	if list, err = client.CoreV1().Namespaces().List(metav1.ListOptions{}); err != nil {
		t.Fatal(err)
	}

	if itemLen == len(list.Items) {
		t.Fatalf("The list is not increased after the namespace %s failed to delete.", testNamespace)
	}
}

func TestUpdating(t *testing.T) {
	testNamespace := "test-updating"

	client, server1, server2, err := createClientSet(t)
	defer server1.TearDownFn()
	defer server2.TearDownFn()
	if err != nil {
		fmt.Printf("The err is %v", err)
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := client.CoreV1().Namespaces().Create((&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}})); err != nil {
		t.Fatal(err)
	}

	var res *v1.Namespace
	if res, err = client.CoreV1().Namespaces().Get(testNamespace, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}

	updatedHashKey := int64(123)
	res.HashKey = updatedHashKey
	if res, err = client.CoreV1().Namespaces().Update(res); err != nil {
		t.Fatal(err)
	}

	if res.HashKey != updatedHashKey {
		t.Fatalf("The haskkey has not been updated from %v to %v", res.HashKey, updatedHashKey)
	}
}

func TestPatching(t *testing.T) {
	testNamespace := "test-patching"

	client, server1, server2, err := createClientSet(t)
	defer server1.TearDownFn()
	defer server2.TearDownFn()
	if err != nil {
		fmt.Printf("The err is %v", err)
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := client.CoreV1().Namespaces().Create((&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}})); err != nil {
		t.Fatal(err)
	}

	var res *v1.Namespace
	if res, err = client.CoreV1().Namespaces().Get(testNamespace, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	oldData, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}

	updatedGeneratedName := "123"
	res.GenerateName = updatedGeneratedName
	newData, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1.Namespace{})

	if _, err := client.CoreV1().Namespaces().Patch(string(testNamespace), types.StrategicMergePatchType, patchBytes, "status"); err != nil {
		t.Fatal(err)
	}
	if res, err = client.CoreV1().Namespaces().Get(testNamespace, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	if res.GenerateName != updatedGeneratedName {
		t.Fatalf("The generated name %s has not been patched to.", updatedGeneratedName)
	}
}
