/*
Copyright 2018 The Kubernetes Authors.
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

package config

import (
	"k8s.io/kubernetes/perf-tests/clusterloader2/api"
)

// ClusterLoaderConfig represents all single test run parameters used by CLusterLoader.
type ClusterLoaderConfig struct {
	ClusterConfig     ClusterConfig
	ReportDir         string
	EnableExecService bool
	TestScenario      api.TestScenario
	PrometheusConfig  PrometheusConfig
}

// ClusterConfig is a structure that represents cluster description.
type ClusterConfig struct {
	KubeConfigPath             string
	RPAccess                   bool
	Nodes                      int
	Provider                   string
	EtcdCertificatePath        string
	EtcdKeyPath                string
	EtcdInsecurePort           int
	MasterIPs                  []string
	MasterInternalIPs          []string
	MasterName                 string
	KubemarkRootKubeConfigPath string
	Apiserverextranum          int //APISERVER_EXTRA_NUM
	DeleteStaleNamespaces      bool
	// SSHToMasterSupported determines whether SSH access to master machines is possible.
	// If false (impossible for many  providers), ClusterLoader will skip operations requiring it.
	SSHToMasterSupported bool
	// APIServerPprofByClientEnabled determines whether kube-apiserver pprof endpoint can be accessed
	// using kubernetes client. If false, clusterloader will avoid collecting kube-apiserver profiles.
	APIServerPprofByClientEnabled bool
	KubeletPort                   int
}

// PrometheusConfig represents all flags used by prometheus.
type PrometheusConfig struct {
	EnableServer       bool
	TearDownServer     bool
	ScrapeEtcd         bool
	ScrapeNodeExporter bool
	ScrapeKubelets     bool
	ScrapeKubeProxy    bool
}

// GetMasterIp returns the first master ip, added for backward compatibility.
// TODO(mmatt): Remove this method once all the codebase is migrated to support multiple masters.
func (c *ClusterConfig) GetMasterIp() string {
	if len(c.MasterIPs) > 0 {
		return c.MasterIPs[0]
	}
	return ""
}

// GetMasterInternalIp returns the first internal master ip, added for backward compatibility.
// TODO(mmatt): Remove this method once all the codebase is migrated to support multiple masters.
func (c *ClusterConfig) GetMasterInternalIp() string {
	if len(c.MasterInternalIPs) > 0 {
		return c.MasterInternalIPs[0]
	}
	return ""
}
