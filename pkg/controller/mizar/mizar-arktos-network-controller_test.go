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
	"fmt"
	"github.com/stretchr/testify/assert"
	"strconv"
	"strings"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
	utilfeaturetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/kubernetes/pkg/features"
)

func TestGenerateVPCSpecWithoutVPCRangeOverlap(t *testing.T) {
	defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.MizarVPCRangeNoOverlap, true)()

	fakeClient := func() *dynamicfakeclient.FakeDynamicClient {
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
	assert.Equal(t, 0, len(c.vpcCache.vpcUsedCache))
	assert.Equal(t, c.vpcCache.vpcRangeStart*256, c.vpcCache.vpcNextAvailableRange)

	// Generate vpc start ips
	expectedTotal := (c.vpcCache.vpcRangeEnd - c.vpcCache.vpcRangeStart + 1) * 256
	if mizarInternalIPStart >= c.vpcCache.vpcRangeStart && mizarInternalIPStart <= c.vpcCache.vpcRangeEnd {
		expectedTotal -= 256
	}

	generatedVPC := 0
	for i := c.vpcCache.vpcRangeStart; i <= c.vpcCache.vpcRangeEnd-1; i++ {
		for j := 0; j < 256; j++ {
			ipSeg1, ipSeg2, vpcSpec, err, permErr := c.generateVPCSpec("vpc1")
			assert.Nil(t, err)
			assert.Nil(t, permErr)
			verifyIpStart(t, ipSeg1)
			verifyIpSeg2(t, ipSeg2)
			verifyVPCSpec(t, vpcSpec)
			verifyIP(t, vpcSpec.Spec.IP)
			if i < mizarInternalIPStart {
				assert.Equal(t, i, ipSeg1)
			} else if i >= mizarInternalIPStart {
				assert.Equal(t, i+1, ipSeg1)
			}
			generatedVPC++
			assert.Equal(t, generatedVPC, len(c.vpcCache.vpcUsedCache))
		}
	}

	// Check used IP cache
	for i := c.vpcCache.vpcRangeStart; i <= c.vpcCache.vpcRangeEnd; i++ {
		if i == mizarInternalIPStart {
			continue
		}
		for j := 0; j < 256; j++ {
			key := i*256 + j
			value, isOK := c.vpcCache.vpcUsedCache[key]
			assert.True(t, isOK)
			assert.True(t, value)
		}
	}
	assert.Equal(t, generatedVPC, len(c.vpcCache.vpcUsedCache))

	// Check permanent error
	_, _, vpcSpec, err, permErr := c.generateVPCSpec("test")
	assert.NotNil(t, permErr)
	assert.Nil(t, err)
	assert.Nil(t, vpcSpec)
}

func TestPopolateVPCCache_VPC0Only(t *testing.T) {
	defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.MizarVPCRangeNoOverlap, true)()

	fakeClient := dynamicfakeclient.NewSimpleDynamicClient(runtime.NewScheme())
	fakeClient.PrependReactor("list", "vpcs", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, newUnstructuredList(newUnstructuredVPC("vpc0", "20.0.0.0")), nil
	})

	c := &MizarArktosNetworkController{
		dynamicClient: fakeClient,
	}
	c.vpcCache = generateVPCUsedCache()
	c.populateCache()

	// Check initial values
	assert.Equal(t, 11, c.vpcCache.vpcRangeStart)
	assert.Equal(t, 50, c.vpcCache.vpcRangeEnd)
	assert.Equal(t, 0, len(c.vpcCache.vpcUsedCache))
	assert.Equal(t, c.vpcCache.vpcRangeStart*256, c.vpcCache.vpcNextAvailableRange)
}

func TestPopolateVPCCache(t *testing.T) {
	defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.MizarVPCRangeNoOverlap, true)()

	fakeClient := dynamicfakeclient.NewSimpleDynamicClient(runtime.NewScheme())
	fakeClient.PrependReactor("list", "vpcs", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, newUnstructuredList(newUnstructuredVPC("vpc1", "21.0.0.0")), nil
	})

	c := &MizarArktosNetworkController{
		dynamicClient: fakeClient,
	}
	c.vpcCache = generateVPCUsedCache()
	c.populateCache()

	// Check initial values
	assert.Equal(t, 11, c.vpcCache.vpcRangeStart)
	assert.Equal(t, 50, c.vpcCache.vpcRangeEnd)
	assert.Equal(t, 1, len(c.vpcCache.vpcUsedCache))
	assert.Equal(t, 21*256+1, c.vpcCache.vpcNextAvailableRange)
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
	defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.MizarVPCRangeNoOverlap, false)()

	for i := 0; i < 1000; i++ {
		ipSeg1, ipSeg2, vpcSpec, tempErr, permErr := c.generateVPCSpec("vpc1")
		verifyIpStart(t, ipSeg1)
		verifyIpSeg2(t, ipSeg2)
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

func verifyIpSeg2(t *testing.T, seg2 int) {
	assert.True(t, seg2 >= 0 && seg2 <= 255, "Second segment of VPC should be in range [0, 255]", seg2)
}

func verifyIP(t *testing.T, ipString string) {
	segs := strings.Split(ipString, ".")
	assert.Equal(t, 4, len(segs))
	for i := 0; i < 4; i++ {
		seg, err := strconv.Atoi(segs[i])
		assert.Nil(t, err)
		verifyIpSeg2(t, seg)
	}
}

func verifyVPCSpec(t *testing.T, vpcSpec *MizarVPC) {
	assert.True(t, vpcSpec.TypeMeta.APIVersion == "mizar.com/v1")
	assert.True(t, vpcSpec.TypeMeta.Kind == "Vpc")
	assert.True(t, vpcSpec.Metadata.Name == "vpc1")
	assert.True(t, vpcSpec.Spec.Status == "Init")
}

func TestGenerateSubnetSpec(t *testing.T) {
	c := &MizarArktosNetworkController{}
	ipSeg1, ipSeg2, vpcSpec, tempErr, permErr := c.generateVPCSpec("vpc1")
	verifyIpStart(t, ipSeg1)
	verifyIpSeg2(t, ipSeg2)
	verifyVPCSpec(t, vpcSpec)
	assert.Nil(t, tempErr)
	assert.Nil(t, permErr)

	subnetSpecData, err := generateSubnetSpec(vpcSpec.Metadata.Name, "subnet1", vpcSpec.Spec.IP)
	assert.Nil(t, err)
	assert.NotNil(t, subnetSpecData)
	var unmarshallData MizarSubnet
	err = json.Unmarshal(subnetSpecData, &unmarshallData)
	assert.Nil(t, err, "Unexpected unmarshalling error")
	assert.Equal(t, unmarshallData.Metadata.Name, "subnet1")
	verifyIP(t, unmarshallData.Spec.IP)
}

func newUnstructuredList(items ...*unstructured.Unstructured) *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	for i := range items {
		list.Items = append(list.Items, *items[i])
	}
	return list
}

func newUnstructuredVPC(vpcName string, ip string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": fmt.Sprintf("%s/%s", resource_group, resource_version),
			"kind":       "Vpc",
			"metadata": map[string]interface{}{
				"tenant":    metav1.TenantSystem,
				"namespace": metav1.NamespaceDefault,
				"name":      vpcName,
			},
			"spec": map[string]interface{}{
				"ip":      ip,
				"prefix":  "8",
				"divider": 1,
				"status":  "Init",
			},
		},
	}
}
