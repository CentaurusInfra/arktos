/*
Copyright 2021 Authors of Arktos.

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

package openstack

import "fmt"

var flavors = map[string]FlavorType{}
var flavorList = []*FlavorType{}

var ERROR_FLAVOR_NOT_FOUND = fmt.Errorf("flavor not found")

// slim down version of the openstack flavor data structure
type FlavorType struct {
	Id          int    `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	MemoryMb    int    `json:"memoryMb,omitempty"`
	Vcpus       int    `json:"vcpus,omitempty"`
	RootGb      int    `json:"rootGb,omitempty"`
	EphemeralGb int    `json:"ephemeralGb,omitempty"`
}

// for 130 release, only READ operation with static list of flavors
func initFlavorsCache() {
	flavors = make(map[string]FlavorType)
	flavors["m1.tiny"] = FlavorType{1, "m1.tiny", 512, 1, 0, 0}
	flavors["m1.small"] = FlavorType{2, "m1.small", 2048, 1, 0, 0}
	flavors["m1.medium"] = FlavorType{3, "m1.medium", 4096, 2, 0, 0}
	flavors["m1.large"] = FlavorType{4, "m1.large", 8192, 4, 0, 0}
	flavors["m1.xlarge"] = FlavorType{5, "m1.xlarge", 16384, 8, 0, 0}

	flavorList = make([]*FlavorType, len(flavors))
	i := 0
	for _, v := range flavors {
		temp := v
		flavorList[i] = &temp
		i++
	}
}

func GetFalvor(name string) (*FlavorType, error) {
	if flavor, found := flavors[name]; found {
		return &flavor, nil
	}

	return nil, ERROR_FLAVOR_NOT_FOUND
}

func ListFalvors() []*FlavorType {
	return flavorList
}
