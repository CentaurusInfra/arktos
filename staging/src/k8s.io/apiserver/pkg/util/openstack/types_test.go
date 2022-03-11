/*
Copyright 2021 Authors of Arktos.

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

package openstack

import (
	"k8s.io/klog"
	"strings"
	"testing"
)

type input struct {
	replicas int
	server   ServerType
	imageRef string
	vcpu     int
	memInMi  int
}

var expectedJson1 = `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"testvm","tenant":"system","namespace":"kube-system","creationTimestamp":null,"labels":{"openstsckApi":"true"},"annotations":{"VirtletCPUModel":"host-model"}},"spec":{"virtualMachine":{"name":"testvm","image":"m1.tiny","resources":{"limits":{"cpu":"1","memory":"512Mi"},"requests":{"cpu":"1","memory":"512Mi"}},"imagePullPolicy":"IfNotPresent","publicKey":"ssh-rsa AAA"}}}`

func TestConstructVmPodRequestBody(t *testing.T) {
	tests := []struct {
		name               string
		input              input
		expectedJsonString string
		expectedError      error
	}{
		{
			name:               "basic valid test",
			input:              input{server: ServerType{Name: "testvm", Key_name: "ssh-rsa AAA"}, imageRef: "m1.tiny", vcpu: 1, memInMi: 512},
			expectedJsonString: expectedJson1,
			expectedError:      nil,
		},
	}

	for _, test := range tests {
		b, err := constructVmPodRequestBody(test.input.server, test.input.imageRef, test.input.vcpu, test.input.memInMi)

		if err != test.expectedError {
			t.Fatal(err)
		}

		klog.Infof("vmPodBody: %s", string(b))
		if strings.Compare(string(b), test.expectedJsonString) != 0 {
			t.Fatal(err)
		}
	}
}

var expectedJson2 = `{"apiVersion":"apps/v1","kind":"ReplicaSet","metadata":{"name":"testvm","tenant":"system","namespace":"kube-system","creationTimestamp":null},"spec":{"replicas":3,"selector":{"matchLabels":{"ln":"testvm"}},"template":{"metadata":{"creationTimestamp":null,"labels":{"ln":"testvm","openstsckApi":"true"},"annotations":{"VirtletCPUModel":"host-model"}},"spec":{"virtualMachine":{"name":"testvm","image":"m1.tiny","resources":{"limits":{"cpu":"1","memory":"512Mi"},"requests":{"cpu":"1","memory":"512Mi"}},"imagePullPolicy":"IfNotPresent","publicKey":"ssh-rsa AAA"}}}}}`

func TestConstructReplicasetRequestBody(t *testing.T) {
	tests := []struct {
		name               string
		input              input
		expectedJsonString string
		expectedError      error
	}{
		{
			name:               "basic valid test",
			input:              input{replicas: 3, server: ServerType{Name: "testvm", Key_name: "ssh-rsa AAA"}, imageRef: "m1.tiny", vcpu: 1, memInMi: 512},
			expectedJsonString: expectedJson2,
			expectedError:      nil,
		},
	}

	for _, test := range tests {
		b, err := constructReplicasetRequestBody(test.input.replicas, test.input.server, test.input.imageRef, test.input.vcpu, test.input.memInMi)

		if err != test.expectedError {
			t.Fatal(err)
		}

		klog.Infof("replicasetBody: %s", string(b))
		if strings.Compare(string(b), test.expectedJsonString) != 0 {
			t.Fatal(err)
		}
	}
}
