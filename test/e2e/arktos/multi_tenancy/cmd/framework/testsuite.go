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

package framework

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v2"
)

type TestSuite struct {
	FilePath  string
	Variables map[string]string `yaml:"Variables,omitempty"`
	Tests     []TestCase        `yaml:"Tests,omitempty"`
	Failures  []string
}

func (ts *TestSuite) LoadTestSuite(filePath string, tc *TestConfig) error {
	ts.FilePath = filePath
	testSuiteFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error in reading file %v, err: %v ", filePath, err)
	}

	var tempTestSuite TestSuite
	err = yaml.UnmarshalStrict(testSuiteFile, &tempTestSuite)
	if err != nil {
		// ignore the yaml.TypeError in the first round of unmarshing as it is most likely due to that
		// the variable resolution is not done
		// If there is a real Type error, the second round of unmarshalling, which is done after the
		// resolution, will catch it
		if _, type_error := err.(*yaml.TypeError); !type_error {
			return fmt.Errorf("error in unmarshling original yaml file %v, \n\nerr: %#v ", filePath, err)
		}
	}

	allVariables := MergeStringMaps(tc.CommonVariables, tempTestSuite.Variables)
	resolved_output, _ := ioutil.ReadFile(filePath)
	for key, value := range allVariables {
		// generate random strings for variables if the value is "random_[string_length]"
		if random_generate, _ := regexp.MatchString("random_[0-9]+", value); random_generate {
			length_str := value[len("random_"):]
			length, err := strconv.Atoi(length_str)
			if err != nil {
				return fmt.Errorf("Error in parsing Variable, key: %v, value: %v, err: %v", key, value, err)
			}
			value = RandomString(length)
		}
		resolved_output = bytes.ReplaceAll(resolved_output, []byte("${"+key+"}"), []byte(value))
	}

	if err = yaml.UnmarshalStrict(resolved_output, ts); err != nil {
		return fmt.Errorf("error in unmarshling resolved yaml file %v, err: %v ", filePath, err)
	}

	for _, t := range ts.Tests {
		if errList := t.Validate(tc); !errList.IsEmpty() {
			return fmt.Errorf("error in validating yaml file %v, test case: %v, err: %v ", filePath, t, errList)
		}
	}

	return nil
}

func (ts *TestSuite) Run(tc *TestConfig) {
	LogInfo("\nStart Running Test Suite %q\n", ts.FilePath)

	for i, t := range ts.Tests {
		fmt.Println("----------------------------------------------------------------------")
		errList := t.Run(tc)
		if !errList.IsEmpty() {
			ts.Failures = append(ts.Failures, fmt.Sprintf("Test Command #%v: %q, Error %v", i+1, t.Command, errList))
		}
	}

	fmt.Println("")
}
