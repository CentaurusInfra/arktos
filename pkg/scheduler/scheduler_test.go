/*
Copyright 2014 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

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

package scheduler

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	corelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/scheduler/algorithm"
	"k8s.io/kubernetes/pkg/scheduler/algorithm/predicates"
	"k8s.io/kubernetes/pkg/scheduler/algorithm/priorities"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/pkg/scheduler/core"
	"k8s.io/kubernetes/pkg/scheduler/factory"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
)

// Avoid token expired in the Test functions
var TestToken, _ = requestToken("52.24.61.210")

// Global instance id for testing
var INSTANCEID string = ""

// EmptyFramework is an empty framework used in tests.
// Note: If the test runs in goroutine, please don't use this variable to avoid a race condition.
var EmptyFramework, _ = framework.NewFramework(EmptyPluginRegistry, nil, EmptyPluginConfig)

// EmptyPluginConfig is an empty plugin config used in tests.
var EmptyPluginConfig = []kubeschedulerconfig.PluginConfig{}

type fakePodConditionUpdater struct {
	GetPodConditions func(*v1.Pod)
}

func (fc fakePodConditionUpdater) Update(pod *v1.Pod, podCondition *v1.PodCondition) error {
	var status = &pod.Status
	pod.Status.Conditions = append(status.Conditions, *podCondition)
	fc.GetPodConditions(pod)
	return nil
}

type fakePodPhaseUpdater struct {
	GetPodPhase func(*v1.Pod)
}

func (fpu fakePodPhaseUpdater) Update(pod *v1.Pod, podPhase v1.PodPhase) error {
	pod.Status.Phase = podPhase
	fpu.GetPodPhase(pod)
	return nil
}

type nodeLister struct {
	corelister.NodeLister
}

func (n *nodeLister) List() ([]*v1.Node, error) {
	return n.NodeLister.List(labels.Everything())
}

func podWithValidSpec(id, desiredHost string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:     id,
			UID:      types.UID(id),
			SelfLink: fmt.Sprintf("/api/v1/%s/%s", string(v1.ResourcePods), id),
		},
		Spec: v1.PodSpec{
			Nics: []v1.Nic{
				{Uuid: "4c673550-e58d-459d-9332-93a17f30bed1"},
			},
			VirtualMachine: &v1.VirtualMachine{
				KeyPairName: "KeyMy",
				Name:        desiredHost,
				Image:       "92806f76-f715-4512-9e34-5feb35186b8e",
				Scheduling: v1.GlobalScheduling{
					SecurityGroup: []v1.OpenStackSecurityGroup{
						{Name: "a19891f1-1092-44fb-a75c-e6601ed769e4"},
					},
				},
				FlavorRef: "d1",
			},
		},
	}
}

func podWithXLargeFlavorSpec(id, desiredHost string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:     id,
			UID:      types.UID(id),
			SelfLink: fmt.Sprintf("/api/v1/%s/%s", string(v1.ResourcePods), id),
		},
		Spec: v1.PodSpec{
			Nics: []v1.Nic{
				{Uuid: "4c673550-e58d-459d-9332-93a17f30bed1"},
			},
			VirtualMachine: &v1.VirtualMachine{
				KeyPairName: "KeyMy",
				Name:        desiredHost,
				Image:       "92806f76-f715-4512-9e34-5feb35186b8e",
				Scheduling: v1.GlobalScheduling{
					SecurityGroup: []v1.OpenStackSecurityGroup{
						{Name: "a19891f1-1092-44fb-a75c-e6601ed769e4"},
					},
				},
				FlavorRef: "5", // CPU = 8, RAM = 16GB, Disk = 160GB
			},
		},
	}
}

func podWithInvalidSpec(id, desiredHost string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:     id,
			UID:      types.UID(id),
			SelfLink: fmt.Sprintf("/api/v1/%s/%s", string(v1.ResourcePods), id),
		},
		Spec: v1.PodSpec{
			Nics: []v1.Nic{
				{Uuid: "4c673550-e58d-459d-9332-93a17f30bed1"},
			},
			VirtualMachine: &v1.VirtualMachine{
				KeyPairName: "KeyMy",
				Name:        desiredHost,
				Image:       "5f2327cb-ef5c--2a16b7455812", // invalid image id
				Scheduling: v1.GlobalScheduling{
					SecurityGroup: []v1.OpenStackSecurityGroup{
						{Name: "a19891f1-1092-44fb-a75c-e6601ed769e4"},
					},
				},
				FlavorRef: "d1",
			},
		},
	}
}

func deletingPod(id string) *v1.Pod {
	deletionTimestamp := metav1.Now()
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              id,
			UID:               types.UID(id),
			DeletionTimestamp: &deletionTimestamp,
			SelfLink:          fmt.Sprintf("/api/v1/%s/%s", string(v1.ResourcePods), id),
		},
		Spec: v1.PodSpec{
			NodeName: "",
		},
	}
}

func PredicateOne(pod *v1.Pod, meta predicates.PredicateMetadata, nodeInfo *schedulernodeinfo.NodeInfo) (bool, []predicates.PredicateFailureReason, error) {
	return true, nil, nil
}

func PriorityOne(pod *v1.Pod, nodeNameToInfo map[string]*schedulernodeinfo.NodeInfo, nodes []*v1.Node) (schedulerapi.HostPriorityList, error) {
	return []schedulerapi.HostPriority{}, nil
}

type mockScheduler struct {
	result core.ScheduleResult
	err    error
}

func (es mockScheduler) GlobalSchedule(pod *v1.Pod) (core.ScheduleResult, error) {
	return es.result, es.err
}

func (es mockScheduler) Schedule(pod *v1.Pod, ml algorithm.NodeLister, pc *framework.PluginContext) (core.ScheduleResult, error) {
	return es.result, es.err
}

func (es mockScheduler) Predicates() map[string]predicates.FitPredicate {
	return nil
}
func (es mockScheduler) Prioritizers() []priorities.PriorityConfig {
	return nil
}

func (es mockScheduler) Preempt(pod *v1.Pod, nodeLister algorithm.NodeLister, scheduleErr error) (*v1.Node, []*v1.Pod, []*v1.Pod, error) {
	return nil, nil, nil, nil
}

func TestSchedulerCreation(t *testing.T) {
	client := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(client, 0)

	testSource := "testProvider"
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(t.Logf).Stop()

	defaultBindTimeout := int64(30)
	factory.RegisterFitPredicate("PredicateOne", PredicateOne)
	factory.RegisterPriorityFunction("PriorityOne", PriorityOne, 1)
	factory.RegisterAlgorithmProvider(testSource, sets.NewString("PredicateOne"), sets.NewString("PriorityOne"))

	stopCh := make(chan struct{})
	defer close(stopCh)
	_, err := New(client,
		informerFactory.Core().V1().Nodes(),
		factory.NewPodInformer(client, 0),
		informerFactory.Core().V1().PersistentVolumes(),
		informerFactory.Core().V1().PersistentVolumeClaims(),
		informerFactory.Core().V1().ReplicationControllers(),
		informerFactory.Apps().V1().ReplicaSets(),
		informerFactory.Apps().V1().StatefulSets(),
		informerFactory.Core().V1().Services(),
		informerFactory.Policy().V1beta1().PodDisruptionBudgets(),
		informerFactory.Storage().V1().StorageClasses(),
		eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "scheduler"}),
		kubeschedulerconfig.SchedulerAlgorithmSource{Provider: &testSource},
		stopCh,
		EmptyPluginRegistry,
		nil,
		EmptyPluginConfig,
		WithBindTimeoutSeconds(defaultBindTimeout))

	if err != nil {
		t.Fatalf("Failed to create scheduler: %v", err)
	}
}

func TestSchedulerCreation_UnsupportedAlgorithmSource(t *testing.T) {
	client := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(client, 0)

	testSource := "testProvider"
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(t.Logf).Stop()

	defaultBindTimeout := int64(30)
	factory.RegisterFitPredicate("PredicateOne", PredicateOne)
	factory.RegisterPriorityFunction("PriorityOne", PriorityOne, 1)
	factory.RegisterAlgorithmProvider(testSource, sets.NewString("PredicateOne"), sets.NewString("PriorityOne"))

	algoSource := [3]kubeschedulerconfig.SchedulerAlgorithmSource{{}, {Policy: &kubeschedulerconfig.SchedulerPolicySource{}}}

	stopCh := make(chan struct{})
	defer close(stopCh)
	for _, as := range algoSource {
		_, err := New(client,
			informerFactory.Core().V1().Nodes(),
			factory.NewPodInformer(client, 0),
			informerFactory.Core().V1().PersistentVolumes(),
			informerFactory.Core().V1().PersistentVolumeClaims(),
			informerFactory.Core().V1().ReplicationControllers(),
			informerFactory.Apps().V1().ReplicaSets(),
			informerFactory.Apps().V1().StatefulSets(),
			informerFactory.Core().V1().Services(),
			informerFactory.Policy().V1beta1().PodDisruptionBudgets(),
			informerFactory.Storage().V1().StorageClasses(),
			eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "scheduler"}),
			as,
			stopCh,
			EmptyPluginRegistry,
			nil,
			EmptyPluginConfig,
			WithBindTimeoutSeconds(defaultBindTimeout))

		if err == nil {
			t.Fatalf("Expected create scheduler failed but success")
		}
	}
}

func TestScheduler(t *testing.T) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(t.Logf).Stop()

	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}

	table := []struct {
		name                     string
		sendPod                  *v1.Pod
		algo                     core.ScheduleAlgorithm
		expectPodPhase           v1.PodPhase
		expectPodConditionsTypes v1.PodConditionType
		eventReason              string
	}{
		{
			name:           "pod scheduled successfully",
			sendPod:        podWithValidSpec("test4pod", "provider-instance-test-4"),
			algo:           mockScheduler{core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 1, FeasibleNodes: 1}, nil},
			expectPodPhase: v1.PodRunning,
			eventReason:    "Scheduled",
		},
		{
			name:                     "error pod failed scheduling with invalid host",
			sendPod:                  podWithValidSpec("scheduler-unit-test", "scheduler-instance-unit-test"),
			algo:                     mockScheduler{core.ScheduleResult{SuggestedHost: "172.31.5.108", EvaluatedNodes: 1, FeasibleNodes: 1}, nil},
			expectPodConditionsTypes: v1.PodScheduleFailed,
			eventReason:              "Rescheduled",
		},
		{
			name:                     "error pod failed scheduling with invalid Podspec",
			sendPod:                  podWithInvalidSpec("scheduler-unit-test", "scheduler-instance-unit-test"),
			algo:                     mockScheduler{core.ScheduleResult{SuggestedHost: "172.31.5.108", EvaluatedNodes: 1, FeasibleNodes: 1}, nil},
			expectPodConditionsTypes: v1.PodScheduleFailed,
			eventReason:              "Rescheduled",
		},
	}

	stop := make(chan struct{})
	defer close(stop)
	client := clientsetfake.NewSimpleClientset(&testNode)
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	nl := informerFactory.Core().V1().Nodes().Lister()

	informerFactory.Start(stop)
	informerFactory.WaitForCacheSync(stop)

	for _, item := range table {
		t.Run(item.name, func(t *testing.T) {
			var gotPodPhase v1.PodPhase
			var gotPodConditionsType v1.PodConditionType

			s := NewFromConfig(&factory.Config{
				NodeLister: &nodeLister{nl},
				Algorithm:  item.algo,
				PodConditionUpdater: fakePodConditionUpdater{
					GetPodConditions: func(pod *v1.Pod) {
						gotPodConditionsType = pod.Status.Conditions[0].Type
					},
				},
				PodPhaseUpdater: fakePodPhaseUpdater{
					GetPodPhase: func(pod *v1.Pod) {
						gotPodPhase = pod.Status.Phase
					},
				},
				NextPod: func() *v1.Pod {
					return item.sendPod
				},
				Framework: EmptyFramework,
				Recorder:  eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "scheduler"}),
			})
			called := make(chan struct{})
			events := eventBroadcaster.StartEventWatcher(func(e *v1.Event) {
				if e, a := item.eventReason, e.Reason; e != a {
					t.Errorf("expected %v, got %v", e, a)
				}
				close(called)
			})
			s.scheduleOne()
			<-called
			if e, g := item.expectPodPhase, gotPodPhase; !reflect.DeepEqual(e, g) {
				t.Errorf("pod phase: wanted %v, got %v", e, g)
			}
			if e, g := item.expectPodConditionsTypes, gotPodConditionsType; !reflect.DeepEqual(e, g) {
				t.Errorf("pod conditions type: wanted %v, got %v", e, g)
			}

			events.Stop()
			time.Sleep(1 * time.Second) // sleep 1 second as called channel cannot be passed into eventBroadcaster.StartEventWatcher
		})
	}
}

func TestServerCreate_SingleServerRequest(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken

	pod := podWithValidSpec("scheduler-unit-test", "scheduler-instance-unit-test")
	manifest := &(pod.Spec)
	instanceID, err := serverCreate(result.SuggestedHost, token, manifest)

	if err != nil {
		t.Errorf("expected instance create success but fail")
	} else {
		INSTANCEID = instanceID
	}
}

func TestServerCreate_SingleServerRequestWithInvalidHost(t *testing.T) {
	// Invalid Host
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "100.31.14.23", UID: types.UID("100.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken

	pod := podWithValidSpec("scheduler-unit-test", "scheduler-instance-unit-test")
	manifest := &(pod.Spec)
	_, err := serverCreate(result.SuggestedHost, token, manifest)

	if err == nil {
		t.Errorf("expected instance create fail but success")
	}
}

func TestServerCreate_MultipleServerRequestWithInvalidToken(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	// Invalid token array
	token := []string{"", "ejlke-eireriu", "xcvdf-eweweas"}

	pod := podWithValidSpec("scheduler-unit-test", "scheduler-instance-unit-test")
	manifest := &(pod.Spec)
	for _, tk := range token {
		_, err := serverCreate(result.SuggestedHost, tk, manifest)
		if err == nil {
			t.Errorf("expected instance create fail but success")
		}
	}
}

func TestRequestToken_SingleRequestWithOneValidHost(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	_, err := requestToken(result.SuggestedHost)
	if err != nil {
		t.Errorf("excepted token request success, but fail")
	}
}

func TestRequestToken_SingleRequestWithOneInvalidHost(t *testing.T) {
	// Invalid Host
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "100.31.14.23", UID: types.UID("100.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	_, err := requestToken(result.SuggestedHost)
	if err == nil {
		t.Errorf("excepted token request fail, but success")
	}
}

func TestCheckInstanceStatus_ACTIVEStatus(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	instanceID := INSTANCEID
	token := TestToken

	instanceStatus, err := checkInstanceStatus(result.SuggestedHost, token, instanceID)
	if err != nil {
		t.Errorf("check instance status process failed")
	} else if instanceStatus != "ACTIVE" {
		t.Errorf("expected instance status is ACTIVE, but is %v", instanceStatus)
	}
}

func TestCheckInstanceStatus_InvalidHost(t *testing.T) {
	// Invalid Host
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "100.31.14.23", UID: types.UID("100.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	instanceID := INSTANCEID
	token := TestToken

	_, err := checkInstanceStatus(result.SuggestedHost, token, instanceID)
	if err == nil {
		t.Errorf("expected instance status check failed but success")
	}
}

func TestCheckInstanceStatus_MultipleInvalidInstanceID(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	// Invalid instanceID array
	instanceID := []string{"efewer-23sdf", "", "ssopc-xiksddaz"}
	token := TestToken

	for _, id := range instanceID {
		_, err := checkInstanceStatus(result.SuggestedHost, token, id)
		if err == nil {
			t.Errorf("expected instance status check failed but success")
		}
	}
}

func TestCheckInstanceStatus_MultipleInvalidToken(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	instanceID := INSTANCEID
	// Invalid token array
	token := []string{"aasoijdoijw-sdofisu", "", "lkodpopo-zxcxcaa"}

	for _, tk := range token {
		_, err := checkInstanceStatus(result.SuggestedHost, tk, instanceID)
		if err == nil {
			t.Errorf("expected instance status check failed but success")
		}
	}
}

func TestDeleteInstance_SingleRequestWithInvalidHost(t *testing.T) {
	// Invalid Host
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "100.31.14.23", UID: types.UID("100.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken
	// Make sure this instanceID exist when testing delete instance request
	instanceID := INSTANCEID

	err := deleteInstance(result.SuggestedHost, token, instanceID)
	if err == nil {
		t.Errorf("expected instance delete fail but success")
	}
}

func TestDeleteInstance_MultipleRequestWithInvalidToken(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	// Invalid token array
	token := []string{"", "sadasda-wewjkejwke", "iroix-sdxxvv"}
	// Make sure this instanceID exist when testing delete instance request
	instanceID := INSTANCEID

	for _, tk := range token {
		err := deleteInstance(result.SuggestedHost, tk, instanceID)
		if err == nil {
			t.Errorf("expected instance delete fail but success")
		}
	}
}

func TestDeleteInstance_MultipleRequestWithInvalidInstanceID(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken
	// Invalid instanceID array
	instanceID := []string{"", "saksjdh-23asd", "bxnmb-dufioewx"}

	for _, instance_id := range instanceID {
		err := deleteInstance(result.SuggestedHost, token, instance_id)
		if err == nil {
			t.Errorf("expected instance delete fail but success")
		}
	}
}

func TestDeleteInstance_SingleRequest(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken
	// Make sure this instanceID exist when testing delete instance request
	instanceID := INSTANCEID

	err := deleteInstance(result.SuggestedHost, token, instanceID)
	if err != nil {
		t.Errorf("expected instance delete success but fail")
	}
}

func TestDeleteInstance_MultipleRequest(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken
	// Make sure this instanceID exist when testing delete instance request
	var instanceID [5]string

	for i := 0; i < 5; i++ {
		pod := podWithValidSpec("scheduler-unit-test-"+strconv.Itoa(i), "scheduler-instance-unit-test-"+strconv.Itoa(i))
		manifest := &(pod.Spec)
		id, _ := serverCreate(result.SuggestedHost, token, manifest)
		instanceID[i] = id
	}

	for j := 0; j < len(instanceID); j++ {
		err := deleteInstance(result.SuggestedHost, token, instanceID[j])
		if err != nil {
			t.Errorf("expected instance delete success but fail")
		}
	}
}

func TestTokenExpired_SingleRequestWithUnexpiredToken(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	// New token
	token := TestToken

	if tokenExpired(result.SuggestedHost, token) {
		t.Errorf("expected token not expired but expired")
	}
}

func TestTokenExpired_SingleRequestWithExpiredToken(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	// Expired token
	token := "ousoidfoisufoiu--ero2o3i23unsd-3343kjhjkhkj"

	if !tokenExpired(result.SuggestedHost, token) {
		t.Errorf("expected token expired but not expired")
	}
}

func TestSchedulerFailedSchedulingReasons(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "52.24.61.210", UID: types.UID("52.24.61.210")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken
	// Make sure this instanceID exist when testing delete instance request
	var instanceID [3]string

	for i := 0; i < 3; i++ {
		pod := podWithXLargeFlavorSpec("scheduler-xlarge-unit-test-"+strconv.Itoa(i), "scheduler-xlarge-instance-unit-test-"+strconv.Itoa(i))
		manifest := &(pod.Spec)
		id, err := serverCreate(result.SuggestedHost, token, manifest)
		if err != nil && err.Error() == "Instance capacity has reached its limit" {
			break
		} else {
			instanceID[i] = id
		}
	}

	for j := 0; j < len(instanceID); j++ {
		deleteInstance(result.SuggestedHost, token, instanceID[j])
	}
}
