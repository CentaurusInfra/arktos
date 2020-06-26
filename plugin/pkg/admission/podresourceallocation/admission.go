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

package podresourceallocation

import (
	"fmt"
	"io"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/helper/qos"
	"k8s.io/kubernetes/pkg/auth/nodeidentifier"
	"k8s.io/kubernetes/pkg/features"
)

// PluginName is a string with the name of the plugin
const PluginName = "PodResourceAllocation"

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return NewPodResourceAllocation(), nil
	})
}

// Plugin contains the client used by the admission controller
type Plugin struct {
	*admission.Handler
	nodeidentifier nodeidentifier.NodeIdentifier
}

var _ admission.ValidationInterface = &Plugin{}

// NewPodResourceAllocation creates a new instance of the PodResourceAllocation admission controller
func NewPodResourceAllocation() *Plugin {
	return &Plugin{
		Handler:        admission.NewHandler(admission.Create, admission.Update),
		nodeidentifier: nodeidentifier.NewDefaultNodeIdentifier(),
	}
}

// Admit may mutate the pod's ResourcesAllocated or ResizePolicy fields for compatibility with older client versions
func (p *Plugin) Admit(attributes admission.Attributes, o admission.ObjectInterfaces) (err error) {
	if !utilfeature.DefaultFeatureGate.Enabled(features.InPlacePodVerticalScaling) {
		return nil
	}
	// Ignore all calls to subresources or resources other than pods.
	if len(attributes.GetSubresource()) != 0 || attributes.GetResource().GroupResource() != api.Resource("pods") {
		return nil
	}

	pod, ok := attributes.GetObject().(*api.Pod)
	if !ok {
		return apierrors.NewBadRequest("Resource was marked with kind Pod but was unable to be converted")
	}

	op := attributes.GetOperation()
	// mutation to set defaults is done in admission plugin rather than in defaults
	// because doing it here allows us to support create/update from older client versions
	switch op {
	case admission.Create:
		podQoS := qos.GetPodQOS(pod)
		for i := range pod.Spec.Workloads() {
			// set container resize policy to nil for best-effort class
			if podQoS == api.PodQOSBestEffort {
				pod.Spec.WorkloadInfo[i].ResizePolicy = nil
				continue
			}
			// set resources allocated equal to requests, if not specified
			if pod.Spec.WorkloadInfo[i].Resources.Requests != nil {
				if pod.Spec.WorkloadInfo[i].ResourcesAllocated == nil {
					pod.Spec.WorkloadInfo[i].ResourcesAllocated = make(api.ResourceList)
					for key, value := range pod.Spec.WorkloadInfo[i].Resources.Requests {
						pod.Spec.WorkloadInfo[i].ResourcesAllocated[key] = value.DeepCopy()
					}
				}
			}
			// set resize policy to defaults, if not specified
			resources := make(map[api.ResourceName]bool)
			for _, p := range pod.Spec.WorkloadInfo[i].ResizePolicy {
				resources[p.ResourceName] = true
			}
			if _, found := resources[api.ResourceCPU]; !found {
				pod.Spec.WorkloadInfo[i].ResizePolicy = append(pod.Spec.WorkloadInfo[i].ResizePolicy,
					api.ResizePolicy{
						ResourceName: api.ResourceCPU,
						Policy:       api.NoRestart,
					})
			}
			if _, found := resources[api.ResourceMemory]; !found {
				pod.Spec.WorkloadInfo[i].ResizePolicy = append(pod.Spec.WorkloadInfo[i].ResizePolicy,
					api.ResizePolicy{
						ResourceName: api.ResourceMemory,
						Policy:       api.NoRestart,
					})
			}
		}
		pod.Spec.SetWorkloads()

	case admission.Update:
		oldPod, ok := attributes.GetOldObject().(*api.Pod)
		if !ok {
			return apierrors.NewBadRequest("Resource was marked with kind Pod but was unable to be converted")
		}

		if len(pod.Spec.Workloads()) != len(oldPod.Spec.Workloads()) {
			return admission.NewForbidden(attributes, fmt.Errorf("Pod updates may not add or remove containers"))
		}
		// if ResourcesAllocated or ResizePolicy fields are being dropped due to older client versions
		// because they do not know about these new fields, then just copy the fields over
		for i, w := range pod.Spec.Workloads() {
			// ResizePolicy is never empty. If not specified by user, it is set to defaults
			if w.ResizePolicy == nil || len(w.ResizePolicy) == 0 {
				pod.Spec.WorkloadInfo[i].ResizePolicy = oldPod.Spec.Workloads()[i].ResizePolicy
			}
			// if Resources.Requests is not nil, ResourcesAllocated must be non-nil as well
			if oldPod.Spec.Workloads()[i].Resources.Requests != nil && w.ResourcesAllocated == nil {
				pod.Spec.WorkloadInfo[i].ResourcesAllocated = oldPod.Spec.Workloads()[i].ResourcesAllocated
			}
		}
		pod.Spec.SetWorkloads()
	}
	return nil
}

// Validate will deny any request to a pod's ResourcesAllocated unless the user is system node account
// It also ensures new Pods have ResourcesAllocated equal to desired Resources, if it is set
func (p *Plugin) Validate(attributes admission.Attributes, o admission.ObjectInterfaces) (err error) {
	if !utilfeature.DefaultFeatureGate.Enabled(features.InPlacePodVerticalScaling) {
		return nil
	}
	// Ignore all calls to subresources or resources other than pods.
	if len(attributes.GetSubresource()) != 0 || attributes.GetResource().GroupResource() != api.Resource("pods") {
		return nil
	}

	pod, ok := attributes.GetObject().(*api.Pod)
	if !ok {
		return apierrors.NewBadRequest("Resource was marked with kind Pod but was unable to be converted")
	}
	op := attributes.GetOperation()
	switch op {
	case admission.Create:
		// newly created pods must have ResourcesAllocated field either empty or equal to Requests
		for _, w := range pod.Spec.Workloads() {
			if w.Resources.Requests == nil {
				continue
			}
			if w.ResourcesAllocated == nil || len(w.ResourcesAllocated) != len(w.Resources.Requests) ||
				!reflect.DeepEqual(w.ResourcesAllocated, w.Resources.Requests) {
				return apierrors.NewBadRequest("Resource allocation must equal desired resources for new pod")
			}
		}

	case admission.Update:
		oldPod, ok := attributes.GetOldObject().(*api.Pod)
		if !ok {
			return apierrors.NewBadRequest("Resource was marked with kind Pod but was unable to be converted")
		}

		if len(pod.Spec.Workloads()) != len(oldPod.Spec.Workloads()) {
			return admission.NewForbidden(attributes, fmt.Errorf("Pod updates may not add or remove containers"))
		}
		// only node can update ResourcesAllocated field (for CPU and memory fields only - checked during validation)
		// also verify that node is updating ResourcesAllocated only for pods that are bound to it
		// noderestriction plugin (if enabled) ensures that node isn't modifying anything besides ResourcesAllocated
		userInfo := attributes.GetUserInfo()
		nodeName, isNode := p.nodeidentifier.NodeIdentity(userInfo)
		for i, w := range pod.Spec.Workloads() {
			if w.Resources.Requests == nil || len(w.Resources.Requests) == 0 {
				continue
			}
			if w.ResourcesAllocated != nil &&
				!reflect.DeepEqual(w.ResourcesAllocated, oldPod.Spec.Workloads()[i].ResourcesAllocated) {
				if !isNode {
					return admission.NewForbidden(attributes, fmt.Errorf("Only node can modify pod resource allocations"))
				}
				if pod.Spec.NodeName != nodeName {
					return admission.NewForbidden(attributes, fmt.Errorf("Node %s can't modify resource allocation for pod bound to node %s", nodeName, pod.Spec.NodeName))
				}
			}
		}
	}
	return nil
}
