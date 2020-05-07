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

package apiserverupdate

import (
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"testing"
)

const (
	masterIP1 = "192.168.1.1"

	serviceGroupId1 = "1"
)

func TestSetAPIServerConfig(t *testing.T) {
	t.Log("1. Test update argument later won't affect APIServerConfig in global")
	ssMap1 := make(map[string]v1.EndpointSubset)
	ssMap1[serviceGroupId1] = v1.EndpointSubset{
		Addresses:      []v1.EndpointAddress{{IP: masterIP1}},
		ServiceGroupId: serviceGroupId1,
	}

	SetAPIServerConfig(ssMap1)

	ssMap1[serviceGroupId1].Addresses[0].Hostname = "123"
	ssMap2 := GetAPIServerConfig()
	assert.NotNil(t, ssMap2)
	ss2, isOK := ssMap2[serviceGroupId1]
	assert.True(t, isOK)
	assert.NotNil(t, ss2)
	assert.Equal(t, serviceGroupId1, ss2.ServiceGroupId)
	assert.Equal(t, 1, len(ss2.Addresses))
	assert.Equal(t, masterIP1, ss2.Addresses[0].IP)
	assert.Equal(t, "", ss2.Addresses[0].Hostname)
}
