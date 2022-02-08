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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog"
	"strconv"

	v1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
)

type EventType string

const (
	EventType_Create EventType = "Create"
	EventType_Update EventType = "Update"
	EventType_Delete EventType = "Delete"

	InternalIP v1.NodeAddressType = "InternalIP"
	ExternalIP v1.NodeAddressType = "ExternalIP"

	Arktos_Network_Name string = "arktos.futurewei.com/network"
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

	klog.V(3).Infof("Endpoint Name: %s, Namespace: %s, Tenant: %s, Backend Ips: %s, Ports: %s",
		endpoints.Name, endpoints.Namespace, endpoints.Tenant, string(backendIpsJson), string(portsJson))

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
	var labels string

	if len(pod.Labels) != 0 {
		// Labels is a list of key-value pairs. Here is converting this
		// list of key-value pairs into json first,
		// then later this json will be convert into string
		// because grpc message type is string
		labelJson, err := json.Marshal(pod.Labels)
		if err != nil {
			klog.Errorf("Error in parsing pod labels into json: %v", err)
		}
		labels = string(labelJson)
	} else {
		labels = ""
	}

	if value, exists := pod.Labels[Arktos_Network_Name]; exists {
		network = value
	} else {
		network = ""
	}

	klog.V(3).Infof("Pod Name: %s, HostIP: %s, Namespace: %s, Tenant: %s, Labels: %s, Arktos network: %v",
		pod.Name, pod.Status.HostIP, pod.Namespace, pod.Tenant, labels, network)

	return &BuiltinsPodMessage{
		Name:          pod.Name,
		HostIp:        pod.Status.HostIP,
		Namespace:     pod.Namespace,
		Tenant:        pod.Tenant,
		Labels:        labels,
		ArktosNetwork: network,
		Phase:         string(pod.Status.Phase),
		Vpc:           string(pod.Annotations[mizarAnnotationsVpcKey]),
		Subnet:        string(pod.Annotations[mizarAnnotationsSubnetKey]),
	}
}

func ConvertToNodeContract(node *v1.Node) *BuiltinsNodeMessage {
	ip := ""
	for _, item := range node.Status.Addresses {
		if item.Type == InternalIP {
			ip = item.Address
			break
		}
	}

	klog.V(3).Infof("Node Name: %s, IP: %s", node.Name, ip)
	return &BuiltinsNodeMessage{
		Name: node.Name,
		Ip:   ip,
	}
}

func ConvertToNamespaceContract(namespace *v1.Namespace) *BuiltinsNamespaceMessage {
	var labels string

	if len(namespace.Labels) != 0 {
		// Labels is a list of key-value pairs. Here is converting this
		// list of key-value pairs into json first,
		// then later this json will be convert into string
		// because grpc message type is string
		labelJson, err := json.Marshal(namespace.Labels)
		if err != nil {
			klog.Errorf("Error in parsing namespace labels into json: %v", err)
		}
		labels = string(labelJson)
	} else {
		labels = ""
	}

	klog.V(3).Infof("Namespace Name: %s, Tenant: %s, Labels: %s",
		namespace.Name, namespace.Tenant, labels)

	return &BuiltinsNamespaceMessage{
		Name:   namespace.Name,
		Tenant: namespace.Tenant,
		Labels: labels,
	}
}

func ConvertToNetworkPolicyContract(policy *networking.NetworkPolicy) *BuiltinsNetworkPolicyMessage {
	klog.V(3).Infof("NetworkPolicy Name: %s, Namespace: %s, Tenant: %s",
		policy.Name, policy.Namespace, policy.Tenant)
	policyJson, err := json.Marshal(parseNetworkPolicySpecToMsg(policy.Spec))
	if err != nil {
		klog.Errorf("Error in parsing network policy spec into json: %v", err)
	}
	klog.V(3).Infof("Policy: %s", string(policyJson))

	return &BuiltinsNetworkPolicyMessage{
		Name:      policy.Name,
		Namespace: policy.Namespace,
		Tenant:    policy.Tenant,
		Policy:    string(policyJson),
	}
}

func parseNetworkPolicySpecToMsg(nps networking.NetworkPolicySpec) MizarNetworkPolicyPolicySpecMsg {
	policyMsg := MizarNetworkPolicyPolicySpecMsg{
		PodSel: parseNetworkPolicyPodSelectorToMsg(nps),
		In:     parseNetworkPolicyIngressRulesToMsg(nps.Ingress),
		Out:    parseNetworkPolicyEgressRulesToMsg(nps.Egress),
		Type:   policyTypesToStringArray(nps.PolicyTypes),
	}

	return policyMsg
}

func parseNetworkPolicyPodSelectorToMsg(nps networking.NetworkPolicySpec) MizarNetworkPolicyPodSelector {
	if len(nps.PodSelector.MatchLabels) == 0 {
		return MizarNetworkPolicyPodSelector{}
	}
	return MizarNetworkPolicyPodSelector{
		MatchLabels: nps.PodSelector.MatchLabels,
	}
}

func policyTypesToStringArray(pts []networking.PolicyType) []string {
	strPts := []string{}
	if pts != nil {
		for _, p := range pts {
			strPts = append(strPts, string(p))
		}
	}
	return strPts
}

func parseNetworkPolicyIngressRulesToMsg(npirs []networking.NetworkPolicyIngressRule) []MizarNetworkPolicyIngressMsg {
	ingressPorts := []MizarNetworkPolicyPortSelector{}
	froms := []MizarNetworkPolicyRule{}
	ingressRules := []MizarNetworkPolicyIngressMsg{}

	if len(npirs) == 0 {
		return ingressRules
	}

	for _, npir := range npirs {
		for _, port := range npir.Ports {
			var ppl v1.Protocol
			var portNum string
			if port.Protocol != nil {
				ppl = *port.Protocol
			} else {
				ppl = v1.ProtocolTCP
			}
			if port.Port.Type == intstr.Int {
				portNum = strconv.Itoa(int(port.Port.IntVal))
			} else {
				portNum = port.Port.StrVal
			}
			sel := MizarNetworkPolicyPortSelector{
				Protocol: string(ppl),
				Port:     portNum,
			}
			ingressPorts = append(ingressPorts, sel)
		}

		for _, from := range npir.From {
			if from.PodSelector != nil && from.NamespaceSelector != nil {
				podMsg := MizarNetworkPolicyPodSelector{
					MatchLabels: from.PodSelector.MatchLabels,
				}
				namespaceMsg := MizarNetworkPolicyNamespaceSelector{
					MatchLabels: from.NamespaceSelector.MatchLabels,
				}
				fromMsg := MizarNetworkPolicyRule{
					P: podMsg,
					N: namespaceMsg,
				}
				froms = append(froms, fromMsg)
			} else if from.PodSelector != nil {
				podMsg := MizarNetworkPolicyPodSelector{
					MatchLabels: from.PodSelector.MatchLabels,
				}
				fromMsg := MizarNetworkPolicyRule{
					P: podMsg,
				}
				froms = append(froms, fromMsg)
			} else if from.NamespaceSelector != nil {
				namespaceMsg := MizarNetworkPolicyNamespaceSelector{
					MatchLabels: from.NamespaceSelector.MatchLabels,
				}
				fromMsg := MizarNetworkPolicyRule{
					N: namespaceMsg,
				}
				froms = append(froms, fromMsg)
			} else if from.IPBlock != nil {
				ipblockMsg := MizarNetworkPolicyIPBlock{
					Cidr:   from.IPBlock.CIDR,
					Except: from.IPBlock.Except,
				}
				fromMsg := MizarNetworkPolicyRule{
					I: ipblockMsg,
				}
				froms = append(froms, fromMsg)
			}
		}
		ingressMsg := MizarNetworkPolicyIngressMsg{
			Ports: ingressPorts,
			From:  froms,
		}
		ingressRules = append(ingressRules, ingressMsg)
	}
	return ingressRules
}

func parseNetworkPolicyEgressRulesToMsg(npers []networking.NetworkPolicyEgressRule) []MizarNetworkPolicyEgressMsg {
	egressPorts := []MizarNetworkPolicyPortSelector{}
	tos := []MizarNetworkPolicyRule{}
	egressRules := []MizarNetworkPolicyEgressMsg{}

	if len(npers) == 0 {
		return nil
	}

	for _, nper := range npers {
		for _, port := range nper.Ports {
			var ppl v1.Protocol
			var portNum string
			if port.Protocol != nil {
				ppl = *port.Protocol
			} else {
				ppl = v1.ProtocolTCP
			}
			if port.Port.Type == intstr.Int {
				portNum = strconv.Itoa(int(port.Port.IntVal))
			} else {
				portNum = port.Port.StrVal
			}
			sel := MizarNetworkPolicyPortSelector{
				Protocol: string(ppl),
				Port:     portNum,
			}
			egressPorts = append(egressPorts, sel)
		}

		for _, to := range nper.To {
			if to.PodSelector != nil && to.NamespaceSelector != nil {
				podMsg := MizarNetworkPolicyPodSelector{
					MatchLabels: to.PodSelector.MatchLabels,
				}
				namespaceMsg := MizarNetworkPolicyNamespaceSelector{
					MatchLabels: to.NamespaceSelector.MatchLabels,
				}
				toMsg := MizarNetworkPolicyRule{
					P: podMsg,
					N: namespaceMsg,
				}
				tos = append(tos, toMsg)
			} else if to.PodSelector != nil {
				podMsg := MizarNetworkPolicyPodSelector{
					MatchLabels: to.PodSelector.MatchLabels,
				}
				toMsg := MizarNetworkPolicyRule{
					P: podMsg,
				}
				tos = append(tos, toMsg)
			} else if to.NamespaceSelector != nil {
				namespaceMsg := MizarNetworkPolicyNamespaceSelector{
					MatchLabels: to.NamespaceSelector.MatchLabels,
				}
				toMsg := MizarNetworkPolicyRule{
					N: namespaceMsg,
				}
				tos = append(tos, toMsg)
			} else if to.IPBlock != nil {
				ipblockMsg := MizarNetworkPolicyIPBlock{
					Cidr:   to.IPBlock.CIDR,
					Except: to.IPBlock.Except,
				}
				toMsg := MizarNetworkPolicyRule{
					I: ipblockMsg,
				}
				tos = append(tos, toMsg)
			}
		}
		egressMsg := MizarNetworkPolicyEgressMsg{
			Ports: egressPorts,
			To:    tos,
		}
		egressRules = append(egressRules, egressMsg)
	}
	return egressRules
}
