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
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog"
)

const ShellToUse = "bash"

type FontColor string

const (
	ResetColor  = FontColor("\033[0m")
	RedColor    = FontColor("\033[31m")
	GreenColor  = FontColor("\033[32m")
	YellowColor = FontColor("\033[33m")
	BlueColor   = FontColor("\033[34m")
	PurpleColor = FontColor("\033[35m")
	CyanColor   = FontColor("\033[36m")
	GrayColor   = FontColor("\033[37m")
	WhiteColor  = FontColor("\033[97m")
)

func ApplyColor(s string, color FontColor) string {
	return string(color) + s + string(ResetColor)
}

func RandomString(length int) string {
	// If we see a crazy length, set it the default length of 8
	if length <= 0 || length > 128 {
		length = 8
	}

	result := fmt.Sprintf("%v", uuid.NewUUID())[0:length]

	return result
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

	return exitCode, string(output), nil
}

func LogError(format string, a ...interface{}) {
	klog.Infof(format, a...)
	fmt.Printf(ApplyColor(format, RedColor), a...)
}

func LogWarning(format string, a ...interface{}) {
	klog.Warningf(format, a...)
	fmt.Printf(ApplyColor(format, YellowColor), a...)
}

func LogInfo(format string, a ...interface{}) {
	klog.Infof(format, a...)
	fmt.Printf(ApplyColor(format, CyanColor), a...)
}

func LogSuccess(format string, a ...interface{}) {
	klog.Infof(format, a...)
	fmt.Printf(ApplyColor(format, GreenColor), a...)
}

func LogNormal(format string, a ...interface{}) {
	klog.Infof(format, a...)
	fmt.Printf(format, a...)
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Merge maps, where the later map overrides the previous if there are conflicts
func MergeStringMaps(maps ...map[string]string) map[string]string {
	merged := make(map[string]string)

	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}

	return merged
}

func SetDefaultIfNil(ptr **int, defaultPtr *int) {
	if *ptr == nil {
		defaultValue := *defaultPtr
		*ptr = &defaultValue
	}
}
