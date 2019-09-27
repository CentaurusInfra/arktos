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
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/workload-controller-manager/app"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/api/types/v1alpha1"
	clientV1alpha1 "k8s.io/kubernetes/pkg/cloudfabric-controller/clientset/v1alpha1"

	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	controllerManagerConfig "k8s.io/kubernetes/cmd/workload-controller-manager/app/config"
)

var kubeconfig string
var controllerconfigfilepath string

const (
	// WorkloadControllerManagerUserAgent is the userAgent name when starting workload-controller managers.
	WorkloadControllerManagerUserAgent = "workload-controller-manager"
)

func init() {
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfigEnv, "absolute path to the kubeconfig file")
	flag.StringVar(&controllerconfigfilepath, "controllerconfig", "", "absolute path to the controllerconfig file")
	flag.Parse()
}

func main() {
	var config *rest.Config
	var err error

	logs.InitLogs()
	defer logs.FlushLogs()

	controllerconfig, err := controllerManagerConfig.NewControllerConfig(controllerconfigfilepath)

	if err != nil {
		panic(err)
	} else {
		fmt.Println("using controller configuration from ", controllerconfigfilepath)
	}

	if kubeconfig == "" {
		fmt.Println("using in-cluster configuration")
		config, err = rest.InClusterConfig()
	} else {
		fmt.Println("using configuration from ", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		panic(err)
	}

	v1alpha1.AddToScheme(scheme.Scheme)

	clientSet, err := clientV1alpha1.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	controllermanagers, err := clientSet.ControllerManagers("default").List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Printf("controllermanagers found: %+v\n", controllermanagers)

	/*
		store := WatchResources(clientSet)

		for {
			controllermanagersFromStore := store.List()
			fmt.Printf("controllermanagers in store: %d\n", len(controllermanagersFromStore))

			time.Sleep(2 * time.Second)
		}*/

	controllerManagerKubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}

	client, err := clientset.NewForConfig(restclient.AddUserAgent(controllerManagerKubeConfig, WorkloadControllerManagerUserAgent))
	if err != nil {
		panic(err)
	}

	//eventRecorder := createRecorder(client, WorkloadControllerManagerUserAgent)

	c := &controllerManagerConfig.Config{
		Client:                  client,
		ControllerManagerConfig: controllerManagerKubeConfig,
		ControllerTypeConfig:    controllerconfig,
		//EventRecorder:        eventRecorder,
	}

	app.StartControllerManager(c.Complete(), wait.NeverStop)
}

/*
func createRecorder(kubeClient clientset.Interface, userAgent string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	return eventBroadcaster.NewRecorder(clientgokubescheme.Scheme, v1.EventSource{Component: userAgent})
}
*/
