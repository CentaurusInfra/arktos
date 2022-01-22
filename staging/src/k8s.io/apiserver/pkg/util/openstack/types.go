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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

/*

{
  "apiVersion":"v1",
  "kind":"Pod",
  "metadata":{
    "annotations":{
      "VirtletCPUModel":"host-model"
    },
    "name":"%s",
    "namespace":"kube-system",
    "tenant":"system"
  },
  "spec":{
    "virtualMachine":{
      "image":"%s",
      "imagePullPolicy":"IfNotPresent",
      "keyPairName":"foobar",
      "name":"%s",
      "publicKey":"ssh-rsa AAA",
      "resources":{
        "limits":{
          "cpu":"%d",
          "memory":"%dMi"
        },
        "requests":{
          "cpu":"%d",
          "memory":"%dMi"
        }
      }
    }
  }
}
 */

 type vmRequestBody struct {
 	apiVersion string  `default:"v1"`
 	kind string `default:"Pod"`
 	metadata metav1.ObjectMeta
 	spec v1.PodSpec
 }

func getRequestBody() (string, error){
	t := vmRequestBody{}

	b, err := json.Marshal(t)

	if err != nil {
		return "", err
	}

	return string(b), nil

}