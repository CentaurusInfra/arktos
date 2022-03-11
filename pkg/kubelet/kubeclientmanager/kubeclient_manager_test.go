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
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
)

func TestNewKubeClientManagerRunOnce(t *testing.T) {
	NewKubeClientManager()
	ClientManager.puid2api["john"] = 1
	NewKubeClientManager()

	if len(ClientManager.puid2api) != 1 {
		t.Error("KubeClientManager has been initialized more than once")
	}
}

func TestRegisterPodSourceServer(t *testing.T) {
	testcases := []struct {
		pod         *v1.Pod
		source      string
		expectedMap map[types.UID]int
	}{
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					UID: "",
				},
			},
			source:      "api",
			expectedMap: map[types.UID]int{},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "nonempty",
					UID:    "jane",
				},
			},
			source:      "noprefix",
			expectedMap: map[types.UID]int{},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "nonempty",
					UID:    "john",
				},
			},
			source: "api",
			expectedMap: map[types.UID]int{
				"john": 0,
			},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "nonempty",
					UID:    "alex",
				},
			},
			source: "api0",
			expectedMap: map[types.UID]int{
				"alex": 0,
			},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "nonempty",
					UID:    "AlEX",
				},
			},
			source: "api1",
			expectedMap: map[types.UID]int{
				"AlEX": 1,
			},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "nonempty",
					UID:    "AlEX",
				},
			},
			source: "api999",
			expectedMap: map[types.UID]int{
				"AlEX": 999,
			},
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Tenant: "nonempty",
					UID:    "AlEX",
				},
			},
			source:      "apixxx",
			expectedMap: map[types.UID]int{},
		},
	}

	for i, test := range testcases {
		newKubeClientManagerFunc()()
		ClientManager.RegisterPodSourceServer(test.source, test.pod)
		if !reflect.DeepEqual(test.expectedMap, ClientManager.puid2api) {
			t.Errorf("case %d failed: expected %v, got %v", i, test.expectedMap, ClientManager.puid2api)
		}
	}
}

func TestGetTPClient(t *testing.T) {
	client0 := &clientset.Clientset{}
	client1 := &clientset.Clientset{}

	testcases := []struct {
		kubeClients    []clientset.Interface
		puid           types.UID
		puid2api       map[types.UID]int
		expectedClient clientset.Interface
	}{
		{
			kubeClients: nil, // invalid Client array
			puid:        "ron",
			puid2api: map[types.UID]int{
				"kip": 1,
			},
			expectedClient: nil,
		},
		{
			kubeClients: []clientset.Interface{ // invalid Client array
			},
			puid: "jane",
			puid2api: map[types.UID]int{
				"ale": 1,
			},
			expectedClient: nil,
		},
		{
			kubeClients: []clientset.Interface{
				client0,
				client1,
			},
			puid: "apple",
			puid2api: map[types.UID]int{
				"apple": 0,
			},
			expectedClient: client0,
		},
		{
			kubeClients: []clientset.Interface{
				client0,
				client1,
			},
			puid: "apple",
			puid2api: map[types.UID]int{
				"apple": 1,
			},
			expectedClient: client1,
		},
		{
			kubeClients: []clientset.Interface{
				client0,
				client1,
			},
			puid: "alex",
			puid2api: map[types.UID]int{
				"joe":   1,
				"katie": 0,
			},
			expectedClient: client0,
		},
	}
	for i, test := range testcases {
		newKubeClientManagerFunc()()
		ClientManager.puid2api = test.puid2api
		actualClient := ClientManager.GetTPClient(test.kubeClients, test.puid)
		if test.expectedClient != actualClient {
			t.Errorf("case %d failed: expected %v, got %v", i, test.expectedClient, actualClient)
		}
	}
}
