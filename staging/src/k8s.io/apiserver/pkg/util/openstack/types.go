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

	v12 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

const (
	LABEL_SELECTOR_NAME="ln"
	DEFAULT_NAMESPACE="kube-system"
	DEFAULT_TENANT="system"
)
type batchRequestBody struct {
	ApiVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	MetaData   metav1.ObjectMeta  `json:"metadata"`
	Spec       v12.ReplicaSetSpec `json:"spec"`
}

func constructReplicasetRequestBody(replicas int, serverName, imageRef string, vcpu, memInMi int) ([]byte, error) {
	t := batchRequestBody{}
	t.ApiVersion = "apps/v1"
	t.Kind = "ReplicaSet"
	t.MetaData = metav1.ObjectMeta{
		Name:      serverName,
		Namespace: DEFAULT_NAMESPACE,
		Tenant:    DEFAULT_TENANT,
	}

	i := int32(replicas)
	t.Spec = v12.ReplicaSetSpec{
		Replicas: &i,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{LABEL_SELECTOR_NAME: serverName},
		},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"VirtletCPUModel": "host-model"},
				Labels:      map[string]string{LABEL_SELECTOR_NAME: serverName},
			},
			Spec: v1.PodSpec{
				VirtualMachine: constructVMSpec(serverName, imageRef, vcpu, memInMi),
			},
		},
	}

	b, err := json.Marshal(t)

	if err != nil {
		return nil, err
	}

	return b, nil

}

type vmRequestBody struct {
	ApiVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	MetaData   metav1.ObjectMeta `json:"metadata"`
	Spec       v1.PodSpec        `json:"spec"`
}

func constructVmPodRequestBody(serverName, imageRef string, vcpu, memInMi int) ([]byte, error) {
	t := vmRequestBody{}
	t.ApiVersion = "v1"
	t.Kind = "Pod"
	t.MetaData = metav1.ObjectMeta{
		Name:        serverName,
		Namespace:   DEFAULT_NAMESPACE,
		Tenant:      DEFAULT_TENANT,
		Annotations: map[string]string{"VirtletCPUModel": "host-model"},
	}

	t.Spec = v1.PodSpec{
		VirtualMachine: constructVMSpec(serverName, imageRef, vcpu, memInMi),
	}

	b, err := json.Marshal(t)

	if err != nil {
		return nil, err
	}

	return b, nil

}

func constructVMSpec(serverName, imageRef string, vcpu, memInMi int) *v1.VirtualMachine {
	return &v1.VirtualMachine{
		Image:       imageRef,
		ImagePullPolicy: v1.PullIfNotPresent,
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
	}
}