/*
Copyright 2021 The Arktos Authors.

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
	"strings"
	"testing"
)

type input struct {
	serverName string
	imageRef   string
	vcpu       int
	memInMi    int
}

var expectedJson1 = `
{
   "apiVersion":"v1",
   "kind":"Pod",
   "MetaData":{
      "name":"testvm",
      "tenant":"system",
      "namespace":"kube-system",
      "creationTimestamp":null,
      "annotations":{
         "VirtletCPUModel":"host-model"
      }
   },
   "spec":{
      "virtualMachine":{
         "name":"testvm",
         "image":"m1.tiny",
         "resources":{
            "limits":{
               "cpu":"1",
               "memory":"512Mi"
            },
            "requests":{
               "cpu":"1",
               "memory":"512Mi"
            }
         },
         "keyPairName":"foobar",
         "publicKey":"ssh-rsa AAA"
      }
   }
}`

//TODO: fix UT
func TestGetRequestBody(t *testing.T) {
	tests := []struct {
		name               string
		input              input
		expectedJsonString string

		expectedError error
	}{
		{
			name:               "basic valid test",
			input:              input{serverName: "testvm", imageRef: "m1.tiny", vcpu: 1, memInMi: 512},
			expectedJsonString: expectedJson1,
			expectedError:      nil,
		},
	}

	for _, test := range tests {
		b, err := constructVmPodRequestBody(test.input.serverName, test.input.imageRef, test.input.vcpu, test.input.memInMi)

		if err != test.expectedError {
			t.Fatal(err)
		}

		if strings.Compare(string(b), test.expectedJsonString) != 0 {
			t.Fatal(err)
		}
	}
}
