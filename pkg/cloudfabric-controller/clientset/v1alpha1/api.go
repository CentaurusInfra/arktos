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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/api/types/v1alpha1"
)

// DefaultV1Alpha1Interface interface for default controller managers
type DefaultV1Alpha1Interface interface {
	ControllerManagers(namespace string) ControllerManagerInterface
}

// DefaultV1Alpha1Client client for default controller managers
type DefaultV1Alpha1Client struct {
	restClient rest.Interface
}

// NewForConfig new config for default controller managers
func NewForConfig(c *rest.Config) (*DefaultV1Alpha1Client, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: v1alpha1.GroupName, Version: v1alpha1.GroupVersion}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &DefaultV1Alpha1Client{restClient: client}, nil
}

// ControllerManagers interface for default controller managers
func (c *DefaultV1Alpha1Client) ControllerManagers(namespace string) ControllerManagerInterface {
	return &controllerManagerClient{
		restClient: c.restClient,
		ns:         namespace,
	}
}
