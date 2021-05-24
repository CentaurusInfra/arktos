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
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
)

const (
	GrpcPort = "50052"
)

type IGrpcAdaptor interface {
	CreateArktosNetwork(grpcHost string, msg *BuiltinsArktosMessage) *ReturnCode
	CreateService(grpcHost string, msg *BuiltinsServiceMessage) *ReturnCode
	UpdateService(grpcHost string, msg *BuiltinsServiceMessage) *ReturnCode
	DeleteService(grpcHost string, msg *BuiltinsServiceMessage) *ReturnCode
	UpdateServiceEndpoint(grpcHost string, msg *BuiltinsServiceEndpointMessage) *ReturnCode
	CreateServiceEndpoint(grpcHost string, msg *BuiltinsServiceEndpointMessage) *ReturnCode
	UpdatePod(grpcHost string, pod *v1.Pod) *ReturnCode
	CreatePod(grpcHost string, pod *v1.Pod) *ReturnCode
	DeletePod(grpcHost string, pod *v1.Pod) *ReturnCode
	UpdateNetworkPolicy(grpcHost string, policy *networking.NetworkPolicy) *ReturnCode
	CreateNetworkPolicy(grpcHost string, policy *networking.NetworkPolicy) *ReturnCode
	DeleteNetworkPolicy(grpcHost string, policy *networking.NetworkPolicy) *ReturnCode
	CreateNode(grpcHost string, node *v1.Node) *ReturnCode
	UpdateNode(grpcHost string, node *v1.Node) *ReturnCode
	DeleteNode(grpcHost string, node *v1.Node) *ReturnCode
	UpdateNamespace(grpcHost string, namespace *v1.Namespace) *ReturnCode
	CreateNamespace(grpcHost string, namespace *v1.Namespace) *ReturnCode
	DeleteNamespace(grpcHost string, namespace *v1.Namespace) *ReturnCode
}

type GrpcAdaptor struct {
}

// CreateArktosNetwork is to invoke grpc func of CreateArktosNetwork
func (grpcAdaptor *GrpcAdaptor) CreateArktosNetwork(grpcHost string, msg *BuiltinsArktosMessage) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.CreateArktosNetwork(ctx, msg)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// CreateService is to invoke grpc func of CreateService
func (grpcAdaptor *GrpcAdaptor) CreateService(grpcHost string, msg *BuiltinsServiceMessage) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.CreateService(ctx, msg)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// UpdateService is to invoke grpc func of UpdateService
func (grpcAdaptor *GrpcAdaptor) UpdateService(grpcHost string, msg *BuiltinsServiceMessage) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.UpdateService(ctx, msg)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// DeleteService is to invoke grpc func of DeleteService
func (grpcAdaptor *GrpcAdaptor) DeleteService(grpcHost string, msg *BuiltinsServiceMessage) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.DeleteService(ctx, msg)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// UpdateServiceEndpoint is to invoke grpc func of UpdateServiceEndpoint
func (grpcAdaptor *GrpcAdaptor) UpdateServiceEndpoint(grpcHost string, msg *BuiltinsServiceEndpointMessage) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.UpdateServiceEndpoint(ctx, msg)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// CreateServiceEndpoint is to invoke grpc func of CreateServiceEndpoint
func (grpcAdaptor *GrpcAdaptor) CreateServiceEndpoint(grpcHost string, msg *BuiltinsServiceEndpointMessage) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.CreateServiceEndpoint(ctx, msg)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// UpdatePod is to invoke grpc func of UpdatePod
func (grpcAdaptor *GrpcAdaptor) UpdatePod(grpcHost string, pod *v1.Pod) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.UpdatePod(ctx, ConvertToPodContract(pod))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// CreatePod is to invoke grpc func of CreatePod
func (grpcAdaptor *GrpcAdaptor) CreatePod(grpcHost string, pod *v1.Pod) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.CreatePod(ctx, ConvertToPodContract(pod))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// DeletePod is to invoke grpc func of DeletePod
func (grpcAdaptor *GrpcAdaptor) DeletePod(grpcHost string, pod *v1.Pod) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.DeletePod(ctx, ConvertToPodContract(pod))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// CreateNode is to invoke grpc func of CreateNode
func (grpcAdaptor *GrpcAdaptor) CreateNode(grpcHost string, node *v1.Node) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.CreateNode(ctx, ConvertToNodeContract(node))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// UpdateNode is to invoke grpc func of UpdateNode
func (grpcAdaptor *GrpcAdaptor) UpdateNode(grpcHost string, node *v1.Node) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.UpdateNode(ctx, ConvertToNodeContract(node))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// DeleteNode is to invoke grpc func of DeleteNode
func (grpcAdaptor *GrpcAdaptor) DeleteNode(grpcHost string, node *v1.Node) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.DeleteNode(ctx, ConvertToNodeContract(node))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// UpdateNetworkPolicy is to invoke grpc func of UpdateNetworkPolicy
func (grpcAdaptor *GrpcAdaptor) UpdateNetworkPolicy(grpcHost string, policy *networking.NetworkPolicy) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.UpdateNetworkPolicy(ctx, ConvertToNetworkPolicyContract(policy))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// CreateNetworkPolicy is to invoke grpc func of CreateNetworkPolicy
func (grpcAdaptor *GrpcAdaptor) CreateNetworkPolicy(grpcHost string, policy *networking.NetworkPolicy) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.CreateNetworkPolicy(ctx, ConvertToNetworkPolicyContract(policy))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// DeleteNetworkPolicy is to invoke grpc func of DeleteNetworkPolicy
func (grpcAdaptor *GrpcAdaptor) DeleteNetworkPolicy(grpcHost string, policy *networking.NetworkPolicy) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.DeleteNetworkPolicy(ctx, ConvertToNetworkPolicyContract(policy))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// UpdateNamespace is to invoke grpc func of UpdateNamespace
func (grpcAdaptor *GrpcAdaptor) UpdateNamespace(grpcHost string, namespace *v1.Namespace) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.UpdateNamespace(ctx, ConvertToNamespaceContract(namespace))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// CreateNamespace is to invoke grpc func of CreateNamespace
func (grpcAdaptor *GrpcAdaptor) CreateNamespace(grpcHost string, namespace *v1.Namespace) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.CreateNamespace(ctx, ConvertToNamespaceContract(namespace))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

// DeleteNamespace is to invoke grpc func of DeleteNamespace
func (grpcAdaptor *GrpcAdaptor) DeleteNamespace(grpcHost string, namespace *v1.Namespace) *ReturnCode {
	client, ctx, conn, cancel, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	defer conn.Close()
	defer cancel()
	returnCode, err := client.DeleteNamespace(ctx, ConvertToNamespaceContract(namespace))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

func getReturnCodeFromError(err *error) *ReturnCode {
	return &ReturnCode{
		Code:    CodeType_TEMP_ERROR,
		Message: fmt.Sprintf("Grpc call failed: %s", (*err).Error()),
	}
}

func getGrpcClient(grpcHost string) (BuiltinsServiceClient, context.Context, *grpc.ClientConn, context.CancelFunc, error) {
	address := fmt.Sprintf("%s:%s", grpcHost, GrpcPort)
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, nil, conn, nil, err
	}

	client := NewBuiltinsServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	return client, ctx, conn, cancel, nil
}
