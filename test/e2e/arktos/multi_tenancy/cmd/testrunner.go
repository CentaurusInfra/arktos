/*
Copyright 2020 Authors of Arktos.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this tsFile except in compliance with the License.
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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog"
	"k8s.io/kubernetes/test/e2e/arktos/multi_tenancy/cmd/framework"
)

var (
	startTime           = time.Now()
	testConfig          framework.TestConfig
	testSuiteFiles      []string
	invalidTestSuites   []string
	validTestSuites     []*framework.TestSuite
	testSuiteDirFlag    string
	testSuiteFileFlag   string
	commonVariableFlag  string
	successTestSuiteNum int
	failedTestSuiteNum  int
)

func initFlags() {
	basedir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	defaultTestSuiteDir := filepath.Join(basedir, "test_suites")

	flag.IntVar(&testConfig.MaxRetryCount, "MaxRetryCount", 10, "maximal retry counts of a test command")
	flag.IntVar(&testConfig.MaxRetryInterval, "MaxRetryInterval", 60, "maximal retry interval in seconds")
	flag.IntVar(&testConfig.MaxTimeOut, "MaxTimeOut", 300, "maximal timeout in seconds allowed for a test command")
	flag.IntVar(&testConfig.DefaultRetryCount, "DefaultRetryCount", 0, "default retry counts of a test command")
	flag.IntVar(&testConfig.DefaultRetryInterval, "DefaultRetryInterval", 5, "default retry interval in seconds")
	flag.IntVar(&testConfig.DefaultTimeOut, "DefaultTimeOut", 2, "default timeout in seconds allowed for a test command")
	flag.BoolVar(&testConfig.Verbose, "Verbose", false, "Extra logging if true")

	flag.StringVar(&testSuiteDirFlag, "TestSuiteDir", defaultTestSuiteDir, "The directory of test suite files")
	flag.StringVar(&testSuiteFileFlag, "TestSuiteFiles", "", "The test suite files")
	flag.StringVar(&commonVariableFlag, "CommonVar", "", "Common variable definition used across test suites.")

	flag.Parse()

	validateFlags()
}

func validateFlags() {
	if errs := testConfig.Validate(); !errs.IsEmpty() {
		framework.LogError("\nThe test config is invalid: %v \n", errs)
		os.Exit(1)
	}

	if strings.TrimSpace(testSuiteFileFlag) == "" {
		framework.LogError("\nNo test suite file is specified.\n")
		os.Exit(1)
	}

	for _, matchingPattern := range strings.Fields(testSuiteFileFlag) {
		fullPattern := filepath.Join(testSuiteDirFlag, strings.TrimSpace(matchingPattern))
		matchingFiles, err := filepath.Glob(fullPattern)

		if err != nil {
			framework.LogError("\nFailed to get test suite file(s) %q due error: %v.\n", fullPattern, err)
			os.Exit(1)
		}

		// A pattern that does not match any file is unacceptable.
		// Such a pattern is usually a mistake.
		if len(matchingFiles) == 0 {
			framework.LogError("\nNo file(s) match %q.\n", fullPattern)
			os.Exit(1)
		}

		testSuiteFiles = append(testSuiteFiles, matchingFiles...)
	}

	if len(commonVariableFlag) > 0 {
		testConfig.CommonVariables = make(map[string]string)
		for _, kv := range strings.Split(commonVariableFlag, ",") {
			if len(strings.Split(kv, ":")) != 2 {
				framework.LogError("\n(%q) is not a valid variable definition. Ignored\n", kv)
				continue
			}

			parts := strings.Split(kv, ":")
			if strings.TrimSpace(parts[0]) == "" {
				framework.LogError("\n(%q) is not a valid variable definition. Ignored\n", kv)
				continue
			}

			testConfig.CommonVariables[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
}

func verifyLocalClusterUp() {
	var test framework.TestCase
	test.Command = "kubectl get nodes"

	errList := test.Run(&testConfig)
	if !errList.IsEmpty() {
		framework.LogError("\nArktos cluster is not up for test. Or you don't have cluster admin privilege.\n")
		os.Exit(1)
	}
}

func validateTestSuites() {
	for _, tsFile := range testSuiteFiles {
		var ts framework.TestSuite
		framework.LogInfo("\nValidating Test Suite %q...", tsFile)
		if err := ts.LoadTestSuite(tsFile, &testConfig); err != nil {
			invalidTestSuites = append(invalidTestSuites, tsFile)
			framework.LogError("\nWill skip Test Suite %q due to: %v\n", tsFile, err)
		} else {
			framework.LogSuccess("Validated")
			validTestSuites = append(validTestSuites, &ts)
		}
	}

	fmt.Println("")
}

func printSummary() {
	framework.LogInfo("~~~~~~~~~~~~~~~~~~~~~~~~~~~~ Test Run Summary ~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	framework.LogInfo("\nStarted %v \nFinished %v\n\n", startTime.Format(time.RFC3339), time.Now().Format(time.RFC3339))

	if len(invalidTestSuites) > 0 {
		framework.LogError("%d test suite file(s) are invalid\n", len(invalidTestSuites))
		for _, ts := range invalidTestSuites {
			framework.LogNormal("\t%v\n", ts)
		}
	}

	for _, ts := range validTestSuites {
		if len(ts.Failures) == 0 {
			framework.LogSuccess("\nTest Suite %v succeeded.\n", ts.FilePath)
			successTestSuiteNum++
			continue
		}

		framework.LogError("Test Suite %v has %d failures\n", ts.FilePath, len(ts.Failures))
		for _, failure := range ts.Failures {
			framework.LogWarning("\t" + failure + "\n")
		}
		failedTestSuiteNum++
	}
	framework.LogInfo("\nTotal %v test suite files, %v invalid, %d succeeded, %d failed.\n", len(testSuiteFiles), len(invalidTestSuites), successTestSuiteNum, failedTestSuiteNum)
}

func main() {
	defer klog.Flush()

	initFlags()

	verifyLocalClusterUp()

	validateTestSuites()

	for _, ts := range validTestSuites {
		ts.Run(&testConfig)
	}

	printSummary()

	exitCode := 0
	if len(testSuiteFiles) != successTestSuiteNum {
		exitCode = 1
	}

	os.Exit(exitCode)
}
