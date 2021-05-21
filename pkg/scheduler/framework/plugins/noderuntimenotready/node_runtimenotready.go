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

package noderuntimenotready

import (
	"context"
	"k8s.io/klog"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	"k8s.io/kubernetes/pkg/scheduler/nodeinfo"
)

// NodeRuntimeNotReady is a plugin that priorities nodes according to the node runtime ready or not
type NodeRuntimeNotReady struct {
}

var _ framework.FilterPlugin = &NodeRuntimeNotReady{}

// Name is the name of the plugin used in the plugin registry and configurations.
const Name = "NodeRuntimeNotReady"

const (
	// ErrReasonUnknownCondition is used for NodeUnknownCondition predicate error.
	ErrReasonUnknownCondition = "node(s) had unknown conditions"
	// ErrNodeRuntimeNotReady is used for CheckNodeRuntimeNotReady predicate error.
	ErrNodeRuntimeNotReady = "node(s) runtime is not ready"

	// noSchedulerToleration of pods will be respected by runtime readiness predicate
)

var noScheduleToleration = v1.Toleration{
	Operator: "Exists",
	Effect:   v1.TaintEffectNoSchedule,
}

func (pl *NodeRuntimeNotReady) Name() string {
	return Name
}

func (pl *NodeRuntimeNotReady) Filter(ctx context.Context, _ *framework.CycleState, pod *v1.Pod, nodeInfo *nodeinfo.NodeInfo) *framework.Status {
	if nodeInfo == nil || nodeInfo.Node() == nil {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonUnknownCondition)
	}

	// any pod having toleration of Exists NoSchedule bypass the runtime readiness check
	for _, toleration := range pod.Spec.Tolerations {
		if toleration == noScheduleToleration {
			return nil
		}
	}

	var podRequestedRuntimeReady v1.NodeConditionType
	if pod.Spec.VirtualMachine == nil {
		podRequestedRuntimeReady = v1.NodeContainerRuntimeReady
	} else {
		podRequestedRuntimeReady = v1.NodeVmRuntimeReady
	}

	for _, cond := range nodeInfo.Node().Status.Conditions {
		if cond.Type == podRequestedRuntimeReady && cond.Status == v1.ConditionTrue {
			klog.V(5).Infof("Found ready node runtime condition for pod [%s], condition [%v]", pod.Name, cond)
			return nil
		}
	}

	return framework.NewStatus(framework.Unschedulable, ErrNodeRuntimeNotReady)
}

// New initializes a new plugin and returns it.
func New(_ *runtime.Unknown, _ framework.FrameworkHandle) (framework.Plugin, error) {
	return &NodeRuntimeNotReady{}, nil
}
