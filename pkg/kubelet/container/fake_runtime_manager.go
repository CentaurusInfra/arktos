/*
Copyright 2015 The Kubernetes Authors.

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
	internalapi "k8s.io/cri-api/pkg/apis"
)

// fakeRuntimeManager is a fake runtime manager for testing.
type fakeRuntimeManager struct {
	runtimeService internalapi.RuntimeService
	imageService   internalapi.ImageManagerService
}

func NewFakeRuntimeManager(runtimeService internalapi.RuntimeService, imageService internalapi.ImageManagerService) RuntimeManager {
	return &fakeRuntimeManager{
		runtimeService: runtimeService,
		imageService:   imageService,
	}
}

// GetRuntimeServiceByPod returns the runtime service for a given pod.
func (m *fakeRuntimeManager) GetRuntimeServiceByPod(pod *v1.Pod) (internalapi.RuntimeService, error) {
	return m.runtimeService, nil
}

// GetAllRuntimeServices returns all the runtime services.
func (m *fakeRuntimeManager) GetAllRuntimeServices() ([]internalapi.RuntimeService, error) {
	return []internalapi.RuntimeService{m.runtimeService}, nil
}

// GetAllImageServices returns all the image services.
func (m *fakeRuntimeManager) GetAllImageServices() ([]internalapi.ImageManagerService, error) {
	return []internalapi.ImageManagerService{m.imageService}, nil
}
