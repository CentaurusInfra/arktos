/*
Copyright 2021 Authors of Arktos.

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
	"k8s.io/klog"
	"testing"
)

var outputStr1 = `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"testvm","tenant":"system","namespace":"kube-system","creationTimestamp":null,"labels":{"openstsckApi":"true"},"annotations":{"VirtletCPUModel":"host-model"}},"spec":{"virtualMachine":{"name":"testvm","image":"download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img","resources":{"limits":{"cpu":"1","memory":"512Mi"},"requests":{"cpu":"1","memory":"512Mi"}},"imagePullPolicy":"IfNotPresent"}}}`

func TestConvertServerFromOpenstackRequest(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput []byte
		expectedError  error
	}{
		{
			name:           "all valid, basic test",
			input:          `{"server":{"name":"testvm","imageRef":"cirros-0.5.1","flavorRef":"m1.tiny"}}`,
			expectedOutput: []byte(outputStr1),
			expectedError:  nil,
		},
		{
			name:           "Non-existing flavor",
			input:          `{"server":{"name":"testvm","imageRef":"cirros-0.5.1","flavorRef":"NotExistFlavor"}}`,
			expectedOutput: nil,
			expectedError:  ERROR_FLAVOR_NOT_FOUND,
		},
		{
			name:           "Non-existing image",
			input:          `{"server":{"name":"testvm","imageRef":"NotExistImage","flavorRef":"m1.tiny"}}`,
			expectedOutput: nil,
			expectedError:  ERROR_IMAGE_NOT_FOUND,
		},
	}

	for _, test := range tests {
		actualBytes, err := ConvertServerFromOpenstackRequest([]byte(test.input))

		if err != test.expectedError {
			t.Fatal(err)
		}

		klog.Infof("server output: %s", string(actualBytes))
		if bytes.Compare(actualBytes, test.expectedOutput) != 0 {
			t.Fatal(err)
		}
	}
}

func TestConvertActionFromOpenstackRequest(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput []byte
		expectedError  error
	}{
		{
			name:           "all valid, basic reboot action",
			input:          `{"reboot":{"type":"HARD"}}`,
			expectedOutput: []byte(`{"apiVersion":"v1","kind":"CustomAction","operation":"reboot","rebootParams":{"delayInSeconds":10}}`),
			expectedError:  nil,
		},
		{
			name:           "all valid, basic snapshot action",
			input:          `{"snapshot":{"name":"foobar","metadata":{"meta_var":"meta_val"}}}`,
			expectedOutput: []byte(`{"apiVersion":"v1","kind":"CustomAction","operation":"snapshot","snapshotParams":{"snapshotName":"foobar"}}`),
			expectedError:  nil,
		},
		{
			name:           "all valid, basic rebuild action",
			input:          `{"restore":{"ImageRef":"foobar","metadata":{"meta_var":"meta_val"}}}`,
			expectedOutput: []byte(`{"apiVersion":"v1","kind":"CustomAction","operation":"restore","restoreParams":{"snapshotID":"foobar"}}`),
			expectedError:  nil,
		},
		{
			name:           "Non-existing flavor",
			input:          `{"not-exist":{"type":"HARD"}}`,
			expectedOutput: nil,
			expectedError:  ERROR_UNKNOWN_ACTION,
		},
	}

	for _, test := range tests {
		actualBytes, err := ConvertActionFromOpenstackRequest([]byte(test.input))

		if err != test.expectedError {
			t.Fatal(err)
		}

		klog.Infof("output: %s", string(actualBytes))

		if bytes.Compare(actualBytes, test.expectedOutput) != 0 {
			t.Fatal(err)
		}
	}
}
