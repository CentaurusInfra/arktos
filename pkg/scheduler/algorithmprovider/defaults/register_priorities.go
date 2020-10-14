/*
Copyright 2018 The Kubernetes Authors.
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

package defaults

import (
	"k8s.io/kubernetes/pkg/scheduler/algorithm/priorities"
	"k8s.io/kubernetes/pkg/scheduler/core"
	"k8s.io/kubernetes/pkg/scheduler/factory"
)

func init() {
	// Register functions that extract metadata used by priorities computations.
	factory.RegisterPriorityMetadataProducerFactory(
		func(args factory.PluginFactoryArgs) priorities.PriorityMetadataProducer {
			return priorities.NewPriorityMetadataFactory(args.ServiceLister, args.ControllerLister, args.ReplicaSetLister, args.StatefulSetLister)
		})

	// EqualPriority is a prioritizer function that gives an equal weight of one to all nodes
	// Register the priority function so that its available
	// but do not include it as part of the default priorities
	factory.RegisterPriorityFunction2(priorities.EqualPriority, core.EqualPriorityMap, nil, 1)
	// Optional, cluster-autoscaler friendly priority function - give used nodes higher priority.
	factory.RegisterPriorityFunction2(
		priorities.RequestedToCapacityRatioPriority,
		priorities.RequestedToCapacityRatioResourceAllocationPriorityDefault().PriorityMap,
		nil,
		1)

	// Prioritize nodes by least requested utilization.
	factory.RegisterPriorityFunction2(priorities.LeastRequestedPriority, priorities.LeastRequestedPriorityMap, nil, 1)

	// Prioritizes nodes to help achieve balanced resource usage
	factory.RegisterPriorityFunction2(priorities.BalancedResourceAllocation, priorities.BalancedResourceAllocationMap, nil, 1)
}
