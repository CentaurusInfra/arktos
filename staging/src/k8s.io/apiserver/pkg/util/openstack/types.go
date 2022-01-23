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
    "strconv"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog"
)


type vmRequestBody struct {
 	apiVersion string  `default:"v1"`
 	kind string `default:"Pod"`
 	metadata metav1.ObjectMeta
 	spec v1.PodSpec
 }

func getRequestBody(serverName, imageRef string, vcpu, memInMi int) (string, error){
	t := vmRequestBody{}
	t.metadata = metav1.ObjectMeta{
		Name: serverName,
		Namespace: "kube-system",
		Tenant: "system",
		Annotations: map[string]string{"VirtletCPUModel":"host-model"},
		}

	t.spec = v1.PodSpec{
		VirtualMachine: &v1.VirtualMachine{
			Image: imageRef,
			KeyPairName: "foobar",
			Name: serverName,
			PublicKey: "ssh-rsa AAA",
			Resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(strconv.Itoa(vcpu)),
					v1.ResourceMemory: resource.MustParse(strconv.Itoa(memInMi)+"Mi"),
				},
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(strconv.Itoa(vcpu)),
					v1.ResourceMemory: resource.MustParse(strconv.Itoa(memInMi)+"Mi"),
				},
			},

			},
	}

	klog.Infof("debug: requestBody: %v", t)

	b, err := json.Marshal(t)

	if err != nil {
		klog.Infof("debug: failed Marshaling request body. error: %v", err)
		return "", err
	}

	klog.Infof("debug: request body: %s", string(b))
	return string(b), nil

}