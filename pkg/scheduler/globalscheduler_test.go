package scheduler

import (
	"fmt"
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
	"testing"
)

// Avoid token expired in the Test functions
var TestToken, _ = requestToken("172.31.14.23")

// EmptyPluginConfig is an empty plugin config used in tests.
var EmptyPluginConfig = []kubeschedulerconfig.PluginConfig{}

// EmptyFramework is an empty framework used in tests.
// Note: If the test runs in goroutine, please don't use this variable to avoid a race condition.
var EmptyFramework, _ = framework.NewFramework(EmptyPluginRegistry, nil, EmptyPluginConfig)

func PredicateOne(pod *v1.Pod, meta predicates.PredicateMetadata, nodeInfo *schedulernodeinfo.NodeInfo) (bool, []predicates.PredicateFailureReason, error) {
	return true, nil, nil
}

func PriorityOne(pod *v1.Pod, nodeNameToInfo map[string]*schedulernodeinfo.NodeInfo, nodes []*v1.Node) (schedulerapi.HostPriorityList, error) {
	return []schedulerapi.HostPriority{}, nil
}

type nodeLister struct {
	corelister.NodeLister
}

func (n *nodeLister) List() ([]*v1.Node, error) {
	return n.NodeLister.List(labels.Everything())
}

type fakeBinder struct {
	b func(binding *v1.Binding) error
}

func (fb fakeBinder) Bind(binding *v1.Binding) error { return fb.b(binding) }

type fakePodPhaseUpdater struct{}

func (fp fakePodPhaseUpdater) Update(pod *v1.Pod, podPhase v1.PodPhase) error {
	return nil
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

func podWithSpec() *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test15pod",
		},
		Spec: v1.PodSpec{
			Nics: []v1.Nic{
				{Uuid: "506ceacd-7395-4afc-9385-3f913a3e0620"},
			},
			VirtualMachine: &v1.VirtualMachine{
				KeyPairName: "KeyMy",
				Name:        "provider-instance-test-15",
				Image:       "9405536b-7dbf-48d4-8120-5e2e4cf2bf0a",
				Scheduling: v1.GlobalScheduling{
					SecurityGroup: []v1.OpenStackSecurityGroup{
						{Name: "7e1736d4-ed68-49a3-84b2-d48b5b4474d8"},
					},
				},
				Resources: v1.ResourceRequirements{
					FlavorRef: "d1",
				},
			},
		},
	}
}

func TestRequestToken_SingleRequestWithOneValidHost(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
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

func TestRequestToken_MultipleRequestWithOneValidHost(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	// Request 1000 times
	for i := 0; i < 1000; i++ {
		_, err := requestToken(result.SuggestedHost)
		if err != nil {
			t.Errorf("excepted token request success, but fail")
		}
	}
}

func TestCheckInstanceStatus_ACTIVEStatus(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	instanceID := "583795aa-7ce1-4093-aa9c-0d4bbea94c43"
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
	instanceID := "583795aa-7ce1-4093-aa9c-0d4bbea94c43"
	token := TestToken

	_, err := checkInstanceStatus(result.SuggestedHost, token, instanceID)
	if err == nil {
		t.Errorf("expected instance status check failed but success")
	}
}

func TestCheckInstanceStatus_InvalidInstanceID(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	// Invalid instanceID array
	instanceID := []string{"efewer-23sdf", ""}
	token := TestToken

	for _, id := range instanceID {
		_, err := checkInstanceStatus(result.SuggestedHost, token, id)
		if err == nil {
			t.Errorf("expected instance status check failed but success")
		}
	}
}

func TestCheckInstanceStatus_InvalidToken(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	instanceID := "583795aa-7ce1-4093-aa9c-0d4bbea94c43"
	// Invalid token array
	token := []string{"aasoijdoijw-sdofisu", ""}

	for _, tk := range token {
		_, err := checkInstanceStatus(result.SuggestedHost, tk, instanceID)
		if err == nil {
			t.Errorf("expected instance status check failed but success")
		}
	}
}

func TestServerCreate_SingleServerRequest(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken

	table := []struct {
		metadataName  string
		nicId         string
		keyPairName   string
		vmName        string
		image         string
		securityGroup string
		flavorRef     string
	}{
		{
			metadataName:  "test15pod",
			nicId:         "506ceacd-7395-4afc-9385-3f913a3e0620",
			keyPairName:   "KeyMy",
			vmName:        "provider-instance-test-15",
			image:         "9405536b-7dbf-48d4-8120-5e2e4cf2bf0a",
			securityGroup: "7e1736d4-ed68-49a3-84b2-d48b5b4474d8",
			flavorRef:     "d1",
		},
	}

	for _, item := range table {
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: item.metadataName,
			},
			Spec: v1.PodSpec{
				Nics: []v1.Nic{
					{Uuid: item.nicId},
				},
				VirtualMachine: &v1.VirtualMachine{
					KeyPairName: item.keyPairName,
					Name:        item.vmName,
					Image:       item.image,
					Scheduling: v1.GlobalScheduling{
						SecurityGroup: []v1.OpenStackSecurityGroup{
							{Name: item.securityGroup},
						},
					},
					Resources: v1.ResourceRequirements{
						FlavorRef: item.flavorRef,
					},
				},
			},
		}
		manifest := &(pod.Spec)
		_, err := serverCreate(result.SuggestedHost, token, manifest)

		if err != nil {
			t.Errorf("expected instance create success but fail")
		}
	}
}

func TestServerCreate_SingleServerRequestWithInvalidHost(t *testing.T) {
	// Invalid Host
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "100.31.14.23", UID: types.UID("100.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken

	table := []struct {
		metadataName  string
		nicId         string
		keyPairName   string
		vmName        string
		image         string
		securityGroup string
		flavorRef     string
	}{
		{
			metadataName:  "test15pod",
			nicId:         "fb3b536b-c07a-42f8-97bd-6d279ff07dd3",
			keyPairName:   "KeyMy",
			vmName:        "provider-instance-test-15",
			image:         "0644079b-33f4-4a55-a180-7fa7f2eec8c8",
			securityGroup: "d3bc9641-08ba-4a15-b8af-9e035e4d4ae7",
			flavorRef:     "d1",
		},
	}

	for _, item := range table {
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: item.metadataName,
			},
			Spec: v1.PodSpec{
				Nics: []v1.Nic{
					{Uuid: item.nicId},
				},
				VirtualMachine: &v1.VirtualMachine{
					KeyPairName: item.keyPairName,
					Name:        item.vmName,
					Image:       item.image,
					Scheduling: v1.GlobalScheduling{
						SecurityGroup: []v1.OpenStackSecurityGroup{
							{Name: item.securityGroup},
						},
					},
					Resources: v1.ResourceRequirements{
						FlavorRef: item.flavorRef,
					},
				},
			},
		}
		manifest := &(pod.Spec)
		_, err := serverCreate(result.SuggestedHost, token, manifest)

		if err == nil {
			t.Errorf("expected instance create fail but success")
		}
	}
}

func TestServerCreate_SingleServerRequestWithInvalidToken(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	// Invalid token array
	token := []string{"", "ejlke-eireriu"}

	table := []struct {
		metadataName  string
		nicId         string
		keyPairName   string
		vmName        string
		image         string
		securityGroup string
		flavorRef     string
	}{
		{
			metadataName:  "test15pod",
			nicId:         "fb3b536b-c07a-42f8-97bd-6d279ff07dd3",
			keyPairName:   "KeyMy",
			vmName:        "provider-instance-test-15",
			image:         "0644079b-33f4-4a55-a180-7fa7f2eec8c8",
			securityGroup: "d3bc9641-08ba-4a15-b8af-9e035e4d4ae7",
			flavorRef:     "d1",
		},
	}

	for _, item := range table {
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: item.metadataName,
			},
			Spec: v1.PodSpec{
				Nics: []v1.Nic{
					{Uuid: item.nicId},
				},
				VirtualMachine: &v1.VirtualMachine{
					KeyPairName: item.keyPairName,
					Name:        item.vmName,
					Image:       item.image,
					Scheduling: v1.GlobalScheduling{
						SecurityGroup: []v1.OpenStackSecurityGroup{
							{Name: item.securityGroup},
						},
					},
					Resources: v1.ResourceRequirements{
						FlavorRef: item.flavorRef,
					},
				},
			},
		}
		manifest := &(pod.Spec)
		for _, tk := range token {
			_, err := serverCreate(result.SuggestedHost, tk, manifest)
			if err == nil {
				t.Errorf("expected instance create fail but success")
			}
		}
	}
}

func TestDeleteInstance_SingleRequest(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken
	// Make sure this instanceID exist when testing delete instance request
	instanceID := "6579d464-63f2-460d-a41e-1b187a98d113"

	err := deleteInstance(result.SuggestedHost, token, instanceID)
	if err != nil {
		t.Errorf("expected instance delete success but fail")
	}
}

func TestDeleteInstance_SingleRequestWithInvalidHost(t *testing.T) {
	// Invalid Host
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "100.31.14.23", UID: types.UID("100.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken
	// Make sure this instanceID exist when testing delete instance request
	instanceID := "8922cd62-ada8-47d3-8647-52089f47f1d3"

	err := deleteInstance(result.SuggestedHost, token, instanceID)
	if err == nil {
		t.Errorf("expected instance delete fail but success")
	}
}

func TestDeleteInstance_SingleRequestWithInvalidToken(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	// Invalid token array
	token := []string{"", "sadasda-wewjkejwke"}
	// Make sure this instanceID exist when testing delete instance request
	instanceID := "34d16057-ae9e-4758-a81a-1c4d102c3c42"

	for _, tk := range token {
		err := deleteInstance(result.SuggestedHost, tk, instanceID)
		if err == nil {
			t.Errorf("expected instance delete fail but success")
		}
	}
}

func TestDeleteInstance_SingleRequestWithInvalidInstanceID(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	token := TestToken
	// Invalid instanceID array
	instanceID := []string{"", "saksjdh-23asd"}

	for _, instance_id := range instanceID {
		err := deleteInstance(result.SuggestedHost, token, instance_id)
		if err == nil {
			t.Errorf("expected instance delete fail but success")
		}
	}
}

func TestTokenExpired_SingleRequestWithUnexpiredToken(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	// New token
	token := TestToken

	if tokenExpired(result.SuggestedHost, token) {
		t.Errorf("expected token not expired but expired")
	}
}

func TestTokenExpired_SingleRequestWithExpiredToken(t *testing.T) {
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}
	result := core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}
	// Expired token
	token := "ousoidfoisufoiu--ero2o3i23unsd-3343kjhjkhkj"

	if !tokenExpired(result.SuggestedHost, token) {
		t.Errorf("expected token expired but not expired")
	}
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
