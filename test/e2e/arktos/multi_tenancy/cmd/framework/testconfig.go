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
	"fmt"
)

type TestConfig struct {
	MaxRetryCount        int
	MaxRetryInterval     int
	MaxTimeOut           int
	DefaultRetryCount    int
	DefaultRetryInterval int
	DefaultTimeOut       int
	Verbose              bool
	CommonVariables      map[string]string
}

func (tc *TestConfig) Validate() []error {
	errors := []error{}

	if tc.MaxRetryCount < 0 {
		errors = append(errors, fmt.Errorf("MaxRetryCount cannot be negative"))
	}

	if tc.MaxRetryInterval < 0 {
		errors = append(errors, fmt.Errorf("MaxRetryInterval cannot be negative"))
	}

	if tc.MaxTimeOut < 0 {
		errors = append(errors, fmt.Errorf("MaxTimeOut cannot be negative"))
	}

	if tc.DefaultRetryCount < 0 || tc.DefaultRetryCount > tc.MaxRetryCount {
		errors = append(errors, fmt.Errorf("Invalid DefaultRetryCount %d, should be in the range of [0, %d]", tc.DefaultRetryCount, tc.MaxRetryCount))
	}

	if tc.DefaultRetryInterval < 0 || tc.DefaultRetryInterval > tc.MaxRetryInterval {
		errors = append(errors, fmt.Errorf("Invalid DefaultRetryInterval %d, should be in the range of [0, %d]", tc.DefaultRetryInterval, tc.MaxRetryInterval))
	}

	if tc.DefaultTimeOut < 0 || tc.DefaultTimeOut > tc.MaxTimeOut {
		errors = append(errors, fmt.Errorf("Invalid DefaultTimeOut %d, should be in the range of [0, %d]", tc.DefaultTimeOut, tc.MaxTimeOut))
	}

	return errors
}
