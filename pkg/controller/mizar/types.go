/*
Copyright 2022 Authors of Arktos.
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

type TypeMeta struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
}

type ObjectMeta struct {
	Tenant    string `json:"tenant,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

type MizarVPCSpec struct {
	IP      string `json:"ip,omitempty"`
	Prefix  string `json:"prefix,omitempty"`
	Divider int    `json:"dividers,omitempty"`
	Status  string `json:"status,omitempty"`
}

type MizarVPC struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta   `json:"metadata,omitempty"`
	Spec     MizarVPCSpec `json:"spec,omitempty"`
}

type MizarSubnetSpec struct {
	IP       string `json:"ip,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
	Bouncers int    `json:"dividers,omitempty"`
	VPC      string `json:"vpc,omitempty"`
	Status   string `json:"status,omitempty"`
}

type MizarSubnet struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta      `json:"metadata,omitempty"`
	Spec     MizarSubnetSpec `json:"spec,omitempty"`
}

type MizarNetworkPolicyPortSelector struct {
	Protocol string `json:"protocol"`
	Port     string `json:"port"`
}

type MizarNetworkPolicyPodSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

type MizarNetworkPolicyNamespaceSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

type MizarNetworkPolicyIPBlock struct {
	Cidr   string   `json:"cidr,omitempty"`
	Except []string `json:"except,omitempty"`
}

type MizarNetworkPolicyRule struct {
	P MizarNetworkPolicyPodSelector       `json:"podSelector,omitempty"`
	N MizarNetworkPolicyNamespaceSelector `json:"namespaceSelector,omitempty"`
	I MizarNetworkPolicyIPBlock           `json:"ipBlock,omitempty"`
}

type MizarNetworkPolicyIngressMsg struct {
	Ports []MizarNetworkPolicyPortSelector `json:"ports"`
	From  []MizarNetworkPolicyRule         `json:"from"`
}

type MizarNetworkPolicyEgressMsg struct {
	Ports []MizarNetworkPolicyPortSelector `json:"ports"`
	To    []MizarNetworkPolicyRule         `json:"to"`
}

type MizarNetworkPolicyPolicySpecMsg struct {
	PodSel MizarNetworkPolicyPodSelector  `json:"podSelector,omitempty"`
	In     []MizarNetworkPolicyIngressMsg `json:"ingress,omitempty"`
	Out    []MizarNetworkPolicyEgressMsg  `json:"egress,omitempty"`
	Type   []string                       `json:"policyTypes,omitempty"`
}
