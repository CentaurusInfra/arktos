/*
Copyright 2014 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

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

package node

import (
	"testing"
)

type UrlTestCase struct {
	uri         string
	expectedErr bool
}

func TestValidateUrl(t *testing.T) {
	testcases := []UrlTestCase{
		{
			uri:         "http://1.1.1.1:8888",
			expectedErr: false,
		},
		{
			uri:         "https://1.1.1.1:8080",
			expectedErr: false,
		},
		{
			// missing port
			uri:         "http://1.1.1.1",
			expectedErr: true,
		},
		{
			// missing scheme
			uri:         "1.1.1.1:8080",
			expectedErr: true,
		},
		{
			// invalid ip address
			uri:         "http://256.256.256.256:8080",
			expectedErr: true,
		},
		{
			// empty url
			uri:         "",
			expectedErr: true,
		},
	}

	for _, tc := range testcases {
		err := ValidateUrl(tc.uri)

		if tc.expectedErr && err == nil {
			t.Errorf("%v: unexpected no-error", tc.uri)
		}

		if !tc.expectedErr && err != nil {
			t.Errorf("%v: got unexpected error: %v", tc.uri, err)
		}
	}

}
