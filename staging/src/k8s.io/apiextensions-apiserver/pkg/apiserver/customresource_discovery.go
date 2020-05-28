/*
Copyright 2017 The Kubernetes Authors.
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

package apiserver

import (
	"net/http"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	apidiscovery "k8s.io/apiserver/pkg/endpoints/discovery"
	apifilters "k8s.io/apiserver/pkg/endpoints/filters"
)

type versionDiscoveryHandler struct {
	// TODO, writing is infrequent, optimize this
	discoveryLock sync.RWMutex
	discoveryMap  map[string]map[schema.GroupVersion]*apidiscovery.APIVersionHandler

	delegate http.Handler
}

func (r *versionDiscoveryHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	tenant, err := apifilters.GetTenantFromQueryThenContext(req)
	pathParts := splitPath(req.URL.Path)
	// only match /apis/<group>/<version>
	if len(pathParts) != 3 || pathParts[0] != "apis" || err != nil {
		r.delegate.ServeHTTP(w, req)
		return
	}

	discovery, ok := r.getDiscovery(tenant, schema.GroupVersion{Group: pathParts[1], Version: pathParts[2]})
	if !ok {
		r.delegate.ServeHTTP(w, req)
		return
	}

	discovery.ServeHTTP(w, req)
}

func (r *versionDiscoveryHandler) getDiscovery(tenant string, gv schema.GroupVersion) (*apidiscovery.APIVersionHandler, bool) {
	r.discoveryLock.RLock()
	defer r.discoveryLock.RUnlock()

	if r.discoveryMap[tenant] != nil && r.discoveryMap[tenant][gv] != nil {
		return r.discoveryMap[tenant][gv], true
	}

	return nil, false
}

func (r *versionDiscoveryHandler) setDiscovery(tenant string, gv schema.GroupVersion, discovery *apidiscovery.APIVersionHandler) {
	r.discoveryLock.Lock()
	defer r.discoveryLock.Unlock()

	if _, ok := r.discoveryMap[tenant]; !ok {
		gvMap := make(map[schema.GroupVersion]*apidiscovery.APIVersionHandler)
		gvMap[gv] = discovery
		r.discoveryMap[tenant] = map[schema.GroupVersion]*apidiscovery.APIVersionHandler{gv: discovery}
		return
	}

	r.discoveryMap[tenant][gv] = discovery
}

func (r *versionDiscoveryHandler) unsetDiscovery(tenant string, gv schema.GroupVersion) {
	r.discoveryLock.Lock()
	defer r.discoveryLock.Unlock()
	if r.discoveryMap[tenant] != nil && r.discoveryMap[tenant][gv] != nil {
		delete(r.discoveryMap[tenant], gv)
		if len(r.discoveryMap[tenant]) == 0 {
			delete(r.discoveryMap, tenant)
		}
	}
}

type groupDiscoveryHandler struct {
	// TODO, writing is infrequent, optimize this
	discoveryLock sync.RWMutex
	discoveryMap  map[string]map[string]*apidiscovery.APIGroupHandler

	delegate http.Handler
}

func (r *groupDiscoveryHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	tenant, err := apifilters.GetTenantFromQueryThenContext(req)
	pathParts := splitPath(req.URL.Path)
	// only match /apis/<group>
	if len(pathParts) != 2 || pathParts[0] != "apis" || err != nil {
		r.delegate.ServeHTTP(w, req)
		return
	}

	discovery, ok := r.getDiscovery(tenant, pathParts[1])
	if !ok {
		r.delegate.ServeHTTP(w, req)
		return
	}

	discovery.ServeHTTP(w, req)
}

func (r *groupDiscoveryHandler) getDiscovery(tenant string, group string) (*apidiscovery.APIGroupHandler, bool) {
	r.discoveryLock.RLock()
	defer r.discoveryLock.RUnlock()

	if r.discoveryMap[tenant] != nil && r.discoveryMap[tenant][group] != nil {
		return r.discoveryMap[tenant][group], true
	}

	return nil, false
}

func (r *groupDiscoveryHandler) setDiscovery(tenant string, group string, discovery *apidiscovery.APIGroupHandler) {
	r.discoveryLock.Lock()
	defer r.discoveryLock.Unlock()

	if _, ok := r.discoveryMap[tenant]; !ok {
		tenantMap := make(map[string]*apidiscovery.APIGroupHandler)
		tenantMap[group] = discovery
		r.discoveryMap[tenant] = tenantMap
		return
	}

	r.discoveryMap[tenant][group] = discovery
}

func (r *groupDiscoveryHandler) unsetDiscovery(tenant string, group string) {
	r.discoveryLock.Lock()
	defer r.discoveryLock.Unlock()

	if r.discoveryMap[tenant] != nil && r.discoveryMap[tenant][group] != nil {
		delete(r.discoveryMap[tenant], group)
		if len(r.discoveryMap[tenant]) == 0 {
			delete(r.discoveryMap, tenant)
		}
	}
}

// splitPath returns the segments for a URL path.
func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}
