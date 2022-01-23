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
	"strings"
	"testing"
)

func TestGetRequestBody(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		expectedJsonString string

		expectedError error
	}{
		{
			name:               "default test",
			input:              "",
			expectedJsonString: "",
			expectedError:      nil,
		},
	}

	for _, test := range tests {
		actualJsonString, err := getRequestBody()

		if err != test.expectedError {
			t.Fatal(err)
		}

		if strings.Compare(actualJsonString, test.expectedJsonString) != 0 {
			t.Fatal(err)
		}
	}
}
