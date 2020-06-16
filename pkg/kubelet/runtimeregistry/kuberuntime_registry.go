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
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	internalapi "k8s.io/cri-api/pkg/apis"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/kubelet/remote"
	"strings"
	"time"
)

const (
	RuntimeRequestTimeout = 15 * time.Second
	ContainerWorkloadType = "container"
	VmworkloadType        = "vm"
	UnknownType           = "TypeUnknown"
)

type RuntimeService struct {
	Name         string
	WorkloadType string
	EndpointUrl  string
	ServiceApi   internalapi.RuntimeService
	IsDefault    bool
	// primary runtime service the runtime service for cluster daemonset workload types
	// default to container runtime service
	// from runtime's perspective, nodeReady when the primary runtime service ready on the node
	IsPrimary bool
}

type ImageService struct {
	Name         string
	WorkloadType string
	EndpointUrl  string
	ServiceApi   internalapi.ImageManagerService
	IsDefault    bool
}

type KubeRuntimeRegistry struct {
	// gRPC service clients
	RuntimeServices map[string]*RuntimeService
	ImageServices   map[string]*ImageService
}

type Interface interface {
	// Get all runtime services supported on the node
	GetAllRuntimeServices() (map[string]*RuntimeService, error)
	// Get all image services supported on the node
	GetAllImageServices() (map[string]*ImageService, error)
	// Get the primary runtime service for the Arktos cluster
	GetPrimaryRuntimeService() (*RuntimeService, error)
	// Get the runtime service for particular workload type(VM or container)
	GetRuntimeServiceByWorkloadType(workloadtype string) (*RuntimeService, error)
	// Get the runtime service for particular workload type(VM or container)
	GetImageServiceByWorkloadType(workloadtype string) (*ImageService, error)
	// Get status for all runtime services
	GetAllRuntimeStatus() (map[string]map[string]bool, error)
}

func NewKubeRuntimeRegistry(remoteRuntimeEndpoints string) (*KubeRuntimeRegistry, error) {

	rs, is, err := buildRuntimeServicesMapFromAgentCommandArgs(remoteRuntimeEndpoints)

	if err != nil {
		klog.Errorf("Failed create the runtime service maps. Error: %v", err)
		return nil, err
	}

	return &KubeRuntimeRegistry{RuntimeServices: rs, ImageServices: is}, nil
}

func (r *KubeRuntimeRegistry) GetAllRuntimeServices() (map[string]*RuntimeService, error) {
	return r.RuntimeServices, nil
}

func (r *KubeRuntimeRegistry) GetAllImageServices() (map[string]*ImageService, error) {
	return r.ImageServices, nil
}

func (r *KubeRuntimeRegistry) GetPrimaryRuntimeService() (*RuntimeService, error) {
	for _, runtimeService := range r.RuntimeServices {
		if runtimeService.IsPrimary == true {
			return runtimeService, nil
		}
	}

	return nil, fmt.Errorf("primary runtime servcie is not defined")
}

func (r *KubeRuntimeRegistry) GetRuntimeServiceByWorkloadType(workloadtype string) (*RuntimeService, error) {
	for _, runtimeService := range r.RuntimeServices {
		if runtimeService.WorkloadType == workloadtype {
			return runtimeService, nil
		}
	}

	return nil, fmt.Errorf("runtime servcie for workload type %v is not defined", workloadtype)
}

func (r *KubeRuntimeRegistry) GetImageServiceByWorkloadType(workloadtype string) (*ImageService, error) {
	for _, imageService := range r.ImageServices {
		if imageService.WorkloadType == workloadtype {
			return imageService, nil
		}
	}

	return nil, fmt.Errorf("image servcie for workload type %v is not defined", workloadtype)
}

// Get status for all runtime services
func (r *KubeRuntimeRegistry) GetAllRuntimeStatus() (map[string]map[string]bool, error) {
	statuses := make(map[string]map[string]bool)
	vmServices := make(map[string]bool)
	containerServices := make(map[string]bool)

	for runtimeName, runtimeService := range r.RuntimeServices {
		workloadType := runtimeService.WorkloadType
		runtimeReady := true

		status, err := runtimeService.ServiceApi.Status()
		if err != nil || status == nil {
			runtimeReady = false
		}

		for _, c := range status.GetConditions() {
			if c.Status != true {
				runtimeReady = false
				break
			}
		}

		if workloadType == VmworkloadType {
			vmServices[runtimeName] = runtimeReady
		} else {
			containerServices[runtimeName] = runtimeReady
		}
	}

	statuses[VmworkloadType] = vmServices
	statuses[ContainerWorkloadType] = containerServices

	return statuses, nil
}

// remoteRuntimeEndpoint format:  runtimeName:workloadType:endpoint;runtimeName:workloadType:endpoint;
// first one is assumed as the default runtime service
// for legacy cluster agent without meeting the current format requirement,
// "default" will be the runtimeName and "container" will be the workloadType
// returns error if any of the runtime endpoint is malformatted or cannot be retrieved from remote service endpoint
// TODO: 1. runtime services set from configmap
//       2. support image_service_endpoint if needed
//
func buildRuntimeServicesMapFromAgentCommandArgs(remoteRuntimeEndpoints string) (map[string]*RuntimeService, map[string]*ImageService, error) {
	if remoteRuntimeEndpoints == "" {
		return nil, nil, fmt.Errorf("runtimeEndpoints is empty")
	}

	runtimeServices := make(map[string]*RuntimeService)
	imageServices := make(map[string]*ImageService)

	defer func() {
		klog.V(4).Infof("runtime services: %v\n", runtimeServices)
		for _, service := range runtimeServices {
			klog.V(4).Infof("runtime service name:%s, workloadType:%s, endPointUrl:%s, isDefault:%v, serviceApi:%v",
				service.Name, service.WorkloadType, service.EndpointUrl, service.IsDefault, service.ServiceApi)
		}
		klog.V(4).Infof("image services: %v\n", imageServices)
		for _, service := range imageServices {
			klog.V(4).Infof("image service name:%s, workloadType:%s, endPointUrl:%s, isDefault:%v, serviceApi:%v",
				service.Name, service.WorkloadType, service.EndpointUrl, service.IsDefault, service.ServiceApi)
		}
	}()

	endpoints := strings.Split(remoteRuntimeEndpoints, ";")

	// Mostly legacy cluster agent
	if len(endpoints) == 1 {
		name, workloadType, endpointUrl, err := parseEndpoint(endpoints[0])
		if err != nil {
			return nil, nil, err
		}

		rs, is, err := getRuntimeAndImageServices(endpointUrl, endpointUrl, metav1.Duration{RuntimeRequestTimeout})
		if err != nil {
			return nil, nil, err
		}

		runtimeServices[name] = &RuntimeService{name,
			workloadType, endpointUrl, rs, true, true}
		imageServices[name] = &ImageService{name,
			workloadType, endpointUrl, is, true}

		return runtimeServices, imageServices, nil
	}

	firstContainerType := true
	firstVmType := true

	// Arktos node agent format. multiple runtime services.
	for _, endpoint := range endpoints {
		name, workloadType, endpointUrl, err := parseEndpoint(endpoint)
		if err != nil {
			return nil, nil, err
		}

		rs, is, err := getRuntimeAndImageServices(endpointUrl, endpointUrl, metav1.Duration{RuntimeRequestTimeout})
		if err != nil {
			return nil, nil, err
		}

		setDefault := false
		setPrimry := false

		if workloadType == ContainerWorkloadType && firstContainerType == true ||
			workloadType == VmworkloadType && firstVmType == true {
			setDefault = true
		}

		// Consider first container runtime as primary that must be ready for the node ready condition
		if workloadType == ContainerWorkloadType && firstContainerType == true {
			setPrimry = true
		}

		runtimeServices[name] = &RuntimeService{name,
			workloadType, endpointUrl, rs, setDefault, setPrimry}
		imageServices[name] = &ImageService{name,
			workloadType, endpointUrl, is, setDefault}

		if workloadType == ContainerWorkloadType {
			firstContainerType = false
		}
		if workloadType == VmworkloadType {
			firstVmType = false
		}
	}

	return runtimeServices, imageServices, nil
}

func parseEndpoint(endpoint string) (string, string, string, error) {
	if endpoint == "" {
		return "", "", "", fmt.Errorf("endpoint is empty")
	}

	endpointEle := strings.Split(endpoint, ",")
	var name, workloadType, endpointUrl string

	switch len(endpointEle) {
	case 1:
		// case 1: old format from kubelet commandline, The commandline itself is the endpointUrl
		name = "default"
		workloadType = ContainerWorkloadType // this should be for all workload types if there is only one endpoint
		endpointUrl = endpointEle[0]
	case 3:
		// case 2: only one runtime service endpoint is specified and is formatted as name,worklaodtype,endpointUrl
		name = endpointEle[0]
		workloadType = endpointEle[1]
		endpointUrl = endpointEle[2]
	default:
		return "", "", "", fmt.Errorf("runtimeEndpoint element [%s] with format error", endpointEle)
	}

	return name, workloadType, endpointUrl, nil
}

func getRuntimeAndImageServices(remoteRuntimeEndpoint string, remoteImageEndpoint string, runtimeRequestTimeout metav1.Duration) (internalapi.RuntimeService, internalapi.ImageManagerService, error) {
	rs, err := remote.NewRemoteRuntimeService(remoteRuntimeEndpoint, runtimeRequestTimeout.Duration)
	if err != nil {
		return nil, nil, err
	}
	is, err := remote.NewRemoteImageService(remoteImageEndpoint, runtimeRequestTimeout.Duration)
	if err != nil {
		return nil, nil, err
	}
	return rs, is, err
}
