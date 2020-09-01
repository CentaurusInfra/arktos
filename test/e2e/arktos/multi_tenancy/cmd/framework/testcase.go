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

	BeforeTest        string `yaml:"BeforeTest,omitempty"`
	BeforeTestMessage string `yaml:"BeforeTestMessage,omitempty"`

	RetryCount    *int `yaml:"RetryCount,omitempty"`
	RetryInterval *int `yaml:"RetryInterval,omitempty"`

	TerminateAfter *int `yaml:"TerminateAfter,omitempty"`

	ShouldFail          bool     `yaml:"ShouldFail,omitempty"`
	OutputShouldBe      string   `yaml:"OutputShouldBe,omitempty"`
	OutputShouldContain []string `yaml:"OutputShouldContain,omitempty"`

	OutputShouldNotContain []string `yaml:"OutputShouldNotContain,omitempty"`
	AfterTestMessage       string   `yaml:"AfterTestMessage,omitempty"`
}

func (t *TestCase) Validate(tc *TestConfig) *ErrorList {
	errList := NewErrorList()

	errList.Add(ErrorIfEmpty("Command", t.Command))
	errList.Add(ErrorIfOutOfBounds("RetryCount", t.RetryCount, 0, tc.MaxRetryCount))
	errList.Add(ErrorIfOutOfBounds("RetryInterval", t.RetryInterval, 0, tc.MaxRetryInterval))
	errList.Add(ErrorIfOutOfBounds("TimeOut", t.TimeOut, 0, tc.MaxTimeOut))

	return errList
}

// Complete() set the settings to the default values, if not specified in the test case definition
func (t *TestCase) Complete(tc *TestConfig) {
	SetDefaultIfNil(&t.RetryCount, &tc.DefaultRetryCount)
	SetDefaultIfNil(&t.RetryInterval, &tc.DefaultRetryInterval)
	SetDefaultIfNil(&t.TimeOut, &tc.DefaultTimeOut)
}

func (t *TestCase) Run(tc *TestConfig) *ErrorList {
	if t.BeforeTestMessage != "" {
		LogInfo(t.BeforeTestMessage + "\n")
	}

	errList := t.Test(tc)

	if t.AfterTestMessage != "" {
		LogInfo(t.AfterTestMessage + "\n")
	}

	return errList
}

func (t *TestCase) Test(tc *TestConfig) *ErrorList {
	var errList *ErrorList
	var exitCode int
	var output string
	var err error

	t.Complete(tc)

	retries := *t.RetryCount
	LogNormal("Testing %q...", t.Command)

	if t.BeforeTest != "" {
		LogWarning("\nRun pre-test configuration command %q", t.BeforeTest)
		exitCode, output, err = ExecCommandLine(t.BeforeTest, 0)

		if exitCode != 0 || err != nil {
			beforeTestErr := fmt.Sprintf("\nPre-test command %q returns with error: %v, exitcode: %v", t.BeforeTest, err, exitCode)
			LogError(beforeTestErr)
			LogNormal("\noutput:\n%v", output)
			return NewErrorList(fmt.Errorf(beforeTestErr))
		}

		LogSuccess(" Done\n")
	}

	retryFunc := func(errs *ErrorList) {
		if retries > 0 {
			LogWarning("\nError: %v, retrying after %v seconds ...", errs, *t.RetryInterval)
			time.Sleep(time.Duration(*t.RetryInterval) * time.Second)
		}
		retries--
	}

	for retries >= 0 {
		exitCode, output, err = ExecCommandLine(t.Command, *t.TimeOut)
		if tc.Verbose {
			LogNormal("\nexit code: %v, output:\n%v", exitCode, output)
		}

		if err != nil {
			errList = NewErrorList(err)
			retryFunc(errList)
			continue
		}

		errList = t.CheckTestResult(exitCode, output)
		if !errList.IsEmpty() {
			retryFunc(errList)
			continue
		}

		LogSuccess(" PASSED\n")
		return errList
	}

	LogError("\nFAILED: %v \n", errList.String())
	if !tc.Verbose {
		LogNormal("exit code: %v, output:\n%v", exitCode, output)
	}

	return errList
}

func (t *TestCase) CheckTestResult(exitCode int, output string) *ErrorList {
	errList := NewErrorList()
	if t.ShouldFail && exitCode == 0 {
		errList.Add(fmt.Errorf("Succeeded unexpectedly"))
	}

	if !t.ShouldFail && exitCode != 0 {
		errList.Add(fmt.Errorf("Command failed"))
	}

	if t.OutputShouldBe != "" && output != t.OutputShouldBe {
		errList.Add(fmt.Errorf("unexpected output: %q, expected: %q", output, t.OutputShouldBe))
	}

	if len(t.OutputShouldContain) > 0 {
		for _, expectedMatch := range t.OutputShouldContain {
			if !strings.Contains(output, expectedMatch) {
				errList.Add(fmt.Errorf("Did not find the match in output: %q", expectedMatch))
			}
		}
	}

	if len(t.OutputShouldNotContain) > 0 {
		for _, expectedNotMatch := range t.OutputShouldNotContain {
			if strings.Contains(output, expectedNotMatch) {
				errList.Add(fmt.Errorf("Find unexpected match in output: %q", expectedNotMatch))
			}
		}
	}

	return errList
}
