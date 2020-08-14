/*
Copyright 2019 The Kubernetes Authors.
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

package stats

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/metrics/testutil"
	statsapi "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

type mockSummaryProvider struct {
	mock.Mock
}

func (m *mockSummaryProvider) Get(updateStats bool) (*statsapi.Summary, error) {
	args := m.Called(updateStats)
	return args.Get(0).(*statsapi.Summary), args.Error(1)
}

func (m *mockSummaryProvider) GetCPUAndMemoryStats() (*statsapi.Summary, error) {
	args := m.Called()
	return args.Get(0).(*statsapi.Summary), args.Error(1)
}

func TestCollectResourceMetrics(t *testing.T) {
	testTime := metav1.NewTime(time.Unix(2, 0)) // a static timestamp: 2000

	tests := []struct {
		name                 string
		config               ResourceMetricsConfig
		summary              *statsapi.Summary
		summaryErr           error
		expectedMetricsNames []string
		expectedMetrics      string
	}{
		{
			name:                 "error getting summary",
			config:               Config(),
			summary:              nil,
			summaryErr:           fmt.Errorf("failed to get summary"),
			expectedMetricsNames: []string{"scrape_error"},
			expectedMetrics: `
				# HELP scrape_error [ALPHA] (Deprecated since 1.18.0) 1 if there was an error while getting container metrics, 0 otherwise
				# TYPE scrape_error gauge
				scrape_error 1
			`,
		},
		{
			name:   "arbitrary node metrics",
			config: Config(),
			summary: &statsapi.Summary{
				Node: statsapi.NodeStats{
					CPU: &statsapi.CPUStats{
						Time:                 testTime,
						UsageCoreNanoSeconds: uint64Ptr(10000000000),
					},
					Memory: &statsapi.MemoryStats{
						Time:            testTime,
						WorkingSetBytes: uint64Ptr(1000),
					},
				},
			},
			summaryErr: nil,
			expectedMetricsNames: []string{
				"node_cpu_usage_seconds_total",
				"node_memory_working_set_bytes",
				"scrape_error",
			},
			expectedMetrics: `
				# HELP node_cpu_usage_seconds_total [ALPHA] (Deprecated since 1.18.0) Cumulative cpu time consumed by the node in core-seconds
				# TYPE node_cpu_usage_seconds_total gauge
				node_cpu_usage_seconds_total 10 2000
				# HELP node_memory_working_set_bytes [ALPHA] (Deprecated since 1.18.0) Current working set of the node in bytes
				# TYPE node_memory_working_set_bytes gauge
				node_memory_working_set_bytes 1000 2000
				# HELP scrape_error [ALPHA] (Deprecated since 1.18.0) 1 if there was an error while getting container metrics, 0 otherwise
				# TYPE scrape_error gauge
				scrape_error 0
			`,
		},
		{
			name:   "arbitrary container metrics for different container, pods and namespaces",
			config: Config(),
			summary: &statsapi.Summary{
				Pods: []statsapi.PodStats{
					{
						PodRef: statsapi.PodReference{
							Name:      "pod_a",
							Namespace: "namespace_a",
						},
						Containers: []statsapi.ContainerStats{
							{
								Name: "container_a",
								CPU: &statsapi.CPUStats{
									Time:                 testTime,
									UsageCoreNanoSeconds: uint64Ptr(10000000000),
								},
								Memory: &statsapi.MemoryStats{
									Time:            testTime,
									WorkingSetBytes: uint64Ptr(1000),
								},
							},
							{
								Name: "container_b",
								CPU: &statsapi.CPUStats{
									Time:                 testTime,
									UsageCoreNanoSeconds: uint64Ptr(10000000000),
								},
								Memory: &statsapi.MemoryStats{
									Time:            testTime,
									WorkingSetBytes: uint64Ptr(1000),
								},
							},
						},
					},
					{
						PodRef: statsapi.PodReference{
							Name:      "pod_b",
							Namespace: "namespace_b",
						},
						Containers: []statsapi.ContainerStats{
							{
								Name: "container_a",
								CPU: &statsapi.CPUStats{
									Time:                 testTime,
									UsageCoreNanoSeconds: uint64Ptr(10000000000),
								},
								Memory: &statsapi.MemoryStats{
									Time:            testTime,
									WorkingSetBytes: uint64Ptr(1000),
								},
							},
						},
					},
				},
			},
			summaryErr: nil,
			expectedMetricsNames: []string{
				"container_cpu_usage_seconds_total",
				"container_memory_working_set_bytes",
				"scrape_error",
			},
			expectedMetrics: `
				# HELP scrape_error [ALPHA] (Deprecated since 1.18.0) 1 if there was an error while getting container metrics, 0 otherwise
				# TYPE scrape_error gauge
				scrape_error 0
				# HELP container_cpu_usage_seconds_total [ALPHA] (Deprecated since 1.18.0) Cumulative cpu time consumed by the container in core-seconds
				# TYPE container_cpu_usage_seconds_total gauge
				container_cpu_usage_seconds_total{container="container_a",namespace="namespace_a",pod="pod_a"} 10 2000
				container_cpu_usage_seconds_total{container="container_a",namespace="namespace_b",pod="pod_b"} 10 2000
				container_cpu_usage_seconds_total{container="container_b",namespace="namespace_a",pod="pod_a"} 10 2000
				# HELP container_memory_working_set_bytes [ALPHA] (Deprecated since 1.18.0) Current working set of the container in bytes
				# TYPE container_memory_working_set_bytes gauge
				container_memory_working_set_bytes{container="container_a",namespace="namespace_a",pod="pod_a"} 1000 2000
				container_memory_working_set_bytes{container="container_a",namespace="namespace_b",pod="pod_b"} 1000 2000
				container_memory_working_set_bytes{container="container_b",namespace="namespace_a",pod="pod_a"} 1000 2000
			`,
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockSummaryProvider{}
			provider.On("GetCPUAndMemoryStats").Return(tc.summary, tc.summaryErr)
			collector := NewPrometheusResourceMetricCollector(provider, tc.config)

			registry := testutil.NewFakeKubeRegistry("1.18.0")
			registry.CustomMustRegister(collector)
			if err := testutil.GatherAndCompare(registry, strings.NewReader(tc.expectedMetrics), tc.expectedMetricsNames...); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func uint64Ptr(u uint64) *uint64 {
	return &u
}
