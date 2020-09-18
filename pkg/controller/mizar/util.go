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

import (
	"encoding/json"
	"strconv"
	v1 "k8s.io/api/core/v1"	
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

func ConvertToServiceEndpointContract(endpoints *v1.Endpoints, service *v1.Service) *BuiltinsServiceEndpointMessage {
	backendIps := []string{}
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			backendIps = append(backendIps, address.IP)
		}
	}
	backendIpsJson, _ := json.Marshal(backendIps)

	ports := []*PortsMessage{}
	for _, port := range service.Spec.Ports {
		portsMessage := &PortsMessage{
			FrontendPort: strconv.Itoa(int(port.Port)),
			BackendPort:  strconv.Itoa(int(port.TargetPort.IntVal)),
			Protocol:     string(port.Protocol),
		}
		ports = append(ports, portsMessage)
	}
	portsJson, _ := json.Marshal(ports)

	return &BuiltinsServiceEndpointMessage{
		Name:           endpoints.Name,
		Namespace:      endpoints.Namespace,
		Tenant:         endpoints.Tenant,
		BackendIps:     []string{},
		Ports:          []*PortsMessage{},
		BackendIpsJson: string(backendIpsJson),
		PortsJson:      string(portsJson),
	}
}

func ConvertToPodContract(pod *v1.Pod) *BuiltinsPodMessage {
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
	ip := ""
	for _, item := range node.Status.Addresses {
		if item.Type == "InternalIP" {
			ip = item.Address
			break
		}
	}
	return &BuiltinsNodeMessage{
		Name: node.Name,
		Ip:   ip,
	}
}
