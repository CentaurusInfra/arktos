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
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/testutil"
)

const (
	testGrpcHost = "10.0.1.17"

	mizarNodeControllerWorkerCount = 2

	rpId0 = "rp0"
)

func TestMizarNodeController_Create(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mizarNodeController, _, nodeInformer, grpcAdaptorMock, node := getNewMizarNodeController()
	mizarNodeController.nodeListersSynced = make(map[string]cache.InformerSynced, 1)

	go mizarNodeController.Run(mizarNodeControllerWorkerCount, ctx.Done())

	go nodeInformer.Informer().Run(ctx.Done())
	waitForMockDataReadyWithTimeout(t, grpcAdaptorMock)

	testCheckEqual(t, node, grpcAdaptorMock.node)
}

func TestMizarNodeController_Update(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mizarNodeController, kubeClient, nodeInformer, grpcAdaptorMock, node := getNewMizarNodeController()
	mizarNodeController.nodeListersSynced = make(map[string]cache.InformerSynced, 1)
	mizarNodeController.nodeListersSynced[rpId0] = alwaysReady
	go mizarNodeController.Run(mizarNodeControllerWorkerCount, ctx.Done())

	syncNodeStore(nodeInformer, kubeClient)
	go nodeInformer.Informer().Run(ctx.Done())

	waitForMockDataReadyWithTimeout(t, grpcAdaptorMock)
	testCheckEqual(t, node, grpcAdaptorMock.node)
}

func TestMizarNodeController_Retry(t *testing.T) {
	retryCount := 10

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mizarNodeController, _, nodeInformer, grpcAdaptorMock, node := getNewMizarNodeController()
	mizarNodeController.nodeListersSynced = make(map[string]cache.InformerSynced, 1)
	mizarNodeController.nodeListersSynced[rpId0] = alwaysReady
	grpcAdaptorMock.returnCodeFunc = func(grpcAdaptorMock *GrpcAdaptorMock) *ReturnCode {
		if grpcAdaptorMock.retryCount < retryCount {
			grpcAdaptorMock.retryCount++
			return &ReturnCode{
				Code: CodeType_TEMP_ERROR,
			}
		} else {
			return &ReturnCode{
				Code: CodeType_OK,
			}
		}
	}
	go mizarNodeController.Run(mizarNodeControllerWorkerCount, ctx.Done())

	go nodeInformer.Informer().Run(ctx.Done())
	waitForMockDataReadyWithTimeout(t, grpcAdaptorMock)

	testCheckEqual(t, node, grpcAdaptorMock.node)
	testCheckEqual(t, retryCount, grpcAdaptorMock.retryCount)
}

func syncNodeStore(nodeInformer coreinformers.NodeInformer, kubeClient *testutil.FakeNodeHandler) error {
	nodes, err := kubeClient.List(metav1.ListOptions{})
	nodes.Items[0].ResourceVersion = "old version"
	if err != nil {
		return err
	}
	newElems := make([]interface{}, 0, len(nodes.Items))
	for i := range nodes.Items {
		newElems = append(newElems, &nodes.Items[i])
	}
	return nodeInformer.Informer().GetStore().Replace(newElems, "newRV")
}

func getNewMizarNodeController() (*MizarNodeController, *testutil.FakeNodeHandler, coreinformers.NodeInformer, *GrpcAdaptorMock, *v1.Node) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "node0",
			CreationTimestamp: metav1.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC),
			Labels: map[string]string{
				v1.LabelZoneRegion:        "region1",
				v1.LabelZoneFailureDomain: "zone1",
			},
			ResourceVersion: "test version",
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:               v1.NodeReady,
					Status:             v1.ConditionTrue,
					LastHeartbeatTime:  metav1.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
					LastTransitionTime: metav1.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
				},
			},
		},
	}
	kubeClient :=
		&testutil.FakeNodeHandler{
			Existing:     []*v1.Node{node},
			DeletedNodes: []*v1.Node{},
			Clientset:    fake.NewSimpleClientset(),
		}
	factory := informers.NewSharedInformerFactory(kubeClient, controller.NoResyncPeriodFunc())
	nodeInformer := factory.Core().V1().Nodes()
	nodeInformerMap := make(map[string]coreinformers.NodeInformer, 1)
	nodeInformerMap[rpId0] = nodeInformer

	kubeClient.CreateHook = func(c *testutil.FakeNodeHandler, n *v1.Node) bool { return true }

	grpcAdaptor := NewGrpcAdaptorMock()
	return NewMizarNodeController(nodeInformer, nodeInformerMap, kubeClient, testGrpcHost, grpcAdaptor), kubeClient, nodeInformer, grpcAdaptor, node
}
