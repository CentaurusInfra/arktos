/*
Copyright 2014 The Kubernetes Authors.
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

package app

import (
	// This file exists to force the desired plugin implementations to be linked.
	// This should probably be part of some configuration fed into the build for a
	// given binary target.

	"fmt"

	_ "k8s.io/kubernetes/pkg/cloudprovider/providers"
	// Volume plugins
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/awsebs"
	"k8s.io/kubernetes/pkg/volume/azure_dd"
	"k8s.io/kubernetes/pkg/volume/cinder"
	"k8s.io/kubernetes/pkg/volume/csi"
	"k8s.io/kubernetes/pkg/volume/fc"
	"k8s.io/kubernetes/pkg/volume/gcepd"
	"k8s.io/kubernetes/pkg/volume/iscsi"
	"k8s.io/kubernetes/pkg/volume/portworx"
	"k8s.io/kubernetes/pkg/volume/rbd"
	"k8s.io/kubernetes/pkg/volume/scaleio"
	"k8s.io/kubernetes/pkg/volume/storageos"
	volumeutil "k8s.io/kubernetes/pkg/volume/util"
	"k8s.io/kubernetes/pkg/volume/vsphere_volume"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubernetes/pkg/features"
)

// ProbeAttachableVolumePlugins collects all volume plugins for the attach/
// detach controller.
// The list of plugins is manually compiled. This code and the plugin
// initialization code for kubelet really, really need a through refactor.
func ProbeAttachableVolumePlugins() []volume.VolumePlugin {
	allPlugins := []volume.VolumePlugin{}

	allPlugins = append(allPlugins, awsebs.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, gcepd.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, cinder.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, portworx.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, vsphere_volume.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, azure_dd.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, scaleio.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, storageos.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, fc.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, iscsi.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, rbd.ProbeVolumePlugins()...)
	if utilfeature.DefaultFeatureGate.Enabled(features.CSIPersistentVolume) {
		allPlugins = append(allPlugins, csi.ProbeVolumePlugins()...)
	}
	return allPlugins
}

// AttemptToLoadRecycler tries decoding a pod from a filepath for use as a recycler for a volume.
// If successful, this method will set the recycler on the config.
// If unsuccessful, an error is returned. Function is exported for reuse downstream.
func AttemptToLoadRecycler(path string, config *volume.VolumeConfig) error {
	if path != "" {
		recyclerPod, err := volumeutil.LoadPodFromFile(path)
		if err != nil {
			return err
		}
		if err = volume.ValidateRecyclerPodTemplate(recyclerPod); err != nil {
			return fmt.Errorf("pod specification (%v): %v", path, err)
		}
		config.RecyclerPodTemplate = recyclerPod
	}
	return nil
}
