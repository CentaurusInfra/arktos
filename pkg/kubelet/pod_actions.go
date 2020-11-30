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

package kubelet

import (
	"fmt"
	"strings"

	"k8s.io/api/core/v1"
	"k8s.io/klog"
)

func (kl *Kubelet) DoRebootVM(pod *v1.Pod) error {
	klog.V(4).Infof("Rebooting VM %s-%s", pod.Name, pod.Spec.Workloads()[0].Name)
	if err := kl.containerRuntime.RebootVM(pod, pod.Spec.Workloads()[0].Name); err != nil {
		klog.V(4).Infof("Failed Reboot VM %s-%s. with error: %v", pod.Name, pod.Spec.Workloads()[0].Name, err)
		return err
	}

	klog.V(4).Infof("Successfully rebooted VM %s-%s", pod.Name, pod.Spec.Workloads()[0].Name)
	return nil
}

func (kl *Kubelet) DoSnapshotVM(pod *v1.Pod, snapshotID string) (string, error) {
	klog.V(4).Infof("Creating snapshot %s for VM %s-%s", snapshotID, pod.Name, pod.Spec.VirtualMachine.Name)
	if err := kl.containerRuntime.CreateSnapshot(pod, pod.Spec.VirtualMachine.Name, snapshotID); err != nil {
		klog.Errorf("Failed to create snapshot for VM %s-%s with error: %v", pod.Name, pod.Spec.VirtualMachine.Name, err)
		return "", err
	}

	klog.V(4).Infof("Successfully created snapshot %s for VM %s-%s", snapshotID, pod.Name, pod.Spec.VirtualMachine.Name)
	return snapshotID, nil
}

func (kl *Kubelet) DoRestoreVM(pod *v1.Pod, snapshotID string) error {
	klog.V(4).Infof("Restoring to snapshot %s for VM %s-%s", snapshotID, pod.Name, pod.Spec.VirtualMachine.Name)
	if err := kl.containerRuntime.RestoreToSnapshot(pod, pod.Spec.VirtualMachine.Name, snapshotID); err != nil {
		klog.Errorf("Failed to restore to snapshot %s for VM %s-%s with error: %v", snapshotID,
			pod.Name, pod.Spec.VirtualMachine.Name, err)
		return err
	}

	klog.V(4).Infof("Successfully restored to snapshot %s for VM %s-%s", snapshotID, pod.Name, pod.Spec.VirtualMachine.Name)
	return nil
}

func (kl *Kubelet) DoPodAction(action *v1.Action, pod *v1.Pod) {
	var err error
	var errStr string
	actionStatus := v1.ActionStatus{
		Complete: true,
		PodName:  action.Spec.PodName,
		Error:    errStr,
	}

	actionOp := strings.Split(action.Name, "-")[0]
	switch actionOp {
	case string(v1.RebootOp):
		// Reboot (VM) Pod
		if err = kl.DoRebootVM(pod); err == nil {
			klog.V(4).Infof("Performed reboot action for Pod %s", action.Spec.PodName)
			actionStatus.RebootStatus = &v1.RebootStatus{RebootSuccessful: true}

			// update the restart counter in pod.Status.VirtualMachineStatus
			newStatus := pod.Status.DeepCopy()
			newStatus.VirtualMachineStatus.RestartCount++
			kl.statusManager.SetPodStatus(pod, *newStatus)
		}
	case string(v1.SnapshotOp):
		// Take snapshot of (VM) Pod
		var snapshotID string
		if snapshotID, err = kl.DoSnapshotVM(pod, action.Spec.SnapshotAction.SnapshotName); err == nil {
			klog.V(4).Infof("Performed snapshot action for Pod %s", action.Spec.PodName)
			actionStatus.SnapshotStatus = &v1.SnapshotStatus{SnapshotID: snapshotID}
		}
	case string(v1.RestoreOp):
		// Restore (VM) Pod to specified snapshot ID
		if err = kl.DoRestoreVM(pod, action.Spec.RestoreAction.SnapshotID); err == nil {
			klog.V(4).Infof("Performed restore action for Pod %s", action.Spec.PodName)
			actionStatus.RestoreStatus = &v1.RestoreStatus{RestoreSuccessful: true}
		}
	default:
		errStr = fmt.Sprintf("Action %s is not supported", action.Name)
	}

	if err != nil {
		errStr = err.Error()
	}
	if errStr != "" {
		klog.V(2).Infof("Action %s for Pod %s failed. Error: %s", action.Name, action.Spec.PodName, errStr)
	}
	actionStatus.Error = errStr
	action.Status = actionStatus
	// TODO: akrtos-scale-out: this should be pod specific clientset
	//
	if _, err := kl.kubeClient[0].CoreV1().Actions(action.Namespace).UpdateStatus(action); err != nil {
		klog.Errorf("Update Action status for %s failed. Error: %+v", action.Name, err)
	}
}
