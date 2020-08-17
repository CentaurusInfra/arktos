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
	"k8s.io/klog"
	"os"
	"time"
)

const defaultResyncPeriod = 12 * time.Hour // default resync value from kube controller manager

type controllerManagerConfig struct {
	ReportHealthIntervalInSecond int              `json:"reportHealthIntervalInSecond"`
	QPS                          float32          `json:"qps""`
	Types                        []controllerType `json:"controllers"`
	ResyncPeriodStr              string           `json:"resyncPeriod"`
}

type controllerType struct {
	Type    string `json:"type"`
	Workers int    `json:"workers"`
}

// ControllerConfig is the config to load controller configurations
type ControllerConfig struct {
	typemap                      map[string]int
	reportHealthIntervalInSecond int
	qps                          float32
	resyncPeriod                 time.Duration
}

// NewControllerConfig to load configuration from a local file
func NewControllerConfig(filePath string) (ControllerConfig, error) {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return ControllerConfig{}, err
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return ControllerConfig{}, err
	}

	var types controllerManagerConfig

	json.Unmarshal([]byte(byteValue), &types)

	var controllerMap map[string]int
	controllerMap = make(map[string]int)
	for _, controllerType := range types.Types {
		controllerMap[controllerType.Type] = controllerType.Workers
	}

	resyncPeriod, err := time.ParseDuration(types.ResyncPeriodStr)
	if err != nil {
		resyncPeriod = defaultResyncPeriod
	}

	return ControllerConfig{
		typemap:                      controllerMap,
		reportHealthIntervalInSecond: types.ReportHealthIntervalInSecond,
		qps:                          types.QPS,
		resyncPeriod:                 resyncPeriod,
	}, nil
}

func (c *ControllerConfig) GetWorkerNumber(controllerType string) (int, bool) {
	workerNumber, isOK := c.typemap[controllerType]
	return workerNumber, isOK
}

func (c *ControllerConfig) GetReportHealthIntervalInSecond() int {
	return c.reportHealthIntervalInSecond
}

func (c *ControllerConfig) GetQPS() float32 {
	if c.qps == 0 {
		c.qps = 20
		klog.Info("Configured QPS is 0. Force setting to 20")
	}

	return c.qps
}

func (c *ControllerConfig) GetDeafultResyncPeriod() time.Duration {
	if c.resyncPeriod == 0 {
		c.resyncPeriod = defaultResyncPeriod
		klog.Infof("Configured resync period is 0. Force setting to %v", defaultResyncPeriod)
	}
	return c.resyncPeriod
}
