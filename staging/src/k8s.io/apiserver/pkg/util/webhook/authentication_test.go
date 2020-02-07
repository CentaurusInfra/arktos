/*
Copyright 2017 The Kubernetes Authors.
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

package webhook

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestAuthenticationDetection(t *testing.T) {
	tests := []struct {
		name       string
		kubeconfig clientcmdapi.Config
		serverName string
		expected   rest.KubeConfig
	}{
		{
			name:       "empty",
			serverName: "foo.com",
		},
		{
			name:       "fallback to current context",
			serverName: "foo.com",
			kubeconfig: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"bar.com": {Token: "bar"},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"ctx": {
						AuthInfo: "bar.com",
					},
				},
				CurrentContext: "ctx",
			},
			expected: rest.KubeConfig{BearerToken: "bar"},
		},
		{
			name:       "exact match",
			serverName: "foo.com",
			kubeconfig: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo.com": {Token: "foo"},
					"*.com":   {Token: "foo-star"},
					"bar.com": {Token: "bar"},
				},
			},
			expected: rest.KubeConfig{BearerToken: "foo"},
		},
		{
			name:       "partial star match",
			serverName: "foo.com",
			kubeconfig: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"*.com":   {Token: "foo-star"},
					"bar.com": {Token: "bar"},
				},
			},
			expected: rest.KubeConfig{BearerToken: "foo-star"},
		},
		{
			name:       "full star match",
			serverName: "foo.com",
			kubeconfig: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"*":       {Token: "star"},
					"bar.com": {Token: "bar"},
				},
			},
			expected: rest.KubeConfig{BearerToken: "star"},
		},
		{
			name:       "skip bad in cluster config",
			serverName: "kubernetes.default.svc",
			kubeconfig: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"*":       {Token: "star"},
					"bar.com": {Token: "bar"},
				},
			},
			expected: rest.KubeConfig{BearerToken: "star"},
		},
		{
			name:       "most selective",
			serverName: "one.two.three.com",
			kubeconfig: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"*.two.three.com": {Token: "first"},
					"*.three.com":     {Token: "second"},
					"*.com":           {Token: "third"},
				},
			},
			expected: rest.KubeConfig{BearerToken: "first"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := defaultAuthenticationInfoResolver{kubeconfig: tc.kubeconfig}
			actual, err := resolver.ClientConfigFor(tc.serverName)
			if err != nil {
				t.Fatal(err)
			}
			actualConfig := actual.GetConfig()
			actualConfig.UserAgent = ""
			actualConfig.Timeout = 0

			if !equality.Semantic.DeepEqual(*actualConfig, tc.expected) {
				t.Errorf("%v", diff.ObjectReflectDiff(tc.expected, *actualConfig))
			}
		})
	}

}
