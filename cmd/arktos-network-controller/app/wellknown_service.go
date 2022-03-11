/*
Copyright 2020 Authors of Arktos.

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

package app

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	v1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

func createOrGetKubernetesService(net *v1.Network, svcClient kubernetes.Interface) (*corev1.Service, error) {
	nsK8s := metav1.NamespaceDefault
	nameKubernetes := types.KubernetesServiceName + "-" + net.Name
	kubernetesService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameKubernetes,
			Tenant:    net.Tenant,
			Namespace: nsK8s,
			Labels: map[string]string{
				v1.NetworkLabel:      net.Name,
				clusterAddonLabelKey: nameKubernetes,
				"provider":           "kubernetes",
				"component":          "apiserver",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Protocol:   "TCP",
					Port:       443,
					TargetPort: intstr.FromInt(6443),
				},
			},
			Selector: map[string]string{
				clusterAddonLabelKey: nameKubernetes,
			},
			SessionAffinity: corev1.ServiceAffinityNone,
			Type:            corev1.ServiceTypeClusterIP,
		},
	}
	return getOrCreateWellKnownService(svcClient, &kubernetesService)
}

func createOrGetDNSService(net *v1.Network, svcClient kubernetes.Interface) (*corev1.Service, error) {
	nsDNS := metav1.NamespaceSystem
	nameDNS := dnsServiceDefaultName + "-" + net.Name
	dnsService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameDNS,
			Tenant:    net.Tenant,
			Namespace: nsDNS,
			Labels: map[string]string{
				v1.NetworkLabel:      net.Name,
				clusterAddonLabelKey: nameDNS,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "dns",
					Protocol:   "UDP",
					Port:       53,
					TargetPort: intstr.FromInt(53),
				},
				{
					Name:       "dns-tcp",
					Protocol:   "TCP",
					Port:       53,
					TargetPort: intstr.FromInt(53),
				},
				{
					Name:       "metrics",
					Protocol:   "TCP",
					Port:       9153,
					TargetPort: intstr.FromInt(9153),
				},
			},
			Selector: map[string]string{
				clusterAddonLabelKey: nameDNS,
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	return getOrCreateWellKnownService(svcClient, &dnsService)
}

func getOrCreateWellKnownService(svcClient kubernetes.Interface, svc *corev1.Service) (*corev1.Service, error) {
	result, err := svcClient.CoreV1().ServicesWithMultiTenancy(svc.Namespace, svc.Tenant).Get(svc.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		result, err = svcClient.CoreV1().ServicesWithMultiTenancy(svc.Namespace, svc.Tenant).Create(svc)
		if err != nil {
			klog.Errorf("Failed to create service %v/%v/%v. Error %v", svc.Tenant, svc.Namespace, svc.Name, err)
			return nil, err
		}
	}
	return result, err
}
