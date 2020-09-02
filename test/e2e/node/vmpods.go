/*
Copyright 2016 The Kubernetes Authors.
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

package node

import (
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/kubernetes/test/e2e/framework"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

// smoke level e2e case
var _ = SIGDescribe("Basic VM Type test", func() {
	f := framework.NewDefaultFramework("podswithvmtype")

	framework.KubeDescribe("Pods with vm type", func() {
		var podClient *framework.PodClient
		ginkgo.BeforeEach(func() {
			podClient = f.PodClient()
		})

		framework.ConformanceIt("should be submitted and removed [Arktos-CI]", func() {
			ginkgo.By("creating the pod")
			name := "pod-" + string(uuid.NewUUID())
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"name": name,
					},
				},
				Spec: v1.PodSpec{
					VirtualMachine: &v1.VirtualMachine{
						KeyPairName: "foobar",
						Name:        "vm1",
						Image:       imageutils.GetE2EImage(imageutils.Cirros),
						Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("1"),
								v1.ResourceMemory: resource.MustParse("500Mi"),
							},
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("1"),
								v1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
					},
				},
			}

			ginkgo.By("submitting the pod to kubernetes")
			podClient.Create(pod)

			ginkgo.By("verifying QOS class is set on the pod")
			pod, err := podClient.Get(name, metav1.GetOptions{})
			framework.ExpectNoError(err, "failed to query pod")
			gomega.Expect(pod.Status.QOSClass == v1.PodQOSGuaranteed)
		})
	})
})
