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
	"fmt"
	"os"

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
