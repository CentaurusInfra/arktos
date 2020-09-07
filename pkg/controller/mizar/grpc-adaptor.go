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
	fmt "fmt"
	"time"

	"google.golang.org/grpc"

	v1 "k8s.io/api/core/v1"
)

const (
	port = "50052"
)

// GrpcCreatePod is to invoking grpc func of CreatePod
func GrpcCreatePod(grpcHost string, pod *v1.Pod) *ReturnCode {
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

func getReturnCodeFromError(err *error) *ReturnCode {
	return &ReturnCode{
		Code:    CodeType_TEMP_ERROR,
		Message: fmt.Sprintf("Grpc call failed: %s", (*err).Error()),
	}
}

func getGrpcClient(grpcHost string) (BuiltinsServiceClient, context.Context, *grpc.ClientConn, context.CancelFunc, error) {
	address := fmt.Sprintf("%s:%s", grpcHost, port)
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, nil, conn, nil, err
	}

	client := NewBuiltinsServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	return client, ctx, conn, cancel, nil
}
