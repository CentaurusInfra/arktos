/*
Copyright 2017 The Kubernetes Authors.

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
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/algorithm"
	"k8s.io/kubernetes/pkg/scheduler/algorithm/predicates"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api"
	"k8s.io/kubernetes/pkg/scheduler/factory"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	internalqueue "k8s.io/kubernetes/pkg/scheduler/internal/queue"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
	"k8s.io/kubernetes/pkg/scheduler/core"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/apimachinery/pkg/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/scheduler/algorithm/priorities"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/record"
	"k8s.io/apimachinery/pkg/types"
)

// FakeConfigurator is an implementation for test.
type FakeConfigurator struct {
	Config *factory.Config
}

// GetPredicateMetadataProducer is not implemented yet.
func (fc *FakeConfigurator) GetPredicateMetadataProducer() (predicates.PredicateMetadataProducer, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetPredicates is not implemented yet.
func (fc *FakeConfigurator) GetPredicates(predicateKeys sets.String) (map[string]predicates.FitPredicate, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetHardPodAffinitySymmetricWeight is not implemented yet.
func (fc *FakeConfigurator) GetHardPodAffinitySymmetricWeight() int32 {
	panic("not implemented")
}

// MakeDefaultErrorFunc is not implemented yet.
func (fc *FakeConfigurator) MakeDefaultErrorFunc(backoff *internalqueue.PodBackoffMap, podQueue internalqueue.SchedulingQueue) func(pod *v1.Pod, err error) {
	return nil
}

// GetNodeLister is not implemented yet.
func (fc *FakeConfigurator) GetNodeLister() corelisters.NodeLister {
	return nil
}

// GetClient is not implemented yet.
func (fc *FakeConfigurator) GetClient() clientset.Interface {
	return nil
}

// GetScheduledPodLister is not implemented yet.
func (fc *FakeConfigurator) GetScheduledPodLister() corelisters.PodLister {
	return nil
}

// Create returns FakeConfigurator.Config
func (fc *FakeConfigurator) Create() (*factory.Config, error) {
	return fc.Config, nil
}

// CreateFromProvider returns FakeConfigurator.Config
func (fc *FakeConfigurator) CreateFromProvider(providerName string) (*factory.Config, error) {
	return fc.Config, nil
}

// CreateFromConfig returns FakeConfigurator.Config
func (fc *FakeConfigurator) CreateFromConfig(policy schedulerapi.Policy) (*factory.Config, error) {
	return fc.Config, nil
}

// CreateFromKeys returns FakeConfigurator.Config
func (fc *FakeConfigurator) CreateFromKeys(predicateKeys, priorityKeys sets.String, extenders []algorithm.SchedulerExtender) (*factory.Config, error) {
	return fc.Config, nil
}

// EmptyPluginRegistry is an empty plugin registry used in tests.
var EmptyPluginRegistry = framework.Registry{}

// EmptyFramework is an empty framework used in tests.
// Note: If the test runs in goroutine, please don't use this variable to avoid a race condition.
var EmptyFramework, _ = framework.NewFramework(EmptyPluginRegistry, nil, EmptyPluginConfig)

// EmptyPluginConfig is an empty plugin config used in tests.
var EmptyPluginConfig = []kubeschedulerconfig.PluginConfig{}

type fakeBinder struct {
	b func(binding *v1.Binding) error
}

func (fb fakeBinder) Bind(binding *v1.Binding) error { return fb.b(binding) }

type fakePodConditionUpdater struct{}

func (fc fakePodConditionUpdater) Update(pod *v1.Pod, podCondition *v1.PodCondition) error {
	return nil
}

type fakePodPreemptor struct{}

func (fp fakePodPreemptor) GetUpdatedPod(pod *v1.Pod) (*v1.Pod, error) {
	return pod, nil
}

func (fp fakePodPreemptor) DeletePod(pod *v1.Pod) error {
	return nil
}

func (fp fakePodPreemptor) SetNominatedNodeName(pod *v1.Pod, nomNodeName string) error {
	return nil
}

func (fp fakePodPreemptor) RemoveNominatedNodeName(pod *v1.Pod) error {
	return nil
}

type nodeLister struct {
	corelisters.NodeLister
}

func (n *nodeLister) List() ([]*v1.Node, error) {
	return n.NodeLister.List(labels.Everything())
}

func podWithID(id, desiredHost string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:     id,
			UID:      types.UID(id),
			SelfLink: fmt.Sprintf("/api/v1/%s/%s", string(v1.ResourcePods), id),
		},
		Spec: v1.PodSpec{
			NodeName: desiredHost,
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

func podWithPort(id, desiredHost string, port int) *v1.Pod {
	pod := podWithID(id, desiredHost)
	pod.Spec.Containers = []v1.Container{
		{Name: "ctr", Ports: []v1.ContainerPort{{HostPort: int32(port)}}},
	}
	return pod
}

func podWithResources(id, desiredHost string, limits v1.ResourceList, requests v1.ResourceList) *v1.Pod {
	pod := podWithID(id, desiredHost)
	pod.Spec.Containers = []v1.Container{
		{Name: "ctr", Resources: v1.ResourceRequirements{Limits: limits, Requests: requests}, ResourcesAllocated: requests},
	}
	return pod
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