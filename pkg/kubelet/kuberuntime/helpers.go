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
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/features"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/remote"
	"k8s.io/kubernetes/pkg/kubelet/runtimeregistry"
)

type podsByID []*kubecontainer.Pod

func (b podsByID) Len() int           { return len(b) }
func (b podsByID) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b podsByID) Less(i, j int) bool { return b[i].ID < b[j].ID }

type containersByID []*kubecontainer.Container

func (b containersByID) Len() int           { return len(b) }
func (b containersByID) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b containersByID) Less(i, j int) bool { return b[i].ID.ID < b[j].ID.ID }

// Newest first.
type podSandboxByCreated []*runtimeapi.PodSandbox

func (p podSandboxByCreated) Len() int           { return len(p) }
func (p podSandboxByCreated) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p podSandboxByCreated) Less(i, j int) bool { return p[i].CreatedAt > p[j].CreatedAt }

type containerStatusByCreated []*kubecontainer.ContainerStatus

func (c containerStatusByCreated) Len() int           { return len(c) }
func (c containerStatusByCreated) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c containerStatusByCreated) Less(i, j int) bool { return c[i].CreatedAt.After(c[j].CreatedAt) }

// toKubeContainerState converts runtimeapi.ContainerState to kubecontainer.ContainerState.
func toKubeContainerState(state runtimeapi.ContainerState) kubecontainer.ContainerState {
	switch state {
	case runtimeapi.ContainerState_CONTAINER_CREATED:
		return kubecontainer.ContainerStateCreated
	case runtimeapi.ContainerState_CONTAINER_RUNNING:
		return kubecontainer.ContainerStateRunning
	case runtimeapi.ContainerState_CONTAINER_EXITED:
		return kubecontainer.ContainerStateExited
	case runtimeapi.ContainerState_CONTAINER_UNKNOWN:
		return kubecontainer.ContainerStateUnknown
	}

	return kubecontainer.ContainerStateUnknown
}

// toRuntimeProtocol converts v1.Protocol to runtimeapi.Protocol.
func toRuntimeProtocol(protocol v1.Protocol) runtimeapi.Protocol {
	switch protocol {
	case v1.ProtocolTCP:
		return runtimeapi.Protocol_TCP
	case v1.ProtocolUDP:
		return runtimeapi.Protocol_UDP
	case v1.ProtocolSCTP:
		return runtimeapi.Protocol_SCTP
	}

	klog.Warningf("Unknown protocol %q: defaulting to TCP", protocol)
	return runtimeapi.Protocol_TCP
}

// toKubeContainer converts runtimeapi.Container to kubecontainer.Container.
func (m *kubeGenericRuntimeManager) toKubeContainer(c *runtimeapi.Container) (*kubecontainer.Container, error) {
	if c == nil || c.Id == "" || c.Image == nil {
		return nil, fmt.Errorf("unable to convert a nil pointer to a runtime container")
	}

	annotatedInfo := getContainerInfoFromAnnotations(c.Annotations)
	return &kubecontainer.Container{
		ID:      kubecontainer.ContainerID{Type: m.runtimeName, ID: c.Id},
		Name:    c.GetMetadata().GetName(),
		ImageID: c.ImageRef,
		Image:   c.Image.Image,
		Hash:    annotatedInfo.Hash,
		State:   toKubeContainerState(c.State),
	}, nil
}

// sandboxToKubeContainer converts runtimeapi.PodSandbox to kubecontainer.Container.
// This is only needed because we need to return sandboxes as if they were
// kubecontainer.Containers to avoid substantial changes to PLEG.
// TODO: Remove this once it becomes obsolete.
func (m *kubeGenericRuntimeManager) sandboxToKubeContainer(s *runtimeapi.PodSandbox) (*kubecontainer.Container, error) {
	if s == nil || s.Id == "" {
		return nil, fmt.Errorf("unable to convert a nil pointer to a runtime container")
	}

	return &kubecontainer.Container{
		ID:    kubecontainer.ContainerID{Type: m.runtimeName, ID: s.Id},
		State: kubecontainer.SandboxToContainerState(s.State),
	}, nil
}

// getImageUser gets uid or user name that will run the command(s) from image. The function
// guarantees that only one of them is set.
func (m *kubeGenericRuntimeManager) getImageUser(pod *v1.Pod, image string) (*int64, string, error) {
	imageService, err := m.GetImageServiceByPod(pod)
	if err != nil {
		return nil, "", err
	}

	imageStatus, err := imageService.ImageStatus(&runtimeapi.ImageSpec{Image: image})
	if err != nil {
		return nil, "", err
	}

	if imageStatus != nil {
		if imageStatus.Uid != nil {
			return &imageStatus.GetUid().Value, "", nil
		}

		if imageStatus.Username != "" {
			return nil, imageStatus.Username, nil
		}
	}

	// If non of them is set, treat it as root.
	return new(int64), "", nil
}

// isInitContainerFailed returns true if container has exited and exitcode is not zero
// or is in unknown state.
func isInitContainerFailed(status *kubecontainer.ContainerStatus) bool {
	if status.State == kubecontainer.ContainerStateExited && status.ExitCode != 0 {
		return true
	}

	if status.State == kubecontainer.ContainerStateUnknown {
		return true
	}

	return false
}

// getStableKey generates a key (string) to uniquely identify a
// (pod, container) tuple. The key should include the content of the
// container, so that any change to the container generates a new key.
func getStableKey(pod *v1.Pod, container *v1.Container) string {
	hash := strconv.FormatUint(kubecontainer.HashContainer(container), 16)
	return fmt.Sprintf("%s_%s_%s_%s_%s", pod.Name, pod.Namespace, string(pod.UID), container.Name, hash)
}

// logPathDelimiter is the delimiter used in the log path.
const logPathDelimiter = "_"

// buildContainerLogsPath builds log path for container relative to pod logs directory.
func buildContainerLogsPath(containerName string, restartCount int) string {
	return filepath.Join(containerName, fmt.Sprintf("%d.log", restartCount))
}

// BuildContainerLogsDirectory builds absolute log directory path for a container in pod.
func BuildContainerLogsDirectory(podTenant, podNamespace, podName string, podUID types.UID, containerName string) string {
	return filepath.Join(BuildPodLogsDirectory(podTenant, podNamespace, podName, podUID), containerName)
}

// BuildPodLogsDirectory builds absolute log directory path for a pod sandbox.
func BuildPodLogsDirectory(podTenant, podNamespace, podName string, podUID types.UID) string {
	return filepath.Join(podLogsRootDirectory, strings.Join([]string{podTenant, podNamespace, podName,
		string(podUID)}, logPathDelimiter))
}

// parsePodUIDFromLogsDirectory parses pod logs directory name and returns the pod UID.
// It supports both the old pod log directory /var/log/pods/UID, and the new pod log
// directory /var/log/pods/NAMESPACE_NAME_UID.
func parsePodUIDFromLogsDirectory(name string) types.UID {
	parts := strings.Split(name, logPathDelimiter)
	return types.UID(parts[len(parts)-1])
}

// toKubeRuntimeStatus converts the runtimeapi.RuntimeStatus to kubecontainer.RuntimeStatus.
func toKubeRuntimeStatus(status *runtimeapi.RuntimeStatus) *kubecontainer.RuntimeStatus {
	conditions := []kubecontainer.RuntimeCondition{}
	for _, c := range status.GetConditions() {
		conditions = append(conditions, kubecontainer.RuntimeCondition{
			Type:    kubecontainer.RuntimeConditionType(c.Type),
			Status:  c.Status,
			Reason:  c.Reason,
			Message: c.Message,
		})
	}
	return &kubecontainer.RuntimeStatus{Conditions: conditions}
}

// getSeccompProfileFromAnnotations gets seccomp profile from annotations.
// It gets pod's profile if containerName is empty.
func (m *kubeGenericRuntimeManager) getSeccompProfileFromAnnotations(annotations map[string]string, containerName string) string {
	// try the pod profile.
	profile, profileOK := annotations[v1.SeccompPodAnnotationKey]
	if containerName != "" {
		// try the container profile.
		cProfile, cProfileOK := annotations[v1.SeccompContainerAnnotationKeyPrefix+containerName]
		if cProfileOK {
			profile = cProfile
			profileOK = cProfileOK
		}
	}

	if !profileOK {
		return ""
	}

	if strings.HasPrefix(profile, "localhost/") {
		name := strings.TrimPrefix(profile, "localhost/")
		fname := filepath.Join(m.seccompProfileRoot, filepath.FromSlash(name))
		return "localhost/" + fname
	}

	return profile
}

func ipcNamespaceForPod(pod *v1.Pod) runtimeapi.NamespaceMode {
	if pod != nil && pod.Spec.HostIPC {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_POD
}

func networkNamespaceForPod(pod *v1.Pod) runtimeapi.NamespaceMode {
	if pod != nil && pod.Spec.HostNetwork {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_POD
}

func pidNamespaceForPod(pod *v1.Pod) runtimeapi.NamespaceMode {
	if pod != nil {
		if pod.Spec.HostPID {
			return runtimeapi.NamespaceMode_NODE
		}
		if utilfeature.DefaultFeatureGate.Enabled(features.PodShareProcessNamespace) && pod.Spec.ShareProcessNamespace != nil && *pod.Spec.ShareProcessNamespace {
			return runtimeapi.NamespaceMode_POD
		}
	}
	// Note that PID does not default to the zero value for v1.Pod
	return runtimeapi.NamespaceMode_CONTAINER
}

// namespacesForPod returns the runtimeapi.NamespaceOption for a given pod.
// An empty or nil pod can be used to get the namespace defaults for v1.Pod.
func namespacesForPod(pod *v1.Pod) *runtimeapi.NamespaceOption {
	return &runtimeapi.NamespaceOption{
		Ipc:     ipcNamespaceForPod(pod),
		Network: networkNamespaceForPod(pod),
		Pid:     pidNamespaceForPod(pod),
	}
}

func getRuntimeAndImageServices(remoteRuntimeEndpoint string, remoteImageEndpoint string, runtimeRequestTimeout metav1.Duration) (internalapi.RuntimeService, internalapi.ImageManagerService, error) {
	rs, err := remote.NewRemoteRuntimeService(remoteRuntimeEndpoint, runtimeRequestTimeout.Duration)
	if err != nil {
		return nil, nil, err
	}
	is, err := remote.NewRemoteImageService(remoteImageEndpoint, runtimeRequestTimeout.Duration)
	if err != nil {
		return nil, nil, err
	}
	return rs, is, err
}

// assumed the runtime and image service are configured with the same name at the spec
func getImageServiceNameFromPodSpec(pod *v1.Pod) *string {
	return getRuntimeServiceNameFromPodSpec(pod)
}

// TODO: for now, just return nil for now to simulate the legacy app without the runtimeServiceName specified
//       where default runtime will be used
//      replace with the runtimeServiceName after the POD spec type is updated
func getRuntimeServiceNameFromPodSpec(pod *v1.Pod) *string {
	return nil
}

// Retrieve the runtime service with PODID
func (m *kubeGenericRuntimeManager) GetRuntimeServiceByPodID(podId types.UID) (internalapi.RuntimeService, error) {
	klog.V(4).Infof("Retrieve runtime service for podID %v", podId)
	// firstly check the pod-runtimeService cache
	if runtimeService, found := m.podRuntimeServiceMap[string(podId)]; found {
		klog.V(4).Infof("Got runtime service [%v] for podID %v", runtimeService, podId)
		return runtimeService, nil
	}

	// if not found in the cache, then query the runtime services
	runtimeServices, err := m.runtimeRegistry.GetAllRuntimeServices()
	if err != nil {
		klog.Errorf("GetAllRuntimeServices failed: %v", err)
		return nil, err
	}

	var filter *runtimeapi.PodSandboxFilter

	for _, runtimeService := range runtimeServices {
		// ensure runtime is ready before query the container from runtime service. might consider retries and
		// further verify the RPC error here for refined error handling
		// continue to the rest of runtime services
		_, err := runtimeService.ServiceApi.Status()
		if err != nil {
			continue
		}

		resp, err := runtimeService.ServiceApi.ListPodSandbox(filter)
		if err != nil {
			klog.Errorf("ListPodSandbox failed: %v", err)
			return nil, err
		}

		for _, item := range resp {
			if item.Metadata.Uid == string(podId) {
				klog.V(4).Infof("Got runtime service [%v] for podID %v", runtimeService, podId)
				m.addPodRuntimeService(string(podId), runtimeService.ServiceApi)
				return runtimeService.ServiceApi, nil
			}
		}
	}

	return nil, fmt.Errorf("failed find runtimeService with podId %v", podId)
}

// GetRuntimeServiceByPod returns the runtime service for a given pod from its SPEC
// the GetRuntimeServiceByPod is called when POD is being created, i.e. the pod-runtime map and runtime service
// will not have it
func (m *kubeGenericRuntimeManager) GetRuntimeServiceByPod(pod *v1.Pod) (internalapi.RuntimeService, error) {
	klog.V(4).Infof("Retrieve runtime service for POD %s", pod.Name)
	runtimeName := getRuntimeServiceNameFromPodSpec(pod)

	if runtimeName == nil || *runtimeName == "" {
		klog.V(4).Infof("Get default runtime service for POD %s", pod.Name)
		if pod.Spec.VirtualMachine != nil {
			return m.GetDefaultRuntimeServiceForWorkload(runtimeregistry.VmworkloadType)
		} else {
			return m.GetDefaultRuntimeServiceForWorkload(runtimeregistry.ContainerWorkloadType)
		}
	}

	runtimeServices, err := m.runtimeRegistry.GetAllRuntimeServices()
	if err != nil {
		klog.Errorf("GetAllRuntimeServices failed: %v", err)
		return nil, err
	}

	if runtimeService, found := runtimeServices[*runtimeName]; found {
		klog.V(4).Infof("Got runtime service [%v] for POD %s", runtimeService, pod.Name)
		return runtimeService.ServiceApi, nil
	}

	// this should not be reached
	return nil, fmt.Errorf("cannot find specified runtime service: %s", *runtimeName)
}

// Retrieve the runtime service for a container with containerID
func (m *kubeGenericRuntimeManager) GetRuntimeServiceByContainerID(id kubecontainer.ContainerID) (internalapi.RuntimeService, error) {

	return m.GetRuntimeServiceByContainerIDString(id.ID)
}

// TODO: build pod-container relationship map and get the runtime service from the pod-runtimeService map first
func (m *kubeGenericRuntimeManager) GetRuntimeServiceByContainerIDString(id string) (internalapi.RuntimeService, error) {
	klog.V(4).Infof("Retrieve runtime service for containerID %s", id)
	runtimeServices, err := m.runtimeRegistry.GetAllRuntimeServices()
	if err != nil {
		klog.Errorf("GetAllRuntimeServices failed: %v", err)
		return nil, err
	}

	var filter *runtimeapi.ContainerFilter

	for _, runtimeService := range runtimeServices {
		// ensure runtime is ready before query the container from runtime service. might consider retries and
		// further verify the RPC error here for refined error handling
		// continue to the rest of runtime services
		_, err := runtimeService.ServiceApi.Status()
		if err != nil {
			continue
		}

		resp, err := runtimeService.ServiceApi.ListContainers(filter)
		if err != nil {
			klog.Errorf("ListContainers failed: %v", err)
			return nil, err
		}

		for _, item := range resp {
			if item.Id == id {
				klog.V(4).Infof("Got runtime service [%v] for containerID %s", runtimeService, id)
				return runtimeService.ServiceApi, nil
			}
		}
	}

	return nil, ErrContainerDoesNotExistAtRuntimeService
}

// GetAllRuntimeServices returns all the runtime services.
// TODO: dedup the slice elements OR ensure the buildRuntimeService method does the dedup logic
//       cases as: runtimeName1:EndpointUrl1;runtimeName2:EndpointUrl2;runtimeName3:EndpointUrl2
//                 GetAllRuntimeServices should return array of EndpointUrl1 and EndpointUrl2
//
func (m *kubeGenericRuntimeManager) GetDefaultRuntimeServiceForWorkload(workloadType string) (internalapi.RuntimeService, error) {
	runtimeServices, err := m.runtimeRegistry.GetAllRuntimeServices()
	if err != nil {
		klog.Errorf("GetAllRuntimeServices failed: %v", err)
		return nil, err
	}

	for _, service := range runtimeServices {
		if service.WorkloadType == workloadType && service.IsDefault {
			klog.V(4).Infof("Got default runtime service [%v] for workload type %s", service.ServiceApi, workloadType)
			return service.ServiceApi, nil
		}
	}

	return nil, fmt.Errorf("cannot find default runtime service for worload type: %s", workloadType)
}

// Retrieve the image service for a POD with the POD SPEC
func (m *kubeGenericRuntimeManager) GetImageServiceByPod(pod *v1.Pod) (internalapi.ImageManagerService, error) {
	klog.V(4).Infof("Retrieve image service for POD %s", pod.Name)
	runtimeName := getImageServiceNameFromPodSpec(pod)

	if runtimeName == nil || *runtimeName == "" {
		klog.V(4).Infof("Get default image service for POD %s", pod.Name)
		if pod.Spec.VirtualMachine != nil {
			return m.GetDefaultImageServiceForWorkload(runtimeregistry.VmworkloadType)
		} else {
			return m.GetDefaultImageServiceForWorkload(runtimeregistry.ContainerWorkloadType)
		}
	}

	imageServices, err := m.runtimeRegistry.GetAllImageServices()
	if err != nil {
		klog.Errorf("GetAllImageServices failed: %v", err)
		return nil, err
	}

	if imageService, found := imageServices[*runtimeName]; found {
		klog.V(4).Infof("Got image service [%v] for POD %s", imageService, pod.Name)
		return imageService.ServiceApi, nil
	}

	// this should not be reached
	return nil, fmt.Errorf("cannot find specified image service: %s", *runtimeName)
}

func (m *kubeGenericRuntimeManager) GetDefaultImageServiceForWorkload(workloadType string) (internalapi.ImageManagerService, error) {
	imageService, err := m.runtimeRegistry.GetImageServiceByWorkloadType(workloadType)
	if err != nil {
		klog.Errorf("GetAllImageServices failed: %v", err)
		return nil, err
	}

	return imageService.ServiceApi, nil
}
