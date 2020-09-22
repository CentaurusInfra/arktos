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

package mizar

import (
	"encoding/json"
	"testing"
)

func TestCheckEqual(t *testing.T, expected interface{}, actual interface{}) {
	expectedJson := JsonMarshal(expected)
	actualJson := JsonMarshal(actual)
	if actualJson != expectedJson {
		t.Fatalf("actual is not same as expected. actual: %s, expected: %s", actualJson, expectedJson)
	}
}

func JsonMarshal(v interface{}) string {
	encoded, _ := json.Marshal(v)
	return string(encoded)
}
