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
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	utilfeaturetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/kubernetes/pkg/features"
	"sync"
	"testing"
)

func TestGenerateVPCSpecWithoutVPCRangeOverlap(t *testing.T) {
	defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.MizarVPCRangeOverlap, true)()

	fakeClient := func() *dynamicfakeclient.FakeDynamicClient{
		return dynamicfakeclient.NewSimpleDynamicClient(runtime.NewScheme())
	}

	c := &MizarArktosNetworkController{
		dynamicClient: fakeClient(),
	}
	c.vpcCache = generateVPCUsedCache()
	c.populateCache()

	// Check initial values
	assert.Equal(t, 11, c.vpcCache.vpcRangeStart)
	assert.Equal(t, 50, c.vpcCache.vpcRangeEnd)
	assert.Equal(t, 1, len(c.vpcCache.vpcUsedCache))
	value, isOK := c.vpcCache.vpcUsedCache[mizarInternalIPStart]
	assert.True(t, isOK)
	assert.True(t, value)

	// Generate vpc start ips
	expectedTotal := c.vpcCache.vpcRangeEnd - c.vpcCache.vpcRangeStart + 1
	if mizarInternalIPStart >= c.vpcCache.vpcRangeStart && mizarInternalIPStart <= c.vpcCache.vpcRangeEnd {
		expectedTotal--
	}

	generatedVPC := 0
	for i:= c.vpcCache.vpcRangeStart; i <= c.vpcCache.vpcRangeEnd - 1; i++ {
		vpcIPStart, vpcSpec, err, permErr := c.generateVPCSpec("vpc1")
		assert.Nil(t, err)
		assert.Nil(t, permErr)
		verifyVPCSpec(t, vpcSpec)
		if i < mizarInternalIPStart {
			assert.Equal(t, i, vpcIPStart)
		} else if i >= mizarInternalIPStart {
			assert.Equal(t, i + 1, vpcIPStart)
		}
		generatedVPC++
		assert.Equal(t, generatedVPC + 1, len(c.vpcCache.vpcUsedCache))
	}

	// Check used IP cache
	for i := c.vpcCache.vpcRangeStart; i <= c.vpcCache.vpcRangeEnd; i++ {
		value, isOK := c.vpcCache.vpcUsedCache[i]
		assert.True(t, isOK)
		assert.True(t, value)
	}
	assert.Equal(t, generatedVPC + 1, len(c.vpcCache.vpcUsedCache))

	// Check permanent error
	_, vpcSpec, err, permErr := c.generateVPCSpec("test")
	assert.NotNil(t, permErr)
	assert.Nil(t, err)
	assert.Nil(t, vpcSpec)
}

func generateVPCUsedCache() *vpcUsedCache {
	return &vpcUsedCache{
		vpcRangeStart: 11,
		vpcRangeEnd:   50,
		vpcUsedCache:  make(map[int]bool),
		vpcCacheLock:  sync.RWMutex{},
	}
}

func TestGenerateVPCSpecWithVPCRangeOverlap(t *testing.T) {
	c := &MizarArktosNetworkController{}
	defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.MizarVPCRangeOverlap, false)()

	for i := 0; i < 1000; i++ {
		ipStart, vpcSpec, tempErr, permErr := c.generateVPCSpec("vpc1")
		verifyIpStart(t, ipStart)
		verifyVPCSpec(t, vpcSpec)
		assert.Nil(t, tempErr)
		assert.Nil(t, permErr)

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
	c := &MizarArktosNetworkController{}
	ipStart, vpcSpec, tempErr, permErr := c.generateVPCSpec("vpc1")
	assert.Nil(t, tempErr)
	assert.Nil(t, permErr)

	subnetSpecData, err := generateSubnetSpec(vpcSpec.Metadata.Name, "subnet1", ipStart)
	assert.Nil(t, err)
	assert.NotNil(t, subnetSpecData)
	var unmarshallData MizarSubnet
	err = json.Unmarshal(subnetSpecData, &unmarshallData)
	assert.Nil(t, err, "Unexpected unmarshalling error")
	assert.Equal(t, unmarshallData.Metadata.Name, "subnet1")
}
