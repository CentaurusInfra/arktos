/*
Copyright 2014 The Kubernetes Authors.
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

package clientcmd

import (
	"io"
	"sync"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/klog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// DeferredLoadingClientConfig is a ClientConfig interface that is backed by a client config loader.
// It is used in cases where the loading rules may change after you've instantiated them and you want to be sure that
// the most recent rules are used.  This is useful in cases where you bind flags to loading rule parameters before
// the parse happens and you want your calling code to be ignorant of how the values are being mutated to avoid
// passing extraneous information down a call stack
type DeferredLoadingClientConfig struct {
	loader         ClientConfigLoader
	overrides      *ConfigOverrides
	fallbackReader io.Reader

	clientConfigs []ClientConfig
	loadingLock   sync.Mutex

	// provided for testing
	icc InClusterConfig
}

// InClusterConfig abstracts details of whether the client is running in a cluster for testing.
type InClusterConfig interface {
	ClientConfig
	Possible() bool
}

// NewNonInteractiveDeferredLoadingClientConfig creates a ConfigClientClientConfig using the passed context name
func NewNonInteractiveDeferredLoadingClientConfig(loader ClientConfigLoader, overrides *ConfigOverrides) ClientConfig {
	return &DeferredLoadingClientConfig{loader: loader, overrides: overrides, icc: &inClusterClientConfig{overrides: overrides}}
}

// NewInteractiveDeferredLoadingClientConfig creates a ConfigClientClientConfig using the passed context name and the fallback auth reader
func NewInteractiveDeferredLoadingClientConfig(loader ClientConfigLoader, overrides *ConfigOverrides, fallbackReader io.Reader) ClientConfig {
	return &DeferredLoadingClientConfig{loader: loader, overrides: overrides, icc: &inClusterClientConfig{overrides: overrides}, fallbackReader: fallbackReader}
}

func (config *DeferredLoadingClientConfig) createClientConfig() ([]ClientConfig, error) {
	if len(config.clientConfigs) == 0 {
		config.loadingLock.Lock()
		defer config.loadingLock.Unlock()

		if len(config.clientConfigs) == 0 {
			mergedConfigs, err := config.loader.Load()
			if err != nil {
				return nil, err
			}

			for _, mergedConfig := range mergedConfigs {
				var mergedClientConfig ClientConfig
				if config.fallbackReader != nil {
					mergedClientConfig = NewInteractiveClientConfig(*mergedConfig, config.overrides.CurrentContext, config.overrides, config.fallbackReader, config.loader)
				} else {
					mergedClientConfig = NewNonInteractiveClientConfig(*mergedConfig, config.overrides.CurrentContext, config.overrides, config.loader)
				}
				//config.clientConfig = mergedClientConfig
				config.clientConfigs = append(config.clientConfigs, mergedClientConfig)
			}
		}
	}

	return config.clientConfigs, nil
}

func (c *DeferredLoadingClientConfig) RawConfig() ([]clientcmdapi.Config, error) {
	mergedConfigs, err := c.createClientConfig()
	if err != nil || len(mergedConfigs) == 0 {
		return []clientcmdapi.Config{}, err
	}

	errlist := []error{}
	configReturns := []clientcmdapi.Config{}
	for _, mergedConfig := range mergedConfigs {
		configs, err := mergedConfig.RawConfig()
		if err != nil {
			errlist = append(errlist, err)
		} else {
			for _, config := range configs {
				configReturns = append(configReturns, config)
			}
		}
	}

	return configReturns, utilerrors.NewAggregate(errlist)
}

// ClientConfig implements ClientConfig
func (config *DeferredLoadingClientConfig) ClientConfig() (*restclient.Config, error) {
	createdClientConfigs, err := config.createClientConfig()
	if err != nil || len(createdClientConfigs) == 0 {
		return nil, err
	}

	klog.V(6).Infof("createdClientConfigs len %d", len(createdClientConfigs))
	var returnConfigs *restclient.Config
	var returnConfig *restclient.Config
	isDefault := true
	for _, createdClientConfig := range createdClientConfigs {
		// load the configuration and return on non-empty errors and if the
		// content differs from the default config
		returnConfig, err = createdClientConfig.ClientConfig()

		if returnConfig != nil {
			for _, configToAdd := range returnConfig.GetAllConfigs() {
				if returnConfigs == nil {
					returnConfigs = restclient.NewAggregatedConfig(configToAdd)
				} else {
					returnConfigs.AddConfig(configToAdd)
				}
			}
		}

		switch {
		case err != nil:
			if !IsEmptyConfig(err) {
				// return on any error except empty config
				return nil, err
			}
		case returnConfig != nil:
			// the configuration is valid, but if this is equal to the defaults we should try
			// in-cluster configuration
			for _, configToCheck := range returnConfig.GetAllConfigs() {
				if !config.loader.IsDefaultConfig(configToCheck) {
					isDefault = false
				}
			}
		}
	}

	if !isDefault {
		klog.V(6).Infof("return configs len %d", len(returnConfigs.GetAllConfigs()))
		return returnConfigs, nil
	}

	// check for in-cluster configuration and use it
	if config.icc.Possible() {
		klog.V(4).Infof("Using in-cluster configuration")
		return config.icc.ClientConfig()
	}

	// return the result of the merged client config
	if returnConfigs != nil {
		klog.V(6).Infof("return configs len %d", len(returnConfigs.GetAllConfigs()))
	}
	return returnConfigs, err
}

// Tenant implements KubeConfig
func (config *DeferredLoadingClientConfig) Tenant() (string, bool, error) {
	mergedKubeConfigs, err := config.createClientConfig()
	if err != nil || len(mergedKubeConfigs) == 0 {
		return "", false, err
	}

	// TODO - verify single kubeconfig is enough
	mergedKubeConfig := mergedKubeConfigs[0]
	te, overridden, err := mergedKubeConfig.Tenant()
	// if we get an error and it is not empty config, or if the merged config defined an explicit tenant, or
	// if in-cluster config is not possible, return immediately
	if (err != nil && !IsEmptyConfig(err)) || overridden || !config.icc.Possible() {
		// return on any error except empty config
		return te, overridden, err
	}

	if len(te) > 0 {
		// if we got a non-default tenant from the kubeconfig, use it
		if te != metav1.TenantSystem {
			return te, false, nil
		}

		// if we got a default tenant, determine whether it was explicit or implicit
		if raw, err := mergedKubeConfig.RawConfig(); err == nil {
			// determine the current context
			currentContext := raw[0].CurrentContext
			if config.overrides != nil && len(config.overrides.CurrentContext) > 0 {
				currentContext = config.overrides.CurrentContext
			}
			if context := raw[0].Contexts[currentContext]; context != nil && len(context.Tenant) > 0 {
				return te, false, nil
			}
		}
	}

	klog.V(4).Infof("Using in-cluster tenant")

	// allow the tenant from the service account token directory to be used.
	return config.icc.Tenant()
}

// Namespace implements KubeConfig
func (config *DeferredLoadingClientConfig) Namespace() (string, bool, error) {
	mergedKubeConfigs, err := config.createClientConfig()
	if err != nil || len(mergedKubeConfigs) == 0 {
		return "", false, err
	}

	// TODO - verify single kubeconfig is enough
	mergedKubeConfig := mergedKubeConfigs[0]
	ns, overridden, err := mergedKubeConfig.Namespace()
	// if we get an error and it is not empty config, or if the merged config defined an explicit namespace, or
	// if in-cluster config is not possible, return immediately
	if (err != nil && !IsEmptyConfig(err)) || overridden || !config.icc.Possible() {
		// return on any error except empty config
		return ns, overridden, err
	}

	if len(ns) > 0 {
		// if we got a non-default namespace from the kubeconfig, use it
		if ns != "default" {
			return ns, false, nil
		}

		// if we got a default namespace, determine whether it was explicit or implicit
		if raw, err := mergedKubeConfig.RawConfig(); err == nil {
			// determine the current context
			currentContext := raw[0].CurrentContext
			if config.overrides != nil && len(config.overrides.CurrentContext) > 0 {
				currentContext = config.overrides.CurrentContext
			}
			if context := raw[0].Contexts[currentContext]; context != nil && len(context.Namespace) > 0 {
				return ns, false, nil
			}
		}
	}

	klog.V(4).Infof("Using in-cluster namespace")

	// allow the namespace from the service account token directory to be used.
	return config.icc.Namespace()
}

// ConfigAccess implements ClientConfig
func (config *DeferredLoadingClientConfig) ConfigAccess() ConfigAccess {
	return config.loader
}
