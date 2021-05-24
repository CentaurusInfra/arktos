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
	v1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
)

type ReturnCodeFunc func(grpcAdaptorMock *GrpcAdaptorMock) *ReturnCode

type GrpcAdaptorMock struct {
	grpcHost               string
	arktosMessage          *BuiltinsArktosMessage
	serviceMessage         *BuiltinsServiceMessage
	serviceEndpointMessage *BuiltinsServiceEndpointMessage
	pod                    *v1.Pod
	node                   *v1.Node
	namespace              *v1.Namespace
	policy                 *networking.NetworkPolicy
	returnCodeFunc         ReturnCodeFunc
	retryCount             int
}

func NewGrpcAdaptorMock() *GrpcAdaptorMock {
	grpcAdaptorMock := new(GrpcAdaptorMock)
	grpcAdaptorMock.returnCodeFunc = func(grpcAdaptorMock *GrpcAdaptorMock) *ReturnCode {
		return &ReturnCode{
			Code: CodeType_OK,
		}
	}

	return grpcAdaptorMock
}

// MockSetupReturnCode is to setup the function of returning *ReturnCode for test purpose
func (grpcAdaptor *GrpcAdaptorMock) MockSetupReturnCode(returnCodeFunc ReturnCodeFunc) {
	grpcAdaptor.returnCodeFunc = returnCodeFunc
}

// CreateArktosNetwork is to invoke grpc func of CreateArktosNetwork
func (grpcAdaptor *GrpcAdaptorMock) CreateArktosNetwork(grpcHost string, msg *BuiltinsArktosMessage) *ReturnCode {
	grpcAdaptor.arktosMessage = msg
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// CreateService is to invoke grpc func of CreateService
func (grpcAdaptor *GrpcAdaptorMock) CreateService(grpcHost string, msg *BuiltinsServiceMessage) *ReturnCode {
	grpcAdaptor.serviceMessage = msg
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// UpdateService is to invoke grpc func of UpdateService
func (grpcAdaptor *GrpcAdaptorMock) UpdateService(grpcHost string, msg *BuiltinsServiceMessage) *ReturnCode {
	grpcAdaptor.serviceMessage = msg
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// DeleteService is to invoke grpc func of DeleteService
func (grpcAdaptor *GrpcAdaptorMock) DeleteService(grpcHost string, msg *BuiltinsServiceMessage) *ReturnCode {
	grpcAdaptor.serviceMessage = msg
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// UpdateServiceEndpoint is to invoke grpc func of UpdateServiceEndpoint
func (grpcAdaptor *GrpcAdaptorMock) UpdateServiceEndpoint(grpcHost string, msg *BuiltinsServiceEndpointMessage) *ReturnCode {
	grpcAdaptor.serviceEndpointMessage = msg
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// CreateServiceEndpoint is to invoke grpc func of CreateServiceEndpoint
func (grpcAdaptor *GrpcAdaptorMock) CreateServiceEndpoint(grpcHost string, msg *BuiltinsServiceEndpointMessage) *ReturnCode {
	grpcAdaptor.serviceEndpointMessage = msg
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// UpdatePod is to invoke grpc func of UpdatePod
func (grpcAdaptor *GrpcAdaptorMock) UpdatePod(grpcHost string, pod *v1.Pod) *ReturnCode {
	grpcAdaptor.pod = pod
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// CreatePod is to invoke grpc func of CreatePod
func (grpcAdaptor *GrpcAdaptorMock) CreatePod(grpcHost string, pod *v1.Pod) *ReturnCode {
	grpcAdaptor.pod = pod
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// DeletePod is to invoke grpc func of DeletePod
func (grpcAdaptor *GrpcAdaptorMock) DeletePod(grpcHost string, pod *v1.Pod) *ReturnCode {
	grpcAdaptor.pod = pod
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// CreateNode is to invoke grpc func of CreateNode
func (grpcAdaptor *GrpcAdaptorMock) CreateNode(grpcHost string, node *v1.Node) *ReturnCode {
	grpcAdaptor.node = node
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// UpdateNode is to invoke grpc func of UpdateNode
func (grpcAdaptor *GrpcAdaptorMock) UpdateNode(grpcHost string, node *v1.Node) *ReturnCode {
	grpcAdaptor.node = node
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// DeleteNode is to invoke grpc func of DeleteNode
func (grpcAdaptor *GrpcAdaptorMock) DeleteNode(grpcHost string, node *v1.Node) *ReturnCode {
	grpcAdaptor.node = node
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// UpdateNetworkPolicy is to invoke grpc func of UpdateNetworkPolicy
func (grpcAdaptor *GrpcAdaptorMock) UpdateNetworkPolicy(grpcHost string, policy *networking.NetworkPolicy) *ReturnCode {
	grpcAdaptor.policy = policy
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// CreateNetworkPolicy is to invoke grpc func of CreateNetworkPolicy
func (grpcAdaptor *GrpcAdaptorMock) CreateNetworkPolicy(grpcHost string, policy *networking.NetworkPolicy) *ReturnCode {
	grpcAdaptor.policy = policy
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// DeleteNetworkPolicy is to invoke grpc func of DeleteNetworkPolicy
func (grpcAdaptor *GrpcAdaptorMock) DeleteNetworkPolicy(grpcHost string, policy *networking.NetworkPolicy) *ReturnCode {
	grpcAdaptor.policy = policy
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// UpdateNamespace is to invoke grpc func of UpdateNamespace
func (grpcAdaptor *GrpcAdaptorMock) UpdateNamespace(grpcHost string, namespace *v1.Namespace) *ReturnCode {
	grpcAdaptor.namespace = namespace
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// CreateNamespace is to invoke grpc func of CreateNamespace
func (grpcAdaptor *GrpcAdaptorMock) CreateNamespace(grpcHost string, namespace *v1.Namespace) *ReturnCode {
	grpcAdaptor.namespace = namespace
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}

// DeleteNamespace is to invoke grpc func of DeleteNamespace
func (grpcAdaptor *GrpcAdaptorMock) DeleteNamespace(grpcHost string, namespace *v1.Namespace) *ReturnCode {
	grpcAdaptor.namespace = namespace
	grpcAdaptor.grpcHost = grpcHost
	return grpcAdaptor.returnCodeFunc(grpcAdaptor)
}
