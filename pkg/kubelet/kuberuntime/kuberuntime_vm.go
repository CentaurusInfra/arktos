/*
Copyright 2016 The Kubernetes Authors.

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
	"k8s.io/api/core/v1"
	"k8s.io/klog"
)

// RebootVM reboot the VM OS in the pod
func (m *kubeGenericRuntimeManager) RebootVM(pod *v1.Pod, vmName string) error {
	klog.V(4).Infof("Rebooting VM container %s", vmName)

	// TODO: consider to have a container life cycle stage for this, i.e. the OS process
	//       is rebooted.
	// Reboot the VM the container.
	runtimeService, _ := m.GetRuntimeServiceByPod(pod)

	// get the containerId for the vm from pod and vm name
	containerID, err := m.containerRefManager.GetContainerIDByRef(pod)
	if err != nil {
		klog.V(4).Infof("failed getting containerID. error %v", err)
		return err
	}
	klog.V(4).Infof("Retrieved containerID %v for VM %s", containerID, vmName)

	return runtimeService.RebootVM(containerID.ID)
}
