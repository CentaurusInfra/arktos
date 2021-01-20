/*
Copyright 2014 The Kubernetes Authors.
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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"reflect"
	"testing"
)

func TestNewKubeClientManagerRunOnce(t *testing.T) {
	NewKubeClientManager()
	ClientManager.tenant2api["john"] = 1
	NewKubeClientManager()

	if len(ClientManager.tenant2api) != 1 {
		t.Error("KubeClientManager has been initialized more than once")
	}
}

func TestRegisterTenantSourceServer(t *testing.T) {
	testcases := []struct {
		pod         *v1.Pod
		source      string
		expectedMap map[string]int
	}{
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "",
				},
			},
			source:      "api",
			expectedMap: map[string]int{},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "jane",
				},
			},
			source:      "noprefix",
			expectedMap: map[string]int{},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "john",
				},
			},
			source: "api",
			expectedMap: map[string]int{
				"john": 0,
			},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "alex",
				},
			},
			source: "api0",
			expectedMap: map[string]int{
				"alex": 0,
			},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "AlEX",
				},
			},
			source: "api1",
			expectedMap: map[string]int{
				"alex": 1,
			},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "AlEX",
				},
			},
			source: "api999",
			expectedMap: map[string]int{
				"alex": 999,
			},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "AlEX",
				},
			},
			source:      "apixxx",
			expectedMap: map[string]int{},
		},
	}

	for i, test := range testcases {
		newKubeClientManagerFunc()()
		ClientManager.RegisterTenantSourceServer(test.source, test.pod)
		if !reflect.DeepEqual(test.expectedMap, ClientManager.tenant2api) {
			t.Errorf("case %d faile: expected %v, got %v", i, test.expectedMap, ClientManager.tenant2api)
		}
	}
}

func TestGetTPClient(t *testing.T) {
	client0 := &clientset.Clientset{}
	client1 := &clientset.Clientset{}

	testcases := []struct {
		kubeClients    []clientset.Interface
		tenant         string
		tenant2api     map[string]int
		expectedClient clientset.Interface
	}{
		{
			kubeClients: nil, // invalid Client array
			tenant:      "ron",
			tenant2api: map[string]int{
				"kip": 1,
			},
			expectedClient: nil,
		},
		{
			kubeClients: []clientset.Interface{ // invalid Client array
			},
			tenant: "jane",
			tenant2api: map[string]int{
				"ale": 1,
			},
			expectedClient: nil,
		},
		{
			kubeClients: []clientset.Interface{
				client0,
				client1,
			},
			tenant: "apple",
			tenant2api: map[string]int{
				"apple": 0,
			},
			expectedClient: client0,
		},
		{
			kubeClients: []clientset.Interface{
				client0,
				client1,
			},
			tenant: "apple",
			tenant2api: map[string]int{
				"apple": 1,
			},
			expectedClient: client1,
		},
		{
			kubeClients: []clientset.Interface{
				client0,
				client1,
			},
			tenant: "alex",
			tenant2api: map[string]int{
				"joe":   1,
				"katie": 0,
			},
			expectedClient: client0,
		},
	}
	for i, test := range testcases {
		newKubeClientManagerFunc()()
		ClientManager.tenant2api = test.tenant2api
		actualClient := ClientManager.GetTPClient(test.kubeClients, test.tenant)
		if test.expectedClient != actualClient {
			t.Errorf("case %d failed: expected %v, got %v", i, test.expectedClient, actualClient)
		}
	}
}
