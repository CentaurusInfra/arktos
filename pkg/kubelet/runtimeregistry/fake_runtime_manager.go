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

package runtimeregistry

import (
	"fmt"
	internalapi "k8s.io/cri-api/pkg/apis"
)

// fakeRuntimeManager is a fake runtime manager for testing.
type FakeRuntimeManager struct {
	runtimeServices map[string]*RuntimeService
	imageServices   map[string]*ImageService
}

func NewFakeRuntimeManager(runtimeService internalapi.RuntimeService, imageService internalapi.ImageManagerService) *FakeRuntimeManager {
	rs := RuntimeService{Name: "test", WorkloadType: "container", ServiceApi: runtimeService, IsPrimary: true, IsDefault: true, EndpointUrl: ""}
	is := ImageService{Name: "test", WorkloadType: "container", ServiceApi: imageService, IsDefault: true, EndpointUrl: ""}

	return &FakeRuntimeManager{
		runtimeServices: map[string]*RuntimeService{rs.Name: &rs},
		imageServices:   map[string]*ImageService{is.Name: &is},
	}
}

func (r *FakeRuntimeManager) GetAllRuntimeServices() (map[string]*RuntimeService, error) {
	return r.runtimeServices, nil
}

func (r *FakeRuntimeManager) GetAllImageServices() (map[string]*ImageService, error) {
	return r.imageServices, nil
}

func (r *FakeRuntimeManager) GetPrimaryRuntimeService() (*RuntimeService, error) {
	for _, runtimeService := range r.runtimeServices {
		if runtimeService.IsPrimary == true {
			return runtimeService, nil
		}
	}

	return nil, fmt.Errorf("primary runtime servcie is not defined")
}

func (r *FakeRuntimeManager) GetRuntimeServiceByWorkloadType(workloadtype string) (*RuntimeService, error) {
	for _, runtimeService := range r.runtimeServices {
		if runtimeService.WorkloadType == workloadtype {
			return runtimeService, nil
		}
	}

	return nil, fmt.Errorf("runtime servcie for workload type %v is not defined", workloadtype)
}

func (r *FakeRuntimeManager) GetImageServiceByWorkloadType(workloadtype string) (*ImageService, error) {
	for _, imageService := range r.imageServices {
		if imageService.WorkloadType == workloadtype {
			return imageService, nil
		}
	}

	return nil, fmt.Errorf("image servcie for workload type %v is not defined", workloadtype)
}

// Get status for all runtime services
func (r *FakeRuntimeManager) GetAllRuntimeStatus() (map[string]map[string]bool, error) {
	return nil, nil
}
