/*
Copyright 2015 The Kubernetes Authors.

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
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	dockertypes "github.com/docker/docker/api/types"
	dockerapi "github.com/docker/docker/client"
	"k8s.io/api/core/v1"
	"k8s.io/klog"
)

func GetVirtletLibvirtContainer() (*dockerapi.Client, *dockertypes.Container, error) {
	//TODO: This is OK for demo, but use CRI instead of docker direct.
	//TODO: Don't do this everytime - store it in KL
	dockercli, err := dockerapi.NewClientWithOpts(dockerapi.FromEnv)
	if err != nil {
		return nil, nil, err
	}

	containers, err := dockercli.ContainerList(context.Background(), dockertypes.ContainerListOptions{})
	if err != nil {
		return nil, nil, err
	}

	i := 0
	found := false
	for i = range containers {
		if strings.Contains(strings.Join(containers[i].Names, " "), "libvirt_virtlet") {
			found = true
			break
		}
	}

	if !found {
		return nil, nil, errors.New("virtlet libvirt container not found")
	}

	return dockercli, &containers[i], nil
}

func GetVirtletVMDomainID(pod *v1.Pod) (string, error) {
	podVMContainer := pod.Status.ContainerStatuses[0]
	re := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}`)
	domainVMID := re.FindStringSubmatch(podVMContainer.ContainerID)
	if len(domainVMID) != 1 {
		return "", errors.New(fmt.Sprintf("Could not find domain VM ID from virtlet container %s", podVMContainer.ContainerID))
	}
	domainID := fmt.Sprintf("virtlet-%s-%s", domainVMID[0], podVMContainer.Name)
	return domainID, nil
}

func (kl *Kubelet) DoRebootVM(pod *v1.Pod) error {
	klog.V(4).Infof("Rebooting VM %s-%s", pod.Name, pod.Spec.Workloads()[0].Name)
	if err := kl.containerRuntime.RebootVM(pod, pod.Spec.Workloads()[0].Name); err != nil {
		klog.V(4).Infof("Failed Reboot VM %s-%s. with error: %v", pod.Name, pod.Spec.Workloads()[0].Name, err)
		return err
	}

	klog.V(4).Infof("Successfully rebooted VM %s-%s", pod.Name, pod.Spec.Workloads()[0].Name)
	return nil
}

func DoSnapshotVirtletVM(pod *v1.Pod, snapshotName string) (string, error) {
	var snapshotID string
	if _, libvirtContainer, err := GetVirtletLibvirtContainer(); err != nil {
		return "", err
	} else {
		domainID, derr := GetVirtletVMDomainID(pod)
		if derr != nil {
			return "", derr
		}
		//TODO: Figure out how to use dockercli exec instead of os.exec
		klog.V(4).Infof("Snapshotting virtlet VM %s", domainID)
		cmd := "docker"
		cmdArgs := []string{"exec", "-i", libvirtContainer.ID, "virsh", "snapshot-create-as", domainID}
		if snapshotName != "" {
			cmdArgs = append(cmdArgs, "--name", snapshotName)
		}
		out, err := exec.Command(cmd, cmdArgs...).CombinedOutput()
		if err != nil {
			klog.Errorf("Libvirt container command failed. Error: %+v - %+v", err, out)
			return "", errors.New(fmt.Sprintf("Libvirt container command failed for pod %s. Error: %s - %s", pod.Name, err.Error(), out))
		}
		klog.V(4).Infof("Snapshot virtlet VM command output: %s", out)
		re := regexp.MustCompile(` `)
		snapshotID = re.Split(string(out), -1)[2]
	}
	return snapshotID, nil
}

func DoRestoreVirtletVM(pod *v1.Pod, snapshotID string) error {
	if _, libvirtContainer, err := GetVirtletLibvirtContainer(); err != nil {
		return err
	} else {
		domainID, derr := GetVirtletVMDomainID(pod)
		if derr != nil {
			return derr
		}
		//TODO: Figure out how to use dockercli exec instead of os.exec
		klog.V(4).Infof("Restoring virtlet VM %s to snapshot %s", domainID, snapshotID)
		cmd := "docker"
		cmdArgs := []string{"exec", "-i", libvirtContainer.ID, "virsh", "snapshot-revert", domainID, "--snapshotname", snapshotID}
		out, err := exec.Command(cmd, cmdArgs...).CombinedOutput()
		if err != nil {
			klog.Errorf("Libvirt container command failed. Error: %+v - %+v", err, out)
			return errors.New(fmt.Sprintf("Libvirt container command failed for pod %s. Error: %s - %s", pod.Name, err.Error(), out))
		}
		klog.V(4).Infof("Revert virtlet VM command output: %s", out)
	}
	return nil
}

func (kl *Kubelet) DoPodAction(action *v1.Action, pod *v1.Pod) {
	var err error
	var errStr string
	podActionStatus := &v1.PodActionStatus{
		PodName: action.Spec.PodAction.PodName,
	}

	actionOp := strings.Split(action.Name, "-")[0]
	switch actionOp {
	case string(v1.RebootOp):
		// Reboot (VM) Pod
		if err = kl.DoRebootVM(pod); err == nil {
			klog.V(4).Infof("Performed reboot action for Pod %s", action.Spec.PodAction.PodName)
			podActionStatus.RebootStatus = &v1.RebootStatus{RebootSuccessful: true}
		}
	case string(v1.SnapshotOp):
		// Take snapshot of (VM) Pod
		var snapshotID string
		if snapshotID, err = DoSnapshotVirtletVM(pod, action.Spec.PodAction.SnapshotAction.SnapshotName); err == nil {
			klog.V(4).Infof("Performed snapshot action for Pod %s", action.Spec.PodAction.PodName)
			podActionStatus.SnapshotStatus = &v1.SnapshotStatus{SnapshotID: snapshotID}
		}
	case string(v1.RestoreOp):
		// Restore (VM) Pod to specified snapshot ID
		if err = DoRestoreVirtletVM(pod, action.Spec.PodAction.RestoreAction.SnapshotID); err == nil {
			klog.V(4).Infof("Performed restore action for Pod %s", action.Spec.PodAction.PodName)
			podActionStatus.RestoreStatus = &v1.RestoreStatus{RestoreSuccessful: true}
		}
	default:
		errStr = fmt.Sprintf("Action %s is not supported", action.Name)
	}

	if err != nil {
		errStr = err.Error()
	}
	if errStr != "" {
		klog.V(2).Infof("Action %s for Pod %s failed. Error: %s", action.Name, action.Spec.PodAction.PodName, errStr)
	}
	action.Status = v1.ActionStatus{
		Complete:        true,
		PodActionStatus: podActionStatus,
		Error:           errStr,
	}
	if _, err := kl.kubeClient.CoreV1().Actions(action.Namespace).UpdateStatus(action); err != nil {
		klog.Errorf("Update Action status for %s failed. Error: %+v", action.Name, err)
	}
}
