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

package mizarcontrollers

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
	client, ctx, err := getGrpcClient(grpcHost)
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	returnCode, err := client.CreatePod(ctx, ConvertToPodContract(pod))
	if err != nil {
		return getReturnCodeFromError(&err)
	}
	return returnCode
}

func getReturnCodeFromError(err *error) *ReturnCode {
	return &ReturnCode{
		Code:    CodeType_TEMP_ERROR,
		Message: fmt.Sprintf("Hit grpc error: %v", err),
	}
}

func getGrpcClient(grpcHost string) (BuiltinsServiceClient, context.Context, error) {
	address := fmt.Sprintf("%s:%s", grpcHost, port)
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()

	client := NewBuiltinsServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return client, ctx, nil
}
