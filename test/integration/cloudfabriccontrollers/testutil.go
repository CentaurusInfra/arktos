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

package cloudfabriccontrollers

import (
	"github.com/grafov/bcast"
	"k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/test/integration/framework"
	"net/http/httptest"
	"testing"
	"time"

	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	controller "k8s.io/kubernetes/pkg/cloudfabric-controller/controllerframework"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/replicaset"
)

const (
	Interval = 100 * time.Millisecond
	Timeout  = 60 * time.Second
)

// Run RS controller and informers
func RunControllerAndInformers(t *testing.T, cim *controller.ControllerInstanceManager, rsc *replicaset.ReplicaSetController, informers informers.SharedInformerFactory, podNum int) chan struct{} {
	stopCh := make(chan struct{})
	informers.Start(stopCh)
	waitToObserveControllerInstances(t, informers.Core().V1().ControllerInstances().Informer())
	go cim.Run(stopCh)

	waitToObservePods(t, informers.Core().V1().Pods().Informer(), podNum)
	go rsc.Run(5, stopCh)

	return stopCh
}

func waitToObserveControllerInstances(t *testing.T, coInformer cache.SharedIndexInformer) {
	if err := wait.PollImmediate(Interval, Timeout, func() (bool, error) {
		coInformer.GetIndexer().List()
		return true, nil
	}); err != nil {
		t.Fatalf("Error encountered when waiting for podInformer to observe the pods: %v", err)
	}
}

// wait for the podInformer to observe the pods. Call this function before
// running the RS controller to prevent the rc manager from creating new pods
// rather than adopting the existing ones.
func waitToObservePods(t *testing.T, podInformer cache.SharedIndexInformer, podNum int) {
	if err := wait.PollImmediate(Interval, Timeout, func() (bool, error) {
		objects := podInformer.GetIndexer().List()
		return len(objects) == podNum, nil
	}); err != nil {
		t.Fatalf("Error encountered when waiting for podInformer to observe the pods: %v", err)
	}
}

func RmSetup(t *testing.T) (*httptest.Server, framework.CloseFunc, *controller.ControllerInstanceManager, *replicaset.ReplicaSetController, informers.SharedInformerFactory, clientset.Interface) {
	s, closeFn := rmSetupMaster(t)
	cim, rsc, informers, clientSet := RmSetupControllerMaster(t, s)

	return s, closeFn, cim, rsc, informers, clientSet
}

func rmSetupMaster(t *testing.T) (*httptest.Server, framework.CloseFunc) {
	masterConfig := framework.NewIntegrationTestMasterConfig()
	_, s, closeFn := framework.RunAMaster(masterConfig)
	return s, closeFn
}

func RmSetupControllerMaster(t *testing.T, s *httptest.Server) (*controller.ControllerInstanceManager, *replicaset.ReplicaSetController, informers.SharedInformerFactory, clientset.Interface) {
	kubeConfig := restclient.KubeConfig{Host: s.URL}
	configs := restclient.NewAggregatedConfig(&kubeConfig)
	clientSet, err := clientset.NewForConfig(configs)
	if err != nil {
		t.Fatalf("Error in create clientset: %v", err)
	}
	resyncPeriod := 12 * time.Hour
	informers := informers.NewSharedInformerFactory(clientset.NewForConfigOrDie(restclient.AddUserAgent(configs, "rs-informers")), resyncPeriod)

	// controller instance manager set up
	cim := controller.GetInstanceHandler()
	if cim == nil {
		cimUpdateChGrp := bcast.NewGroup()
		go cimUpdateChGrp.Broadcast(0)
		cim = controller.NewControllerInstanceManager(
			informers.Core().V1().ControllerInstances(),
			clientset.NewForConfigOrDie(restclient.AddUserAgent(configs, "controller-instance-manager")),
			cimUpdateChGrp,
		)
	}
	cimUpdateCh := cim.GetUpdateChGrp().Join()

	rsResetChGrp := bcast.NewGroup()
	go rsResetChGrp.Broadcast(0)

	rsInformer := informers.Apps().V1().ReplicaSets()
	rsResetCh := rsResetChGrp.Join()
	rsInformer.Informer().AddResetCh(rsResetCh, "ReplicaSet_Controller", "")

	podInformer := informers.Core().V1().Pods()
	podResetCh := rsResetChGrp.Join()
	podInformer.Informer().AddResetCh(podResetCh, "ReplicaSet_Controller", "ReplicaSet")
	podResetCh2 := rsResetChGrp.Join()
	podInformer.Informer().AddResetCh(podResetCh2, "ReplicaSet_Controller", "")

	rsc := replicaset.NewReplicaSetController(
		rsInformer,
		podInformer,
		clientset.NewForConfigOrDie(restclient.AddUserAgent(configs, "replicaset-controller")),
		replicaset.BurstReplicas,
		cimUpdateCh,
		rsResetChGrp,
	)

	if err != nil {
		t.Fatalf("Failed to create replicaset controller")
	}
	rsc.PrintRangeAndStatus()

	return cim, rsc, informers, clientSet
}

func GenerateUUIDInControllerRange(controllerBase *controller.ControllerBase) (types.UID, int64) {
	for {
		uid := uuid.NewUUID()
		hashKey := fuzzer.GetHashOfUUID(uid)
		if controllerBase.IsInRange(hashKey) {
			return uid, hashKey
		}
	}
}

func CleanupControllers(controllers ...*controller.ControllerBase) {
	for _, controller := range controllers {
		controller.DeleteController()
	}
}
