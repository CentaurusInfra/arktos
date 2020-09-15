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

package mizar

import v1 "k8s.io/api/core/v1"

type EventType int

const (
	EventType_Create EventType = 0
	EventType_Update EventType = 1
	EventType_Delete EventType = 2
	EventType_Resume EventType = 3
)

type KeyWithEventType struct {
	EventType EventType
	Key       string
)

type EventType string

const (
	EventType_Create EventType = "Create"
	EventType_Update EventType = "Update"
	EventType_Delete EventType = "Delete"
)

type KeyWithEventType struct {
	EventType       EventType
	Key             string
	ResourceVersion string
}

type StartHandler func(interface{}, string)

func ConvertToServiceContract(service *v1.Service) *BuiltinsServiceMessage {
	return &BuiltinsServiceMessage{
		Name:          service.Name,
		ArktosNetwork: "TBD",
		Namespace:     service.Namespace,
		Tenant:        service.Tenant,
		Ip:            "TBD",
	}
}

func ConvertToServiceEndpointContract(endpoints *v1.Endpoints) *BuiltinsServiceEndpointMessage {
	return &BuiltinsServiceEndpointMessage{
		Name:       endpoints.Name,
		BackendIps: []string{"TBD"},
		Ports:      []*PortsMessage{},
	}
}

func ConvertToPodContract(pod *v1.Pod) *BuiltinsPodMessage {
	return &BuiltinsPodMessage{
		Name:      pod.Name,
		HostIp:    pod.Status.HostIP,
		Namespace: pod.Namespace,
		Tenant:    pod.Tenant,
		Phase:     string(pod.Status.Phase),
	var network string
	if value, exists := pod.Labels["arktos.futurewei.com/network"]; exists {
		network = value
	} else {
		network = ""
	}

	return &BuiltinsPodMessage{
		Name:          pod.Name,
		HostIp:        pod.Status.HostIP,
		Namespace:     pod.Namespace,
		Tenant:        pod.Tenant,
		ArktosNetwork: network,
		Phase:         string(pod.Status.Phase),
	}
}

func ConvertToNodeContract(node *v1.Node) *BuiltinsNodeMessage {
	return &BuiltinsNodeMessage{
		Name: node.Name,
		Ip:   string(node.Status.Addresses[0].Address),
	}
}
