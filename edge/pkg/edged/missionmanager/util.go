/*
Copyright 2015 The Kubernetes Authors.
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

package missionmanager

import (
	"context"
	"os"
	"os/exec"
	"time"

	"k8s.io/klog/v2"
)

const ShellToUse = "bash"

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func ExecCommandLine(commandline string, timeout int) (int, string, error) {
	var cmd *exec.Cmd
	if timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		cmd = exec.CommandContext(ctx, ShellToUse, "-c", commandline)
	} else {
		cmd = exec.Command(ShellToUse, "-c", commandline)
	}

	exitCode := 0
	var output []byte
	var err error

	if output, err = cmd.CombinedOutput(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}

	if exitCode != 0 || err != nil {
		klog.Errorf("Command (%v) failed: exitcode: %v, output (%v), error: %v", commandline, exitCode, string(output), err)
	} else {
		klog.V(3).Infof("Running Command (%v) succeeded", commandline)
	}

	return exitCode, string(output), err
}
