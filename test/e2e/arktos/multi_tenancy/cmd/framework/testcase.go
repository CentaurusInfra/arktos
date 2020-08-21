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
	"strings"
	"time"
)

type TestCase struct {
	Command string `yaml:"Command,omitempty"`
	TimeOut *int   `yaml:"TimeOut,omitempty"`

	RunBefore string `yaml:"RunBefore,omitempty"`

	RetryCount    *int `yaml:"RetryCount,omitempty"`
	RetryInterval *int `yaml:"RetryInterval,omitempty"`

	TerminateAfter *int `yaml:"TerminateAfter,omitempty"`

	ShouldFail          bool     `yaml:"ShouldFail,omitempty"`
	OutputShouldBe      string   `yaml:"OutputShouldBe,omitempty"`
	OutputShouldContain []string `yaml:"OutputShouldContain,omitempty"`

	OutputShouldNotContain []string `yaml:"OutputShouldNotContain,omitempty"`
}

func (t *TestCase) Validate(tc *TestConfig) []error {
	errList := []error{}

	if t.Command == "" {
		errList = append(errList, fmt.Errorf("Test command is empty!"))
	}

	if t.RetryCount != nil && (*(t.RetryCount) < 0 || *(t.RetryCount) > tc.MaxRetryCount) {
		errList = append(errList, fmt.Errorf("Invalid RetryCount: %v, should be in the range of [0, %d]", *(t.RetryCount), tc.MaxRetryCount))
	}

	if t.RetryInterval != nil && (*(t.RetryInterval) < 0 || *(t.RetryInterval) > tc.MaxRetryInterval) {
		errList = append(errList, fmt.Errorf("Invalid RetryInterval: %v, should be in the range of [0, %d]", *(t.RetryInterval), tc.MaxRetryInterval))
	}

	if t.TimeOut != nil && (*(t.TimeOut) < 0 || *(t.TimeOut) > tc.MaxTimeOut) {
		errList = append(errList, fmt.Errorf("Invalid TimeOut: %v, tc.MaxRetryInterval", *(t.TimeOut), tc.MaxTimeOut))
	}

	return errList
}

func (t *TestCase) Complete(tc *TestConfig) {
	if t.RetryCount == nil {
		t.RetryCount = &tc.DefaultRetryCount
	}

	if t.RetryInterval == nil {
		t.RetryInterval = &tc.DefaultRetryInterval
	}

	if t.TimeOut == nil {
		t.TimeOut = &tc.DefaultTimeOut
	}
}

func (t *TestCase) Run(tc *TestConfig) []error {
	errList := []error{}
	var testErrList []error
	var exitCode int
	var output string
	var err error

	t.Complete(tc)
	
	if t.RunBefore != "" {
		LogWarning("Run pre-test configuration command %q", t.RunBefore)
		exitCode, output, err = ExecCommandLine(t.RunBefore, 0)
		if exitCode != 0 {
			// it is by design that pre-test configuration command is allowed to fail
			LogWarning("Pre-test command %q returns with error: %v, exit code: %v, command output: %q. Moving on ...", t.RunBefore, exitCode, output)
		}
	}

	retries := *t.RetryCount
	LogNormal("Testing %q...", t.Command)
	for {
		exitCode, output, err = ExecCommandLine(t.Command, *t.TimeOut)
		if tc.Verbose {
			LogNormal("\nexit code: %v, output:\n%v", exitCode, output)
		}

		if err == nil {
			testErrList = t.CheckTestResult(exitCode, output)
			if len(testErrList) == 0 {
				LogSuccess(" PASSED\n")
				break
			}
		}

		if retries <= 0 {
			if err != nil {
				errList = append(errList, err)
				LogError("\nFAILED: %v\n", err)
			} else {
				errList = append(errList, testErrList...)
				LogError("\nFAILED: %v \n", testErrList)
				if !tc.Verbose {
					LogNormal("exit code: %v, output:\n%v", exitCode, output)
				}
			}
			break
		}

		LogWarning("\nFailed, retrying after %v seconds ...", *t.RetryInterval)

		retries--
		time.Sleep(time.Duration(*t.RetryInterval) * time.Second)
	}

	return errList
}

func (t *TestCase) CheckTestResult(exitCode int, output string) []error {
	errList := []error{}
	if t.ShouldFail && exitCode == 0 {
		errList = append(errList, fmt.Errorf("Command succeeded unexpectedly"))
	}

	if !t.ShouldFail && exitCode != 0 {
		errList = append(errList, fmt.Errorf("Command failed unexpectedly"))
	}

	if t.OutputShouldBe != "" && output != t.OutputShouldBe {
		errList = append(errList, fmt.Errorf("unexpected output: %q, expected: %q", output, t.OutputShouldBe))
	}

	if len(t.OutputShouldContain) > 0 {
		for _, expectedMatch := range t.OutputShouldContain {
			if !strings.Contains(output, expectedMatch) {
				errList = append(errList, fmt.Errorf("Did not find the match in output : %q", expectedMatch))
			}
		}
	}

	if len(t.OutputShouldNotContain) > 0 {
		for _, expectedNotMatch := range t.OutputShouldNotContain {
			if strings.Contains(output, expectedNotMatch) {
				errList = append(errList, fmt.Errorf("Find unexpected match in output : %q", expectedNotMatch))
			}
		}
	}

	return errList
}
