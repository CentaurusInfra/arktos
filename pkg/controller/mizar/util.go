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
