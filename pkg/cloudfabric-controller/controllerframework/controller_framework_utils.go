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

package controllerframework

import (
	"k8s.io/api/core/v1"
	"math"
	"sort"
)

// Sort Controller Instances by controller key
func SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceMap map[string]v1.ControllerInstance) []controllerInstanceLocal {
	// copy map
	var sortedControllerInstancesLocal []controllerInstanceLocal
	for _, controllerInstance := range controllerInstanceMap {
		instance := controllerInstanceLocal{
			instanceName:  controllerInstance.Name,
			controllerKey: controllerInstance.ControllerKey,
			workloadNum:   controllerInstance.WorkloadNum,
			isLocked:      controllerInstance.IsLocked,
		}
		sortedControllerInstancesLocal = append(sortedControllerInstancesLocal, instance)
	}

	sort.Slice(sortedControllerInstancesLocal, func(i, j int) bool {
		return sortedControllerInstancesLocal[i].controllerKey < sortedControllerInstancesLocal[j].controllerKey
	})

	if len(sortedControllerInstancesLocal) > 0 {
		sortedControllerInstancesLocal[0].lowerboundKey = 0
	}

	for i := 1; i < len(sortedControllerInstancesLocal); i++ {
		sortedControllerInstancesLocal[i].lowerboundKey = sortedControllerInstancesLocal[i-1].controllerKey
	}

	return sortedControllerInstancesLocal
}

// Generate controllerKey for new controller instance. It is to find and split a scope for new controller instance.
// Scope Splitting Principles:
// 1. We always find existing scope with biggest size, and split it.
// 2. If there are more than one scope at the biggest size, we chose the one with most ongoing work load, and split it.
// 3. If both existing scope size and ongoing work are even, we choose first scope and split it.
func GenerateKey(c *ControllerBase) int64 {
	if len(c.sortedControllerInstancesLocal) == 0 {
		return math.MaxInt64
	}

	candidate := c.sortedControllerInstancesLocal[0]
	for i := 1; i < len(c.sortedControllerInstancesLocal); i++ {
		item := c.sortedControllerInstancesLocal[i]

		// There are two conditions to be met then change candidate:
		// 1. if the space is bigger
		// 2. or the space size is same, but with more work load
		// When splitting odd size space, the sub spaces has 1 difference in size. So ignore difference of 1 when comparing two spaces' size.
		// Which is said, we consider it is bigger when it's bigger more than 1, and we consider both are equal even they have diff 1.
		if item.Size() > candidate.Size()+1 ||
			(math.Abs(float64(item.Size()-candidate.Size())) <= 1 && item.workloadNum > candidate.workloadNum) {
			candidate = item
		}
	}

	spaceToSplit := candidate.controllerKey - candidate.lowerboundKey

	// Add one to space to guarantee the first half will have more than second half when space to split is not even.
	// But don't apply if the scope starting from 0 because it already got extra space from number 0.
	if spaceToSplit != math.MaxInt64 && candidate.lowerboundKey != 0 {
		spaceToSplit++
	}
	return candidate.lowerboundKey + spaceToSplit/2
}
