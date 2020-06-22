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

package runtimeregistry

import (
	"errors"
	"k8s.io/kubernetes/pkg/kubelet/remote"
	"testing"

	"github.com/stretchr/testify/assert"
	fakeruntime "k8s.io/kubernetes/pkg/kubelet/remote/fake"
)

func TestBuildRuntimeServiceMap(t *testing.T) {
	fakeRuntimeEndpoint := "unix:///tmp/kubelet_remote.sock"
	fakeRuntimeEndpoint2 := "unix:///tmp/kubelet_remote.sock"

	fakeRuntime := fakeruntime.NewFakeRemoteRuntime()
	fakeRuntime.Start(fakeRuntimeEndpoint)
	defer fakeRuntime.Stop()

	fakeRuntime2 := fakeruntime.NewFakeRemoteRuntime()
	fakeRuntime2.Start(fakeRuntimeEndpoint2)
	defer fakeRuntime2.Stop()

	testCases := []struct {
		description             string
		remoteRuntimeEndpoints  string
		expectedRuntimeServices map[string]*RuntimeService
		expectedImageServices   map[string]*ImageService
		expectedError           error
	}{
		{
			"legacy cluster endpoint",
			fakeRuntimeEndpoint,
			makeExpectedRuntimeServiceForLegacyEndpoint(fakeRuntimeEndpoint),
			makeExpectedImageServiceForLegacyEndpoint(fakeRuntimeEndpoint),
			nil,
		},
		{
			"arktos formatted endpoints, vm and container with same endpoint",
			"containerRuntime,container," + fakeRuntimeEndpoint + ";" + "vmRuntime,vm," + fakeRuntimeEndpoint,
			map[string]*RuntimeService{
				"containerRuntime": {"containerRuntime", "container", fakeRuntimeEndpoint, nil, true, true},
				"vmRuntime":        {"vmRuntime", "vm", fakeRuntimeEndpoint, nil, true, false},
			},
			map[string]*ImageService{
				"containerRuntime": {"containerRuntime", "container", fakeRuntimeEndpoint, nil, true},
				"vmRuntime":        {"vmRuntime", "vm", fakeRuntimeEndpoint, nil, true},
			},
			nil,
		},
		{
			"arktos formatted endpoints, vm and container with different endpoints",
			"containerRuntime,container," + fakeRuntimeEndpoint + ";" + "vmRuntime,vm," + fakeRuntimeEndpoint2,
			map[string]*RuntimeService{
				"containerRuntime": {"containerRuntime", "container", fakeRuntimeEndpoint, nil, true, true},
				"vmRuntime":        {"vmRuntime", "vm", fakeRuntimeEndpoint2, nil, true, false},
			},
			map[string]*ImageService{
				"containerRuntime": {"containerRuntime", "container", fakeRuntimeEndpoint, nil, true},
				"vmRuntime":        {"vmRuntime", "vm", fakeRuntimeEndpoint2, nil, true},
			},
			nil,
		},
		{
			"Invalid case: empty input",
			"",
			nil,
			nil,
			errors.New("runtimeEndpoints is empty"),
		},
	}
	for _, tc := range testCases {
		t.Logf("TestCase: %s", tc.description)
		actualRuntimeServices, actualImageServices, err := buildRuntimeServicesMapFromAgentCommandArgs(tc.remoteRuntimeEndpoints)

		if err != nil && tc.expectedError.Error() != err.Error() {
			t.Errorf("Expected Error %v; actual error: %v", tc.expectedError, err)
		}
		if !compareRuntimeService(tc.expectedRuntimeServices, actualRuntimeServices) {
			assert.Equal(t, tc.expectedRuntimeServices, actualRuntimeServices)
		}
		if !compareImageService(tc.expectedImageServices, actualImageServices) {
			assert.Equal(t, tc.expectedImageServices, actualImageServices)
		}
	}
}

func makeExpectedRuntimeServiceForLegacyEndpoint(endpoint string) map[string]*RuntimeService {
	runtime, _ := remote.NewRemoteRuntimeService(endpoint, RuntimeRequestTimeout)
	return map[string]*RuntimeService{
		"default": {"default", "container", endpoint, runtime, true, true},
	}
}
func makeExpectedImageServiceForLegacyEndpoint(endpoint string) map[string]*ImageService {
	runtime, _ := remote.NewRemoteImageService(endpoint, RuntimeRequestTimeout)
	return map[string]*ImageService{
		"default": {"default", "container", endpoint, runtime, true},
	}
}

func compareRuntimeService(expected map[string]*RuntimeService, actual map[string]*RuntimeService) bool {
	if len(expected) != len(actual) {
		return false
	}

	for k := range expected {
		if expected[k].Name != actual[k].Name ||
			expected[k].WorkloadType != actual[k].WorkloadType ||
			expected[k].EndpointUrl != actual[k].EndpointUrl ||
			expected[k].IsDefault != actual[k].IsDefault {
			return false
		}
	}
	return true
}
func compareImageService(expected map[string]*ImageService, actual map[string]*ImageService) bool {
	if len(expected) != len(actual) {
		return false
	}

	for k := range expected {
		if expected[k].Name != actual[k].Name ||
			expected[k].WorkloadType != actual[k].WorkloadType ||
			expected[k].EndpointUrl != actual[k].EndpointUrl ||
			expected[k].IsDefault != actual[k].IsDefault {
			return false
		}
	}
	return true
}
