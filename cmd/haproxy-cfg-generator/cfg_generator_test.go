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

package main

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type TestCase struct {
	tp_ips     []string
	rp_ip      string
	targetFile string
	tlsMode    string
}

func TestCreateHaproxyCfg(t *testing.T) {
	testCases := []TestCase{
		{
			tp_ips:     []string{"1.1.1.1"},
			rp_ip:      "9.9.9.9",
			targetFile: "data/sample_one_tp_haproxy.cfg",
		},
		{
			tp_ips:     []string{"1.1.1.1", "2.2.2.2"},
			rp_ip:      "9.9.9.9",
			targetFile: "data/sample_two_tp_haproxy.cfg",
		},
		{
			tp_ips:     []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"},
			rp_ip:      "9.9.9.9",
			targetFile: "data/sample_three_tp_haproxy.cfg",
			tlsMode:    BRIDGING,
		},
		{
			tp_ips:     []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4"},
			rp_ip:      "9.9.9.9",
			targetFile: "data/sample_four_tp_haproxy.cfg",
			tlsMode:    BRIDGING,
		},
		{
			tp_ips:     []string{"1.1.1.1", "2.2.2.2"},
			rp_ip:      "9.9.9.9",
			targetFile: "data/sample_two_tp_haproxy_offloading_mode.cfg",
			tlsMode:    OFFLOADING,
		},
	}

	for index, testCase := range testCases {
		generatorCfg := NewDefaultConfig()
		if testCase.tlsMode != "" {
			generatorCfg.tlsMode = testCase.tlsMode
		}
		generatorCfg.templateFile = "data/haproxy.cfg.template"
		generatorCfg.tenantPartititonIPs = testCase.tp_ips
		generatorCfg.resourcePartitionIP = testCase.rp_ip

		contentString := CreateHaproxyCfg(generatorCfg)
		lines := strings.Split(string(contentString), "\n")
		contentString = ""
		for _, line := range lines {
			if strings.HasPrefix(line, "KUBEMARK_ONLY:") {
				continue
			}
			if strings.HasPrefix(line, "ONEBOX_ONLY:") {
				contentString += line[len("ONEBOX_ONLY:"):] + "\n"
				continue
			}
			contentString += line + "\n"
		}

		targetFile, err := filepath.Abs(testCase.targetFile)
		if err != nil {
			t.Fatalf("Error in get absoluate path to: %v, err:  %v", testCase.targetFile, err)
		}
		expected, err := ioutil.ReadFile(targetFile)
		if err != nil {
			t.Fatalf("Error in opening target file for comparison: %v, err:  %v", targetFile, err)
		}
		expectedString := string(expected)

		if strings.Trim(contentString, "\n") != strings.Trim(expectedString, "\n") {
			// write the content to a temp file and use diff command to show the difference
			tempFile, err := ioutil.TempFile("/tmp", "haproxy-cfg")
			if err != nil {
				t.Fatalf("unable to create a temp file: %v", err)
			}
			ioutil.WriteFile(tempFile.Name(), []byte(contentString), 0644)

			output, _ := exec.Command("diff", tempFile.Name(), targetFile).CombinedOutput()

			t.Errorf("Test Case %v: cfg file generated (%v) is different from expected (%v), diff output is as follows: \n %v", index, tempFile.Name(), targetFile, string(output))
		}
	}
}
