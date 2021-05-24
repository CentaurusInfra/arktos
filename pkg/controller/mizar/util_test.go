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
	"strconv"
	"testing"

	v1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	testAddress           = "test-address"
	testHostIP            = "test-hostip"
	testIP                = "test-ip"
	testLabelNetworkValue = "test-network"
	testName              = "test-name"
	testNamespace         = "test-namespace"
	testPhase             = "test-phase"
	testProtocal          = "test-protocal"
	testTenant            = "test-tenant"

	testPort1 = 123
	testPort2 = 4567
)

func TestConvertToServiceEndpointContract(t *testing.T) {
	// Arrange - no backendIps nor ports
	endpoints := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Tenant:    testTenant,
		},
	}
	service := &v1.Service{}
	backendIps := []string{}
	ports := []*PortsMessage{}
	expected := &BuiltinsServiceEndpointMessage{
		Name:           testName,
		Namespace:      testNamespace,
		Tenant:         testTenant,
		BackendIpsJson: jsonMarshal(backendIps),
		PortsJson:      jsonMarshal(ports),
	}

	// Act
	actual := ConvertToServiceEndpointContract(endpoints, service)

	// Assert
	testCheckEqual(t, expected, actual)

	// Arrange - with backendIps
	endpoints.Subsets = []v1.EndpointSubset{
		{
			Addresses: []v1.EndpointAddress{
				{
					IP: testIP,
				},
			},
		},
	}
	backendIps = append(backendIps, testIP)
	expected.BackendIpsJson = jsonMarshal(backendIps)

	// Act
	actual = ConvertToServiceEndpointContract(endpoints, service)

	// Assert
	testCheckEqual(t, expected, actual)

	// Arrange - with ports
	service.Spec = v1.ServiceSpec{
		Ports: []v1.ServicePort{
			{
				Port: testPort1,
				TargetPort: intstr.IntOrString{
					IntVal: testPort2,
				},
				Protocol: testProtocal,
			},
		},
	}

	ports = append(ports, &PortsMessage{
		FrontendPort: strconv.Itoa(testPort1),
		BackendPort:  strconv.Itoa(testPort2),
		Protocol:     testProtocal,
	})
	expected.PortsJson = jsonMarshal(ports)

	// Act
	actual = ConvertToServiceEndpointContract(endpoints, service)

	// Assert
	testCheckEqual(t, expected, actual)
}

func TestConvertToPodContract(t *testing.T) {
	// Arrange - no network defined
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Tenant:    testTenant,
		},
		Status: v1.PodStatus{
			HostIP: testHostIP,
			Phase:  testPhase,
		},
	}
	expected := &BuiltinsPodMessage{
		Name:          testName,
		HostIp:        testHostIP,
		Namespace:     testNamespace,
		Tenant:        testTenant,
		Labels:        "",
		ArktosNetwork: "",
		Phase:         testPhase,
	}

	// Act
	actual := ConvertToPodContract(pod)

	// Assert
	testCheckEqual(t, expected, actual)

	// Arrange - with network defined
	pod.Labels = map[string]string{
		Arktos_Network_Name: testLabelNetworkValue,
	}
	expected.ArktosNetwork = testLabelNetworkValue
	expected.Labels = jsonMarshal(pod.Labels)
	// Act
	actual = ConvertToPodContract(pod)

	// Assert
	testCheckEqual(t, expected, actual)
}

func TestConvertToNodeContract(t *testing.T) {
	// Arrange - No IP
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
	}
	expected := &BuiltinsNodeMessage{
		Name: testName,
	}

	// Act
	actual := ConvertToNodeContract(node)

	// Assert
	testCheckEqual(t, expected, actual)

	// Arrange - with non InternalIP
	node.Status = v1.NodeStatus{
		Addresses: []v1.NodeAddress{
			{
				Type:    ExternalIP,
				Address: testAddress,
			},
		},
	}

	// Act
	actual = ConvertToNodeContract(node)

	// Assert
	testCheckEqual(t, expected, actual)

	// Arrange - with InternalIP
	internalAddress := v1.NodeAddress{
		Type:    InternalIP,
		Address: testAddress,
	}
	node.Status.Addresses = append(node.Status.Addresses, internalAddress)
	expected.Ip = testAddress

	// Act
	actual = ConvertToNodeContract(node)

	// Assert
	testCheckEqual(t, expected, actual)
}

func TestConvertToNamespaceContract(t *testing.T) {
	// Arrange - no network defined
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   testName,
			Tenant: testTenant,
		},
	}
	expected := &BuiltinsNamespaceMessage{
		Name:   testName,
		Tenant: testTenant,
		Labels: "",
	}

	// Act
	actual := ConvertToNamespaceContract(namespace)

	// Assert
	testCheckEqual(t, expected, actual)

	// Arrange - with network defined
	namespace.Labels = map[string]string{
		"run": "foo",
	}

	expected.Labels = jsonMarshal(namespace.Labels)
	// Act
	actual = ConvertToNamespaceContract(namespace)

	// Assert
	testCheckEqual(t, expected, actual)
}

func TestConvertToNetworkPolicyContract(t *testing.T) {
	// Arrange - No Spec
	nppolicy := &networking.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Tenant:    testTenant,
		},
	}

	expected := &BuiltinsNetworkPolicyMessage{
		Name:      testName,
		Namespace: testNamespace,
		Tenant:    testTenant,
	}

	testPolicy := MizarNetworkPolicyPolicySpecMsg{
		PodSel: MizarNetworkPolicyPodSelector{},
		In:     []MizarNetworkPolicyIngressMsg{},
		Out:    []MizarNetworkPolicyEgressMsg{},
		Type:   []string{},
	}

	expected.Policy = jsonMarshal(testPolicy)
	// Act
	actual := ConvertToNetworkPolicyContract(nppolicy)

	// Assert
	testCheckEqual(t, expected, actual)
}
