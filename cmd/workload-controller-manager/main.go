/*
Copyright 2019 The Kubernetes Authors.

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
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	controllerManagerConfig "k8s.io/kubernetes/cmd/workload-controller-manager/app/config"
	"k8s.io/kubernetes/pkg/master/ports"
)

var kubeconfig string
var controllerconfigfilepath string
var workloadControllerPort int

const (
	// WorkloadControllerManagerUserAgent is the userAgent name when starting workload-controller managers.
	WorkloadControllerManagerUserAgent = "workload-controller-manager"
)

func init() {
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfigEnv, "absolute path to the kubeconfig file")
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

	controllerManagerKubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	client, err := clientset.NewForConfig(restclient.AddUserAgent(controllerManagerKubeConfig, WorkloadControllerManagerUserAgent))
	if err != nil {
		return nil, err
	}

	c := &controllerManagerConfig.Config{
		Client:                  client,
		ControllerManagerConfig: controllerManagerKubeConfig,
		ControllerTypeConfig:    controllerconfig,
		//EventRecorder:        eventRecorder,
	}

	klog.Infof("Current workload controller port %d", workloadControllerPort)

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

/*
func createRecorder(kubeClient clientset.Interface, userAgent string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	return eventBroadcaster.NewRecorder(clientgokubescheme.Scheme, v1.EventSource{Component: userAgent})
}
*/
