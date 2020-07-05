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

package podConverter

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"k8s.io/api/core/v1"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/kubelet/container"
	"strconv"
)

// VM related annotations in current virtlet release
// from https://github.com/Mirantis/virtlet/blob/master/pkg/metadata/types/annotations.go
const (
	vcpuCountAnnotationKeyName             = "VirtletVCPUCount"
	diskDriverKeyName                      = "VirtletDiskDriver"
	cloudInitMetaDataKeyName               = "VirtletCloudInitMetaData"
	cloudInitUserDataOverwriteKeyName      = "VirtletCloudInitUserDataOverwrite"
	cloudInitUserDataKeyName               = "VirtletCloudInitUserData"
	cloudInitUserDataScriptKeyName         = "VirtletCloudInitUserDataScript"
	cloudInitImageType                     = "VirtletCloudInitImageType"
	cpuModel                               = "VirtletCPUModel"
	rootVolumeSizeKeyName                  = "VirtletRootVolumeSize"
	libvirtCPUSetting                      = "VirtletLibvirtCPUSetting"
	sshKeysKeyName                         = "VirtletSSHKeys"
	chown9pfsMountsKeyName                 = "VirtletChown9pfsMounts"
	systemUUIDKeyName                      = "VirtletSystemUUID"
	forceDHCPNetworkConfigKeyName          = "VirtletForceDHCPNetworkConfig"
	CloudInitUserDataSourceKeyName         = "VirtletCloudInitUserDataSource"
	SSHKeySourceKeyName                    = "VirtletSSHKeySource"
	cloudInitUserDataSourceKeyKeyName      = "VirtletCloudInitUserDataSourceKey"
	cloudInitUserDataSourceEncodingKeyName = "VirtletCloudInitUserDataSourceEncoding"
	FilesFromDSKeyName                     = "VirtletFilesFromDataSource"
)

const (
	defaultVirtletRootVolumeSize = "4Gi"
	VirtletRuntimeKeyName        = "kubernetes.io/target-runtime"
	// the criproxy uses for the virtlet runtime endpoint prefix
	defaultVirtletRuntimeValue = "virtlet.cloud"

	VPCKeyName  = "VPC"
	NICsKeyName = "NICs"
)

const (
	stringEmpty = ""
)

// Convert a CloudFabric VM pod to virtlet recognizable container based pod with annotations of VM info
// This is an approach for Cloud Fabric 830 release to support VM workload types and integration with
// replicaset controller, networking etc effort
//
func ConvertVmPodToContainerPod(pod *v1.Pod) *v1.Pod {
	if pod.Spec.VirtualMachine == nil {
		fmt.Errorf("invalid vm workload pod")
		return nil
	}
	cpod := pod.DeepCopy()

	// default virtlet annotations
	if cpod.Annotations == nil {
		cpod.Annotations = make(map[string]string)
	}

	cpod.Annotations[rootVolumeSizeKeyName] = defaultVirtletRootVolumeSize
	cpod.Annotations[VirtletRuntimeKeyName] = defaultVirtletRuntimeValue

	// setup the annotations from the pod.podspec.virtualMachine
	// TODO: add more per the need for more VM types
	if pod.Spec.VirtualMachine.PublicKey != stringEmpty {
		cpod.Annotations[sshKeysKeyName] = cpod.Spec.VirtualMachine.PublicKey
	}
	if pod.Spec.VirtualMachine.UserData != nil {
		userData, err := base64.StdEncoding.DecodeString(string(pod.Spec.VirtualMachine.UserData))
		if err != nil {
			fmt.Errorf("failed to get userData with error: %v", err)
			return nil
		}
		cpod.Annotations[cloudInitUserDataKeyName] = string(userData)
	}
	if pod.Spec.VirtualMachine.CloudInitUserDataScript != "" {
		cpod.Annotations[cloudInitUserDataScriptKeyName] = pod.Spec.VirtualMachine.CloudInitUserDataScript
	}

	if pod.Spec.VPC != stringEmpty {
		cpod.Annotations[VPCKeyName] = pod.Spec.VPC
	}
	if pod.Spec.Nics != nil {
		s, err := json.Marshal(pod.Spec.Nics)
		if err != nil {
			fmt.Errorf("failed to get Nics with error: %v", err)
			return nil
		}
		cpod.Annotations[NICsKeyName] = string(s)
	}

	if vcpuCount := getVCpuCountForVm(pod.Spec.VirtualMachine); vcpuCount != 0 {
		cpod.Annotations[vcpuCountAnnotationKeyName] = strconv.FormatInt(vcpuCount, 10)
	}

	// create a container per the virtlet requirements
	if cpod.Spec.Containers == nil {
		cpod.Spec.Containers = []v1.Container{}
	}

	// Info under the pod.Spec.VirtualMachine shall be fully converted to Container spec.
	// Pending future podConverter design evaluation (Arktos VM spec to Libvirt domain xml).
	// for now, just ensure resources were converted along with other essentials
	cpod.Spec.Containers = []v1.Container{
		{
			Name:               pod.Spec.VirtualMachine.Name,
			Image:              pod.Spec.VirtualMachine.Image,
			ImagePullPolicy:    pod.Spec.VirtualMachine.ImagePullPolicy,
			VolumeMounts:       pod.Spec.VirtualMachine.VolumeMounts,
			VolumeDevices:      pod.Spec.VirtualMachine.VolumeDevices,
			TTY:                true,
			Stdin:              true,
			Resources:          pod.Spec.VirtualMachine.Resources,
			ResourcesAllocated: pod.Spec.VirtualMachine.ResourcesAllocated,
		},
	}

	// to have consistent way for the workload type check among the kubelet code, keep the spec.VirtualMachine object
	// the original pod and the pod used in runtime must keep the same status object
	pod.Status = cpod.Status

	return cpod
}

// Libvirt guestOS cpu resource doc:
// https://libvirt.org/docs/libvirt-appdev-guide-python/en-US/html/libvirt_application_development_guide_using_python-Guest_Domains-Domain_Config-Memory_CPU.html
// Libvirt VM domain vcpus specifies the limit of number of vcpus the guest OS can have
// Arktos vm runtime doesn't support guest OS vcpu reset currently, the VM workload should be classified as Guaranteed
// resource model.
// API validation should enforce this when accepting the VM POD to avoid resource level misalignment since scheduler uses
// requested resources in its Predicate function
//
func getVCpuCountForVm(vmSpec *v1.VirtualMachine) int64 {
	return vmSpec.Resources.Limits.Cpu().Value()
}

//TODO: remove the debug function once the feature is stabilized
func DumpPodSyncResult(result container.PodSyncResult) {
	klog.V(6).Infof("Sync results content:\n")
	klog.V(6).Infof("Error if any %v", result.SyncError)
	for _, r := range result.SyncResults {
		klog.V(6).Infof("\tSyncResult: %v", r)
	}
}
