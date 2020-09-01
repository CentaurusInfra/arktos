/*
Copyright The Kubernetes Authors.
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

// Code generated by client-gen. DO NOT EDIT.

package internalversion

import (
	rand "math/rand"
	"sync"
	"time"

	apiserverupdate "k8s.io/client-go/apiserverupdate"
	rest "k8s.io/client-go/rest"
	"k8s.io/code-generator/_examples/apiserver/clientset/internalversion/scheme"
	klog "k8s.io/klog"
)

type SecondExampleInterface interface {
	RESTClient() rest.Interface
	RESTClients() []rest.Interface
	TestTypesGetter
}

// SecondExampleClient is used to interact with features provided by the example.test.apiserver.code-generator.k8s.io group.
type SecondExampleClient struct {
	restClients []rest.Interface
	configs     *rest.Config
	mux         sync.RWMutex
}

func (c *SecondExampleClient) TestTypes(namespace string) TestTypeInterface {
	return newTestTypesWithMultiTenancy(c, namespace, "system")
}

func (c *SecondExampleClient) TestTypesWithMultiTenancy(namespace string, tenant string) TestTypeInterface {
	return newTestTypesWithMultiTenancy(c, namespace, tenant)
}

// NewForConfig creates a new SecondExampleClient for the given config.
func NewForConfig(c *rest.Config) (*SecondExampleClient, error) {
	configs := rest.CopyConfigs(c)
	if err := setConfigDefaults(configs); err != nil {
		return nil, err
	}

	clients := make([]rest.Interface, len(configs.GetAllConfigs()))
	for i, config := range configs.GetAllConfigs() {
		client, err := rest.RESTClientFor(config)
		if err != nil {
			return nil, err
		}
		clients[i] = client
	}

	obj := &SecondExampleClient{
		restClients: clients,
		configs:     configs,
	}

	obj.run()

	return obj, nil
}

// NewForConfigOrDie creates a new SecondExampleClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *SecondExampleClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new SecondExampleClient for the given RESTClient.
func New(c rest.Interface) *SecondExampleClient {
	clients := []rest.Interface{c}
	return &SecondExampleClient{restClients: clients}
}

func setConfigDefaults(configs *rest.Config) error {
	for _, config := range configs.GetAllConfigs() {
		config.APIPath = "/apis"
		if config.UserAgent == "" {
			config.UserAgent = rest.DefaultKubernetesUserAgent()
		}
		if config.GroupVersion == nil || config.GroupVersion.Group != scheme.Scheme.PrioritizedVersionsForGroup("example.test.apiserver.code-generator.k8s.io")[0].Group {
			gv := scheme.Scheme.PrioritizedVersionsForGroup("example.test.apiserver.code-generator.k8s.io")[0]
			config.GroupVersion = &gv
		}
		config.NegotiatedSerializer = scheme.Codecs

		if config.QPS == 0 {
			config.QPS = 5
		}
		if config.Burst == 0 {
			config.Burst = 10
		}
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *SecondExampleClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}

	c.mux.RLock()
	defer c.mux.RUnlock()
	max := len(c.restClients)
	if max == 0 {
		return nil
	}
	if max == 1 {
		return c.restClients[0]
	}

	rand.Seed(time.Now().UnixNano())
	ran := rand.Intn(max)
	return c.restClients[ran]
}

// RESTClients returns all RESTClient that are used to communicate
// with all API servers by this client implementation.
func (c *SecondExampleClient) RESTClients() []rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClients
}

// run watch api server instance updates and recreate connections to new set of api servers
func (c *SecondExampleClient) run() {
	go func(c *SecondExampleClient) {
		member := c.configs.WatchUpdate()
		watcherForUpdateComplete := apiserverupdate.GetClientSetsWatcher()
		watcherForUpdateComplete.AddWatcher()

		for range member.Read {
			// create new client
			clients := make([]rest.Interface, len(c.configs.GetAllConfigs()))
			for i, config := range c.configs.GetAllConfigs() {
				client, err := rest.RESTClientFor(config)
				if err != nil {
					klog.Fatalf("Cannot create rest client for [%+v], err %v", config, err)
					return
				}
				clients[i] = client
			}
			c.mux.Lock()
			klog.Infof("Reset restClients. length %v -> %v", len(c.restClients), len(clients))
			c.restClients = clients
			c.mux.Unlock()
			watcherForUpdateComplete.NotifyDone()
		}
	}(c)
}
