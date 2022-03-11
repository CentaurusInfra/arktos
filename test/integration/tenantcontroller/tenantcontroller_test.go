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

package tenant

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	"k8s.io/arktos-ext/pkg/generated/informers/externalversions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/metadata"
	tenantcontroller "k8s.io/kubernetes/pkg/controller/tenant"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/integration/framework"
)

const testTenant = "johndoe"

func setup(t *testing.T) (*httptest.Server, framework.CloseFunc, *tenantcontroller.TenantController, informers.SharedInformerFactory, clientset.Interface, restclient.Config) {
	masterConfig := framework.NewIntegrationTestMasterConfig()
	_, server, closeFn := framework.RunAMaster(masterConfig)

	kubeConfig := restclient.KubeConfig{Host: server.URL}
	configs := restclient.NewAggregatedConfig(&kubeConfig)
	clientSet, err := clientset.NewForConfig(configs)
	if err != nil {
		t.Fatalf("Error creating clientset: %v", err)
	}
	resyncPeriod := 12 * time.Hour
	informerSet := informers.NewSharedInformerFactory(clientset.NewForConfigOrDie(restclient.AddUserAgent(configs, "cronjob-informers")), resyncPeriod)

	metadataClient, err := metadata.NewForConfig(configs)
	if err != nil {
		t.Fatalf("Errror creating matadata client")
	}

	discoverTenantedResourcesFn := func() ([]*metav1.APIResourceList, error) {
		all, err := clientSet.Discovery().ServerPreferredResources()
		return discovery.FilteredBy(discovery.ResourcePredicateFunc(func(groupVersion string, r *metav1.APIResource) bool {
			return !r.Namespaced && r.Tenanted
		}), all), err
	}
	networkClient := arktos.NewForConfigOrDie(configs)
	networkInformers := externalversions.NewSharedInformerFactory(networkClient, 0)
	controller := tenantcontroller.NewTenantController(clientSet,
		informerSet.Core().V1().Tenants(),
		informerSet.Core().V1().Namespaces(),
		informerSet.Rbac().V1().ClusterRoles(),
		informerSet.Rbac().V1().ClusterRoleBindings(),
		resyncPeriod,
		networkClient,
		networkInformers.Arktos().V1().Networks(),
		"",
		metadataClient,
		discoverTenantedResourcesFn,
		v1.FinalizerArktos)
	return server, closeFn, controller, informerSet, clientSet, *configs
}

func cleanup(t *testing.T, client clientset.Interface, name string) {
	deletePropagation := metav1.DeletePropagationForeground
	err := client.CoreV1().Tenants().Delete(name, &metav1.DeleteOptions{PropagationPolicy: &deletePropagation})
	if err != nil {
		t.Errorf("Failed to delete CronJob: %v", err)
	}
}

func TestSystemTenantCreatedAutomically(t *testing.T) {
	_, closeFn, controller, _, clientSet, _ := setup(t)
	defer closeFn()

	stopCh := make(chan struct{})
	defer close(stopCh)

	go controller.Run(1, stopCh)

	if err := wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
		_, err := clientSet.CoreV1().Tenants().Get(metav1.TenantSystem, metav1.GetOptions{})
		return err == nil, err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestClusterRoleAndBindingBootstrap(t *testing.T) {
	_, closeFn, controller, informerSet, clientSet, _ := setup(t)
	defer closeFn()

	stopCh := make(chan struct{})
	defer close(stopCh)
	defer cleanup(t, clientSet, testTenant)

	informerSet.Start(stopCh)
	go controller.Run(1, stopCh)

	// create a tenant and validate creation
	_, err := clientSet.CoreV1().Tenants().Create(newTenant(testTenant))
	if err != nil {
		t.Fatalf("Error creating tenant %v: %v", testTenant, err)
	}

	tenant, err := clientSet.CoreV1().Tenants().Get(testTenant, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error creating tenant %v: %v", testTenant, err)
	}
	if tenant.Name != testTenant {
		t.Fatalf("tenant %v", tenant)
	}

	// validate cluster rule and binding are automatically bootstrapped
	if err := wait.PollImmediate(1*time.Second, 120*time.Second, func() (bool, error) {
		clusterRoleList, err := clientSet.RbacV1().ClusterRolesWithMultiTenancy(tenant.Name).List(metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Error listing cluster role for tenant %v: %v", testTenant, err)
		}
		clusterRoleBindingList, err := clientSet.RbacV1().ClusterRoleBindingsWithMultiTenancy(tenant.Name).List(metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Error listing cluster role for tenant %v: %v", testTenant, err)
		}

		if len(clusterRoleList.Items) > 0 && len(clusterRoleBindingList.Items) > 0 {
			containsAdminRole := false
			containsAdminBinding := false
			for _, item := range clusterRoleBindingList.Items {
				if item.Name == tenantcontroller.InitialClusterRoleBindingName {
					containsAdminBinding = true
					break
				}
			}
			for _, item := range clusterRoleList.Items {
				if item.Name == tenantcontroller.InitialClusterRoleName {
					containsAdminRole = true
					break
				}
			}
			if containsAdminRole && containsAdminBinding {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func newTenant(name string) *v1.Tenant {
	return &v1.Tenant{
		TypeMeta: metav1.TypeMeta{
			Kind:       name,
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testTenant,
		},
		Spec: v1.TenantSpec{
			StorageClusterId: "0",
		},
	}
}
