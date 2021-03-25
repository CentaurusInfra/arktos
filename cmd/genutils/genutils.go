/*
Copyright 2015 The Kubernetes Authors.
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

package genutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OutDir creates the absolute path name from path and checks path exists.
// Returns absolute path including trailing '/' or error if path does not exist.
func OutDir(path string) (string, error) {
	outDir, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	stat, err := os.Stat(outDir)
	if err != nil {
		return "", err
	}

	if !stat.IsDir() {
		return "", fmt.Errorf("output directory %s is not a directory", outDir)
	}
	outDir = outDir + "/"
	return outDir, nil
}

// ParseKubeConfigFiles gets a string that contains one or multiple kubeconfig files.
// If there are more than one file, separated by comma
// Returns an array of filenames whose existence are validated
func ParseKubeConfigFiles(kubeConfigFilenames string) ([]string, bool) {
	kubeConfigs := strings.Split(kubeConfigFilenames, ",")
	for _, kubeConfig := range kubeConfigs {
		_, err := os.Stat(kubeConfig)
		if err != nil {
			return nil, false
		}
	}
	return kubeConfigs, true
}
