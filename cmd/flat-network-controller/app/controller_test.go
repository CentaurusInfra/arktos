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

// The external network controller is responsible for running controller loops for the flat network providers.
// Most of canonical CNI plugins can be used on so-called flat networks.

package app

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	fakearktosv1 "k8s.io/arktos-ext/pkg/generated/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
	coremock "k8s.io/client-go/testing"
	api "k8s.io/kubernetes/pkg/apis/core"
)

func TestManageFlatNetwork(t *testing.T) {
	tcs := []struct {
		desc           string
		input          *v1.Network
		svcResp        *corev1.Service
		svcRespError   error
		netRespError   error
		expectingError bool
	}{
		{
			desc: "happy path",
			input: &v1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-ne",
					Tenant: "test-te",
				},
				Spec: v1.NetworkSpec{
					Type: "flat",
				},
				Status: v1.NetworkStatus{},
			},
			svcResp: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: corev1.ServiceSpec{
					ClusterIP: "11.22.33.44",
					Type:      "ClusterIP",
				},
				Status: corev1.ServiceStatus{},
			},
			expectingError: false,
		},
		{
			desc: "dns service unable to create",
			input: &v1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-ne",
					Tenant: "test-te",
				},
				Spec: v1.NetworkSpec{
					Type: "flat",
				},
				Status: v1.NetworkStatus{},
			},
			svcResp:        &corev1.Service{},
			svcRespError:   fmt.Errorf("fake test error: failed to create DNS service"),
			expectingError: true,
		},
		{
			desc: "dns service already exists",
			input: &v1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-ne",
					Tenant: "test-te",
				},
				Spec: v1.NetworkSpec{
					Type: "flat",
				},
				Status: v1.NetworkStatus{},
			},
			svcResp: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Tenant:    "test-te",
					Namespace: "kube-system",
					Name:      "kube-dns-test-ne",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "11.22.33.44",
					Type:      "ClusterIP",
				},
				Status: corev1.ServiceStatus{},
			},
			svcRespError:   errors.NewAlreadyExists(api.Resource("service"), "kube-dns-test-ne"),
			expectingError: false,
		},
		{
			desc: "network status update failed",
			input: &v1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-ne",
					Tenant: "test-te",
				},
				Spec: v1.NetworkSpec{
					Type: "flat",
				},
				Status: v1.NetworkStatus{},
			},
			svcResp: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: corev1.ServiceSpec{
					ClusterIP: "11.22.33.44",
					Type:      "ClusterIP",
				},
				Status: corev1.ServiceStatus{},
			},
			netRespError:   fmt.Errorf("fake test error: failed to  update network status"),
			expectingError: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			netClient := fakearktosv1.NewSimpleClientset(tc.input)
			netClient.PrependReactor("update", "networks", func(action coremock.Action) (handled bool, ret runtime.Object, err error) {
				if action.GetSubresource() != "status" {
					t.Fatalf("unexpected update")
				}
				return true, nil, tc.netRespError
			})
			kubeClient := fake.NewSimpleClientset(tc.svcResp)
			kubeClient.PrependReactor("create", "services", func(action coremock.Action) (handled bool, ret runtime.Object, err error) {
				return true, tc.svcResp, tc.svcRespError
			})

			err := manageFlatNetwork(tc.input, netClient, kubeClient, false, "cluster.local")

			if !tc.expectingError && err != nil {
				t.Errorf("got unexpected error: %v", err)
			}

			if tc.expectingError && err == nil {
				t.Error("expected error, got nil")
			}

			if tc.expectingError && err != nil {
				t.Logf("got expected error: %v", err)
			}
		})
	}
}
