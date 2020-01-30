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

package kuberuntime

import (
	"k8s.io/api/core/v1"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
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

func (m *kubeGenericRuntimeManager) CreateSnapshot(pod *v1.Pod, vmName string, snapshotID string) error {
	klog.V(4).Infof("Create snapshot %s for Pod-VM %s-%s", snapshotID, pod.Name, vmName)

	runtimeService, _ := m.GetRuntimeServiceByPod(pod)

	// get the containerId for the vm from pod and vm name
	containerID, err := m.containerRefManager.GetContainerIDByRef(pod)
	if err != nil {
		klog.V(4).Infof("failed getting containerID. error %v", err)
		return err
	}
	klog.V(4).Infof("Retrieved containerID %v for VM %s", containerID, vmName)

	//use the default flag 0
	return runtimeService.CreateSnapshot(containerID.ID, snapshotID, 0)
}

func (m *kubeGenericRuntimeManager) RestoreToSnapshot(pod *v1.Pod, vmName string, snapshotID string) error {
	klog.V(4).Infof("Restore to snapshot %s for Pod-VM %s-%s", snapshotID, pod.Name, vmName)

	runtimeService, _ := m.GetRuntimeServiceByPod(pod)

	// get the containerId for the vm from pod and vm name
	containerID, err := m.containerRefManager.GetContainerIDByRef(pod)
	if err != nil {
		klog.V(4).Infof("failed getting containerID. error %v", err)
		return err
	}
	klog.V(4).Infof("Retrieved containerID %v for VM %s", containerID, vmName)

	//use the default flag 0
	return runtimeService.RestoreToSnapshot(containerID.ID, snapshotID, 0)
}

// VM service interface methods
func (m *kubeGenericRuntimeManager) AttachNetworkInterface(pod *v1.Pod, vmName string, nic *v1.Nic) error {
	klog.V(4).Infof("Attaching NIC %v to Pod-VM %s-%s", nic, pod.Name, vmName)

	runtimeService, _ := m.GetRuntimeServiceByPod(pod)
	return runtimeService.AttachNetworkInterface(pod.Name, vmName, KubeNicToRuntimeNic(nic))
}

func (m *kubeGenericRuntimeManager) DetachNetworkInterface(pod *v1.Pod, vmName string, nic *v1.Nic) error {
	klog.V(4).Infof("Detaching NIC %v to Pod-VM %s-%s", nic, pod.Name, vmName)

	runtimeService, _ := m.GetRuntimeServiceByPod(pod)
	return runtimeService.DetachNetworkInterface(pod.Name, vmName, KubeNicToRuntimeNic(nic))
}

func (m *kubeGenericRuntimeManager) ListNetworkInterfaces(pod *v1.Pod, vmName string) ([]*v1.Nic, error) {
	klog.V(4).Infof("Listing NICs attached to Pod-VM %s-%s", pod.Name, vmName)

	runtimeService, _ := m.GetRuntimeServiceByPod(pod)
	runtimeNics, err := runtimeService.ListNetworkInterfaces(pod.Name, vmName)

	if err != nil {
		return nil, err
	}

	kubeNics := make([]*v1.Nic, len(runtimeNics))
	for i, nic := range runtimeNics {
		kubeNics[i] = RuntimeNicToKubeNic(nic)
	}
	return kubeNics, nil
}

func RuntimeNicToKubeNic(runtimeNic *runtimeapi.NicSpec) *v1.Nic {
	return &v1.Nic{Name: runtimeNic.Name, SubnetName: runtimeNic.SubnetName, PortId: runtimeNic.PortId, IpAddress: runtimeNic.IpAddress}
}

func KubeNicToRuntimeNic(kubeNic *v1.Nic) *runtimeapi.NicSpec {
	return &runtimeapi.NicSpec{Name: kubeNic.Name, SubnetName: kubeNic.SubnetName, PortId: kubeNic.PortId, IpAddress: kubeNic.IpAddress}
}
