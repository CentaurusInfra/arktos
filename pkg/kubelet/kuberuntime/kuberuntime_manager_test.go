/*
Copyright 2016 The Kubernetes Authors.
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

package kuberuntime

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	cadvisorapi "github.com/google/cadvisor/info/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/util/flowcontrol"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	apitest "k8s.io/cri-api/pkg/apis/testing"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/features"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	containertest "k8s.io/kubernetes/pkg/kubelet/container/testing"
)

var (
	fakeCreatedAt int64 = 1
)

func createTestRuntimeManager() (*apitest.FakeRuntimeService, *apitest.FakeImageService, *kubeGenericRuntimeManager, error) {
	return customTestRuntimeManager(&credentialprovider.BasicDockerKeyring{})
}

func customTestRuntimeManager(keyring *credentialprovider.BasicDockerKeyring) (*apitest.FakeRuntimeService, *apitest.FakeImageService, *kubeGenericRuntimeManager, error) {
	fakeRuntimeService := apitest.NewFakeRuntimeService()
	fakeImageService := apitest.NewFakeImageService()
	// Only an empty machineInfo is needed here, because in unit test all containers are besteffort,
	// data in machineInfo is not used. If burstable containers are used in unit test in the future,
	// we may want to set memory capacity.
	machineInfo := &cadvisorapi.MachineInfo{}
	osInterface := &containertest.FakeOS{}
	manager, err := newFakeKubeRuntimeManager(fakeRuntimeService, fakeImageService, machineInfo, osInterface, &containertest.FakeRuntimeHelper{}, keyring)
	return fakeRuntimeService, fakeImageService, manager, err
}

// sandboxTemplate is a sandbox template to create fake sandbox.
type sandboxTemplate struct {
	pod       *v1.Pod
	attempt   uint32
	createdAt int64
	state     runtimeapi.PodSandboxState
}

// containerTemplate is a container template to create fake container.
type containerTemplate struct {
	pod            *v1.Pod
	container      *v1.Container
	sandboxAttempt uint32
	attempt        int
	createdAt      int64
	state          runtimeapi.ContainerState
}

// makeAndSetFakePod is a helper function to create and set one fake sandbox for a pod and
// one fake container for each of its container.
func makeAndSetFakePod(t *testing.T, m *kubeGenericRuntimeManager, fakeRuntime *apitest.FakeRuntimeService,
	pod *v1.Pod) (*apitest.FakePodSandbox, []*apitest.FakeContainer) {
	sandbox := makeFakePodSandbox(t, m, sandboxTemplate{
		pod:       pod,
		createdAt: fakeCreatedAt,
		state:     runtimeapi.PodSandboxState_SANDBOX_READY,
	})

	var containers []*apitest.FakeContainer
	newTemplate := func(c *v1.Container) containerTemplate {
		return containerTemplate{
			pod:       pod,
			container: c,
			createdAt: fakeCreatedAt,
			state:     runtimeapi.ContainerState_CONTAINER_RUNNING,
		}
	}
	for i := range pod.Spec.Containers {
		containers = append(containers, makeFakeContainer(t, m, newTemplate(&pod.Spec.Containers[i])))
	}
	for i := range pod.Spec.InitContainers {
		containers = append(containers, makeFakeContainer(t, m, newTemplate(&pod.Spec.InitContainers[i])))
	}

	fakeRuntime.SetFakeSandboxes([]*apitest.FakePodSandbox{sandbox})
	fakeRuntime.SetFakeContainers(containers)
	return sandbox, containers
}

// makeFakePodSandbox creates a fake pod sandbox based on a sandbox template.
func makeFakePodSandbox(t *testing.T, m *kubeGenericRuntimeManager, template sandboxTemplate) *apitest.FakePodSandbox {
	config, err := m.generatePodSandboxConfig(template.pod, template.attempt)
	assert.NoError(t, err, "generatePodSandboxConfig for sandbox template %+v", template)

	podSandboxID := apitest.BuildSandboxName(config.Metadata)
	return &apitest.FakePodSandbox{
		PodSandboxStatus: runtimeapi.PodSandboxStatus{
			Id:        podSandboxID,
			Metadata:  config.Metadata,
			State:     template.state,
			CreatedAt: template.createdAt,
			Network: &runtimeapi.PodSandboxNetworkStatus{
				Ip: apitest.FakePodSandboxIP,
			},
			Labels: config.Labels,
		},
	}
}

// makeFakePodSandboxes creates a group of fake pod sandboxes based on the sandbox templates.
// The function guarantees the order of the fake pod sandboxes is the same with the templates.
func makeFakePodSandboxes(t *testing.T, m *kubeGenericRuntimeManager, templates []sandboxTemplate) []*apitest.FakePodSandbox {
	var fakePodSandboxes []*apitest.FakePodSandbox
	for _, template := range templates {
		fakePodSandboxes = append(fakePodSandboxes, makeFakePodSandbox(t, m, template))
	}
	return fakePodSandboxes
}

// makeFakeContainer creates a fake container based on a container template.
func makeFakeContainer(t *testing.T, m *kubeGenericRuntimeManager, template containerTemplate) *apitest.FakeContainer {
	sandboxConfig, err := m.generatePodSandboxConfig(template.pod, template.sandboxAttempt)
	assert.NoError(t, err, "generatePodSandboxConfig for container template %+v", template)

	containerConfig, _, err := m.generateContainerConfig(template.container, template.pod, template.attempt, "", template.container.Image)
	assert.NoError(t, err, "generateContainerConfig for container template %+v", template)

	podSandboxID := apitest.BuildSandboxName(sandboxConfig.Metadata)
	containerID := apitest.BuildContainerName(containerConfig.Metadata, podSandboxID)
	imageRef := containerConfig.Image.Image
	return &apitest.FakeContainer{
		ContainerStatus: runtimeapi.ContainerStatus{
			Id:          containerID,
			Metadata:    containerConfig.Metadata,
			Image:       containerConfig.Image,
			ImageRef:    imageRef,
			CreatedAt:   template.createdAt,
			State:       template.state,
			Labels:      containerConfig.Labels,
			Annotations: containerConfig.Annotations,
			LogPath:     filepath.Join(sandboxConfig.GetLogDirectory(), containerConfig.GetLogPath()),
		},
		SandboxID: podSandboxID,
	}
}

// makeFakeContainers creates a group of fake containers based on the container templates.
// The function guarantees the order of the fake containers is the same with the templates.
func makeFakeContainers(t *testing.T, m *kubeGenericRuntimeManager, templates []containerTemplate) []*apitest.FakeContainer {
	var fakeContainers []*apitest.FakeContainer
	for _, template := range templates {
		fakeContainers = append(fakeContainers, makeFakeContainer(t, m, template))
	}
	return fakeContainers
}

// makeTestContainer creates a test api container.
func makeTestContainer(name, image string) v1.Container {
	return v1.Container{
		Name:  name,
		Image: image,
	}
}

// makeTestPod creates a test api pod.
func makeTestPod(podName, podNamespace, podUID string, containers []v1.Container) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID(podUID),
			Name:      podName,
			Namespace: podNamespace,
		},
		Spec: v1.PodSpec{
			Containers: containers,
		},
	}
}

// verifyPods returns true if the two pod slices are equal.
func verifyPods(a, b []*kubecontainer.Pod) bool {
	if len(a) != len(b) {
		return false
	}

	// Sort the containers within a pod.
	for i := range a {
		sort.Sort(containersByID(a[i].Containers))
	}
	for i := range b {
		sort.Sort(containersByID(b[i].Containers))
	}

	// Sort the pods by UID.
	sort.Sort(podsByID(a))
	sort.Sort(podsByID(b))

	return reflect.DeepEqual(a, b)
}

func verifyFakeContainerList(fakeRuntime *apitest.FakeRuntimeService, expected sets.String) (sets.String, bool) {
	actual := sets.NewString()
	for _, c := range fakeRuntime.Containers {
		actual.Insert(c.Id)
	}
	return actual, actual.Equal(expected)
}

// Only extract the fields of interests.
type cRecord struct {
	name    string
	attempt uint32
	state   runtimeapi.ContainerState
}

type cRecordList []*cRecord

func (b cRecordList) Len() int      { return len(b) }
func (b cRecordList) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b cRecordList) Less(i, j int) bool {
	if b[i].name != b[j].name {
		return b[i].name < b[j].name
	}
	return b[i].attempt < b[j].attempt
}

func verifyContainerStatuses(t *testing.T, runtime *apitest.FakeRuntimeService, expected []*cRecord, desc string) {
	actual := []*cRecord{}
	for _, cStatus := range runtime.Containers {
		actual = append(actual, &cRecord{name: cStatus.Metadata.Name, attempt: cStatus.Metadata.Attempt, state: cStatus.State})
	}
	sort.Sort(cRecordList(expected))
	sort.Sort(cRecordList(actual))
	assert.Equal(t, expected, actual, desc)
}

func TestNewKubeRuntimeManager(t *testing.T) {
	_, _, _, err := createTestRuntimeManager()
	assert.NoError(t, err)
}

func TestVersion(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	version, err := m.Version()
	assert.NoError(t, err)
	assert.Equal(t, kubeRuntimeAPIVersion, version.String())
}

func TestContainerRuntimeType(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	runtimeType := m.Type()
	assert.Equal(t, apitest.FakeRuntimeName, runtimeType)
}

func TestGetPodStatus(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	containers := []v1.Container{
		{
			Name:            "foo1",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
		{
			Name:            "foo2",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: containers,
		},
	}

	// Set fake sandbox and faked containers to fakeRuntime.
	makeAndSetFakePod(t, m, fakeRuntime, pod)

	podStatus, err := m.GetPodStatus(pod.UID, pod.Name, pod.Namespace, pod.Tenant)
	assert.NoError(t, err)
	assert.Equal(t, pod.UID, podStatus.ID)
	assert.Equal(t, pod.Name, podStatus.Name)
	assert.Equal(t, pod.Namespace, podStatus.Namespace)
	assert.Equal(t, pod.Tenant, podStatus.Tenant)
	assert.Equal(t, apitest.FakePodSandboxIP, podStatus.IP)
}

func TestGetPods(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "foo1",
					Image: "busybox",
				},
				{
					Name:  "foo2",
					Image: "busybox",
				},
			},
		},
	}

	// Set fake sandbox and fake containers to fakeRuntime.
	fakeSandbox, fakeContainers := makeAndSetFakePod(t, m, fakeRuntime, pod)

	// Convert the fakeContainers to kubecontainer.Container
	containers := make([]*kubecontainer.Container, len(fakeContainers))
	for i := range containers {
		fakeContainer := fakeContainers[i]
		c, err := m.toKubeContainer(&runtimeapi.Container{
			Id:          fakeContainer.Id,
			Metadata:    fakeContainer.Metadata,
			State:       fakeContainer.State,
			Image:       fakeContainer.Image,
			ImageRef:    fakeContainer.ImageRef,
			Labels:      fakeContainer.Labels,
			Annotations: fakeContainer.Annotations,
		})
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		containers[i] = c
	}
	// Convert fakeSandbox to kubecontainer.Container
	sandbox, err := m.sandboxToKubeContainer(&runtimeapi.PodSandbox{
		Id:          fakeSandbox.Id,
		Metadata:    fakeSandbox.Metadata,
		State:       fakeSandbox.State,
		CreatedAt:   fakeSandbox.CreatedAt,
		Labels:      fakeSandbox.Labels,
		Annotations: fakeSandbox.Annotations,
	})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	expected := []*kubecontainer.Pod{
		{
			ID:         types.UID("12345678"),
			Name:       "foo",
			Namespace:  "new",
			Containers: []*kubecontainer.Container{containers[0], containers[1]},
			Sandboxes:  []*kubecontainer.Container{sandbox},
		},
	}

	actual, err := m.GetPods(false)
	assert.NoError(t, err)

	if !verifyPods(expected, actual) {
		t.Errorf("expected %#v, got %#v", expected, actual)
	}
}

func TestKillPod(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "foo1",
					Image: "busybox",
				},
				{
					Name:  "foo2",
					Image: "busybox",
				},
			},
		},
	}

	// Set fake sandbox and fake containers to fakeRuntime.
	fakeSandbox, fakeContainers := makeAndSetFakePod(t, m, fakeRuntime, pod)

	// Convert the fakeContainers to kubecontainer.Container
	containers := make([]*kubecontainer.Container, len(fakeContainers))
	for i := range containers {
		fakeContainer := fakeContainers[i]
		c, err := m.toKubeContainer(&runtimeapi.Container{
			Id:       fakeContainer.Id,
			Metadata: fakeContainer.Metadata,
			State:    fakeContainer.State,
			Image:    fakeContainer.Image,
			ImageRef: fakeContainer.ImageRef,
			Labels:   fakeContainer.Labels,
		})
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		containers[i] = c
	}
	runningPod := kubecontainer.Pod{
		ID:         pod.UID,
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Containers: []*kubecontainer.Container{containers[0], containers[1]},
		Sandboxes: []*kubecontainer.Container{
			{
				ID: kubecontainer.ContainerID{
					ID:   fakeSandbox.Id,
					Type: apitest.FakeRuntimeName,
				},
			},
		},
	}

	err = m.KillPod(pod, runningPod, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(fakeRuntime.Containers))
	assert.Equal(t, 1, len(fakeRuntime.Sandboxes))
	for _, sandbox := range fakeRuntime.Sandboxes {
		assert.Equal(t, runtimeapi.PodSandboxState_SANDBOX_NOTREADY, sandbox.State)
	}
	for _, c := range fakeRuntime.Containers {
		assert.Equal(t, runtimeapi.ContainerState_CONTAINER_EXITED, c.State)
	}
}

func TestSyncPod(t *testing.T) {
	fakeRuntime, fakeImage, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	containers := []v1.Container{
		{
			Name:            "foo1",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
		{
			Name:            "foo2",
			Image:           "alpine",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: containers,
		},
	}

	backOff := flowcontrol.NewBackOff(time.Second, time.Minute)
	result := m.SyncPod(pod, &kubecontainer.PodStatus{}, []v1.Secret{}, backOff)
	assert.NoError(t, result.Error())
	assert.Equal(t, 2, len(fakeRuntime.Containers))
	assert.Equal(t, 2, len(fakeImage.Images))
	assert.Equal(t, 1, len(fakeRuntime.Sandboxes))
	for _, sandbox := range fakeRuntime.Sandboxes {
		assert.Equal(t, runtimeapi.PodSandboxState_SANDBOX_READY, sandbox.State)
	}
	for _, c := range fakeRuntime.Containers {
		assert.Equal(t, runtimeapi.ContainerState_CONTAINER_RUNNING, c.State)
	}
}

func TestPruneInitContainers(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	init1 := makeTestContainer("init1", "busybox")
	init2 := makeTestContainer("init2", "busybox")
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{init1, init2},
		},
	}

	templates := []containerTemplate{
		{pod: pod, container: &init1, attempt: 3, createdAt: 3, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{pod: pod, container: &init1, attempt: 2, createdAt: 2, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{pod: pod, container: &init2, attempt: 1, createdAt: 1, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{pod: pod, container: &init1, attempt: 1, createdAt: 1, state: runtimeapi.ContainerState_CONTAINER_UNKNOWN},
		{pod: pod, container: &init2, attempt: 0, createdAt: 0, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{pod: pod, container: &init1, attempt: 0, createdAt: 0, state: runtimeapi.ContainerState_CONTAINER_EXITED},
	}
	fakes := makeFakeContainers(t, m, templates)
	fakeRuntime.SetFakeContainers(fakes)
	m.addPodRuntimeService(string(pod.UID), fakeRuntime)
	podStatus, err := m.GetPodStatus(pod.UID, pod.Name, pod.Namespace, pod.Tenant)
	assert.NoError(t, err)

	m.pruneInitContainersBeforeStart(pod, podStatus)
	expectedContainers := sets.NewString(fakes[0].Id, fakes[2].Id)
	if actual, ok := verifyFakeContainerList(fakeRuntime, expectedContainers); !ok {
		t.Errorf("expected %v, got %v", expectedContainers, actual)
	}
}

func TestSyncPodWithInitContainers(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	initContainers := []v1.Container{
		{
			Name:            "init1",
			Image:           "init",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
	containers := []v1.Container{
		{
			Name:            "foo1",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
		{
			Name:            "foo2",
			Image:           "alpine",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers:     containers,
			InitContainers: initContainers,
		},
	}

	backOff := flowcontrol.NewBackOff(time.Second, time.Minute)

	m.addPodRuntimeService(string(pod.UID), fakeRuntime)

	// 1. should only create the init container.
	podStatus, err := m.GetPodStatus(pod.UID, pod.Name, pod.Namespace, pod.Tenant)
	assert.NoError(t, err)
	result := m.SyncPod(pod, podStatus, []v1.Secret{}, backOff)
	assert.NoError(t, result.Error())
	expected := []*cRecord{
		{name: initContainers[0].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_RUNNING},
	}
	verifyContainerStatuses(t, fakeRuntime, expected, "start only the init container")

	// 2. should not create app container because init container is still running.
	podStatus, err = m.GetPodStatus(pod.UID, pod.Name, pod.Namespace, pod.Tenant)
	assert.NoError(t, err)
	result = m.SyncPod(pod, podStatus, []v1.Secret{}, backOff)
	assert.NoError(t, result.Error())
	verifyContainerStatuses(t, fakeRuntime, expected, "init container still running; do nothing")

	// 3. should create all app containers because init container finished.
	// Stop init container instance 0.
	sandboxIDs, err := m.getSandboxIDByPodUID(pod.UID, nil)
	require.NoError(t, err)
	sandboxID := sandboxIDs[0]
	initID0, err := fakeRuntime.GetContainerID(sandboxID, initContainers[0].Name, 0)
	require.NoError(t, err)
	fakeRuntime.StopContainer(initID0, 0)
	// Sync again.
	podStatus, err = m.GetPodStatus(pod.UID, pod.Name, pod.Namespace, pod.Tenant)
	assert.NoError(t, err)
	result = m.SyncPod(pod, podStatus, []v1.Secret{}, backOff)
	assert.NoError(t, result.Error())
	expected = []*cRecord{
		{name: initContainers[0].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{name: containers[0].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_RUNNING},
		{name: containers[1].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_RUNNING},
	}
	verifyContainerStatuses(t, fakeRuntime, expected, "init container completed; all app containers should be running")

	// 4. should restart the init container if needed to create a new podsandbox
	// Stop the pod sandbox.
	fakeRuntime.StopPodSandbox(sandboxID)
	// Sync again.
	podStatus, err = m.GetPodStatus(pod.UID, pod.Name, pod.Namespace, pod.Tenant)
	assert.NoError(t, err)
	result = m.SyncPod(pod, podStatus, []v1.Secret{}, backOff)
	assert.NoError(t, result.Error())
	expected = []*cRecord{
		// The first init container instance is purged and no longer visible.
		// The second (attempt == 1) instance has been started and is running.
		{name: initContainers[0].Name, attempt: 1, state: runtimeapi.ContainerState_CONTAINER_RUNNING},
		// All containers are killed.
		{name: containers[0].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{name: containers[1].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_EXITED},
	}
	verifyContainerStatuses(t, fakeRuntime, expected, "kill all app containers, purge the existing init container, and restart a new one")
}

// A helper function to get a basic pod and its status assuming all sandbox and
// containers are running and ready.
func makeBasePodAndStatus() (*v1.Pod, *kubecontainer.PodStatus) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "foo-ns",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "foo1",
					Image: "busybox",
				},
				{
					Name:  "foo2",
					Image: "busybox",
				},
				{
					Name:  "foo3",
					Image: "busybox",
				},
			},
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					ContainerID: "://id2",
					Name:        "foo2",
					Image:       "busybox",
					State:       v1.ContainerState{Running: &v1.ContainerStateRunning{}},
				},
				{
					ContainerID: "://id1",
					Name:        "foo1",
					Image:       "busybox",
					State:       v1.ContainerState{Running: &v1.ContainerStateRunning{}},
				},
				{
					ContainerID: "://id3",
					Name:        "foo3",
					Image:       "busybox",
					State:       v1.ContainerState{Running: &v1.ContainerStateRunning{}},
				},
			},
		},
	}
	status := &kubecontainer.PodStatus{
		ID:        pod.UID,
		Name:      pod.Name,
		Namespace: pod.Namespace,
		SandboxStatuses: []*runtimeapi.PodSandboxStatus{
			{
				Id:       "sandboxID",
				State:    runtimeapi.PodSandboxState_SANDBOX_READY,
				Metadata: &runtimeapi.PodSandboxMetadata{Name: pod.Name, Namespace: pod.Namespace, Uid: "sandboxuid", Attempt: uint32(0)},
				Network:  &runtimeapi.PodSandboxNetworkStatus{Ip: "10.0.0.1"},
			},
		},
		ContainerStatuses: []*kubecontainer.ContainerStatus{
			{
				ID:   kubecontainer.ContainerID{ID: "id1"},
				Name: "foo1", State: kubecontainer.ContainerStateRunning,
				Hash: kubecontainer.HashContainer(&pod.Spec.Containers[0]),
			},
			{
				ID:   kubecontainer.ContainerID{ID: "id2"},
				Name: "foo2", State: kubecontainer.ContainerStateRunning,
				Hash: kubecontainer.HashContainer(&pod.Spec.Containers[1]),
			},
			{
				ID:   kubecontainer.ContainerID{ID: "id3"},
				Name: "foo3", State: kubecontainer.ContainerStateRunning,
				Hash: kubecontainer.HashContainer(&pod.Spec.Containers[2]),
			},
		},
	}
	return pod, status
}

func TestComputePodActions(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	require.NoError(t, err)

	// Createing a pair reference pod and status for the test cases to refer
	// the specific fields.
	basePod, baseStatus := makeBasePodAndStatus()
	noAction := podActions{
		SandboxID:           baseStatus.SandboxStatuses[0].Id,
		ContainersToStart:   []int{},
		ContainersToUpdate:  map[string][]containerToUpdateInfo{},
		ContainersToRestart: []int{},
		ContainersToKill:    map[kubecontainer.ContainerID]containerToKillInfo{},
	}

	for desc, test := range map[string]struct {
		mutatePodFn    func(*v1.Pod)
		mutateStatusFn func(*kubecontainer.PodStatus)
		actions        podActions
	}{
		"everying is good; do nothing": {
			actions: noAction,
		},
		"start pod sandbox and all containers for a new pod": {
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				// No container or sandbox exists.
				status.SandboxStatuses = []*runtimeapi.PodSandboxStatus{}
				status.ContainerStatuses = []*kubecontainer.ContainerStatus{}
			},
			actions: podActions{
				KillPod:             true,
				CreateSandbox:       true,
				Attempt:             uint32(0),
				ContainersToStart:   []int{0, 1, 2},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"restart exited containers if RestartPolicy == Always": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				// The first container completed, restart it,
				status.ContainerStatuses[0].State = kubecontainer.ContainerStateExited
				status.ContainerStatuses[0].ExitCode = 0

				// The second container exited with failure, restart it,
				status.ContainerStatuses[1].State = kubecontainer.ContainerStateExited
				status.ContainerStatuses[1].ExitCode = 111
			},
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToStart:   []int{0, 1},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"restart failed containers if RestartPolicy == OnFailure": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				// The first container completed, don't restart it,
				status.ContainerStatuses[0].State = kubecontainer.ContainerStateExited
				status.ContainerStatuses[0].ExitCode = 0

				// The second container exited with failure, restart it,
				status.ContainerStatuses[1].State = kubecontainer.ContainerStateExited
				status.ContainerStatuses[1].ExitCode = 111
			},
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToStart:   []int{1},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"don't restart containers if RestartPolicy == Never": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				// Don't restart any containers.
				status.ContainerStatuses[0].State = kubecontainer.ContainerStateExited
				status.ContainerStatuses[0].ExitCode = 0
				status.ContainerStatuses[1].State = kubecontainer.ContainerStateExited
				status.ContainerStatuses[1].ExitCode = 111
			},
			actions: noAction,
		},
		"Kill pod and recreate everything if the pod sandbox is dead, and RestartPolicy == Always": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
			},
			actions: podActions{
				KillPod:             true,
				CreateSandbox:       true,
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				Attempt:             uint32(1),
				ContainersToStart:   []int{0, 1, 2},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"Kill pod and recreate all containers (except for the succeeded one) if the pod sandbox is dead, and RestartPolicy == OnFailure": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
				status.ContainerStatuses[1].State = kubecontainer.ContainerStateExited
				status.ContainerStatuses[1].ExitCode = 0
			},
			actions: podActions{
				KillPod:             true,
				CreateSandbox:       true,
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				Attempt:             uint32(1),
				ContainersToStart:   []int{0, 2},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"Kill pod and recreate all containers if the PodSandbox does not have an IP": {
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.SandboxStatuses[0].Network.Ip = ""
			},
			actions: podActions{
				KillPod:             true,
				CreateSandbox:       true,
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				Attempt:             uint32(1),
				ContainersToStart:   []int{0, 1, 2},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"Kill and recreate the container if the container's spec changed": {
			mutatePodFn: func(pod *v1.Pod) {
				pod.Spec.RestartPolicy = v1.RestartPolicyAlways
			},
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.ContainerStatuses[1].Hash = uint64(432423432)
			},
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:    getKillMap(basePod, baseStatus, []int{1}),
				ContainersToStart:   []int{1},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
			},
			// TODO: Add a test case for containers which failed the liveness
			// check. Will need to fake the livessness check result.
		},
		"Verify we do not create a pod sandbox if no ready sandbox for pod with RestartPolicy=Never and all containers exited": {
			mutatePodFn: func(pod *v1.Pod) {
				pod.Spec.RestartPolicy = v1.RestartPolicyNever
			},
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				// no ready sandbox
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
				status.SandboxStatuses[0].Metadata.Attempt = uint32(1)
				// all containers exited
				for i := range status.ContainerStatuses {
					status.ContainerStatuses[i].State = kubecontainer.ContainerStateExited
					status.ContainerStatuses[i].ExitCode = 0
				}
			},
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				Attempt:             uint32(2),
				CreateSandbox:       false,
				KillPod:             true,
				ContainersToStart:   []int{},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    map[kubecontainer.ContainerID]containerToKillInfo{},
			},
		},
		"Verify we create a pod sandbox if no ready sandbox for pod with RestartPolicy=Never and no containers have ever been created": {
			mutatePodFn: func(pod *v1.Pod) {
				pod.Spec.RestartPolicy = v1.RestartPolicyNever
			},
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				// no ready sandbox
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
				status.SandboxStatuses[0].Metadata.Attempt = uint32(2)
				// no visible containers
				status.ContainerStatuses = []*kubecontainer.ContainerStatus{}
			},
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				Attempt:             uint32(3),
				CreateSandbox:       true,
				KillPod:             true,
				ContainersToStart:   []int{0, 1, 2},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    map[kubecontainer.ContainerID]containerToKillInfo{},
			},
		},
		"Kill and recreate the container if the container is in unknown state": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.ContainerStatuses[1].State = kubecontainer.ContainerStateUnknown
			},
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:    getKillMap(basePod, baseStatus, []int{1}),
				ContainersToStart:   []int{1},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
			},
		},
	} {
		pod, status := makeBasePodAndStatus()
		if test.mutatePodFn != nil {
			test.mutatePodFn(pod)
		}
		if test.mutateStatusFn != nil {
			test.mutateStatusFn(status)
		}
		actions := m.computePodActions(pod, status)
		verifyActions(t, &test.actions, &actions, desc)
	}
}

func getKillMap(pod *v1.Pod, status *kubecontainer.PodStatus, cIndexes []int) map[kubecontainer.ContainerID]containerToKillInfo {
	m := map[kubecontainer.ContainerID]containerToKillInfo{}
	for _, i := range cIndexes {
		m[status.ContainerStatuses[i].ID] = containerToKillInfo{
			container: &pod.Spec.Containers[i],
			name:      pod.Spec.Containers[i].Name,
		}
	}
	return m
}

func getKillMapWithInitContainers(pod *v1.Pod, status *kubecontainer.PodStatus, cIndexes []int) map[kubecontainer.ContainerID]containerToKillInfo {
	m := map[kubecontainer.ContainerID]containerToKillInfo{}
	for _, i := range cIndexes {
		m[status.ContainerStatuses[i].ID] = containerToKillInfo{
			container: &pod.Spec.InitContainers[i],
			name:      pod.Spec.InitContainers[i].Name,
		}
	}
	return m
}

func verifyActions(t *testing.T, expected, actual *podActions, desc string) {
	if actual.ContainersToKill != nil {
		// Clear the message field since we don't need to verify the message.
		for k, info := range actual.ContainersToKill {
			info.message = ""
			actual.ContainersToKill[k] = info
		}
	}
	assert.Equal(t, expected, actual, desc)
}

func TestComputePodActionsWithInitContainers(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	require.NoError(t, err)

	// Createing a pair reference pod and status for the test cases to refer
	// the specific fields.
	basePod, baseStatus := makeBasePodAndStatusWithInitContainers()
	noAction := podActions{
		SandboxID:           baseStatus.SandboxStatuses[0].Id,
		ContainersToStart:   []int{},
		ContainersToUpdate:  map[string][]containerToUpdateInfo{},
		ContainersToRestart: []int{},
		ContainersToKill:    map[kubecontainer.ContainerID]containerToKillInfo{},
	}

	for desc, test := range map[string]struct {
		mutatePodFn    func(*v1.Pod)
		mutateStatusFn func(*kubecontainer.PodStatus)
		actions        podActions
	}{
		"initialization completed; start all containers": {
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToStart:   []int{0, 1, 2},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"initialization in progress; do nothing": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.ContainerStatuses[2].State = kubecontainer.ContainerStateRunning
			},
			actions: noAction,
		},
		"Kill pod and restart the first init container if the pod sandbox is dead": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
			},
			actions: podActions{
				KillPod:                  true,
				CreateSandbox:            true,
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				Attempt:                  uint32(1),
				NextInitContainerToStart: &basePod.Spec.InitContainers[0],
				ContainersToStart:        []int{},
				ContainersToUpdate:       map[string][]containerToUpdateInfo{},
				ContainersToRestart:      []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"initialization failed; restart the last init container if RestartPolicy == Always": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.ContainerStatuses[2].ExitCode = 137
			},
			actions: podActions{
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				NextInitContainerToStart: &basePod.Spec.InitContainers[2],
				ContainersToStart:        []int{},
				ContainersToUpdate:       map[string][]containerToUpdateInfo{},
				ContainersToRestart:      []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"initialization failed; restart the last init container if RestartPolicy == OnFailure": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.ContainerStatuses[2].ExitCode = 137
			},
			actions: podActions{
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				NextInitContainerToStart: &basePod.Spec.InitContainers[2],
				ContainersToStart:        []int{},
				ContainersToUpdate:       map[string][]containerToUpdateInfo{},
				ContainersToRestart:      []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"initialization failed; kill pod if RestartPolicy == Never": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.ContainerStatuses[2].ExitCode = 137
			},
			actions: podActions{
				KillPod:             true,
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToStart:   []int{},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"init container state unknown; kill and recreate the last init container if RestartPolicy == Always": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.ContainerStatuses[2].State = kubecontainer.ContainerStateUnknown
			},
			actions: podActions{
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				NextInitContainerToStart: &basePod.Spec.InitContainers[2],
				ContainersToStart:        []int{},
				ContainersToUpdate:       map[string][]containerToUpdateInfo{},
				ContainersToRestart:      []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{2}),
			},
		},
		"init container state unknown; kill and recreate the last init container if RestartPolicy == OnFailure": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.ContainerStatuses[2].State = kubecontainer.ContainerStateUnknown
			},
			actions: podActions{
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				NextInitContainerToStart: &basePod.Spec.InitContainers[2],
				ContainersToStart:        []int{},
				ContainersToUpdate:       map[string][]containerToUpdateInfo{},
				ContainersToRestart:      []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{2}),
			},
		},
		"init container state unknown; kill pod if RestartPolicy == Never": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.ContainerStatuses[2].State = kubecontainer.ContainerStateUnknown
			},
			actions: podActions{
				KillPod:             true,
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToStart:   []int{},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"Pod sandbox not ready, init container failed, but RestartPolicy == Never; kill pod only": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
			},
			actions: podActions{
				KillPod:             true,
				CreateSandbox:       false,
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				Attempt:             uint32(1),
				ContainersToStart:   []int{},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"Pod sandbox not ready, and RestartPolicy == Never, but no visible init containers;  create a new pod sandbox": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *kubecontainer.PodStatus) {
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
				status.ContainerStatuses = []*kubecontainer.ContainerStatus{}
			},
			actions: podActions{
				KillPod:                  true,
				CreateSandbox:            true,
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				Attempt:                  uint32(1),
				NextInitContainerToStart: &basePod.Spec.InitContainers[0],
				ContainersToStart:        []int{},
				ContainersToUpdate:       map[string][]containerToUpdateInfo{},
				ContainersToRestart:      []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
	} {
		pod, status := makeBasePodAndStatusWithInitContainers()
		if test.mutatePodFn != nil {
			test.mutatePodFn(pod)
		}
		if test.mutateStatusFn != nil {
			test.mutateStatusFn(status)
		}
		actions := m.computePodActions(pod, status)
		verifyActions(t, &test.actions, &actions, desc)
	}
}

func makeBasePodAndStatusWithInitContainers() (*v1.Pod, *kubecontainer.PodStatus) {
	pod, status := makeBasePodAndStatus()
	pod.Spec.InitContainers = []v1.Container{
		{
			Name:  "init1",
			Image: "bar-image",
		},
		{
			Name:  "init2",
			Image: "bar-image",
		},
		{
			Name:  "init3",
			Image: "bar-image",
		},
	}
	// Replace the original statuses of the containers with those for the init
	// containers.
	status.ContainerStatuses = []*kubecontainer.ContainerStatus{
		{
			ID:   kubecontainer.ContainerID{ID: "initid1"},
			Name: "init1", State: kubecontainer.ContainerStateExited,
			Hash: kubecontainer.HashContainer(&pod.Spec.InitContainers[0]),
		},
		{
			ID:   kubecontainer.ContainerID{ID: "initid2"},
			Name: "init2", State: kubecontainer.ContainerStateExited,
			Hash: kubecontainer.HashContainer(&pod.Spec.InitContainers[0]),
		},
		{
			ID:   kubecontainer.ContainerID{ID: "initid3"},
			Name: "init3", State: kubecontainer.ContainerStateExited,
			Hash: kubecontainer.HashContainer(&pod.Spec.InitContainers[0]),
		},
	}
	return pod, status
}

func TestComputePodActionsWithNICHotplug(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	require.NoError(t, err)

	_, baseStatus := makeBasePodAndStatusWithNICs()
	noAction := podActions{
		SandboxID:           baseStatus.SandboxStatuses[0].Id,
		ContainersToStart:   []int{},
		ContainersToUpdate:  map[string][]containerToUpdateInfo{},
		ContainersToRestart: []int{},
		ContainersToKill:    map[kubecontainer.ContainerID]containerToKillInfo{},
	}

	for desc, test := range map[string]struct {
		mutatePodFn    func(*v1.Pod)
		mutateStatusFn func(*kubecontainer.PodStatus)
		actions        podActions
	}{
		"everything is good; do nothing": {
			actions: noAction,
		},
		"hotplug vnic if nic plugin requested": {
			mutatePodFn: func(pod *v1.Pod) {
				nicNew := v1.Nic{Name: "eth1", PortId: "abcde"}
				pod.Spec.Nics = append(pod.Spec.Nics, nicNew)
			},
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToStart:   []int{},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    map[kubecontainer.ContainerID]containerToKillInfo{},
				Hotplugs:            ConfigChanges{NICsToAttach: []string{"eth1"}},
			},
		},
		"no hotplug if portID missing": {
			mutatePodFn: func(pod *v1.Pod) {
				nicNew := v1.Nic{Name: "eth1"}
				pod.Spec.Nics = append(pod.Spec.Nics, nicNew)
			},
			actions: noAction,
		},
		"hot plug out secondary nic": {
			mutatePodFn: func(pod *v1.Pod) {
				pod.Spec.Nics = []v1.Nic{
					{
						Name:   "eth0", // primary nic
						PortId: "12345",
					},
				}
			},
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToStart:   []int{},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
				ContainersToKill:    map[kubecontainer.ContainerID]containerToKillInfo{},
				Hotplugs:            ConfigChanges{NICsToDetach: []string{"eth9"}},
			},
		},
	} {
		pod, status := makeBasePodAndStatusWithNICs()
		if test.mutatePodFn != nil {
			test.mutatePodFn(pod)
		}
		if test.mutateStatusFn != nil {
			test.mutateStatusFn(status)
		}
		actions := m.computePodActions(pod, status)
		verifyActions(t, &test.actions, &actions, desc)
	}
}

func makeBasePodAndStatusWithNICs() (*v1.Pod, *kubecontainer.PodStatus) {
	pod, status := makeBasePodAndStatus()
	pod.Spec.Nics = []v1.Nic{
		{
			Name:   "eth0", // primary nic
			PortId: "12345",
		},
		{
			Name:   "eth9", //secondary nic able to plug out
			PortId: "99999",
		},
	}
	status.SandboxStatuses = []*runtimeapi.PodSandboxStatus{
		{
			Metadata: &runtimeapi.PodSandboxMetadata{},
			Network: &runtimeapi.PodSandboxNetworkStatus{
				Ip: "10.0.0.100",
				Nics: []*runtimeapi.NICStatus{
					{Name: "eth0"},
					{Name: "eth9"},
				},
			},
		},
	}
	return pod, status
}

func TestAttachNICs(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	require.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "foo-ns",
		},
		Spec: v1.PodSpec{
			VPC: "vps-demo",
			Nics: []v1.Nic{
				{Name: "eth0", PortId: "0000"},
				{Name: "eth1", PortId: "1111"},
				{Name: "eth2", PortId: "2222"},
			},
		},
	}

	syncResult := m.attachNICs(pod, "sandbox0", []string{"eth1", "eth2"})

	if syncResult.Action != kubecontainer.HotplugDevice {
		t.Errorf("expecting HotplugDevice, got %q", syncResult.Action)
	}
	if syncResult.Error != nil {
		t.Errorf("got unexpected error: %+v", syncResult.Error)
	}
}

func TestDetachNICs(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	require.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "foo-ns",
		},
		Spec: v1.PodSpec{
			VPC: "vps-demo",
			Nics: []v1.Nic{
				{Name: "eth0", PortId: "0000"},
				{Name: "eth1", PortId: "1111"},
			},
		},
	}

	syncResult := m.detachNICs(pod, "sandbox0", []string{"eth2"})

	if syncResult.Action != kubecontainer.HotplugDevice {
		t.Errorf("expecting HotplugDevice, got %q", syncResult.Action)
	}
	if syncResult.Error != nil {
		t.Errorf("got unexpected error: %+v", syncResult.Error)
	}
}

func TestComputePodActionsForPodResize(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.InPlacePodVerticalScaling, true)()
	_, _, m, err := createTestRuntimeManager()
	require.NoError(t, err)

	basePod, baseStatus := makeBasePodAndStatus()
	cpu100m := resource.MustParse("100m")
	cpu200m := resource.MustParse("200m")
	mem100M := resource.MustParse("100Mi")
	mem200M := resource.MustParse("200Mi")
	cpuPolicyNoRestart := v1.ResizePolicy{ResourceName: v1.ResourceCPU, Policy: v1.NoRestart}
	memPolicyNoRestart := v1.ResizePolicy{ResourceName: v1.ResourceMemory, Policy: v1.NoRestart}
	cpuPolicyRestart := v1.ResizePolicy{ResourceName: v1.ResourceCPU, Policy: v1.RestartContainer}
	memPolicyRestart := v1.ResizePolicy{ResourceName: v1.ResourceMemory, Policy: v1.RestartContainer}

	for desc, test := range map[string]struct {
		mutatePodAndStatusFn func(*v1.Pod, *kubecontainer.PodStatus)
		actions              podActions
		mutatePodActionsFn   func(*v1.Pod, *podActions)
	}{
		"Update container CPU and memory resources when spec.Resources and status.Resources differ": {
			mutatePodAndStatusFn: func(pod *v1.Pod, podStatus *kubecontainer.PodStatus) {
				pod.Spec.Containers[1].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M},
				}
				pod.Status.ContainerStatuses[0].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem100M},
				}
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{}),
				ContainersToStart: []int{},
				ContainersToUpdate: map[string][]containerToUpdateInfo{
					cpuLimit: {
						{
							apiContainer:        &basePod.Spec.Containers[1],
							apiContainerStatus:  &basePod.Status.ContainerStatuses[0],
							kubeContainerStatus: baseStatus.ContainerStatuses[1],
						},
					},
					memLimit: {
						{
							apiContainer:        &basePod.Spec.Containers[1],
							apiContainerStatus:  &basePod.Status.ContainerStatuses[0],
							kubeContainerStatus: baseStatus.ContainerStatuses[1],
						},
					},
				},
				ContainersToRestart: []int{},
			},
			mutatePodActionsFn: func(pod *v1.Pod, pa *podActions) {
				pa.ContainersToUpdate[cpuLimit][0].apiContainer.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M},
				}
				pa.ContainersToUpdate[cpuLimit][0].apiContainer.ResizePolicy = []v1.ResizePolicy{cpuPolicyNoRestart, memPolicyNoRestart}
				pa.ContainersToUpdate[memLimit][0].apiContainer.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M},
				}
				pa.ContainersToUpdate[memLimit][0].apiContainer.ResizePolicy = []v1.ResizePolicy{cpuPolicyNoRestart, memPolicyNoRestart}
				pa.ContainersToUpdate[cpuLimit][0].apiContainerStatus.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem100M},
				}
				pa.ContainersToUpdate[memLimit][0].apiContainerStatus.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem100M},
				}
				hash := kubecontainer.HashContainer(&pod.Spec.Containers[1])
				pa.ContainersToUpdate[cpuLimit][0].kubeContainerStatus.Hash = hash
				pa.ContainersToUpdate[memLimit][0].kubeContainerStatus.Hash = hash
			},
		},
		"Update container CPU resources when spec.Resources and status.Resources differ in CPU": {
			mutatePodAndStatusFn: func(pod *v1.Pod, podStatus *kubecontainer.PodStatus) {
				pod.Spec.Containers[1].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M},
				}
				pod.Status.ContainerStatuses[0].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem200M},
				}
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{}),
				ContainersToStart: []int{},
				ContainersToUpdate: map[string][]containerToUpdateInfo{
					cpuLimit: {
						{
							apiContainer:        &basePod.Spec.Containers[1],
							apiContainerStatus:  &basePod.Status.ContainerStatuses[0],
							kubeContainerStatus: baseStatus.ContainerStatuses[1],
						},
					},
				},
				ContainersToRestart: []int{},
			},
			mutatePodActionsFn: func(pod *v1.Pod, pa *podActions) {
				pa.ContainersToUpdate[cpuLimit][0].apiContainer.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M},
				}
				pa.ContainersToUpdate[cpuLimit][0].apiContainer.ResizePolicy = []v1.ResizePolicy{cpuPolicyNoRestart, memPolicyNoRestart}
				pa.ContainersToUpdate[cpuLimit][0].apiContainerStatus.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem200M},
				}
				hash := kubecontainer.HashContainer(&pod.Spec.Containers[1])
				pa.ContainersToUpdate[cpuLimit][0].kubeContainerStatus.Hash = hash
			},
		},
		"Update container memory resources when spec.Resources and status.Resources differ in memory": {
			mutatePodAndStatusFn: func(pod *v1.Pod, podStatus *kubecontainer.PodStatus) {
				pod.Spec.Containers[2].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M},
				}
				pod.Status.ContainerStatuses[2].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem100M},
				}
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{}),
				ContainersToStart: []int{},
				ContainersToUpdate: map[string][]containerToUpdateInfo{
					memLimit: {
						{
							apiContainer:        &basePod.Spec.Containers[2],
							apiContainerStatus:  &basePod.Status.ContainerStatuses[2],
							kubeContainerStatus: baseStatus.ContainerStatuses[2],
						},
					},
				},
				ContainersToRestart: []int{},
			},
			mutatePodActionsFn: func(pod *v1.Pod, pa *podActions) {
				pa.ContainersToUpdate[memLimit][0].apiContainer.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M},
				}
				pa.ContainersToUpdate[memLimit][0].apiContainer.ResizePolicy = []v1.ResizePolicy{cpuPolicyNoRestart, memPolicyNoRestart}
				pa.ContainersToUpdate[memLimit][0].apiContainerStatus.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem100M},
				}
				hash := kubecontainer.HashContainer(&pod.Spec.Containers[2])
				pa.ContainersToUpdate[memLimit][0].kubeContainerStatus.Hash = hash
			},
		},
		"Nothing when spec.Resources and status.Resources are equal": {
			mutatePodAndStatusFn: func(pod *v1.Pod, podStatus *kubecontainer.PodStatus) {
				pod.Spec.Containers[1].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m},
				}
				pod.Status.ContainerStatuses[0].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m},
				}
			},
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:    getKillMap(basePod, baseStatus, []int{}),
				ContainersToStart:   []int{},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{},
			},
		},
		"Update container CPU and memory resources with RestartContainer policy for CPU": {
			mutatePodAndStatusFn: func(pod *v1.Pod, podStatus *kubecontainer.PodStatus) {
				pod.Spec.Containers[2].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M},
				}
				pod.Spec.Containers[2].ResizePolicy = []v1.ResizePolicy{cpuPolicyRestart, memPolicyNoRestart}
				pod.Status.ContainerStatuses[2].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem100M},
				}
				podStatus.ContainerStatuses[2].Hash = kubecontainer.HashContainer(&pod.Spec.Containers[2])
			},
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:    getKillMap(basePod, baseStatus, []int{2}),
				ContainersToStart:   []int{},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{2},
			},
			mutatePodActionsFn: func(pod *v1.Pod, pa *podActions) {
				c := pa.ContainersToKill[baseStatus.ContainerStatuses[2].ID].container
				c.Resources = v1.ResourceRequirements{Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M}}
				c.ResizePolicy = []v1.ResizePolicy{cpuPolicyRestart, memPolicyNoRestart}
			},
		},
		"Update container CPU and memory resources with RestartContainer policy for memory": {
			mutatePodAndStatusFn: func(pod *v1.Pod, podStatus *kubecontainer.PodStatus) {
				pod.Spec.Containers[2].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M},
				}
				pod.Spec.Containers[2].ResizePolicy = []v1.ResizePolicy{cpuPolicyNoRestart, memPolicyRestart}
				pod.Status.ContainerStatuses[2].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem100M},
				}
				podStatus.ContainerStatuses[2].Hash = kubecontainer.HashContainer(&pod.Spec.Containers[2])
			},
			actions: podActions{
				SandboxID:           baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:    getKillMap(basePod, baseStatus, []int{2}),
				ContainersToStart:   []int{},
				ContainersToUpdate:  map[string][]containerToUpdateInfo{},
				ContainersToRestart: []int{2},
			},
			mutatePodActionsFn: func(pod *v1.Pod, pa *podActions) {
				c := pa.ContainersToKill[baseStatus.ContainerStatuses[2].ID].container
				c.Resources = v1.ResourceRequirements{Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M}}
				c.ResizePolicy = []v1.ResizePolicy{cpuPolicyNoRestart, memPolicyRestart}
			},
		},
		"Update container memory resources with RestartContainer policy for CPU": {
			mutatePodAndStatusFn: func(pod *v1.Pod, podStatus *kubecontainer.PodStatus) {
				pod.Spec.Containers[2].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem200M},
				}
				pod.Spec.Containers[2].ResizePolicy = []v1.ResizePolicy{cpuPolicyRestart, memPolicyNoRestart}
				pod.Status.ContainerStatuses[2].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem100M},
				}
				podStatus.ContainerStatuses[2].Hash = kubecontainer.HashContainer(&pod.Spec.Containers[2])
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{}),
				ContainersToStart: []int{},
				ContainersToUpdate: map[string][]containerToUpdateInfo{
					memLimit: {
						{
							apiContainer:        &basePod.Spec.Containers[2],
							apiContainerStatus:  &basePod.Status.ContainerStatuses[2],
							kubeContainerStatus: baseStatus.ContainerStatuses[2],
						},
					},
				},
				ContainersToRestart: []int{},
			},
			mutatePodActionsFn: func(pod *v1.Pod, pa *podActions) {
				ci := pa.ContainersToUpdate[memLimit][0]
				ci.apiContainer.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem200M},
				}
				ci.apiContainer.ResizePolicy = []v1.ResizePolicy{cpuPolicyRestart, memPolicyNoRestart}
				ci.apiContainerStatus.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem100M},
				}
				ci.kubeContainerStatus.Hash = kubecontainer.HashContainer(&pod.Spec.Containers[2])
			},
		},
		"Update container CPU resources with RestartContainer policy for memory": {
			mutatePodAndStatusFn: func(pod *v1.Pod, podStatus *kubecontainer.PodStatus) {
				pod.Spec.Containers[2].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem100M},
				}
				pod.Spec.Containers[2].ResizePolicy = []v1.ResizePolicy{cpuPolicyNoRestart, memPolicyRestart}
				pod.Status.ContainerStatuses[2].Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem100M},
				}
				podStatus.ContainerStatuses[2].Hash = kubecontainer.HashContainer(&pod.Spec.Containers[2])
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{}),
				ContainersToStart: []int{},
				ContainersToUpdate: map[string][]containerToUpdateInfo{
					cpuLimit: {
						{
							apiContainer:        &basePod.Spec.Containers[2],
							apiContainerStatus:  &basePod.Status.ContainerStatuses[2],
							kubeContainerStatus: baseStatus.ContainerStatuses[2],
						},
					},
				},
				ContainersToRestart: []int{},
			},
			mutatePodActionsFn: func(pod *v1.Pod, pa *podActions) {
				ci := pa.ContainersToUpdate[cpuLimit][0]
				ci.apiContainer.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem100M},
				}
				ci.apiContainer.ResizePolicy = []v1.ResizePolicy{cpuPolicyNoRestart, memPolicyRestart}
				ci.apiContainerStatus.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem100M},
				}
				ci.kubeContainerStatus.Hash = kubecontainer.HashContainer(&pod.Spec.Containers[2])
			},
		},
		//TODO: lot more tests
	} {
		pod, status := makeBasePodAndStatus()
		for idx := range pod.Spec.Containers {
			// default resize policy when pod resize feature is enabled
			pod.Spec.Containers[idx].ResizePolicy = []v1.ResizePolicy{cpuPolicyNoRestart, memPolicyNoRestart}
			status.ContainerStatuses[idx].Hash = kubecontainer.HashContainer(&pod.Spec.Containers[idx])
		}
		if test.mutatePodAndStatusFn != nil {
			test.mutatePodAndStatusFn(pod, status)
		}
		if test.mutatePodActionsFn != nil {
			test.mutatePodActionsFn(pod, &test.actions)
		}
		actions := m.computePodActions(pod, status)
		verifyActions(t, &test.actions, &actions, desc)
	}
}

func TestUpdateContainerLimits(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.InPlacePodVerticalScaling, true)()
	fakeRuntime, _, m, err := createTestRuntimeManager()
	m.machineInfo.MemoryCapacity = 17179860387 // 16GB
	assert.NoError(t, err)

	cpu100m := resource.MustParse("100m")
	cpu150m := resource.MustParse("150m")
	cpu200m := resource.MustParse("200m")
	cpu250m := resource.MustParse("250m")
	cpu300m := resource.MustParse("300m")
	cpu350m := resource.MustParse("350m")
	mem100M := resource.MustParse("100Mi")
	mem150M := resource.MustParse("150Mi")
	mem200M := resource.MustParse("200Mi")
	mem250M := resource.MustParse("250Mi")
	mem300M := resource.MustParse("300Mi")
	mem350M := resource.MustParse("350Mi")
	res100m100Mi := v1.ResourceList{v1.ResourceCPU: cpu100m, v1.ResourceMemory: mem100M}
	res150m150Mi := v1.ResourceList{v1.ResourceCPU: cpu150m, v1.ResourceMemory: mem150M}
	res200m200Mi := v1.ResourceList{v1.ResourceCPU: cpu200m, v1.ResourceMemory: mem200M}
	res250m250Mi := v1.ResourceList{v1.ResourceCPU: cpu250m, v1.ResourceMemory: mem250M}
	res300m300Mi := v1.ResourceList{v1.ResourceCPU: cpu300m, v1.ResourceMemory: mem300M}
	res350m350Mi := v1.ResourceList{v1.ResourceCPU: cpu350m, v1.ResourceMemory: mem350M}

	pod, kubeStatus := makeBasePodAndStatus()
	makeAndSetFakePod(t, m, fakeRuntime, pod)

	for dsc, tc := range map[string]struct {
		resourceName          string
		apiSpecResources      []v1.ResourceRequirements
		apiStatusResources    []v1.ResourceRequirements
		kubeStatusResources   []v1.ResourceList
		requiresRestart       []bool
		invokeUpdateResources bool
		expectedKubeResources []v1.ResourceList
	}{
		"Guaranteed QoS Pod - CPU & memory resize": {
			resourceName: cpuLimit,
			apiSpecResources: []v1.ResourceRequirements{
				{Limits: res150m150Mi, Requests: res150m150Mi},
				{Limits: res250m250Mi, Requests: res250m250Mi},
				{Limits: res350m350Mi, Requests: res350m350Mi},
			},
			apiStatusResources: []v1.ResourceRequirements{
				{Limits: res200m200Mi, Requests: res200m200Mi},
				{Limits: res100m100Mi, Requests: res100m100Mi},
				{Limits: res300m300Mi, Requests: res300m300Mi},
			},
			kubeStatusResources:   []v1.ResourceList{res100m100Mi, res200m200Mi, res300m300Mi},
			requiresRestart:       []bool{false, false, false},
			invokeUpdateResources: true,
			expectedKubeResources: []v1.ResourceList{res100m100Mi, res200m200Mi, res300m300Mi},
		},
		//TODO: more test
	} {
		var containersToUpdate []containerToUpdateInfo
		for idx := range pod.Spec.Containers {
			// default resize policy when pod resize feature is enabled
			pod.Spec.Containers[idx].Resources = tc.apiSpecResources[idx]
			pod.Status.ContainerStatuses[idx].Resources = tc.apiStatusResources[idx]
			kubeStatus.ContainerStatuses[idx].Resources.Limits = tc.kubeStatusResources[idx]
			kubeStatus.ContainerStatuses[idx].Hash = kubecontainer.HashContainer(&pod.Spec.Containers[idx])
			cInfo := containerToUpdateInfo{
				apiContainer:        &pod.Spec.Containers[idx],
				apiContainerStatus:  &pod.Status.ContainerStatuses[idx],
				kubeContainerStatus: kubeStatus.ContainerStatuses[idx],
			}
			containersToUpdate = append(containersToUpdate, cInfo)

		}
		fakeRuntime.Called = []string{}
		err := m.updatePodContainerResources(pod, kubeStatus, tc.resourceName, containersToUpdate)
		assert.NoError(t, err, dsc)

		if tc.invokeUpdateResources {
			assert.Contains(t, fakeRuntime.Called, "UpdateContainerResources", dsc)
		}
		for idx := range pod.Spec.Containers {
			assert.Equal(t, tc.expectedKubeResources[idx], kubeStatus.ContainerStatuses[idx].Resources.Limits, dsc)
		}
	}
}

/*func TestSyncPodForResize(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.InPlacePodVerticalScaling, true)()
	fakeRuntime, fakeImage, m, err := createTestRuntimeManager()
	m.machineInfo.MemoryCapacity = 17179860387 // 16GB
	assert.NoError(t, err)

	resources1 := v1.ResourceRequirements{
				Limits: v1.ResourceList{v1.ResourceCPU: resource.MustParse("100m"), v1.ResourceMemory: resource.MustParse("100Mi")},
			}
	resources1a := v1.ResourceRequirements{
				Limits: v1.ResourceList{v1.ResourceCPU: resource.MustParse("150m"), v1.ResourceMemory: resource.MustParse("150Mi")},
			}
	resources2 := v1.ResourceRequirements{
				Limits: v1.ResourceList{v1.ResourceCPU: resource.MustParse("200m"), v1.ResourceMemory: resource.MustParse("200Mi")},
			}
	resources3 := v1.ResourceRequirements{
				Limits: v1.ResourceList{v1.ResourceCPU: resource.MustParse("300m"), v1.ResourceMemory: resource.MustParse("300Mi")},
			}

	containers := []v1.Container{
		{
			Name:            "foo1",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
			Resources:       resources1,
		},
		{
			Name:            "foo2",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
			Resources:       resources2,
		},
		{
			Name:            "foo3",
			Image:           "alpine",
			ImagePullPolicy: v1.PullIfNotPresent,
			Resources:       resources3,
		},
	}
	containerStatuses := []v1.ContainerStatus{
		{
			Name:            "foo1",
			Image:           "busybox",
			Resources:       resources1,
		},
		{
			Name:            "foo3",
			Image:           "alpine",
			Resources:       resources3,
		},
		{
			Name:            "foo2",
			Image:           "busybox",
			Resources:       resources2,
		},
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: containers,
		},
		Status: v1.PodStatus{
			ContainerStatuses: containerStatuses,
		},
	}
	podStatus := &kubecontainer.PodStatus{
		ID:        pod.UID,
		Name:      pod.Name,
		Namespace: pod.Namespace,
		SandboxStatuses: []*runtimeapi.PodSandboxStatus{
			{
				Id:       "sandboxID",
				State:    runtimeapi.PodSandboxState_SANDBOX_READY,
				Metadata: &runtimeapi.PodSandboxMetadata{Name: pod.Name, Namespace: pod.Namespace, Uid: "sandboxuid", Attempt: uint32(0)},
				Network:  &runtimeapi.PodSandboxNetworkStatus{Ip: "10.0.0.1"},
			},
		},
		ContainerStatuses: []*kubecontainer.ContainerStatus{
			{
				ID:   kubecontainer.ContainerID{ID: "id3"},
				Name: "foo3", State: kubecontainer.ContainerStateRunning,
				Hash: kubecontainer.HashContainer(&pod.Spec.Containers[2]),
			},
			{
				ID:   kubecontainer.ContainerID{ID: "id1"},
				Name: "foo1", State: kubecontainer.ContainerStateRunning,
				Hash: kubecontainer.HashContainer(&pod.Spec.Containers[0]),
			},
			{
				ID:   kubecontainer.ContainerID{ID: "id2"},
				Name: "foo2", State: kubecontainer.ContainerStateRunning,
				Hash: kubecontainer.HashContainer(&pod.Spec.Containers[1]),
			},
		},
	}

	backOff := flowcontrol.NewBackOff(time.Second, time.Minute)
	result := m.SyncPod(pod, podStatus, []v1.Secret{}, backOff)
	assert.NoError(t, result.Error())
	assert.Equal(t, 3, len(fakeRuntime.Containers))
	assert.Equal(t, 2, len(fakeImage.Images))
	assert.Equal(t, 1, len(fakeRuntime.Sandboxes))
	for _, sandbox := range fakeRuntime.Sandboxes {
		assert.Equal(t, runtimeapi.PodSandboxState_SANDBOX_READY, sandbox.State)
	}
	for _, c := range fakeRuntime.Containers {
		assert.Equal(t, runtimeapi.ContainerState_CONTAINER_RUNNING, c.State)
	}

	for desc, test := range map[string]struct {
		mutatePodFn       func(*v1.Pod)
		expectedResources []v1.ResourceRequirements
	}{
		"Update cpu and memory limits for a single container": {
			mutatePodFn: func(pod *v1.Pod) {
				pod.Spec.Containers[0].Resources = resources1a
			},
			expectedResources: []v1.ResourceRequirements{
				resources1a,
				resources3,
				resources2,
			},
		},
	} {
		if test.mutatePodFn != nil {
			test.mutatePodFn(pod)
		}
		result := m.SyncPod(pod, podStatus, []v1.Secret{}, backOff)
		assert.NoError(t, result.Error())
		for idx := 0; idx < len(pod.Status.ContainerStatuses); idx++ {
			assert.Equal(t, test.expectedResources[idx], pod.Status.ContainerStatuses[idx].Resources, desc)
		}
	}
}*/
