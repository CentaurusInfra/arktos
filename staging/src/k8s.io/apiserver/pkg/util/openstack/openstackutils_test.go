/*
Copyright 2021 The Arktos Authors.

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

import (
	"bytes"
	"testing"

	"k8s.io/apimachinery/pkg/util/json"
)

//TODO: fix UT
func TestConvertToOpenstackRequest(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput OpenstackServerRequest

		expectedError error
	}{
		{
			name:  "all valid, basic test",
			input: `{"server":{"name":"testvm","imageRef":"6db08272-a856-49da-8909-7c4c73ab0bac","flavorRef":"m1.tiny"}}`,
			expectedOutput: OpenstackServerRequest{
				Server: ServerType{Name: "testvm",
					ImageRef: "6db08272-a856-49da-8909-7c4c73ab0bac",
					Flavor:   "mi.tiny",
				},
			},
			expectedError: nil,
		},
	}

	for _, test := range tests {
		actualBytes, err := ConvertServerFromOpenstackRequest([]byte(test.input))

		if err != test.expectedError {
			t.Fatal(err)
		}

		expectedBytes, err := json.Marshal(test.expectedOutput)

		if err != test.expectedError {
			t.Fatal(err)
		}

		if bytes.Compare(actualBytes, expectedBytes) != 0 {
			t.Fatal(err)
		}
	}
}
