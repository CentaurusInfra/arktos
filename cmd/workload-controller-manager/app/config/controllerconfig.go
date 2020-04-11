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

package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type controllerTypes struct {
	Types []controllerType `json:"controllers"`
}

type controllerType struct {
	Type                         string `json:"type"`
	Workers                      int    `json:"workers"`
	ReportHealthIntervalInSecond int    `json:"reportHealthIntervalInSecond"`
}

// ControllerConfigMap is the config map to load controller configurations
type ControllerConfigMap struct {
	typemap map[string]ControllerConfig
}

// ControllerConfig stores configured values for a controller
type ControllerConfig struct {
	Workers                      int
	ReportHealthIntervalInSecond int
}

// NewControllerConfigMap to load configuration from a local file
func NewControllerConfigMap(filePath string) (ControllerConfigMap, error) {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return ControllerConfigMap{}, err
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return ControllerConfigMap{}, err
	}

	var types controllerTypes

	json.Unmarshal([]byte(byteValue), &types)

	var controllerMap map[string]ControllerConfig
	controllerMap = make(map[string]ControllerConfig)
	for _, controllerType := range types.Types {
		controllerMap[controllerType.Type] = ControllerConfig{Workers: controllerType.Workers, ReportHealthIntervalInSecond: controllerType.ReportHealthIntervalInSecond}
	}
	return ControllerConfigMap{typemap: controllerMap}, nil
}

func (c *ControllerConfigMap) GetControllerConfig(controllerType string) (ControllerConfig, bool) {
	controllerConfig, isOK := c.typemap[controllerType]
	return controllerConfig, isOK
}
