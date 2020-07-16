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

package kubernetes

import (
	"math/rand"
	"testing"
	"time"
)

func TestRandIntnRange(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	for max := 1; max < 11; max++ {
		// For each max, generate 100 * max numbers
		occurrence := make(map[int]int)
		for j := 0; j < 100*max; j++ {
			ran := rand.Intn(max + 1)
			count, isOK := occurrence[ran]
			if isOK {
				occurrence[ran] = count + 1
			} else {
				occurrence[ran] = 1
			}
		}

		// check whether all value exists
		totalCount := 0
		for k := 0; k <= max; k++ {
			count, isOK := occurrence[k]
			if isOK {
				totalCount += count
				t.Logf("Max %d, value %d, count %d", max, k, count)
			} else {
				t.Errorf("Max %d, expected value %d was not generated. map [%v]", max, k, occurrence)
			}
		}
		if totalCount != 100*max {
			t.Errorf("Max %d, expected value count %v does not equal 100 * max = %d", max, totalCount, 100*max)
		}
	}
}
