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
	"fmt"
	"k8s.io/klog"
	v1 "k8s.io/api/core/v1"
)

type EventType string

const (
	EventType_Create EventType = "Create"
	EventType_Update EventType = "Update"
	EventType_Delete EventType = "Delete"
	EventType_Resume EventType = "Resume"
)

type KeyWithEventType struct {
	EventType       EventType
	Key             string
	ResourceVersion string
}

type StartHandler func(interface{}, string)

func ConvertToServiceEndpointContract(endpoints *v1.Endpoints) *BuiltinsServiceEndpointMessage {
	return &BuiltinsServiceEndpointMessage{
		Name:       endpoints.Name,
		BackendIps: []string{"TBD"},
		Ports:      []*PortsMessage{},
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
	var nodeName, nodeAddress string
	if node == nil {
		return nil
	}
	nodeName = node.Name
	if nodeName == "" {
		return nil
	}
	conditions := node.Status.Conditions
	if conditions == nil {
		return nil
	}
	addresses := node.Status.Addresses
	if addresses == nil {
		nodeAddress = ""
	}
	var nodeAddr, nodeAddressType string
	for i := 0; i < len(addresses); i++ {
		nodeAddressType = fmt.Sprintf("%s", addresses[i].Type)
		nodeAddr = fmt.Sprintf("%s", addresses[i].Address)
		if nodeAddressType == NodeInternalIP {
			nodeAddress = nodeAddr
			break
		}
	}
	resource := BuiltinsNodeMessage{
		Name: nodeName,
		Ip:   nodeAddress,
	}
	klog.Infof("Node controller is sending node info to Mizar %v", resource)
	return &BuiltinsNodeMessage{
		Name: nodeName,
		Ip:   nodeAddress,
	}
}

//ServiceEndpoint => ServiceEndpoint gRPC interface
func ConvertToServiceEndpointFrontContract(endpoints *v1.Endpoints, service *v1.Service) *BuiltinsServiceEndpointMessage {
	if endpoints == nil || service == nil {
		klog.Errorf("Endpoints or Service is nil")
		return nil
	}
	//Endpoints port info
	subsets := endpoints.Subsets
	if subsets == nil {
		klog.Warningf("Failed to retrieve endpoints subsets in local cache by tenant, name - %v, %v, %v", endpoints.Namespace, endpoints.Tenant, endpoints.Name)
		return nil
	}
	var endPoint ServiceEndpoint
	for i := 0; i < len(subsets); i++ {
		subset := subsets[i]
		addresses := subset.Addresses
		ports := subset.Ports
		if addresses != nil && ports != nil {
			for j := 0; j < len(addresses); j++ {
				endPoint.addresses = append(endPoint.addresses, addresses[j].IP)
			}
			for j := 0; j < len(ports); j++ {
				epPort := ports[j].Port
				endPoint.ports = append(endPoint.ports, GetFrontPorts(service, epPort))
			}
		}
	}
	var endPointsMessage BuiltinsServiceEndpointMessage
	var portsMessage []*PortsMessage
	if endPoint.ports != nil {
		for i := 0; i < len(endPoint.ports); i++ {
			portMessage := PortsMessage{endPoint.ports[i].frontPort, endPoint.ports[i].backendPort, endPoint.ports[i].protocol}
			portsMessage = append(portsMessage, &portMessage)
		}
	}
	endPointsMessage = BuiltinsServiceEndpointMessage{
		Name:       endpoints.Name,
		Namespace:  endpoints.Namespace,
		Tenant:     endpoints.Tenant,
		BackendIps: endPoint.addresses,
		Ports:      portsMessage,
	}
	klog.Infof("Mizar Endpoints controller is sending endpoints info to Mizar %v", endPointsMessage)
	return &endPointsMessage
}

//This function returns front port, backend port, and protocol
//ServicePort: protocol, port (=service port = front port), targetPort (endpoint port = backend port)
//(e.g) ports: {protocol: TCP, port: 80,  targetPort: 9376 }
func GetFrontPorts(service *v1.Service, epPort int32) Ports {
	var ports Ports
	if service == nil {
		klog.Errorf("Service is nil - End point port: %v", epPort)
		return ports
	}
	serviceports := service.Spec.Ports
	if serviceports == nil {
		klog.Errorf("Service ports are not found - service: %s", service.Name)
		return ports
	}
	for i := 0; i < len(serviceports); i++ {
		serviceport := serviceports[i]
		targetPort := serviceport.TargetPort.IntVal
		if targetPort == epPort {
			ports.frontPort = fmt.Sprintf("%v", serviceport.Port)
			ports.backendPort = fmt.Sprintf("%v", serviceport.TargetPort)
			ports.protocol = fmt.Sprintf("%v", serviceport.Protocol)
			return ports
		}
	}
	return ports
}
