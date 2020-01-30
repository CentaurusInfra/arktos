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

package container

import (
	"k8s.io/api/core/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// fakeRuntimeManager is a fake runtime manager for testing.
type FakeRuntimeManager struct {
	RuntimeService internalapi.RuntimeService
	ImageService   internalapi.ImageManagerService
}

func NewFakeRuntimeManager(runtimeService internalapi.RuntimeService, imageService internalapi.ImageManagerService) RuntimeManager {
	return &FakeRuntimeManager{
		RuntimeService: runtimeService,
		ImageService:   imageService,
	}
}

// GetRuntimeServiceByPod returns the runtime service for a given pod.
func (m *FakeRuntimeManager) GetRuntimeServiceByPod(pod *v1.Pod) (internalapi.RuntimeService, error) {
	return m.RuntimeService, nil
}

// GetAllRuntimeServices returns all the runtime services.
func (m *FakeRuntimeManager) GetAllRuntimeServices() ([]internalapi.RuntimeService, error) {
	return []internalapi.RuntimeService{m.RuntimeService}, nil
}

// GetAllImageServices returns all the image services.
func (m *FakeRuntimeManager) GetAllImageServices() ([]internalapi.ImageManagerService, error) {
	return []internalapi.ImageManagerService{m.ImageService}, nil
}

func (m *FakeRuntimeManager) GetImageServiceByPod(pod *v1.Pod) (internalapi.ImageManagerService, error) {
	return m.ImageService, nil
}

func (m *FakeRuntimeManager) RuntimeVersion(service internalapi.RuntimeService) (Version, error) {
	return nil, nil
}

func (m *FakeRuntimeManager) GetTypedVersion(service internalapi.RuntimeService) (*runtimeapi.VersionResponse, error) {
	return nil, nil
}

func (m *FakeRuntimeManager) GetRuntimeServiceByPodID(podId kubetypes.UID) (internalapi.RuntimeService, error) {
	return m.RuntimeService, nil
}

func (m *FakeRuntimeManager) RuntimeStatus(runtimeService internalapi.RuntimeService) (*RuntimeStatus, error) {
	return nil, nil
}

func (m *FakeRuntimeManager) GetAllRuntimeStatus() (map[string]map[string]bool, error) {
	return nil, nil
}
