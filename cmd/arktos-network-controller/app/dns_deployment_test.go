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
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDeployDNSForNetwork(t *testing.T) {
	const testTagKey = "test-tag"
	const tagValObjExist = "sa-rbac exist"

	tcs := []struct {
		desc            string
		input           *v1.Network
		objects         []runtime.Object
		expectedTestTag string
	}{
		{
			desc: "happy path",
			input: &v1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "net-happy-path",
					Tenant: "test-te",
				},
				Spec: v1.NetworkSpec{
					Type: "flat",
				},
				Status: v1.NetworkStatus{},
			},
			objects: []runtime.Object{},
		},
		{
			desc: "sa-rbac exist",
			input: &v1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "net-sa-rbac-exist",
					Tenant: "test-te",
				},
				Spec: v1.NetworkSpec{
					Type: "flat",
				},
				Status: v1.NetworkStatus{},
			},
			expectedTestTag: tagValObjExist,
			objects: []runtime.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "coredns",
						Namespace:   "kube-system",
						Tenant:      "test-te",
						Annotations: map[string]string{testTagKey: tagValObjExist},
					},
				},
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "system:coredns",
						Tenant:      "test-te",
						Annotations: map[string]string{testTagKey: tagValObjExist},
					},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "system:coredns",
						Tenant:      "test-te",
						Annotations: map[string]string{testTagKey: tagValObjExist},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			namePerNetwork := "coredns-" + tc.input.Name
			kubeClient := fake.NewSimpleClientset(tc.objects...)
			err := deployDNSForNetwork(tc.input, kubeClient, "", "cluster.local", "192.168.0.3", "6443")

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			sa, err := kubeClient.CoreV1().ServiceAccountsWithMultiTenancy("kube-system", tc.input.Tenant).Get("coredns", metav1.GetOptions{})
			if err != nil {
				t.Errorf("failed to find sa %s/kube-syste/coredns: %v", tc.input.Tenant, err)
			}
			if len(tc.expectedTestTag) > 0 {
				val := sa.Annotations[testTagKey]
				if val != tc.expectedTestTag {
					t.Errorf("expected %q; got %q", tc.expectedTestTag, val)
				}
			}

			clusterRole, err := kubeClient.RbacV1().ClusterRolesWithMultiTenancy(tc.input.Tenant).Get("system:coredns", metav1.GetOptions{})
			if err != nil {
				t.Errorf("failed to find cluster role %s/system:coredns: %v", tc.input.Tenant, err)
			}
			if len(tc.expectedTestTag) > 0 {
				val := clusterRole.Annotations[testTagKey]
				if val != tc.expectedTestTag {
					t.Errorf("expected %q; got %q", tc.expectedTestTag, val)
				}
			}

			clusterRoleBinding, err := kubeClient.RbacV1().ClusterRoleBindingsWithMultiTenancy(tc.input.Tenant).Get("system:coredns", metav1.GetOptions{})
			if err != nil {
				t.Errorf("failed to find cluster role %s/system:coredns: %v", tc.input.Tenant, err)
			}
			if len(tc.expectedTestTag) > 0 {
				val := clusterRoleBinding.Annotations[testTagKey]
				if val != tc.expectedTestTag {
					t.Errorf("expected %q; got %q", tc.expectedTestTag, val)
				}
			}

			_, err = kubeClient.CoreV1().ConfigMapsWithMultiTenancy("kube-system", tc.input.Tenant).Get(namePerNetwork, metav1.GetOptions{})
			if err != nil {
				t.Errorf("failed to find configmap %s/kube-system/%s: %v", tc.input.Tenant, namePerNetwork, err)
			}

			_, err = kubeClient.AppsV1().DeploymentsWithMultiTenancy("kube-system", tc.input.Tenant).Get(namePerNetwork, metav1.GetOptions{})
			if err != nil {
				t.Errorf("failed to find deployment %s/kube-system/%s: %v", tc.input.Tenant, namePerNetwork, err)
			}
		})
	}
}
