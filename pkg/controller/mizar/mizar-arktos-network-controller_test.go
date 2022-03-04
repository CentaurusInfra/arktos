/*
Copyright 2022 Authors of Arktos.

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

package mizar

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGenerateVPCSpec(t *testing.T) {
	for i := 0; i < 1000; i++ {
		ipStart, vpcSpec := generateVPCSpec("vpc1")
		verifyIpStart(t, ipStart)
		verifyVPCSpec(t, vpcSpec)

		vpcJsonData, err := json.Marshal(vpcSpec)
		assert.Nil(t, err, "Unexpected marshalling error")
		var unmarshallData MizarVPC
		err = json.Unmarshal(vpcJsonData, &unmarshallData)
		assert.Nil(t, err, "Unexpected unmarshalling error")
		assert.Equal(t, vpcSpec.APIVersion, unmarshallData.APIVersion)
		assert.Equal(t, vpcSpec.Kind, unmarshallData.Kind)
		assert.Equal(t, vpcSpec.Metadata.Name, unmarshallData.Metadata.Name)
		assert.Equal(t, vpcSpec.Spec.IP, unmarshallData.Spec.IP)
		assert.Equal(t, vpcSpec.Spec.Prefix, unmarshallData.Spec.Prefix)
		assert.Equal(t, vpcSpec.Spec.Divider, unmarshallData.Spec.Divider)
		assert.Equal(t, vpcSpec.Spec.Status, unmarshallData.Spec.Status)
	}
}

func verifyIpStart(t *testing.T, ipStart int) {
	assert.True(t, ipStart >= 11 && ipStart <= 99 && ipStart != 20, "VPC started should be in range [11, 20) or [21, 99], got %d", ipStart)
}

func verifyVPCSpec(t *testing.T, vpcSpec *MizarVPC) {
	assert.True(t, vpcSpec.TypeMeta.APIVersion == "mizar.com/v1")
	assert.True(t, vpcSpec.TypeMeta.Kind == "Vpc")
	assert.True(t, vpcSpec.Metadata.Name == "vpc1")
	assert.True(t, vpcSpec.Spec.Status == "Init")
}

func TestGenerateSubnetSpec(t *testing.T) {
	ipStart, vpcSpec := generateVPCSpec("vpc1")

	subnetSpecData, err := generateSubnetSpec(vpcSpec.Metadata.Name, "subnet1", ipStart)
	assert.Nil(t, err)
	assert.NotNil(t, subnetSpecData)
	var unmarshallData MizarSubnet
	err = json.Unmarshal(subnetSpecData, &unmarshallData)
	assert.Nil(t, err, "Unexpected unmarshalling error")
	assert.Equal(t, unmarshallData.Metadata.Name, "subnet1")
}
