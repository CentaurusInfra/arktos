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
	"fmt"
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

func TestReassignControllerKeys(t *testing.T) {
	// empty instances
	outputInstances := reassignControllerKeys([]controllerInstanceLocal{})
	assert.Equal(t, 0, len(outputInstances))

	// single instances
	outputInstances = reassignControllerKeys([]controllerInstanceLocal{{}})
	assert.Equal(t, 1, len(outputInstances))
	assert.Equal(t, int64(math.MaxInt64), outputInstances[0].controllerKey)
	assert.Equal(t, int64(0), outputInstances[0].lowerboundKey)

	// 2 instances
	inputInstances := []controllerInstanceLocal{
		{instanceName: "aa"},
		{instanceName: "bb"},
	}
	outputInstances = reassignControllerKeys((inputInstances))
	assert.Equal(t, 2, len(outputInstances))
	assert.Equal(t, int64(math.MaxInt64), outputInstances[1].controllerKey)
	assert.NotEqual(t, outputInstances[0].controllerKey, outputInstances[1].controllerKey)
	// check other fields are not changed
	assert.Equal(t, inputInstances[0].instanceName, outputInstances[0].instanceName)
	assert.Equal(t, inputInstances[1].instanceName, outputInstances[1].instanceName)
	assert.Equal(t, int64(0), inputInstances[0].lowerboundKey)
	assert.True(t, inputInstances[1].lowerboundKey > inputInstances[0].lowerboundKey)

	// check input and output pointers
	assert.NotEqual(t, fmt.Sprintf("%p", &inputInstances), fmt.Sprintf("%p", &outputInstances))

	// 1 - 1000 instances
	for i := 1; i <= 1000; i++ {
		inputInstances = make([]controllerInstanceLocal, i)
		outputInstances = reassignControllerKeys(inputInstances)
		assert.NotEqual(t, fmt.Sprintf("%p", &inputInstances), fmt.Sprintf("%p", &outputInstances))
		assert.Equal(t, i, len(outputInstances))
		assert.Equal(t, int64(math.MaxInt64), outputInstances[i-1].controllerKey)
		assert.Equal(t, int64(0), outputInstances[0].lowerboundKey)

		expectedInterval := int64(math.MaxInt64 / i)
		for j := 1; j < i; j++ {
			assert.Equal(t, outputInstances[j].lowerboundKey, outputInstances[j-1].controllerKey)

			interval := outputInstances[j].controllerKey - outputInstances[j].lowerboundKey
			assert.True(t, interval > 0)
			assert.True(t, math.Abs(float64(interval-expectedInterval)) <= 1)
		}
	}
}
