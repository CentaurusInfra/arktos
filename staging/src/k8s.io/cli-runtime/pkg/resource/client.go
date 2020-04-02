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

package resource

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// TODO require negotiatedSerializer.  leaving it optional lets us plumb current behavior and deal with the difference after major plumbing is complete

// Separate clientsForGroupVersion and clientForGroupVersion, unstructuredClientsForGroupVersion and unstructuredClientForGroupVersion
// --- trying to avoid creating connections to all api servers when there is no need

func (clientConfigFn ClientConfigFunc) clientForGroupVersion(gv schema.GroupVersion, negotiatedSerializer runtime.NegotiatedSerializer) (RESTClient, error) {
	cfgs, err := clientConfigFn()
	if err != nil {
		return nil, err
	}
	cfg := cfgs.GetConfig()
	if negotiatedSerializer != nil {
		cfg.ContentConfig.NegotiatedSerializer = negotiatedSerializer
	}
	cfg.GroupVersion = &gv
	if len(gv.Group) == 0 {
		cfg.APIPath = "/api"
	} else {
		cfg.APIPath = "/apis"
	}

	return rest.RESTClientFor(cfg)
}

func (clientConfigFn ClientConfigFunc) clientsForGroupVersion(gv schema.GroupVersion, negotiatedSerializer runtime.NegotiatedSerializer) ([]RESTClient, []error) {
	cfgs, err := clientConfigFn()
	if err != nil {
		return nil, []error{err}
	}

	max := len(cfgs.GetAllConfigs())
	clients := make([]RESTClient, max)
	errs := make([]error, max)
	hasError := false
	for i, cfg := range cfgs.GetAllConfigs() {
		if negotiatedSerializer != nil {
			cfg.ContentConfig.NegotiatedSerializer = negotiatedSerializer
		}
		cfg.GroupVersion = &gv
		if len(gv.Group) == 0 {
			cfg.APIPath = "/api"
		} else {
			cfg.APIPath = "/apis"
		}
		clients[i], errs[i] = rest.RESTClientFor(cfg)
		if errs[i] != nil {
			hasError = true
		}
	}

	// flatten erros for easy checking
	if !hasError {
		errs = nil
	}

	return clients, errs
}

func (clientConfigFn ClientConfigFunc) unstructuredClientForGroupVersion(gv schema.GroupVersion) (RESTClient, error) {
	cfgs, err := clientConfigFn()
	if err != nil {
		return nil, err
	}
	cfg := cfgs.GetConfig()
	cfg.ContentConfig = UnstructuredPlusDefaultContentConfig()
	cfg.GroupVersion = &gv
	if len(gv.Group) == 0 {
		cfg.APIPath = "/api"
	} else {
		cfg.APIPath = "/apis"
	}

	return rest.RESTClientFor(cfg)
}

func (clientConfigFn ClientConfigFunc) unstructuredClientsForGroupVersion(gv schema.GroupVersion) ([]RESTClient, []error) {
	cfgs, err := clientConfigFn()
	if err != nil {
		return nil, []error{err}
	}

	max := len(cfgs.GetAllConfigs())
	clients := make([]RESTClient, max)
	errs := make([]error, max)
	hasError := false
	for i, cfg := range cfgs.GetAllConfigs() {
		cfg.ContentConfig = UnstructuredPlusDefaultContentConfig()
		cfg.GroupVersion = &gv
		if len(gv.Group) == 0 {
			cfg.APIPath = "/api"
		} else {
			cfg.APIPath = "/apis"
		}

		clients[i], errs[i] = rest.RESTClientFor(cfg)
		if errs[i] != nil {
			hasError = true
		}
	}

	// flatten erros for easy checking
	if !hasError {
		errs = nil
	}

	return clients, errs
}
