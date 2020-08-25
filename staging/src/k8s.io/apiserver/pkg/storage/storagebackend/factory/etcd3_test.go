/*
Copyright 2020 Authors of Arktos

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

package factory

import (
	"io/ioutil"
	"k8s.io/apiserver/pkg/storage"
	"os"
	"reflect"
	"testing"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectedResult map[string]storage.Interval
	}{
		{
			name:           "load an empty file",
			content:        "",
			expectedResult: map[string]storage.Interval{},
		},
		{
			name:           "load an invalid config file",
			content:        "abc",
			expectedResult: map[string]storage.Interval{},
		},
		{
			name:           "load a config file without enough values",
			content:        "abc,a",
			expectedResult: map[string]storage.Interval{},
		},
		{
			name:           "load a config file with right values",
			content:        "abc,a,b",
			expectedResult: map[string]storage.Interval{"abc": {"a", "b"}},
		},
		{
			name:           "load a config file with multiple lines",
			content:        "abc,a,b\nabcd,e,f",
			expectedResult: map[string]storage.Interval{"abc": {"a", "b"}, "abcd": {"e", "f"}},
		},
		{
			name:           "load a config file with duplicate keys",
			content:        "ef,a,b\nef,e,f",
			expectedResult: map[string]storage.Interval{"ef": {"e", "f"}},
		},
		{
			name:           "load a config file with too many values",
			content:        "abcde,a,b,c,d,e",
			expectedResult: map[string]storage.Interval{"abcde": {"a", "b"}},
		},
	}
	file, err := ioutil.TempFile("", "config")
	defer os.Remove(file.Name())
	if err != nil {
		t.Errorf("Failed to create a config file with error %v", err)
	}
	for _, test := range tests {
		err := ioutil.WriteFile(file.Name(), []byte(test.content), 0644)
		if err != nil {
			t.Errorf("The test %v failed with err %v", test.name, err)
		}
		result, err := parseConfig(file.Name())
		if err != nil {
			t.Errorf("The test %v failed with result %v, err %v", test.name, result, err)
		}
		if !reflect.DeepEqual(result, test.expectedResult) {
			t.Errorf("The test %v failed. The expected result is : %v. The actual result is %v", test.name, test.expectedResult, result)
		}

	}
}
