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
	"github.com/grafov/bcast"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"math"
	"sort"
)

// Sort Controller Instances
func sortControllerInstancesByKeyAndConvertToLocal(controllerInstanceMap map[string]v1.ControllerInstance) []controllerInstanceLocal {
	// copy map
	var sortedControllerInstancesLocal []controllerInstanceLocal
	for _, controllerInstance := range controllerInstanceMap {
		instance := controllerInstanceLocal{
			instanceName:  controllerInstance.Name,
			controllerKey: controllerInstance.ControllerKey,
			workloadNum:   controllerInstance.WorkloadNum,
		}
		sortedControllerInstancesLocal = append(sortedControllerInstancesLocal, instance)
	}

	sort.Slice(sortedControllerInstancesLocal, func(i, j int) bool {
		return sortedControllerInstancesLocal[i].instanceName < sortedControllerInstancesLocal[j].instanceName
	})

	if len(sortedControllerInstancesLocal) > 0 {
		sortedControllerInstancesLocal[0].lowerboundKey = 0
	}

	for i := 1; i < len(sortedControllerInstancesLocal); i++ {
		sortedControllerInstancesLocal[i].lowerboundKey = sortedControllerInstancesLocal[i-1].controllerKey
	}

	return sortedControllerInstancesLocal
}

// evenly split 0 ~ maxInt64 interval
func reassignControllerKeys(instances []controllerInstanceLocal) []controllerInstanceLocal {
	if len(instances) == 0 {
		return instances
	}

	interval := math.MaxInt64 / int64(len(instances))
	startKey := int64(math.MaxInt64)
	instances[0].lowerboundKey = 0
	for i := len(instances) - 1; i >= 0; i-- {
		instances[i].controllerKey = startKey
		startKey -= interval
		if i > 0 {
			instances[i].lowerboundKey = startKey
		}
	}

	return instances
}

// The following are test util. Put here so that can be used in other packages' tests

var alwaysReady = func() bool { return true }
var notifyTimes int

func mockNotifyHander(controllerInstance *v1.ControllerInstance) {
	notifyTimes++
	return
}
func mockCheckInstanceHander() {
	return
}

func CreateTestControllerInstanceManager(stopCh chan struct{}) (*ControllerInstanceManager, informers.SharedInformerFactory) {
	client := fake.NewSimpleClientset()
	informers := informers.NewSharedInformerFactory(client, 0)

	cim := NewControllerInstanceManager(informers.Core().V1().ControllerInstances(), client, nil)
	go cim.Run(stopCh)

	cim.controllerListerSynced = alwaysReady
	cim.notifyHandler = mockNotifyHander
	checkInstanceHandler = mockCheckInstanceHander
	return GetInstanceHandler(), informers
}

func MockCreateControllerInstanceAndResetChs(stopCh chan struct{}) (*bcast.Member, *bcast.Group) {
	cimUpdateChGrp := bcast.NewGroup()
	cimUpdateCh := cimUpdateChGrp.Join()
	informersResetChGrp := bcast.NewGroup()

	cim := GetInstanceHandler()
	if cim == nil {
		cim, _ = CreateTestControllerInstanceManager(stopCh)
		go cim.Run(stopCh)
	}

	return cimUpdateCh, informersResetChGrp
}
