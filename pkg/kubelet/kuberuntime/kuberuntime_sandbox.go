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
	"encoding/json"
	"fmt"
	"k8s.io/api/core/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/features"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/kubelet/util/format"
	"net"
	"net/url"
	"sort"
)

const (
	VPCKeyName  = "VPC"
	NICsKeyName = "NICs"
)

// createPodSandbox creates a pod sandbox and returns (podSandBoxID, message, error).
func (m *kubeGenericRuntimeManager) createPodSandbox(pod *v1.Pod, attempt uint32) (string, string, error) {
	podSandboxConfig, err := m.generatePodSandboxConfig(pod, attempt)
	if err != nil {
		message := fmt.Sprintf("GeneratePodSandboxConfig for pod %q failed: %v", format.Pod(pod), err)
		klog.Error(message)
		return "", message, err
	}

	// Create pod logs directory
	err = m.osInterface.MkdirAll(podSandboxConfig.LogDirectory, 0755)
	if err != nil {
		message := fmt.Sprintf("Create pod log directory for pod %q failed: %v", format.Pod(pod), err)
		klog.Errorf(message)
		return "", message, err
	}

	runtimeHandler := ""
	if utilfeature.DefaultFeatureGate.Enabled(features.RuntimeClass) && m.runtimeClassManager != nil {
		runtimeHandler, err = m.runtimeClassManager.LookupRuntimeHandler(pod.Spec.RuntimeClassName)
		if err != nil {
			message := fmt.Sprintf("CreatePodSandbox for pod %q failed: %v", format.Pod(pod), err)
			return "", message, err
		}
		if runtimeHandler != "" {
			klog.V(2).Infof("Running pod %s with RuntimeHandler %q", format.Pod(pod), runtimeHandler)
		}
	}

	// debugging dump the PodSandboxConfig
	klog.V(6).Infof("PodSandboxConfig: %s", podSandboxConfig.String())

	runtimeService, err := m.GetRuntimeServiceByPod(pod)
	if err != nil {
		message := fmt.Sprintf("GetRuntimeService for pod %q failed: %v", format.Pod(pod), err)
		klog.Error(message)
		return "", message, err
	}

	podSandBoxID, err := runtimeService.RunPodSandbox(podSandboxConfig, runtimeHandler)

	if err != nil {
		message := fmt.Sprintf("CreatePodSandbox for pod %q failed: %v", format.Pod(pod), err)
		klog.Error(message)
		return "", message, err
	}

	// add ppd-runtimeService cache
	m.addPodRuntimeService(string(pod.UID), runtimeService)
	return podSandBoxID, "", nil
}

// generatePodSandboxConfig generates pod sandbox config from v1.Pod.
func (m *kubeGenericRuntimeManager) generatePodSandboxConfig(pod *v1.Pod, attempt uint32) (*runtimeapi.PodSandboxConfig, error) {
	// TODO: deprecating podsandbox resource requirements in favor of the pod level cgroup
	// Refer https://github.com/kubernetes/kubernetes/issues/29871
	podUID := string(pod.UID)
	podSandboxConfig := &runtimeapi.PodSandboxConfig{
		Metadata: &runtimeapi.PodSandboxMetadata{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Tenant:    pod.Tenant,
			Uid:       podUID,
			Attempt:   attempt,
		},
		Labels:      newPodLabels(pod),
		Annotations: newPodAnnotations(pod),
	}

	// Arktos add VPC, NICs info as podspec, so add it to the SandboxConfig as annotations so the runtime service can use them
	// TODO: if the podConverter is kept for long term in Arktos, consider consolidate the set annotation code with it
	if podSandboxConfig.Annotations == nil {
		podSandboxConfig.Annotations = make(map[string]string)
	}
	if pod.Spec.VPC != "" {
		if _, found := podSandboxConfig.Annotations[VPCKeyName]; !found {
			podSandboxConfig.Annotations[VPCKeyName] = pod.Spec.VPC
		}
	}
	if pod.Spec.Nics != nil {
		if _, found := podSandboxConfig.Annotations[NICsKeyName]; !found {
			s, err := json.Marshal(pod.Spec.Nics)
			if err != nil {
				return nil, err
			}
			podSandboxConfig.Annotations[NICsKeyName] = string(s)
		}
	}

	dnsConfig, err := m.runtimeHelper.GetPodDNS(pod)
	if err != nil {
		return nil, err
	}
	podSandboxConfig.DnsConfig = dnsConfig

	if !kubecontainer.IsHostNetworkPod(pod) {
		// TODO: Add domain support in new runtime interface
		hostname, _, err := m.runtimeHelper.GeneratePodHostNameAndDomain(pod)
		if err != nil {
			return nil, err
		}
		podSandboxConfig.Hostname = hostname
	}

	logDir := BuildPodLogsDirectory(pod.Tenant, pod.Namespace, pod.Name, pod.UID)
	podSandboxConfig.LogDirectory = logDir

	portMappings := []*runtimeapi.PortMapping{}
	for _, c := range pod.Spec.Containers {
		containerPortMappings := kubecontainer.MakePortMappings(&c)

		for idx := range containerPortMappings {
			port := containerPortMappings[idx]
			hostPort := int32(port.HostPort)
			containerPort := int32(port.ContainerPort)
			protocol := toRuntimeProtocol(port.Protocol)
			portMappings = append(portMappings, &runtimeapi.PortMapping{
				HostIp:        port.HostIP,
				HostPort:      hostPort,
				ContainerPort: containerPort,
				Protocol:      protocol,
			})
		}

	}
	if len(portMappings) > 0 {
		podSandboxConfig.PortMappings = portMappings
	}

	lc, err := m.generatePodSandboxLinuxConfig(pod)
	if err != nil {
		return nil, err
	}
	podSandboxConfig.Linux = lc

	return podSandboxConfig, nil
}

// generatePodSandboxLinuxConfig generates LinuxPodSandboxConfig from v1.Pod.
func (m *kubeGenericRuntimeManager) generatePodSandboxLinuxConfig(pod *v1.Pod) (*runtimeapi.LinuxPodSandboxConfig, error) {
	cgroupParent := m.runtimeHelper.GetPodCgroupParent(pod)
	lc := &runtimeapi.LinuxPodSandboxConfig{
		CgroupParent: cgroupParent,
		SecurityContext: &runtimeapi.LinuxSandboxSecurityContext{
			Privileged:         kubecontainer.HasPrivilegedContainer(pod),
			SeccompProfilePath: m.getSeccompProfileFromAnnotations(pod.Annotations, ""),
		},
	}

	sysctls := make(map[string]string)
	if utilfeature.DefaultFeatureGate.Enabled(features.Sysctls) {
		if pod.Spec.SecurityContext != nil {
			for _, c := range pod.Spec.SecurityContext.Sysctls {
				sysctls[c.Name] = c.Value
			}
		}
	}

	lc.Sysctls = sysctls

	if pod.Spec.SecurityContext != nil {
		sc := pod.Spec.SecurityContext
		if sc.RunAsUser != nil {
			lc.SecurityContext.RunAsUser = &runtimeapi.Int64Value{Value: int64(*sc.RunAsUser)}
		}
		if sc.RunAsGroup != nil {
			lc.SecurityContext.RunAsGroup = &runtimeapi.Int64Value{Value: int64(*sc.RunAsGroup)}
		}
		lc.SecurityContext.NamespaceOptions = namespacesForPod(pod)

		if sc.FSGroup != nil {
			lc.SecurityContext.SupplementalGroups = append(lc.SecurityContext.SupplementalGroups, int64(*sc.FSGroup))
		}
		if groups := m.runtimeHelper.GetExtraSupplementalGroupsForPod(pod); len(groups) > 0 {
			lc.SecurityContext.SupplementalGroups = append(lc.SecurityContext.SupplementalGroups, groups...)
		}
		if sc.SupplementalGroups != nil {
			for _, sg := range sc.SupplementalGroups {
				lc.SecurityContext.SupplementalGroups = append(lc.SecurityContext.SupplementalGroups, int64(sg))
			}
		}
		if sc.SELinuxOptions != nil {
			lc.SecurityContext.SelinuxOptions = &runtimeapi.SELinuxOption{
				User:  sc.SELinuxOptions.User,
				Role:  sc.SELinuxOptions.Role,
				Type:  sc.SELinuxOptions.Type,
				Level: sc.SELinuxOptions.Level,
			}
		}
	}

	return lc, nil
}

// getKubeletSandboxes lists all (or just the running) sandboxes managed by kubelet.
func (m *kubeGenericRuntimeManager) getKubeletSandboxes(all bool) ([]*runtimeapi.PodSandbox, error) {
	runtimeServices, err := m.runtimeRegistry.GetAllRuntimeServices()
	if err != nil {
		klog.Errorf("GetAllRuntimeServices failed: %v", err)
		return nil, err
	}

	var resps []*runtimeapi.PodSandbox

	for _, runtimeService := range runtimeServices {
		resp, err := m.getKubeletSandboxesByRuntime(runtimeService.ServiceApi, all)
		if err != nil {
			klog.Errorf("getKubeletSandboxesByRuntime failed: %v", err)
			continue
		}

		resps = append(resps, resp...)
	}

	return resps, nil
}

func (m *kubeGenericRuntimeManager) getKubeletSandboxesByRuntime(runtimeService internalapi.RuntimeService, all bool) ([]*runtimeapi.PodSandbox, error) {
	var filter *runtimeapi.PodSandboxFilter
	if !all {
		readyState := runtimeapi.PodSandboxState_SANDBOX_READY
		filter = &runtimeapi.PodSandboxFilter{
			State: &runtimeapi.PodSandboxStateValue{
				State: readyState,
			},
		}
	}

	if runtimeService == nil {
		klog.Errorf("runtimeService is not initialized")
		return nil, fmt.Errorf("runtimeService not initialized")
	}

	resp, err := runtimeService.ListPodSandbox(filter)
	if err != nil {
		klog.Errorf("ListPodSandbox failed: %v", err)
		return nil, err
	}

	return resp, nil
}

// determinePodSandboxIP determines the IP address of the given pod sandbox.
func (m *kubeGenericRuntimeManager) determinePodSandboxIP(podTenant, podNamespace, podName string, podSandbox *runtimeapi.PodSandboxStatus) string {
	if podSandbox.Network == nil {
		klog.Warningf("Pod Sandbox status doesn't have network information, cannot report IP")
		return ""
	}
	ip := podSandbox.Network.Ip
	if len(ip) != 0 && net.ParseIP(ip) == nil {
		// ip could be an empty string if runtime is not responsible for the
		// IP (e.g., host networking).
		klog.Warningf("Pod Sandbox reported an unparseable IP %v", ip)
		return ""
	}
	return ip
}

// getPodSandboxID gets the sandbox id by podUID and returns ([]sandboxID, error).
// Param state could be nil in order to get all sandboxes belonging to same pod.
func (m *kubeGenericRuntimeManager) getSandboxIDByPodUID(podUID kubetypes.UID, state *runtimeapi.PodSandboxState) ([]string, error) {
	filter := &runtimeapi.PodSandboxFilter{
		LabelSelector: map[string]string{types.KubernetesPodUIDLabel: string(podUID)},
	}
	if state != nil {
		filter.State = &runtimeapi.PodSandboxStateValue{
			State: *state,
		}
	}

	runtimeService, err := m.GetRuntimeServiceByPodID(podUID)
	if err != nil {
		return nil, err
	}

	sandboxes, err := runtimeService.ListPodSandbox(filter)
	if err != nil {
		klog.Errorf("ListPodSandbox with pod UID %q failed: %v", podUID, err)
		return nil, err
	}

	if len(sandboxes) == 0 {
		return nil, nil
	}

	// Sort with newest first.
	sandboxIDs := make([]string, len(sandboxes))
	sort.Sort(podSandboxByCreated(sandboxes))
	for i, s := range sandboxes {
		sandboxIDs[i] = s.Id
	}

	return sandboxIDs, nil
}

// GetPortForward gets the endpoint the runtime will serve the port-forward request from.
func (m *kubeGenericRuntimeManager) GetPortForward(podName, podNamespace, podTenant string, podUID kubetypes.UID, ports []int32) (*url.URL, error) {
	sandboxIDs, err := m.getSandboxIDByPodUID(podUID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find sandboxID for pod %s: %v", format.PodDesc(podName, podNamespace, podTenant, podUID), err)
	}
	if len(sandboxIDs) == 0 {
		return nil, fmt.Errorf("failed to find sandboxID for pod %s", format.PodDesc(podName, podNamespace, podTenant, podUID))
	}
	req := &runtimeapi.PortForwardRequest{
		PodSandboxId: sandboxIDs[0],
		Port:         ports,
	}

	runtimeService, err := m.GetRuntimeServiceByPodID(podUID)
	if err != nil {
		return nil, err
	}
	resp, err := runtimeService.PortForward(req)
	if err != nil {
		return nil, err
	}
	return url.Parse(resp.Url)
}
