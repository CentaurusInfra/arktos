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

// Package keyutil contains utilities for managing public/private key pairs.
package clientutil

import (
	"fmt"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func CreateClientFromKubeconfigFile(kubeconfigPath string) (clientset.Interface, error) {

	clientConfig, err := CreateClientConfigFromKubeconfigFile(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	client, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("error while creating clientset with %s, error %v", kubeconfigPath, err.Error())
	}

	return client, nil
}

func CreateClientConfigFromKubeconfigFile(kubeconfigPath string) (*restclient.Config, error) {
	return CreateClientConfigFromKubeconfigFileAndSetQps(kubeconfigPath, 0, 0, "")
}

func CreateClientConfigFromKubeconfigFileAndSetQps(kubeconfigPath string, qps float32, burst int, contentType string) (*restclient.Config, error) {
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
