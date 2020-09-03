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
	"flag"
	"fmt"
	"k8s.io/klog"
	"net"
	"os"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/workload-controller-manager/app"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client metric registration
	_ "k8s.io/kubernetes/pkg/version/prometheus"        // for version metric registration

	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	controllerManagerConfig "k8s.io/kubernetes/cmd/workload-controller-manager/app/config"
	"k8s.io/kubernetes/pkg/master/ports"
)

var kubeconfigFilename string
var controllerconfigfilepath string
var workloadControllerPort int

const (
	// WorkloadControllerManagerUserAgent is the userAgent name when starting workload-controller managers.
	WorkloadControllerManagerUserAgent = "workload-controller-manager"
)

func init() {
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	flag.StringVar(&kubeconfigFilename, "kubeconfig", kubeconfigEnv, "absolute path to the kubeconfig file")
	flag.StringVar(&controllerconfigfilepath, "controllerconfig", "", "absolute path to the controllerconfig file")
	flag.IntVar(&workloadControllerPort, "port", ports.InsecureWorkloadControllerManagerPort, "port for current workload controller manager rest service")
	flag.Parse()
}

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	c, err := getConfig()
	if err != nil {
		panic(err)
	}

	//eventRecorder := createRecorder(client, WorkloadControllerManagerUserAgent)
	app.StartControllerManager(c.Complete(), wait.NeverStop)
}

func getConfig() (*controllerManagerConfig.Config, error) {
	controllerconfig, err := controllerManagerConfig.NewControllerConfig(controllerconfigfilepath)

	if err != nil {
		return nil, err
	} else {
		fmt.Println("using controller configuration from ", controllerconfigfilepath)
	}

	controllerManagerKubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigFilename)
	if err != nil {
		return nil, err
	}

	kubeConfigs := controllerManagerKubeConfig.GetAllConfigs()
	for _, kubeConfig := range kubeConfigs {
		kubeConfig.ContentType = "application/vnd.kubernetes.protobuf"
		kubeConfig.QPS = controllerconfig.GetQPS()
		kubeConfig.Burst = int(kubeConfig.QPS * 2)
	}

	c := &controllerManagerConfig.Config{
		ControllerManagerConfig: controllerManagerKubeConfig,
		ControllerTypeConfig:    controllerconfig,
	}

	klog.Infof("Current workload controller port %d", workloadControllerPort)
	klog.Infof("Rest client QPS %v", controllerManagerKubeConfig.GetConfig().QPS)

	insecureServing := (&apiserveroptions.DeprecatedInsecureServingOptions{
		BindAddress: net.ParseIP("0.0.0.0"),
		BindPort:    workloadControllerPort,
		BindNetwork: "tcp",
	}).WithLoopback()

	err = insecureServing.ApplyTo(&c.InsecureServing, &c.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	return c, nil
}
