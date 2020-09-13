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
	"errors"
	"fmt"
	"k8s.io/kubernetes/pkg/kubelet/runtimeregistry"
	"os"
	"strings"
	"sync"
	"time"

	cadvisorapi "github.com/google/cadvisor/info/v1"
	"go.uber.org/multierr"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/tools/record"
	ref "k8s.io/client-go/tools/reference"
	"k8s.io/client-go/util/flowcontrol"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/kubelet/cm"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/events"
	"k8s.io/kubernetes/pkg/kubelet/images"
	"k8s.io/kubernetes/pkg/kubelet/lifecycle"
	proberesults "k8s.io/kubernetes/pkg/kubelet/prober/results"
	"k8s.io/kubernetes/pkg/kubelet/runtimeclass"
	"k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/kubelet/util/format"
	"k8s.io/kubernetes/pkg/kubelet/util/logreduction"
	"k8s.io/kubernetes/pkg/kubelet/util/podConverter"
)

const (
	// The api version of kubelet runtime api
	kubeRuntimeAPIVersion = "0.1.0"
	// The root directory for pod logs
	podLogsRootDirectory = "/var/log/pods"
	// A minimal shutdown window for avoiding unnecessary SIGKILLs
	minimumGracePeriodInSeconds = 2

	// The expiration time of version cache.
	versionCacheTTL = 60 * time.Second
	// How frequently to report identical errors
	identicalErrorDelay = 1 * time.Minute

	// Resources for which in-place pod vertical scaling is applicable
	memLimit   = "memory-limit"
	cpuLimit   = "cpu-limit"
	cpuRequest = "cpu-request"
)

var (
	// ErrContainerDoesNotExist at runtime service
	ErrContainerDoesNotExistAtRuntimeService = errors.New("Container does not exist at runtime")
	// ErrVersionNotSupported is returned when the api version of runtime interface is not supported
	ErrVersionNotSupported            = errors.New("Runtime api version is not supported")
	defaultRuntimeServiceName         string
	reservedDefaultRuntimeServiceName = "default"
)

// podStateProvider can determine if a pod is deleted ir terminated
type podStateProvider interface {
	IsPodDeleted(kubetypes.UID) bool
	IsPodTerminated(kubetypes.UID) bool
}

type kubeGenericRuntimeManager struct {
	runtimeName         string
	recorder            record.EventRecorder
	osInterface         kubecontainer.OSInterface
	containerRefManager *kubecontainer.RefManager

	// machineInfo contains the machine information.
	machineInfo *cadvisorapi.MachineInfo

	// podStateProvider for getting pod state
	podStateProvider podStateProvider

	// Container GC manager
	containerGC *containerGC

	// Keyring for pulling images
	keyring credentialprovider.DockerKeyring

	// Runner of lifecycle events.
	runner kubecontainer.HandlerRunner

	// RuntimeHelper that wraps kubelet to generate runtime container options.
	runtimeHelper kubecontainer.RuntimeHelper

	// Health check results.
	livenessManager proberesults.Manager

	// If true, enforce container cpu limits with CFS quota support
	cpuCFSQuota bool

	// CPUCFSQuotaPeriod sets the CPU CFS quota period value, cpu.cfs_period_us, defaults to 100ms
	cpuCFSQuotaPeriod metav1.Duration

	// Pod-runtimeService maps
	// Runtime manager maintains the pod-runtime map to optimize the performance in Getters
	// key: podUid string
	// value: runtime service API
	podRuntimeServiceMap   map[string]internalapi.RuntimeService
	podRuntimeServiceMapOp sync.RWMutex

	// The directory path for seccomp profiles.
	seccompProfileRoot string

	// Container management interface for pod container.
	containerManager cm.ContainerManager

	// Internal lifecycle event handlers for container resource management.
	internalLifecycle cm.InternalContainerLifecycle

	// A shim to legacy functions for backward compatibility.
	legacyLogProvider LegacyLogProvider

	// Manage RuntimeClass resources.
	runtimeClassManager *runtimeclass.Manager

	// Cache last per-container error message to reduce log spam
	logReduction *logreduction.LogReduction

	// testing usage only for now. it can be used as an indicator of any runtime status errors detected by the runtime manager
	//
	runtimeStatusErr error

	// runtime registry
	runtimeRegistry runtimeregistry.Interface
}

// KubeGenericRuntime is a interface contains interfaces for container runtime and command.
type KubeGenericRuntime interface {
	kubecontainer.Runtime
	kubecontainer.StreamingRuntime
	kubecontainer.ContainerCommandRunner
}

// LegacyLogProvider gives the ability to use unsupported docker log drivers (e.g. journald)
type LegacyLogProvider interface {
	// Get the last few lines of the logs for a specific container.
	GetContainerLogTail(uid kubetypes.UID, name, namespace, tenant string, containerID kubecontainer.ContainerID) (string, error)
}

// NewKubeGenericRuntimeManager creates a new kubeGenericRuntimeManager
func NewKubeGenericRuntimeManager(
	recorder record.EventRecorder,
	livenessManager proberesults.Manager,
	seccompProfileRoot string,
	containerRefManager *kubecontainer.RefManager,
	machineInfo *cadvisorapi.MachineInfo,
	_podStateProvider podStateProvider,
	osInterface kubecontainer.OSInterface,
	runtimeHelper kubecontainer.RuntimeHelper,
	httpClient types.HttpGetter,
	imageBackOff *flowcontrol.Backoff,
	serializeImagePulls bool,
	imagePullQPS float32,
	imagePullBurst int,
	cpuCFSQuota bool,
	cpuCFSQuotaPeriod metav1.Duration,
	containerManager cm.ContainerManager,
	legacyLogProvider LegacyLogProvider,
	runtimeClassManager *runtimeclass.Manager,
	runtimeRegistry runtimeregistry.Interface,
) (KubeGenericRuntime, error) {
	kubeRuntimeManager := &kubeGenericRuntimeManager{
		recorder:            recorder,
		cpuCFSQuota:         cpuCFSQuota,
		cpuCFSQuotaPeriod:   cpuCFSQuotaPeriod,
		seccompProfileRoot:  seccompProfileRoot,
		livenessManager:     livenessManager,
		containerRefManager: containerRefManager,
		machineInfo:         machineInfo,
		osInterface:         osInterface,
		runtimeHelper:       runtimeHelper,
		keyring:             credentialprovider.NewDockerKeyring(),
		containerManager:    containerManager,
		internalLifecycle:   containerManager.InternalContainerLifecycle(),
		legacyLogProvider:   legacyLogProvider,
		runtimeClassManager: runtimeClassManager,
		logReduction:        logreduction.NewLogReduction(identicalErrorDelay),
	}

	// TODO: retrieve runtimeName dynamically per the runtime being used for a POD
	kubeRuntimeManager.runtimeName = "unknown"
	kubeRuntimeManager.runtimeRegistry = runtimeRegistry

	// If the container logs directory does not exist, create it.
	// TODO: create podLogsRootDirectory at kubelet.go when kubelet is refactored to new runtime interface
	if _, err := osInterface.Stat(podLogsRootDirectory); os.IsNotExist(err) {
		if err := osInterface.MkdirAll(podLogsRootDirectory, 0755); err != nil {
			klog.Errorf("Failed to create directory %q: %v", podLogsRootDirectory, err)
		}
	}

	// late-bind the life cycle handler
	kubeRuntimeManager.runner = lifecycle.NewHandlerRunner(httpClient, kubeRuntimeManager, kubeRuntimeManager)
	kubeRuntimeManager.containerGC = newContainerGC(nil, _podStateProvider, kubeRuntimeManager)

	kubeRuntimeManager.podStateProvider = _podStateProvider
	kubeRuntimeManager.podRuntimeServiceMap = make(map[string]internalapi.RuntimeService)

	return kubeRuntimeManager, nil
}

// Construct a imageManager instance for the kubeRuntimeManager
func (m *kubeGenericRuntimeManager) GetDesiredImagePuller(pod *v1.Pod) (images.ImageManager, error) {

	imageBackOff := flowcontrol.NewBackOff(10*time.Second, 300*time.Second)

	// TODO: get the hardcoded parameters to the ImageManager constructor from config
	return images.NewImageManager(
		kubecontainer.FilterEventRecorder(m.recorder),
		m,
		imageBackOff,
		true,
		0.0,
		100), nil

}

// Type returns the type of the container runtime of a given runtime
func (m *kubeGenericRuntimeManager) RuntimeType(service internalapi.RuntimeService) string {
	typedVersion, err := m.GetTypedVersion(service)
	if err != nil {
		return runtimeregistry.UnknownType
	}

	return typedVersion.RuntimeName
}

func newRuntimeVersion(version string) (*utilversion.Version, error) {
	if ver, err := utilversion.ParseSemantic(version); err == nil {
		return ver, err
	}
	return utilversion.ParseGeneric(version)
}

// Get TypedVersion for a given runtime
func (m *kubeGenericRuntimeManager) GetTypedVersion(service internalapi.RuntimeService) (*runtimeapi.VersionResponse, error) {
	typedVersion, err := service.Version(kubeRuntimeAPIVersion)
	if err != nil {
		klog.Errorf("Get remote runtime typed version failed: %v", err)
		return nil, err
	}

	if typedVersion.Version != kubeRuntimeAPIVersion {
		klog.Errorf("Runtime api version %s is not supported, only %s is supported now",
			typedVersion.Version,
			kubeRuntimeAPIVersion)
		return nil, ErrVersionNotSupported
	}

	return typedVersion, nil
}

// Version returns the version information of the container runtime
// Version is used for node info.
func (m *kubeGenericRuntimeManager) RuntimeVersion(service internalapi.RuntimeService) (kubecontainer.Version, error) {
	typedVersion, err := service.Version(kubeRuntimeAPIVersion)
	if err != nil {
		klog.Errorf("Get remote runtime version failed: %v", err)
		return nil, err
	}

	return newRuntimeVersion(typedVersion.RuntimeVersion)
}

// APIVersion returns the cached API version information of the container runtime.
func (m *kubeGenericRuntimeManager) RuntimeAPIVersion(service internalapi.RuntimeService) (kubecontainer.Version, error) {
	typedVersion, err := m.GetTypedVersion(service)
	if err != nil {
		return nil, err
	}

	return newRuntimeVersion(typedVersion.RuntimeApiVersion)
}

func (m *kubeGenericRuntimeManager) RuntimeStatus(runtimeService internalapi.RuntimeService) (*kubecontainer.RuntimeStatus, error) {
	status, err := runtimeService.Status()
	if err != nil {
		return nil, err
	}

	return toKubeRuntimeStatus(status), nil
}

// Existing Container runtime interface, returns the type of the default runtime
func (m *kubeGenericRuntimeManager) Type() string {
	if runtime, err := m.runtimeRegistry.GetPrimaryRuntimeService(); err == nil {
		return m.RuntimeType(runtime.ServiceApi)
	}
	return "unknownType"
}

// Existing container runtime interface method, return the Version of the default runtime service
func (m *kubeGenericRuntimeManager) Version() (kubecontainer.Version, error) {
	runtime, err := m.runtimeRegistry.GetPrimaryRuntimeService()

	if err == nil {
		return m.RuntimeVersion(runtime.ServiceApi)
	}

	return nil, err
}

// Existing container runtime interface method, return the ApiVersion of the default runtime service
func (m *kubeGenericRuntimeManager) APIVersion() (kubecontainer.Version, error) {
	runtime, err := m.runtimeRegistry.GetPrimaryRuntimeService()

	if err == nil {
		return m.RuntimeAPIVersion(runtime.ServiceApi)
	}

	return nil, err
}

// Existing container runtime interface method, return the status of the default runtime service
func (m *kubeGenericRuntimeManager) Status() (*kubecontainer.RuntimeStatus, error) {
	runtime, err := m.runtimeRegistry.GetPrimaryRuntimeService()

	if err == nil {
		return m.RuntimeStatus(runtime.ServiceApi)
	}

	return nil, err

}

// operations on the pod-runtimeServiceName map
func (m *kubeGenericRuntimeManager) addPodRuntimeService(podId string, runtimeService internalapi.RuntimeService) error {
	m.podRuntimeServiceMapOp.Lock()
	defer m.podRuntimeServiceMapOp.Unlock()
	//just overwrite if entry exists
	m.podRuntimeServiceMap[podId] = runtimeService

	return nil
}
func (m *kubeGenericRuntimeManager) removePodRuntimeService(podId string) error {
	m.podRuntimeServiceMapOp.Lock()
	defer m.podRuntimeServiceMapOp.Unlock()
	if _, found := m.podRuntimeServiceMap[podId]; found {
		delete(m.podRuntimeServiceMap, podId)
	}

	return nil
}
func (m *kubeGenericRuntimeManager) getPodRuntimeService(podId string) internalapi.RuntimeService {
	m.podRuntimeServiceMapOp.RLock()
	defer m.podRuntimeServiceMapOp.RUnlock()
	if runtimeService, found := m.podRuntimeServiceMap[podId]; found {
		return runtimeService
	}
	return nil
}

// GetPods returns a list of containers grouped by pods. The boolean parameter
// specifies whether the runtime returns all containers including those already
// exited and dead containers (used for garbage collection).
func (m *kubeGenericRuntimeManager) GetPods(all bool) ([]*kubecontainer.Pod, error) {
	pods := make(map[kubetypes.UID]*kubecontainer.Pod)
	sandboxes, err := m.getKubeletSandboxes(all)
	if err != nil {
		return nil, err
	}
	for i := range sandboxes {
		s := sandboxes[i]
		if s.Metadata == nil {
			klog.V(4).Infof("Sandbox does not have metadata: %+v", s)
			continue
		}
		podUID := kubetypes.UID(s.Metadata.Uid)
		if _, ok := pods[podUID]; !ok {
			pods[podUID] = &kubecontainer.Pod{
				ID:        podUID,
				Name:      s.Metadata.Name,
				Namespace: s.Metadata.Namespace,
				Tenant:    s.Metadata.Tenant,
			}
		}
		p := pods[podUID]
		converted, err := m.sandboxToKubeContainer(s)
		if err != nil {
			klog.V(4).Infof("Convert %q sandbox %v of pod %q failed: %v", m.runtimeName, s, podUID, err)
			continue
		}
		p.Sandboxes = append(p.Sandboxes, converted)
	}

	containers, err := m.getKubeletContainers(all)
	if err != nil {
		return nil, err
	}
	for i := range containers {
		c := containers[i]
		if c.Metadata == nil {
			klog.V(4).Infof("Container does not have metadata: %+v", c)
			continue
		}

		labelledInfo := getContainerInfoFromLabels(c.Labels)
		pod, found := pods[labelledInfo.PodUID]
		if !found {
			pod = &kubecontainer.Pod{
				ID:        labelledInfo.PodUID,
				Name:      labelledInfo.PodName,
				Namespace: labelledInfo.PodNamespace,
				Tenant:    labelledInfo.PodTenant,
			}
			pods[labelledInfo.PodUID] = pod
		}

		converted, err := m.toKubeContainer(c)
		if err != nil {
			klog.V(4).Infof("Convert %s container %v of pod %q failed: %v", m.runtimeName, c, labelledInfo.PodUID, err)
			continue
		}

		pod.Containers = append(pod.Containers, converted)
	}

	// Convert map to list.
	var result []*kubecontainer.Pod
	for _, pod := range pods {
		result = append(result, pod)
	}

	return result, nil
}

// containerToUpdateInfo contains necessary information to update a container's resources.
type containerToUpdateInfo struct {
	// The specific container in pod.Spec.Containers.
	apiContainer *v1.Container
	// Status of the container in pod.Spec.ContainerStatuses.
	apiContainerStatus *v1.ContainerStatus
	// Status of the runtime container
	kubeContainerStatus *kubecontainer.ContainerStatus
}

// containerToKillInfo contains necessary information to kill a container.
type containerToKillInfo struct {
	// The spec of the container.
	container *v1.Container
	// The name of the container.
	name string
	// The message indicates why the container will be killed.
	message string
}

// ConfigChanges contains dynamic changes detected while pod is alive
type ConfigChanges struct {
	// Device hotplug requests to cope with
	NICsToAttach []string
	NICsToDetach []string
}

// podActions keeps information what to do for a pod.
type podActions struct {
	// Stop all running (regular and init) containers and the sandbox for the pod.
	KillPod bool
	// Whether need to create a new sandbox. If needed to kill pod and create
	// a new pod sandbox, all init containers need to be purged (i.e., removed).
	CreateSandbox bool
	// The id of existing sandbox. It is used for starting containers in ContainersToStart.
	SandboxID string
	// The attempt number of creating sandboxes for the pod.
	Attempt uint32

	// The next init container to start.
	NextInitContainerToStart *v1.Container
	// ContainersToStart keeps a list of indexes for the containers to start,
	// where the index is the index of the specific container in the pod spec (
	// pod.Spec.Containers.
	ContainersToStart []int

	// ContainersToKill keeps a map of containers that need to be killed, note that
	// the key is the container ID of the container, while
	// the value contains necessary information to kill a container.
	ContainersToKill map[kubecontainer.ContainerID]containerToKillInfo

	// ContainersToUpdate keeps a list of containers needing resource limit update.
	// Container resource limit update is applicable only for CPU and memory.
	ContainersToUpdate map[string][]containerToUpdateInfo

	// ContainersToRestart is a list of containers needing resource limit update via restart.
	// Container resource limit update is applicable only for CPU and memory.
	ContainersToRestart []int

	// Hotplugs keeps various device hotplug data
	Hotplugs ConfigChanges
}

// podSandboxChanged checks whether the spec of the pod is changed and returns
// (changed, new attempt, original sandboxID if exist).
func (m *kubeGenericRuntimeManager) podSandboxChanged(pod *v1.Pod, podStatus *kubecontainer.PodStatus) (bool, uint32, string) {
	if len(podStatus.SandboxStatuses) == 0 {
		klog.V(2).Infof("No sandbox for pod %q can be found. Need to start a new one", format.Pod(pod))
		return true, 0, ""
	}

	readySandboxCount := 0
	for _, s := range podStatus.SandboxStatuses {
		if s.State == runtimeapi.PodSandboxState_SANDBOX_READY {
			readySandboxCount++
		}
	}

	// Needs to create a new sandbox when readySandboxCount > 1 or the ready sandbox is not the latest one.
	sandboxStatus := podStatus.SandboxStatuses[0]
	if readySandboxCount > 1 {
		klog.V(2).Infof("More than 1 sandboxes for pod %q are ready. Need to reconcile them", format.Pod(pod))
		return true, sandboxStatus.Metadata.Attempt + 1, sandboxStatus.Id
	}
	if sandboxStatus.State != runtimeapi.PodSandboxState_SANDBOX_READY {
		klog.V(2).Infof("No ready sandbox for pod %q can be found. Need to start a new one", format.Pod(pod))
		return true, sandboxStatus.Metadata.Attempt + 1, sandboxStatus.Id
	}

	// Needs to create a new sandbox when network namespace changed.
	if sandboxStatus.GetLinux().GetNamespaces().GetOptions().GetNetwork() != networkNamespaceForPod(pod) {
		klog.V(2).Infof("Sandbox for pod %q has changed. Need to start a new one", format.Pod(pod))
		return true, sandboxStatus.Metadata.Attempt + 1, ""
	}

	// Needs to create a new sandbox when the sandbox does not have an IP address.
	if !kubecontainer.IsHostNetworkPod(pod) && sandboxStatus.Network.Ip == "" {
		klog.V(2).Infof("Sandbox for pod %q has no IP address.  Need to start a new one", format.Pod(pod))
		return true, sandboxStatus.Metadata.Attempt + 1, sandboxStatus.Id
	}

	return false, sandboxStatus.Metadata.Attempt, sandboxStatus.Id
}

func containerChanged(container *v1.Container, containerStatus *kubecontainer.ContainerStatus) (uint64, uint64, bool) {
	expectedHash := kubecontainer.HashContainer(container)
	return expectedHash, containerStatus.Hash, containerStatus.Hash != expectedHash
}

func shouldRestartOnFailure(pod *v1.Pod) bool {
	return pod.Spec.RestartPolicy != v1.RestartPolicyNever
}

func containerSucceeded(c *v1.Container, podStatus *kubecontainer.PodStatus) bool {
	cStatus := podStatus.FindContainerStatusByName(c.Name)
	if cStatus == nil || cStatus.State == kubecontainer.ContainerStateRunning {
		return false
	}
	return cStatus.ExitCode == 0
}

// computePodActions checks whether the pod spec has changed and returns the changes if true.
func (m *kubeGenericRuntimeManager) computePodActions(pod *v1.Pod, podStatus *kubecontainer.PodStatus) podActions {
	klog.V(5).Infof("Syncing Pod %q: %+v", format.Pod(pod), pod)
	klog.V(5).Infof("podstatus %v", podStatus)
	if podStatus.SandboxStatuses != nil {
		klog.V(5).Infof("pod sandbox length %v", len(podStatus.SandboxStatuses))
		for _, sb := range podStatus.SandboxStatuses {
			klog.V(5).Infof("pod sandbox status %v", sb)
		}
	}

	createPodSandbox, attempt, sandboxID := m.podSandboxChanged(pod, podStatus)
	changes := podActions{
		KillPod:             createPodSandbox,
		CreateSandbox:       createPodSandbox,
		SandboxID:           sandboxID,
		Attempt:             attempt,
		ContainersToStart:   []int{},
		ContainersToKill:    make(map[kubecontainer.ContainerID]containerToKillInfo),
		ContainersToUpdate:  make(map[string][]containerToUpdateInfo),
		ContainersToRestart: []int{},
	}

	// If we need to (re-)create the pod sandbox, everything will need to be
	// killed and recreated, and init containers should be purged.
	if createPodSandbox {
		if !shouldRestartOnFailure(pod) && attempt != 0 && len(podStatus.ContainerStatuses) != 0 {
			// Should not restart the pod, just return.
			// we should not create a sandbox for a pod if it is already done.
			// if all containers are done and should not be started, there is no need to create a new sandbox.
			// this stops confusing logs on pods whose containers all have exit codes, but we recreate a sandbox before terminating it.
			//
			// If ContainerStatuses is empty, we assume that we've never
			// successfully created any containers. In this case, we should
			// retry creating the sandbox.
			changes.CreateSandbox = false
			return changes
		}
		if len(pod.Spec.InitContainers) != 0 {
			// Pod has init containers, return the first one.
			changes.NextInitContainerToStart = &pod.Spec.InitContainers[0]
			return changes
		}
		// Start all containers by default but exclude the ones that succeeded if
		// RestartPolicy is OnFailure.
		for idx, c := range pod.Spec.Containers {
			if containerSucceeded(&c, podStatus) && pod.Spec.RestartPolicy == v1.RestartPolicyOnFailure {
				continue
			}
			changes.ContainersToStart = append(changes.ContainersToStart, idx)
		}
		return changes
	}

	// Check initialization progress.
	initLastStatus, next, done := findNextInitContainerToRun(pod, podStatus)
	if !done {
		if next != nil {
			initFailed := initLastStatus != nil && isInitContainerFailed(initLastStatus)
			if initFailed && !shouldRestartOnFailure(pod) {
				changes.KillPod = true
			} else {
				// Always try to stop containers in unknown state first.
				if initLastStatus != nil && initLastStatus.State == kubecontainer.ContainerStateUnknown {
					changes.ContainersToKill[initLastStatus.ID] = containerToKillInfo{
						name:      next.Name,
						container: next,
						message: fmt.Sprintf("Init container is in %q state, try killing it before restart",
							initLastStatus.State),
					}
				}
				changes.NextInitContainerToStart = next
			}
		}
		// Initialization failed or still in progress. Skip inspecting non-init
		// containers.
		return changes
	}

	// Number of running containers to keep.
	keepCount := 0

	// check the status of containers.
	for idx, container := range pod.Spec.Containers {
		containerStatus := podStatus.FindContainerStatusByName(container.Name)

		// Call internal container post-stop lifecycle hook for any non-running container so that any
		// allocated cpus are released immediately. If the container is restarted, cpus will be re-allocated
		// to it.
		if containerStatus != nil && containerStatus.State != kubecontainer.ContainerStateRunning {
			if err := m.internalLifecycle.PostStopContainer(containerStatus.ID.ID); err != nil {
				klog.Errorf("internal container post-stop lifecycle hook failed for container %v in pod %v with error %v",
					container.Name, pod.Name, err)
			}
		}

		// If container does not exist, or is not running, check whether we
		// need to restart it.
		if containerStatus == nil || containerStatus.State != kubecontainer.ContainerStateRunning {
			if kubecontainer.ShouldContainerBeRestarted(&container, pod, podStatus) {
				message := fmt.Sprintf("Container %+v is dead, but RestartPolicy says that we should restart it.", container)
				klog.V(3).Infof(message)
				changes.ContainersToStart = append(changes.ContainersToStart, idx)
				if containerStatus != nil && containerStatus.State == kubecontainer.ContainerStateUnknown {
					// If container is in unknown state, we don't know whether it
					// is actually running or not, always try killing it before
					// restart to avoid having 2 running instances of the same container.
					changes.ContainersToKill[containerStatus.ID] = containerToKillInfo{
						name:      containerStatus.Name,
						container: &pod.Spec.Containers[idx],
						message:   fmt.Sprintf("Container is in %q state, try killing it before restart", containerStatus.State),
					}
				}
			}
			continue
		}
		// The container is running, but kill the container if any of the following condition is met.
		var message string
		restart := shouldRestartOnFailure(pod)
		if _, _, changed := containerChanged(&container, containerStatus); changed {
			message = fmt.Sprintf("Container %s definition changed", container.Name)
			// Restart regardless of the restart policy because the container
			// spec changed.
			restart = true
		} else if liveness, found := m.livenessManager.Get(containerStatus.ID); found && liveness == proberesults.Failure {
			// If the container failed the liveness probe, we should kill it.
			message = fmt.Sprintf("Container %s failed liveness probe", container.Name)
		} else {
			if utilfeature.DefaultFeatureGate.Enabled(features.InPlacePodVerticalScaling) {
				keepCount++
				apiContainerStatuses := pod.Status.ContainerStatuses
				if pod.Spec.VirtualMachine != nil && pod.Status.VirtualMachineStatus != nil {
					var vmContainerState v1.ContainerState
					if pod.Status.VirtualMachineStatus.State == v1.VmActive {
						vmContainerState = v1.ContainerState{Running: &v1.ContainerStateRunning{StartedAt: *pod.Status.StartTime}}
					}
					vmContainerId := kubecontainer.BuildContainerID(containerStatus.ID.Type, pod.Status.VirtualMachineStatus.VirtualMachineId)
					apiContainerStatuses = []v1.ContainerStatus{
						{
							Name:         pod.Status.VirtualMachineStatus.Name,
							ContainerID:  vmContainerId.String(),
							State:        vmContainerState,
							Ready:        pod.Status.VirtualMachineStatus.Ready,
							RestartCount: pod.Status.VirtualMachineStatus.RestartCount,
							Image:        pod.Status.VirtualMachineStatus.Image,
							ImageID:      pod.Status.VirtualMachineStatus.ImageId,
							Resources:    pod.Status.VirtualMachineStatus.Resources,
						},
					}
				}
				if container.Resources.Limits == nil || len(apiContainerStatuses) == 0 {
					continue
				}
				apiContainerStatus, exists := podutil.GetContainerStatus(apiContainerStatuses, container.Name)
				if !exists || apiContainerStatus.State.Running == nil ||
					containerStatus.State != kubecontainer.ContainerStateRunning ||
					containerStatus.ID.String() != apiContainerStatus.ContainerID ||
					len(diff.ObjectDiff(container.Resources.Requests, container.ResourcesAllocated)) != 0 ||
					len(diff.ObjectDiff(apiContainerStatus.Resources, container.Resources)) == 0 {
					continue
				}
				// If runtime status resources is available from CRI or previous update, compare with it.
				if len(diff.ObjectDiff(containerStatus.Resources, container.Resources)) == 0 {
					continue
				}
				resizePolicy := make(map[v1.ResourceName]v1.ContainerResizePolicy)
				for _, pol := range container.ResizePolicy {
					resizePolicy[pol.ResourceName] = pol.Policy
				}
				determineContainerResize := func(rName v1.ResourceName, specValue, statusValue int64) (bool, bool) {
					if specValue == statusValue {
						return false, false
					}
					if resizePolicy[rName] == v1.RestartContainer {
						return true, true
					}
					return true, false
				}
				markContainerForUpdate := func(rName string, specValue, statusValue int64) {
					cUpdateInfo := containerToUpdateInfo{
						apiContainer:        &pod.Spec.Containers[idx],
						apiContainerStatus:  &apiContainerStatus,
						kubeContainerStatus: containerStatus,
					}
					// Container updates are ordered so that resource decreases are applied before increases
					switch {
					case specValue > statusValue: // append
						changes.ContainersToUpdate[rName] = append(changes.ContainersToUpdate[rName], cUpdateInfo)
					case specValue < statusValue: // prepend
						changes.ContainersToUpdate[rName] = append(changes.ContainersToUpdate[rName], containerToUpdateInfo{})
						copy(changes.ContainersToUpdate[rName][1:], changes.ContainersToUpdate[rName])
						changes.ContainersToUpdate[rName][0] = cUpdateInfo
					}
				}
				specLim := container.Resources.Limits
				specReq := container.Resources.Requests
				statusLim := apiContainerStatus.Resources.Limits
				statusReq := apiContainerStatus.Resources.Requests
				// Runtime container status resources, if set, takes precedence.
				if containerStatus.Resources.Limits != nil {
					statusLim = containerStatus.Resources.Limits
				}
				if containerStatus.Resources.Requests != nil {
					statusReq = containerStatus.Resources.Requests
				}
				resizeMemLim, restartMemLim := determineContainerResize(v1.ResourceMemory, specLim.Memory().Value(), statusLim.Memory().Value())
				resizeCPUReq, restartCPUReq := determineContainerResize(v1.ResourceCPU, specReq.Cpu().MilliValue(), statusReq.Cpu().MilliValue())
				resizeCPULim, restartCPULim := determineContainerResize(v1.ResourceCPU, specLim.Cpu().MilliValue(), statusLim.Cpu().MilliValue())
				if restartMemLim || restartCPULim || restartCPUReq {
					// resize policy requires this container to restart
					changes.ContainersToKill[containerStatus.ID] = containerToKillInfo{
						name:      containerStatus.Name,
						container: &pod.Spec.Containers[idx],
						message:   fmt.Sprintf("Container %s resize requires restart", container.Name),
					}
					changes.ContainersToRestart = append(changes.ContainersToRestart, idx)
					keepCount--
				} else {
					if resizeMemLim {
						markContainerForUpdate(memLimit, specLim.Memory().Value(), statusLim.Memory().Value())
					}
					if resizeCPUReq {
						markContainerForUpdate(cpuRequest, specReq.Cpu().MilliValue(), statusReq.Cpu().MilliValue())
					}
					if resizeCPULim {
						markContainerForUpdate(cpuLimit, specLim.Cpu().MilliValue(), statusLim.Cpu().MilliValue())
					}
				}
				continue
			}
			// Keep the container.
			keepCount++
			continue
		}

		// We need to kill the container, but if we also want to restart the
		// container afterwards, make the intent clear in the message. Also do
		// not kill the entire pod since we expect container to be running eventually.
		if restart {
			message = fmt.Sprintf("%s, will be restarted", message)
			changes.ContainersToStart = append(changes.ContainersToStart, idx)
		}

		changes.ContainersToKill[containerStatus.ID] = containerToKillInfo{
			name:      containerStatus.Name,
			container: &pod.Spec.Containers[idx],
			message:   message,
		}
		klog.V(2).Infof("Container %q (%q) of pod %s: %s", container.Name, containerStatus.ID, format.Pod(pod), message)
	}

	if keepCount == 0 && len(changes.ContainersToStart) == 0 {
		changes.KillPod = true
		if utilfeature.DefaultFeatureGate.Enabled(features.InPlacePodVerticalScaling) {
			if len(changes.ContainersToRestart) != 0 {
				changes.KillPod = false
			}
		}
	}

	// always attempts to identify hotplug nic based on pod spec & pod status (got from runtime)
	if m.canHotplugNIC(pod, podStatus) {
		if len(podStatus.SandboxStatuses) > 0 && podStatus.SandboxStatuses[0].GetNetwork() != nil {
			nicsToAttach, nicsToDetach := computeNICHotplugs(pod.Spec.Nics, podStatus.SandboxStatuses[0].GetNetwork().GetNics())
			if len(nicsToAttach) > 0 {
				changes.Hotplugs.NICsToAttach = nicsToAttach
			}
			if len(nicsToDetach) > 0 {
				changes.Hotplugs.NICsToDetach = nicsToDetach
			}
		}
	}

	return changes
}

func (m *kubeGenericRuntimeManager) canHotplugNIC(pod *v1.Pod, podStatus *kubecontainer.PodStatus) bool {
	// todo[nic-hotplug]: use alternative explicit way to determine the capability of runtime (which may require CRI extension)
	// assuming runtime able to support nic hotplug iff podsandboxstatus contains nic status details
	return len(podStatus.SandboxStatuses) > 0 && len(podStatus.SandboxStatuses[0].GetNetwork().GetNics()) > 0
}

func computeNICHotplugs(vnics []v1.Nic, nicStatuses []*runtimeapi.NICStatus) (plugins, plugouts []string) {
	if len(nicStatuses) == 0 {
		// none nic status details; unable to derive nic hotplugs
		return
	}

	nicsValidInSpec := []string{}
	for _, vnic := range vnics {
		name := vnic.Name
		if name == "" {
			// vnic w/o name, not suitable for hotplug
			// todo[nic-hotplug]: update user/design doc about vnic name requirement
			continue
		}

		if len(strings.TrimSpace(vnic.PortId)) == 0 {
			// portID not filled yet; fine to skip this time
			continue
		}

		nicsValidInSpec = append(nicsValidInSpec, name)
	}

	nicsInStatus := []string{}
	for _, status := range nicStatuses {
		nicsInStatus = append(nicsInStatus, status.Name)
	}

	plugins = getSlicesDifference(nicsValidInSpec, nicsInStatus)
	plugouts = getSlicesDifference(nicsInStatus, nicsValidInSpec)

	return
}

// slice1 - slice2
func getSlicesDifference(slice1, slice2 []string) (difference []string) {
	for _, name := range slice1 {
		found := false
		for _, n := range slice2 {
			if n == name {
				found = true
				break
			}
		}

		if !found {
			difference = append(difference, name)
		}
	}

	return difference
}

func (m *kubeGenericRuntimeManager) SyncPod(pod *v1.Pod, podStatus *kubecontainer.PodStatus, pullSecrets []v1.Secret, backOff *flowcontrol.Backoff) (result kubecontainer.PodSyncResult) {
	if pod.Spec.VirtualMachine != nil {
		return m.SyncPodVm(pod, podStatus, pullSecrets, backOff)
	}
	return m.SyncPodContainer(pod, podStatus, pullSecrets, backOff)
}

func (m *kubeGenericRuntimeManager) SyncPodVm(podin *v1.Pod, podStatus *kubecontainer.PodStatus, pullSecrets []v1.Secret, backOff *flowcontrol.Backoff) (result kubecontainer.PodSyncResult) {
	klog.V(4).Infof("SyncOp VM POD for pod %q", format.Pod(podin))
	pod := podConverter.ConvertVmPodToContainerPod(podin)
	if pod == nil {
		klog.Errorf("failed to get converted pod")
		return
	}
	result = m.SyncPodContainer(pod, podStatus, pullSecrets, backOff)
	podConverter.DumpPodSyncResult(result)
	return result
}

// SyncPod syncs the running pod into the desired pod by executing following steps:
//
//  1. Compute sandbox and container changes.
//  2. Kill pod sandbox if necessary.
//  3. Kill any containers that should not be running.
//  4. Create sandbox if necessary.
//  5. Create init containers.
//  6. Create normal containers.
func (m *kubeGenericRuntimeManager) SyncPodContainer(pod *v1.Pod, podStatus *kubecontainer.PodStatus, pullSecrets []v1.Secret, backOff *flowcontrol.Backoff) (result kubecontainer.PodSyncResult) {
	// Step 1: Compute sandbox and container changes.
	podContainerChanges := m.computePodActions(pod, podStatus)
	klog.V(3).Infof("computePodActions got %+v for pod %q", podContainerChanges, format.Pod(pod))
	if podContainerChanges.CreateSandbox {
		ref, err := ref.GetReference(legacyscheme.Scheme, pod)
		if err != nil {
			klog.Errorf("Couldn't make a ref to pod %q: '%v'", format.Pod(pod), err)
		}
		if podContainerChanges.SandboxID != "" {
			m.recorder.Eventf(ref, v1.EventTypeNormal, events.SandboxChanged, "Pod sandbox changed, it will be killed and re-created.")
		} else {
			klog.V(4).Infof("SyncPod received new pod %q, will create a sandbox for it", format.Pod(pod))
		}
	}

	// Step 2: Kill the pod if the sandbox has changed.
	if podContainerChanges.KillPod {
		if podContainerChanges.CreateSandbox {
			klog.V(4).Infof("Stopping PodSandbox for %q, will start new one", format.Pod(pod))
		} else {
			klog.V(4).Infof("Stopping PodSandbox for %q because all other containers are dead.", format.Pod(pod))
		}

		killResult := m.killPodWithSyncResult(pod, kubecontainer.ConvertPodStatusToRunningPod(m.runtimeName, podStatus), nil)
		result.AddPodSyncResult(killResult)
		if killResult.Error() != nil {
			klog.Errorf("killPodWithSyncResult failed: %v", killResult.Error())
			return
		}

		if podContainerChanges.CreateSandbox {
			m.purgeInitContainers(pod, podStatus)
		}
	} else {
		// Step 3: kill any running containers in this pod which are not to keep.
		for containerID, containerInfo := range podContainerChanges.ContainersToKill {
			klog.V(3).Infof("Killing unwanted container %q(id=%q) for pod %q", containerInfo.name, containerID, format.Pod(pod))
			killContainerResult := kubecontainer.NewSyncResult(kubecontainer.KillContainer, containerInfo.name)
			result.AddSyncResult(killContainerResult)
			if err := m.killContainer(pod, containerID, containerInfo.name, containerInfo.message, nil); err != nil {
				killContainerResult.Fail(kubecontainer.ErrKillContainer, err.Error())
				klog.Errorf("killContainer %q(id=%q) for pod %q failed: %v", containerInfo.name, containerID, format.Pod(pod), err)
				return
			}
		}
	}

	// Keep terminated init containers fairly aggressively controlled
	// This is an optimization because container removals are typically handled
	// by container garbage collector.
	m.pruneInitContainersBeforeStart(pod, podStatus)

	// We pass the value of the podIP down to generatePodSandboxConfig and
	// generateContainerConfig, which in turn passes it to various other
	// functions, in order to facilitate functionality that requires this
	// value (hosts file and downward API) and avoid races determining
	// the pod IP in cases where a container requires restart but the
	// podIP isn't in the status manager yet.
	//
	// We default to the IP in the passed-in pod status, and overwrite it if the
	// sandbox needs to be (re)started.
	podIP := ""
	if podStatus != nil {
		podIP = podStatus.IP
	}

	// Step 4: Create a sandbox for the pod if necessary.
	podSandboxID := podContainerChanges.SandboxID
	if podContainerChanges.CreateSandbox {
		var msg string
		var err error

		klog.V(4).Infof("Creating sandbox for pod %q", format.Pod(pod))
		createSandboxResult := kubecontainer.NewSyncResult(kubecontainer.CreatePodSandbox, format.Pod(pod))
		result.AddSyncResult(createSandboxResult)
		podSandboxID, msg, err = m.createPodSandbox(pod, podContainerChanges.Attempt)
		if err != nil {
			createSandboxResult.Fail(kubecontainer.ErrCreatePodSandbox, msg)
			klog.Errorf("createPodSandbox for pod %q failed: %v", format.Pod(pod), err)
			ref, referr := ref.GetReference(legacyscheme.Scheme, pod)
			if referr != nil {
				klog.Errorf("Couldn't make a ref to pod %q: '%v'", format.Pod(pod), referr)
			}
			m.recorder.Eventf(ref, v1.EventTypeWarning, events.FailedCreatePodSandBox, "Failed create pod sandbox: %v", err)
			return
		}
		klog.V(4).Infof("Created PodSandbox %q for pod %q", podSandboxID, format.Pod(pod))

		runtimeService, err := m.GetRuntimeServiceByPod(pod)
		if err != nil {
			klog.Errorf("Failed to get runtime service to pod %q: '%v'", format.Pod(pod), err)
			result.Fail(err)
			return
		}

		podSandboxStatus, err := runtimeService.PodSandboxStatus(podSandboxID)
		if err != nil {
			ref, referr := ref.GetReference(legacyscheme.Scheme, pod)
			if referr != nil {
				klog.Errorf("Couldn't make a ref to pod %q: '%v'", format.Pod(pod), referr)
			}
			m.recorder.Eventf(ref, v1.EventTypeWarning, events.FailedStatusPodSandBox, "Unable to get pod sandbox status: %v", err)
			klog.Errorf("Failed to get pod sandbox status: %v; Skipping pod %q", err, format.Pod(pod))
			result.Fail(err)
			return
		}

		// If we ever allow updating a pod from non-host-network to
		// host-network, we may use a stale IP.
		if !kubecontainer.IsHostNetworkPod(pod) {
			// Overwrite the podIP passed in the pod status, since we just started the pod sandbox.
			podIP = m.determinePodSandboxIP(pod.Tenant, pod.Namespace, pod.Name, podSandboxStatus)
			klog.V(4).Infof("Determined the ip %q for pod %q after sandbox changed", podIP, format.Pod(pod))
		}
	}

	// Get podSandboxConfig for containers to start.
	configPodSandboxResult := kubecontainer.NewSyncResult(kubecontainer.ConfigPodSandbox, podSandboxID)
	result.AddSyncResult(configPodSandboxResult)
	podSandboxConfig, err := m.generatePodSandboxConfig(pod, podContainerChanges.Attempt)
	if err != nil {
		message := fmt.Sprintf("GeneratePodSandboxConfig for pod %q failed: %v", format.Pod(pod), err)
		klog.Error(message)
		configPodSandboxResult.Fail(kubecontainer.ErrConfigPodSandbox, message)
		return
	}

	// Step 5: start the init container.
	if container := podContainerChanges.NextInitContainerToStart; container != nil {
		// Start the next init container.
		startContainerResult := kubecontainer.NewSyncResult(kubecontainer.StartContainer, container.Name)
		result.AddSyncResult(startContainerResult)
		isInBackOff, msg, err := m.doBackOff(pod, container, podStatus, backOff)
		if isInBackOff {
			startContainerResult.Fail(err, msg)
			klog.V(4).Infof("Backing Off restarting init container %+v in pod %v", container, format.Pod(pod))
			return
		}

		klog.V(4).Infof("Creating init container %+v in pod %v", container, format.Pod(pod))
		if msg, err := m.startContainer(podSandboxID, podSandboxConfig, container, pod, podStatus, pullSecrets, podIP); err != nil {
			startContainerResult.Fail(err, msg)
			utilruntime.HandleError(fmt.Errorf("init container start failed: %v: %s", err, msg))
			return
		}

		// Successfully started the container; clear the entry in the failure
		klog.V(4).Infof("Completed init container %q for pod %q", container.Name, format.Pod(pod))
	}

	startNewContainer := func(idx int) {
		container := &pod.Spec.Containers[idx]
		startContainerResult := kubecontainer.NewSyncResult(kubecontainer.StartContainer, container.Name)
		result.AddSyncResult(startContainerResult)

		isInBackOff, msg, err := m.doBackOff(pod, container, podStatus, backOff)
		if isInBackOff {
			startContainerResult.Fail(err, msg)
			klog.V(4).Infof("Backing Off restarting container %+v in pod %v", container, format.Pod(pod))
			return
		}

		klog.V(4).Infof("Creating container %+v in pod %v", container, format.Pod(pod))
		if msg, err := m.startContainer(podSandboxID, podSandboxConfig, container, pod, podStatus, pullSecrets, podIP); err != nil {
			startContainerResult.Fail(err, msg)
			// known errors that are logged in other places are logged at higher levels here to avoid
			// repetitive log spam
			switch {
			case err == images.ErrImagePullBackOff:
				klog.V(3).Infof("container start failed: %v: %s", err, msg)
			default:
				utilruntime.HandleError(fmt.Errorf("container start failed: %v: %s", err, msg))
			}
		}
	}
	// Step 6: start containers in podContainerChanges.ContainersToStart.
	for _, idx := range podContainerChanges.ContainersToStart {
		startNewContainer(idx)
	}

	// Step 7: For containers in podContainerChanges.ContainersToUpdate[Cpu,Memory] list, invoke UpdateContainerResources
	if utilfeature.DefaultFeatureGate.Enabled(features.InPlacePodVerticalScaling) &&
		(len(podContainerChanges.ContainersToUpdate) > 0 || len(podContainerChanges.ContainersToRestart) > 0) {
		pcm := m.containerManager.NewPodContainerManager()
		podResources := cm.ResourceConfigForPod(pod, m.cpuCFSQuota, uint64((m.cpuCFSQuotaPeriod.Duration)/time.Microsecond))
		if podResources == nil {
			klog.Errorf("Unable to get resource configuration for pod %s", pod.Name)
			return
		}
		setPodCgroupConfig := func(rName string) error {
			var err error
			switch rName {
			case cpuRequest:
				err = pcm.SetPodCgroupCpuConfig(pod, nil, podResources.CpuPeriod, podResources.CpuShares)
			case cpuLimit:
				err = pcm.SetPodCgroupCpuConfig(pod, podResources.CpuQuota, podResources.CpuPeriod, nil)
			case memLimit:
				err = pcm.SetPodCgroupMemoryConfig(pod, *podResources.Memory)
			}
			if err != nil {
				klog.Errorf("Failed to set %s cgroup config for pod %s failed: %v", rName, pod.Name, err)
			}
			return err
		}
		// Memory and CPU are updated separately because memory resizes may be ordered differently than CPU resizes.
		// If resize results in net pod resource increase, set pod cgroup config before resizing containers.
		// If resize results in net pod resource decrease, set pod cgroup config after resizing containers.
		// If an error occurs at any point, abort. Let future syncpod iterations retry the unfinished stuff.
		resizeContainers := func(rName string, currPodCgValue, newPodCgValue int64) error {
			var err error
			if newPodCgValue > currPodCgValue {
				if err = setPodCgroupConfig(rName); err != nil {
					return err
				}
			}
			if len(podContainerChanges.ContainersToUpdate[rName]) > 0 {
				if err = m.updatePodContainerResources(pod, podStatus, rName, podContainerChanges.ContainersToUpdate[rName]); err != nil {
					klog.Errorf("updatePodContainerResources for pod %q resource %s failed: %v", format.Pod(pod), rName, err)
					return err
				}
			}
			if newPodCgValue < currPodCgValue {
				err = setPodCgroupConfig(rName)
			}
			return err
		}
		if len(podContainerChanges.ContainersToUpdate[memLimit]) > 0 || len(podContainerChanges.ContainersToRestart) > 0 {
			currentPodMemoryLimit, err := pcm.GetPodCgroupMemoryConfig(pod)
			if err != nil {
				klog.Errorf("GetPodCgroupMemoryConfig for pod %s failed: %v", pod.Name, err)
				return
			}
			currentPodMemoryUsage, err := pcm.GetPodCgroupMemoryUsage(pod)
			if err != nil {
				klog.Errorf("GetPodCgroupMemoryUsage for pod %s failed: %v", pod.Name, err)
				return
			}
			if currentPodMemoryUsage >= uint64(*podResources.Memory) {
				klog.Errorf("Aborting attempt to set pod memory limit less than current memory usage for pod %s.", pod.Name)
				return
			}
			if errResize := resizeContainers(memLimit, int64(currentPodMemoryLimit), *podResources.Memory); errResize != nil {
				return
			}
		}
		if len(podContainerChanges.ContainersToUpdate[cpuRequest]) > 0 || len(podContainerChanges.ContainersToRestart) > 0 {
			_, _, currentPodCPUShares, err := pcm.GetPodCgroupCpuConfig(pod)
			if err != nil {
				klog.Errorf("GetPodCgroupCpuConfig for pod %s failed: %v", pod.Name, err)
				return
			}
			if errResize := resizeContainers(cpuRequest, int64(currentPodCPUShares), int64(*podResources.CpuShares)); errResize != nil {
				return
			}
		}
		if len(podContainerChanges.ContainersToUpdate[cpuLimit]) > 0 || len(podContainerChanges.ContainersToRestart) > 0 {
			currentPodCpuQuota, _, _, err := pcm.GetPodCgroupCpuConfig(pod)
			if err != nil {
				klog.Errorf("GetPodCgroupCpuConfig for pod %s failed: %v", pod.Name, err)
				return
			}
			if errUpdate := resizeContainers(cpuLimit, currentPodCpuQuota, *podResources.CpuQuota); errUpdate != nil {
				return
			}
		}
		for _, idx := range podContainerChanges.ContainersToRestart {
			startNewContainer(idx)
		}
	}

	if len(podContainerChanges.Hotplugs.NICsToAttach) > 0 {
		// we don't attempt the recovery of failed nic hotplug; fine to ignore error
		// todo[vnic-hotplug]: to handle nic hotplug error to support nic hotplug recovery if it is desired feature
		m.attachNICs(pod, podSandboxID, podContainerChanges.Hotplugs.NICsToAttach)
	}
	if len(podContainerChanges.Hotplugs.NICsToDetach) > 0 {
		m.detachNICs(pod, podSandboxID, podContainerChanges.Hotplugs.NICsToDetach)
	}

	return
}

func (m *kubeGenericRuntimeManager) updatePodContainerResources(pod *v1.Pod, podStatus *kubecontainer.PodStatus, resourceName string, containersToUpdate []containerToUpdateInfo) error {
	type updatedResourcesInfo struct {
		cStatus *kubecontainer.ContainerStatus
		rLimits v1.ResourceList
		rAllocs v1.ResourceList
	}
	updatedResourcesMap := make(map[kubecontainer.ContainerID]updatedResourcesInfo)

	for _, cInfo := range containersToUpdate {
		container := cInfo.apiContainer
		specLimits := container.Resources.Limits
		specAllocs := container.ResourcesAllocated
		defer func() {
			container.Resources.Limits = specLimits
			container.ResourcesAllocated = specAllocs
		}()
		statusLimits := cInfo.apiContainerStatus.Resources.Limits
		statusRequests := cInfo.apiContainerStatus.Resources.Requests
		// Runtime container status resources, if set, takes precedence.
		if cInfo.kubeContainerStatus.Resources.Limits != nil {
			statusLimits = cInfo.kubeContainerStatus.Resources.Limits
		}
		if cInfo.kubeContainerStatus.Resources.Requests != nil {
			statusRequests = cInfo.kubeContainerStatus.Resources.Requests
		}
		switch resourceName {
		case cpuRequest:
			container.Resources.Limits = v1.ResourceList{
				v1.ResourceCPU:    *statusLimits.Cpu(),
				v1.ResourceMemory: *statusLimits.Memory(),
			}
			container.ResourcesAllocated = v1.ResourceList{
				v1.ResourceCPU:    *specAllocs.Cpu(),
				v1.ResourceMemory: *statusRequests.Memory(),
			}
		case cpuLimit:
			container.Resources.Limits = v1.ResourceList{
				v1.ResourceCPU:    *specLimits.Cpu(),
				v1.ResourceMemory: *statusLimits.Memory(),
			}
			container.ResourcesAllocated = v1.ResourceList{
				v1.ResourceCPU:    *statusRequests.Cpu(),
				v1.ResourceMemory: *statusRequests.Memory(),
			}
		case memLimit:
			container.Resources.Limits = v1.ResourceList{
				v1.ResourceCPU:    *statusLimits.Cpu(),
				v1.ResourceMemory: *specLimits.Memory(),
			}
			container.ResourcesAllocated = v1.ResourceList{
				v1.ResourceCPU:    *statusRequests.Cpu(),
				v1.ResourceMemory: *specAllocs.Memory(),
			}
		}
		if err := m.updateContainerResources(pod, container, cInfo.kubeContainerStatus.ID); err != nil {
			klog.Errorf("updateContainerResources %q(id=%q) for pod %q failed: %v", container.Name, cInfo.kubeContainerStatus.ID, format.Pod(pod), err)
			return err
		}
		cInfo.apiContainerStatus.Resources.Limits = container.Resources.Limits
		// NOTE: Ideally, cpu and memory resources information come from ContainerStatus CRI API query.
		//       However, the runtime may not have implemented query support for currently configured resources.
		//	 If this info isn't available via CRI, update pod cache status on successful updateContainerResources.
		if container.Resources.Limits != nil {
			updatedResourcesMap[cInfo.kubeContainerStatus.ID] = updatedResourcesInfo{
				cStatus: cInfo.kubeContainerStatus,
				rLimits: container.Resources.Limits,
				rAllocs: container.ResourcesAllocated,
			}
		}
	}

	if len(updatedResourcesMap) > 0 {
		// Update pod cache runtime container status (cInfo.kubeContainerStatus) with resource from ContainerStatus CRI API
		newPodStatus, err := m.GetPodStatus(podStatus.ID, pod.Name, pod.Namespace, pod.Tenant)
		if err != nil {
			klog.Errorf("GetPodStatus failed for pod %q failed: %v", format.Pod(pod), err)
			return err
		}
		for _, kubeContainerStatus := range newPodStatus.ContainerStatuses {
			if updatedInfo, updated := updatedResourcesMap[kubeContainerStatus.ID]; updated {
				if kubeContainerStatus.Resources.Limits != nil {
					updatedInfo.cStatus.Resources.Limits = kubeContainerStatus.Resources.Limits
				} else {
					updatedInfo.cStatus.Resources.Limits = updatedInfo.rLimits.DeepCopy()
				}
				if kubeContainerStatus.Resources.Requests != nil {
					updatedInfo.cStatus.Resources.Requests = kubeContainerStatus.Resources.Requests
				} else {
					updatedInfo.cStatus.Resources.Requests = updatedInfo.rAllocs.DeepCopy()
				}
			}
		}
	}
	return nil
}

func (m *kubeGenericRuntimeManager) attachNICs(pod *v1.Pod, podSandboxID string, nics []string) *kubecontainer.SyncResult {
	result := kubecontainer.NewSyncResult(kubecontainer.HotplugDevice, pod.Name)

	// todo[vnic-hotplug]: some pod type may not support hotplug at all; avoid calling the method doomed to fail
	if err := m.doAttachNICs(pod, podSandboxID, nics); err != nil {
		result.Fail(err, "error happened on attaching NICs")
	}

	return result
}

func (m *kubeGenericRuntimeManager) doAttachNICs(pod *v1.Pod, sandbox string, nics []string) error {
	var result error

	for _, nicName := range nics {
		nicSpec, ok := getNICSpec(pod, nicName)
		if !ok {
			result = multierr.Append(result, fmt.Errorf("failed to attach nic %q: nic spec not found", nicName))
			continue
		}

		// todo[vnic-hotplug]: consider to use CRI batch method if available
		// todo[vnic-hotplug]: to fill vm id if it is really required by CRI runtime
		if err := m.AttachNetworkInterface(pod, "" /*vmName*/, nicSpec); err != nil {
			result = multierr.Append(result, fmt.Errorf("failed to attach nic %q: %v", nicName, err))
		}
	}

	return result
}

func (m *kubeGenericRuntimeManager) attachNIC(pod *v1.Pod, vmName string, nic *v1.Nic) error {
	ref, referr := ref.GetReference(legacyscheme.Scheme, pod)
	if referr != nil {
		klog.Errorf("Couldn't make a ref to pod %q: '%v'", format.Pod(pod), referr)
	}

	if err := m.AttachNetworkInterface(pod, vmName, nic); err != nil {
		klog.Warningf("error happened when pod %s/%s/%s attaching nic %s: %v", pod.Tenant, pod.Namespace, pod.Name, nic.Name, err)
		if referr == nil {
			m.recorder.Eventf(ref, v1.EventTypeWarning, events.FailedAttachDevice, "network interface %q hotplug had error: %v", nic.Name, err)
		}
		return err
	}

	klog.V(4).Infof("pod %s/%s/%s attached nic hotplug %s", pod.Tenant, pod.Namespace, pod.Name, nic.Name)
	if referr == nil {
		m.recorder.Eventf(ref, v1.EventTypeNormal, events.DeviceAttached, "network interfaces %q hotplug succeeded", nic.Name)
	}
	return nil
}

func (m *kubeGenericRuntimeManager) detachNICs(pod *v1.Pod, podSandboxID string, nics []string) *kubecontainer.SyncResult {
	result := kubecontainer.NewSyncResult(kubecontainer.HotplugDevice, pod.Name)

	// todo[vnic-hotplug]: some pod type may not support hotplug at all; avoid calling the method doomed to fail
	if err := m.doDetachNICs(pod, podSandboxID, nics); err != nil {
		result.Fail(err, "error happened on detaching NICs")
	}

	return result
}

func (m *kubeGenericRuntimeManager) doDetachNICs(pod *v1.Pod, sandbox string, nics []string) error {
	var result error

	for _, nicName := range nics {
		if _, ok := getNICSpec(pod, nicName); ok {
			klog.V(4).Infof("hotplug: pod %s/%s to detach nic %s still in spec; ignoring", pod.Namespace, pod.Name, nicName)
			continue
		}

		// todo[vnic-hotplug]: consider to use CRI batch method if available
		// todo[vnic-hotplug]: to fill vm id if it is really required by CRI runtime
		// todo[vnic-hotplug]: simplify detach nic CRI api to receive nic name only instead of nic spec
		nicSpec := &v1.Nic{Name: nicName}
		if err := m.detachNIC(pod, "" /*vmName*/, nicSpec); err != nil {
			result = multierr.Append(result, fmt.Errorf("failed to detach nic %q: %v", nicName, err))
		}
	}

	return result
}

func (m *kubeGenericRuntimeManager) detachNIC(pod *v1.Pod, vmName string, nic *v1.Nic) error {
	ref, referr := ref.GetReference(legacyscheme.Scheme, pod)
	if referr != nil {
		klog.Errorf("Couldn't make a ref to pod %q: '%v'", format.Pod(pod), referr)
	}

	if err := m.DetachNetworkInterface(pod, vmName, nic); err != nil {
		klog.Warningf("hotplug: error happened when pod %s/%s detaching nic %s: %v", pod.Namespace, pod.Name, nic.Name, err)
		if referr == nil {
			m.recorder.Eventf(ref, v1.EventTypeWarning, events.FailedDetachDevice, "network interface %q hotplug had error: %v", nic.Name, err)
		}
		return err
	}

	klog.V(4).Infof("hotplug: pod %s/%s successfully detached nic %s", pod.Namespace, pod.Name, nic.Name)
	if referr == nil {
		m.recorder.Eventf(ref, v1.EventTypeNormal, events.DeviceDetached, "network interfaces %q hotplug succeeded", nic.Name)
	}
	return nil
}

func getNICSpec(pod *v1.Pod, name string) (*v1.Nic, bool) {
	for _, nic := range pod.Spec.Nics {
		if nic.Name == name {
			return &nic, true
		}
	}

	return nil, false
}

// If a container is still in backoff, the function will return a brief backoff error and
// a detailed error message.
func (m *kubeGenericRuntimeManager) doBackOff(pod *v1.Pod, container *v1.Container, podStatus *kubecontainer.PodStatus, backOff *flowcontrol.Backoff) (bool, string, error) {
	var cStatus *kubecontainer.ContainerStatus
	for _, c := range podStatus.ContainerStatuses {
		if c.Name == container.Name && c.State == kubecontainer.ContainerStateExited {
			cStatus = c
			break
		}
	}

	if cStatus == nil {
		return false, "", nil
	}

	klog.V(3).Infof("checking backoff for container %q in pod %q", container.Name, format.Pod(pod))
	// Use the finished time of the latest exited container as the start point to calculate whether to do back-off.
	ts := cStatus.FinishedAt
	// backOff requires a unique key to identify the container.
	key := getStableKey(pod, container)
	if backOff.IsInBackOffSince(key, ts) {
		if ref, err := kubecontainer.GenerateContainerRef(pod, container); err == nil {
			m.recorder.Eventf(ref, v1.EventTypeWarning, events.BackOffStartContainer, "Back-off restarting failed container")
		}
		err := fmt.Errorf("Back-off %s restarting failed container=%s pod=%s", backOff.Get(key), container.Name, format.Pod(pod))
		klog.V(3).Infof("%s", err.Error())
		return true, err.Error(), kubecontainer.ErrCrashLoopBackOff
	}

	backOff.Next(key, ts)
	return false, "", nil
}

// KillPod kills all the containers of a pod. Pod may be nil, running pod must not be.
// gracePeriodOverride if specified allows the caller to override the pod default grace period.
// only hard kill paths are allowed to specify a gracePeriodOverride in the kubelet in order to not corrupt user data.
// it is useful when doing SIGKILL for hard eviction scenarios, or max grace period during soft eviction scenarios.
func (m *kubeGenericRuntimeManager) KillPod(pod *v1.Pod, runningPod kubecontainer.Pod, gracePeriodOverride *int64) error {
	podid := string(runningPod.ID)
	err := m.killPodWithSyncResult(pod, runningPod, gracePeriodOverride)
	if err.Error() == nil {
		return m.removePodRuntimeService(podid)
	}
	return err.Error()
}

// killPodWithSyncResult kills a runningPod and returns SyncResult.
// Note: The pod passed in could be *nil* when kubelet restarted.
func (m *kubeGenericRuntimeManager) killPodWithSyncResult(pod *v1.Pod, runningPod kubecontainer.Pod, gracePeriodOverride *int64) (result kubecontainer.PodSyncResult) {
	killContainerResults := m.killContainersWithSyncResult(pod, runningPod, gracePeriodOverride)
	for _, containerResult := range killContainerResults {
		result.AddSyncResult(containerResult)
	}

	// stop sandbox, the sandbox will be removed in GarbageCollect
	killSandboxResult := kubecontainer.NewSyncResult(kubecontainer.KillPodSandbox, runningPod.ID)
	result.AddSyncResult(killSandboxResult)
	// Stop all sandboxes belongs to same pod
	for _, podSandbox := range runningPod.Sandboxes {
		runtimeService, err := m.GetRuntimeServiceByPodID(runningPod.ID)
		if err != nil {
			killSandboxResult.Fail(errors.New("GetRuntimeServiceForPod"), err.Error())
			klog.Errorf("Failed to get runtime service for sandbox %q", podSandbox.ID)
			continue
		}

		if err := runtimeService.StopPodSandbox(podSandbox.ID.ID); err != nil {
			killSandboxResult.Fail(kubecontainer.ErrKillPodSandbox, err.Error())
			klog.Errorf("Failed to stop sandbox %q", podSandbox.ID)
			continue
		}
	}

	return
}

// GetPodStatus retrieves the status of the pod, including the
// information of all containers in the pod that are visible in Runtime.
func (m *kubeGenericRuntimeManager) GetPodStatus(uid kubetypes.UID, name, namespace, tenant string) (*kubecontainer.PodStatus, error) {
	// Now we retain restart count of container as a container label. Each time a container
	// restarts, pod will read the restart count from the registered dead container, increment
	// it to get the new restart count, and then add a label with the new restart count on
	// the newly started container.
	// However, there are some limitations of this method:
	//	1. When all dead containers were garbage collected, the container status could
	//	not get the historical value and would be *inaccurate*. Fortunately, the chance
	//	is really slim.
	//	2. When working with old version containers which have no restart count label,
	//	we can only assume their restart count is 0.
	// Anyhow, we only promised "best-effort" restart count reporting, we can just ignore
	// these limitations now.
	// TODO: move this comment to SyncPod.
	podSandboxIDs, err := m.getSandboxIDByPodUID(uid, nil)
	if err != nil {
		return nil, err
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Tenant:    tenant,
			UID:       uid,
			HashKey:   fuzzer.GetHashOfUUID(uid),
		},
	}
	podFullName := format.Pod(pod)
	klog.V(4).Infof("getSandboxIDByPodUID got sandbox IDs %q for pod %q", podSandboxIDs, podFullName)

	sandboxStatuses := make([]*runtimeapi.PodSandboxStatus, len(podSandboxIDs))
	podIP := ""
	runtimeService, err := m.GetRuntimeServiceByPodID(uid)
	if err != nil {
		return nil, err
	}

	for idx, podSandboxID := range podSandboxIDs {
		podSandboxStatus, err := runtimeService.PodSandboxStatus(podSandboxID)
		if err != nil {
			klog.Errorf("PodSandboxStatus of sandbox %q for pod %q error: %v", podSandboxID, podFullName, err)
			return nil, err
		}
		sandboxStatuses[idx] = podSandboxStatus

		// Only get pod IP from latest sandbox
		if idx == 0 && podSandboxStatus.State == runtimeapi.PodSandboxState_SANDBOX_READY {
			podIP = m.determinePodSandboxIP(tenant, namespace, name, podSandboxStatus)

			// incorporate details of nic status got from separate cri extension call
			// if pod sandbox status does not include such info, as fallback measure
			if podSandboxStatus.Network == nil || len(podSandboxStatus.Network.Nics) == 0 {
				m.incorporateNICStatus(pod, podSandboxStatus)
			}
		}
	}

	// Get statuses of all containers visible in the pod.
	containerStatuses, err := m.getPodContainerStatuses(uid, name, namespace, tenant)
	if err != nil {
		if m.logReduction.ShouldMessageBePrinted(err.Error(), podFullName) {
			klog.Errorf("getPodContainerStatuses for pod %q failed: %v", podFullName, err)
		}
		return nil, err
	}
	m.logReduction.ClearID(podFullName)

	//TODO: Ideally for VM type, construct the podStatus for VM type before return
	//      Since for Cloud Fabric 830 release, the runtime is virtlet and it cannot determine
	//      pod workload type here, we rely on the calling functions to update the podStatus
	//      with VirtualMachineStatus, for virtlet, it is the container status.
	return &kubecontainer.PodStatus{
		ID:                uid,
		Name:              name,
		Namespace:         namespace,
		Tenant:            tenant,
		IP:                podIP,
		SandboxStatuses:   sandboxStatuses,
		ContainerStatuses: containerStatuses,
	}, nil
}

func (m *kubeGenericRuntimeManager) incorporateNICStatus(pod *v1.Pod, podSandboxStatus *runtimeapi.PodSandboxStatus) {
	// todo[vnic-hotplug]: avoid calling api for some runtime that does not support nic hotplug
	// todo[vnic-hotplug]: set proper vmName if really required; or eliminate if not
	nics, err := m.ListNetworkInterfaces(pod, "" /*vmName*/)
	if err != nil {
		// fine for some runtime not able to honor such api
		return
	}

	statuses := []*runtimeapi.NICStatus{}
	for _, nic := range nics {
		status := &runtimeapi.NICStatus{
			Name:   nic.Name,
			PortId: nic.PortId,
			State:  runtimeapi.NICState_NIC_UNKNOWN, // todo[vnic-hotplug]: set proper state if available from runtime api
		}
		statuses = append(statuses, status)
	}

	if len(statuses) > 0 {
		if podSandboxStatus.Network == nil {
			podSandboxStatus.Network = &runtimeapi.PodSandboxNetworkStatus{}
		}
		podSandboxStatus.Network.Nics = statuses
	}
}

// GarbageCollect removes dead containers using the specified container gc policy.
func (m *kubeGenericRuntimeManager) GarbageCollect(gcPolicy kubecontainer.ContainerGCPolicy, allSourcesReady bool, evictNonDeletedPods bool) error {
	return m.containerGC.GarbageCollect(gcPolicy, allSourcesReady, evictNonDeletedPods)
}

// UpdatePodCIDR is just a passthrough method to update the runtimeConfig of the shim
// with the podCIDR supplied by the kubelet.
func (m *kubeGenericRuntimeManager) UpdatePodCIDR(podCIDR string) error {
	// TODO(#35531): do we really want to write a method on this manager for each
	// field of the config?
	klog.Infof("updating runtime config through cri with podcidr %v", podCIDR)

	// update each runtimeService with the CIDRfor pod
	// TODO: with multiple runtimes on the node, how to ensure the IPs will not conflict
	runtimeServices, err := m.runtimeRegistry.GetAllRuntimeServices()
	if err != nil {
		return err
	}

	for _, runtimeService := range runtimeServices {

		err := runtimeService.ServiceApi.UpdateRuntimeConfig(
			&runtimeapi.RuntimeConfig{
				NetworkConfig: &runtimeapi.NetworkConfig{
					PodCidr: podCIDR,
				},
			})

		if err != nil {
			return err
		}
	}

	return nil
}
