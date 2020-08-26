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

func (tc *TestConfig) Validate() *ErrorList {
	errors := NewErrorList()

	errors.Add(ErrorIfNegative("MaxRetryCount", &tc.MaxRetryCount))
	errors.Add(ErrorIfNegative("MaxRetryInterval", &tc.MaxRetryInterval))
	errors.Add(ErrorIfNegative("MaxTimeOut", &tc.MaxTimeOut))

	errors.Add(ErrorIfOutOfBounds("DefaultRetryCount", &tc.DefaultRetryCount, 0, tc.MaxRetryCount))
	errors.Add(ErrorIfOutOfBounds("DefaultRetryInterval", &tc.DefaultRetryInterval, 0, tc.MaxRetryInterval))
	errors.Add(ErrorIfOutOfBounds("DefaultTimeOut", &tc.DefaultTimeOut, 0, tc.MaxTimeOut))

	return errors
}
