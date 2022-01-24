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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog"
	v12 "k8s.io/api/apps/v1"
)

type batchRequestBody struct {
	ApiVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	MetaData   metav1.ObjectMeta `json:"metadata"`
	Spec       v12.ReplicaSetSpec       `json:"spec"`
}

func getReplicasetRequestBody(replicas int, serverName, imageRef string, vcpu, memInMi int) (string, error) {
	t := batchRequestBody{}
	t.ApiVersion = "apps/v1"
	t.Kind = "ReplicaSet"
	t.MetaData = metav1.ObjectMeta{
		Name:        serverName,
		Namespace:   "kube-system",
		Tenant:      "system",
	}

	i := int32(replicas)
	t.Spec = v12.ReplicaSetSpec{
		Replicas: &i,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"ln": serverName},
	},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"VirtletCPUModel": "host-model"},
				Labels: map[string]string{"ln": serverName},
			},
			Spec: v1.PodSpec{
				VirtualMachine: &v1.VirtualMachine{
					Image:       imageRef,
					KeyPairName: "foobar",
					Name:        serverName,
					PublicKey:   "ssh-rsa AAA",
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse(strconv.Itoa(vcpu)),
							v1.ResourceMemory: resource.MustParse(strconv.Itoa(memInMi) + "Mi"),
						},
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse(strconv.Itoa(vcpu)),
							v1.ResourceMemory: resource.MustParse(strconv.Itoa(memInMi) + "Mi"),
						},
					},
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

type vmRequestBody struct {
	ApiVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	MetaData   metav1.ObjectMeta `json:"metadata"`
	Spec       v1.PodSpec        `json:"spec"`
}

func getRequestBody(serverName, imageRef string, vcpu, memInMi int) (string, error) {
	t := vmRequestBody{}
	t.ApiVersion = "v1"
	t.Kind = "Pod"
	t.MetaData = metav1.ObjectMeta{
		Name:        serverName,
		Namespace:   "kube-system",
		Tenant:      "system",
		Annotations: map[string]string{"VirtletCPUModel": "host-model"},
	}

	t.Spec = v1.PodSpec{
		VirtualMachine: &v1.VirtualMachine{
			Image:       imageRef,
			KeyPairName: "foobar",
			Name:        serverName,
			PublicKey:   "ssh-rsa AAA",
			Resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(strconv.Itoa(vcpu)),
					v1.ResourceMemory: resource.MustParse(strconv.Itoa(memInMi) + "Mi"),
				},
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(strconv.Itoa(vcpu)),
					v1.ResourceMemory: resource.MustParse(strconv.Itoa(memInMi) + "Mi"),
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
