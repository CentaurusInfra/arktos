package scheduler

import (
	"fmt"
	"testing"
	// "errors"
	"reflect"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/core"
	"k8s.io/kubernetes/pkg/scheduler/algorithm"
	"k8s.io/kubernetes/pkg/scheduler/algorithm/priorities"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/kubernetes/pkg/scheduler/volumebinder"
	"k8s.io/apimachinery/pkg/labels"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/scheduler/factory"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	"k8s.io/kubernetes/pkg/scheduler/algorithm/predicates"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakecache "k8s.io/kubernetes/pkg/scheduler/internal/cache/fake"
	corelister "k8s.io/client-go/listers/core/v1"
	volumescheduling "k8s.io/kubernetes/pkg/controller/volume/scheduling"
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

type fakePodConditionUpdater struct{}
func (fc fakePodConditionUpdater) Update(pod *v1.Pod, podCondition *v1.PodCondition) error {
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
		metadataName string
		nicId string
		keyPairName string
		vmName string
		image string
		securityGroup string
		flavorRef string
	}{
		{
			metadataName: "test15pod",
			nicId: "fb3b536b-c07a-42f8-97bd-6d279ff07dd3",
			keyPairName: "KeyMy",
			vmName: "provider-instance-test-15",
			image: "0644079b-33f4-4a55-a180-7fa7f2eec8c8",
			securityGroup: "d3bc9641-08ba-4a15-b8af-9e035e4d4ae7",
			flavorRef: "d1",
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
					Name: item.vmName,
					Image: item.image,
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
		metadataName string
		nicId string
		keyPairName string
		vmName string
		image string
		securityGroup string
		flavorRef string
	}{
		{
			metadataName: "test15pod",
			nicId: "fb3b536b-c07a-42f8-97bd-6d279ff07dd3",
			keyPairName: "KeyMy",
			vmName: "provider-instance-test-15",
			image: "0644079b-33f4-4a55-a180-7fa7f2eec8c8",
			securityGroup: "d3bc9641-08ba-4a15-b8af-9e035e4d4ae7",
			flavorRef: "d1",
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
					Name: item.vmName,
					Image: item.image,
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
		metadataName string
		nicId string
		keyPairName string
		vmName string
		image string
		securityGroup string
		flavorRef string
	}{
		{
			metadataName: "test15pod",
			nicId: "fb3b536b-c07a-42f8-97bd-6d279ff07dd3",
			keyPairName: "KeyMy",
			vmName: "provider-instance-test-15",
			image: "0644079b-33f4-4a55-a180-7fa7f2eec8c8",
			securityGroup: "d3bc9641-08ba-4a15-b8af-9e035e4d4ae7",
			flavorRef: "d1",
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
					Name: item.vmName,
					Image: item.image,
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

func TestScheduler(t *testing.T) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(t.Logf).Stop()
	// errS := errors.New("scheduler")
	// errB := errors.New("binder")
	testNode := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "172.31.14.23", UID: types.UID("172.31.14.23")}}

	table := []struct {
		name             string
		injectBindError  error
		sendPod          *v1.Pod
		algo             core.ScheduleAlgorithm
		expectErrorPod   *v1.Pod
		expectForgetPod  *v1.Pod
		expectAssumedPod *v1.Pod
		expectError      error
		expectBind       *v1.Binding
		eventReason      string
	}{
		{
			name:             "bind assumed pod scheduled",
			sendPod:          podWithID("foo", ""),
			algo:             mockScheduler{core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 5, FeasibleNodes: 5}, nil},
			expectBind:       &v1.Binding{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: types.UID("foo")}, Target: v1.ObjectReference{Kind: "Node", Name: testNode.Name}},
			expectAssumedPod: podWithID("foo", testNode.Name),
			eventReason:      "Scheduled",
		},
		// {
		// 	name:           "error pod failed scheduling",
		// 	sendPod:        podWithID("foo", ""),
		// 	algo:           mockScheduler{core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 1, FeasibleNodes: 1}, errS},
		// 	expectError:    errS,
		// 	expectErrorPod: podWithID("foo", ""),
		// 	eventReason:    "FailedScheduling",
		// },
		// {
		// 	name:             "error bind forget pod failed scheduling",
		// 	sendPod:          podWithID("foo", ""),
		// 	algo:             mockScheduler{core.ScheduleResult{SuggestedHost: testNode.Name, EvaluatedNodes: 1, FeasibleNodes: 1}, nil},
		// 	expectBind:       &v1.Binding{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: types.UID("foo")}, Target: v1.ObjectReference{Kind: "Node", Name: testNode.Name}},
		// 	expectAssumedPod: podWithID("foo", testNode.Name),
		// 	injectBindError:  errB,
		// 	expectError:      errB,
		// 	expectErrorPod:   podWithID("foo", testNode.Name),
		// 	expectForgetPod:  podWithID("foo", testNode.Name),
		// 	eventReason:      "FailedScheduling",
		// },
		// {
		// 	sendPod:     deletingPod("foo"),
		// 	algo:        mockScheduler{core.ScheduleResult{}, nil},
		// 	eventReason: "FailedScheduling",
		// },
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
			var gotError error
			var gotPod *v1.Pod
			var gotForgetPod *v1.Pod
			var gotAssumedPod *v1.Pod
			var gotBinding *v1.Binding

			s := NewFromConfig(&factory.Config{
				SchedulerCache: &fakecache.Cache{
					ForgetFunc: func(pod *v1.Pod) {
						gotForgetPod = pod
					},
					AssumeFunc: func(pod *v1.Pod) {
						gotAssumedPod = pod
					},
				},
				NodeLister: &nodeLister{nl},
				Algorithm:  item.algo,
				GetBinder: func(pod *v1.Pod) factory.Binder {
					return fakeBinder{func(b *v1.Binding) error {
						gotBinding = b
						return item.injectBindError
					}}
				},
				PodConditionUpdater: fakePodConditionUpdater{},
				Error: func(p *v1.Pod, err error) {
					gotPod = p
					gotError = err
				},
				NextPod: func() *v1.Pod {
					return item.sendPod
				},
				Framework:    EmptyFramework,
				Recorder:     eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "scheduler"}),
				VolumeBinder: volumebinder.NewFakeVolumeBinder(&volumescheduling.FakeVolumeBinderConfig{AllBound: true}),
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
			if e, a := item.expectAssumedPod, gotAssumedPod; !reflect.DeepEqual(e, a) {
				t.Errorf("assumed pod: wanted %v, got %v", e, a)
			}
			if e, a := item.expectErrorPod, gotPod; !reflect.DeepEqual(e, a) {
				t.Errorf("error pod: wanted %v, got %v", e, a)
			}
			if e, a := item.expectForgetPod, gotForgetPod; !reflect.DeepEqual(e, a) {
				t.Errorf("forget pod: wanted %v, got %v", e, a)
			}
			if e, a := item.expectError, gotError; !reflect.DeepEqual(e, a) {
				t.Errorf("error: wanted %v, got %v", e, a)
			}
			if e, a := item.expectBind, gotBinding; !reflect.DeepEqual(e, a) {
				t.Errorf("error: %s", diff.ObjectDiff(e, a))
			}
			events.Stop()
			time.Sleep(1 * time.Second) // sleep 1 second as called channel cannot be passed into eventBroadcaster.StartEventWatcher
		})
	}
}