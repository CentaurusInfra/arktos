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

// Package clientutil contains utilities for creating clientset interface or rest kubeconfig from a given kubeconfig file
package clientutil

import (
	"fmt"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Create clientset from the kubeconfig file
// input: the file path to the kubeconfig file
// output: a clientset or error
func CreateClientFromKubeconfigFile(kubeconfigPath string, userAgent string) (clientset.Interface, error) {

	clientConfig, err := CreateClientConfigFromKubeconfigFile(kubeconfigPath, userAgent)
	if err != nil {
		return nil, err
	}

	client, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("error while creating clientset with %s, error %v", kubeconfigPath, err.Error())
	}

	return client, nil
}

// Create a client configuration with a given kubeconfig file, with default QPS, and contentType
//
func CreateClientConfigFromKubeconfigFile(kubeconfigPath string, userAgent string) (*restclient.Config, error) {
	return CreateClientConfigFromKubeconfigFileAndSetQps(kubeconfigPath, 0, 0, "", userAgent)
}

// Create a client configuration with a given kubeconfig file, with QPS, and contentType set
//
func CreateClientConfigFromKubeconfigFileAndSetQps(kubeconfigPath string, qps float32, burst int, contentType string, userAgent string) (*restclient.Config, error) {
	configs, err := CreateClientConfigFromKubeconfigFileAndSetQpsNoUserAgent(kubeconfigPath, qps, burst, contentType)
	if err != nil {
		return configs, err
	}
	for _, config := range configs.GetAllConfigs() {
		config.UserAgent = userAgent
	}
	return configs, nil
}

func CreateClientConfigFromKubeconfigFileAndSetQpsNoUserAgent(kubeconfigPath string, qps float32, burst int, contentType string) (*restclient.Config, error) {
	clientConfigs, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("error while loading kubeconfig from file %v: %v", kubeconfigPath, err)
	}
	configs, err := clientcmd.NewDefaultClientConfig(*clientConfigs, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error while creating kubeconfig: %v", err)
	}

	// set attributes for the config
	if contentType == "" {
		contentType = "application/json"
	}
	if qps > 0 || burst > 0 {
		for _, config := range configs.GetAllConfigs() {
			config.ContentType = contentType
			config.QPS = qps
			config.Burst = burst
		}
	}
	return configs, nil
}
