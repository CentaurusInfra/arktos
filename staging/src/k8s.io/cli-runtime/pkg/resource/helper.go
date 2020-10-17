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

package resource

import (
	"math/rand"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

var metadataAccessor = meta.NewAccessor()

// Helper provides methods for retrieving or mutating a RESTful
// resource.
type Helper struct {
	// The name of this resource as the server would recognize it
	Resource string
	// A RESTClient capable of mutating this resource.
	RESTClient RESTClient
	// A list of RESTClients connecting to api server with data partitions.
	RESTClients []RESTClient
	// True if the resource type is scoped to namespaces
	NamespaceScoped bool
	// True if the resource type is scoped to tenants
	TenantScoped bool
}

// NewHelper creates a Helper from a ResourceMapping
func NewHelper(clients []RESTClient, mapping *meta.RESTMapping) *Helper {
	h := &Helper{
		Resource:        mapping.Resource.Resource,
		RESTClients:     clients,
		NamespaceScoped: mapping.Scope.Name() == meta.RESTScopeNameNamespace,
		TenantScoped:    mapping.Scope.Name() == meta.RESTScopeNameNamespace || mapping.Scope.Name() == meta.RESTScopeNameTenant,
	}

	max := len(clients)
	if max == 1 {
		h.RESTClient = clients[0]
	}

	if max > 1 {
		rand.Seed(time.Now().UnixNano())
		ran := rand.Intn(max)
		h.RESTClient = clients[ran]
	}

	return h
}

func (m *Helper) Get(namespace, name string, export bool) (runtime.Object, error) {
	req := m.RESTClient.Get().
		NamespaceIfScoped(namespace, m.NamespaceScoped).
		Resource(m.Resource).
		Name(name)
	if export {
		// TODO: I should be part of GetOptions
		req.Param("export", strconv.FormatBool(export))
	}
	return req.Do().Get()
}

func (m *Helper) List(namespace, apiVersion string, export bool, options *metav1.ListOptions) (runtime.Object, error) {
	req := m.RESTClient.Get().
		NamespaceIfScoped(namespace, m.NamespaceScoped).
		Resource(m.Resource).
		VersionedParams(options, metav1.ParameterCodec)
	if export {
		// TODO: I should be part of ListOptions
		req.Param("export", strconv.FormatBool(export))
	}
	return req.Do().Get()
}

func (m *Helper) Watch(namespace, apiVersion string, options *metav1.ListOptions) (watch.Interface, error) {
	options.Watch = true
	aggWatch := watch.NewAggregatedWatcher()

	for _, client := range m.RESTClients {
		aggWatch.AddWatchInterface(client.Get().
			NamespaceIfScoped(namespace, m.NamespaceScoped).
			Resource(m.Resource).
			VersionedParams(options, metav1.ParameterCodec).
			Watch())
	}
	return aggWatch, aggWatch.GetErrors()
}

func (m *Helper) WatchSingle(namespace, name, resourceVersion string) (watch.Interface, error) {
	aggWatch := watch.NewAggregatedWatcher()
	for _, client := range m.RESTClients {
		aggWatch.AddWatchInterface(client.Get().
			NamespaceIfScoped(namespace, m.NamespaceScoped).
			Resource(m.Resource).
			VersionedParams(&metav1.ListOptions{
				ResourceVersion: resourceVersion,
				Watch:           true,
				FieldSelector:   fields.OneTermEqualSelector("metadata.name", name).String(),
			}, metav1.ParameterCodec).
			Watch())
	}
	return aggWatch, aggWatch.GetErrors()
}

func (m *Helper) Delete(namespace, name string) (runtime.Object, error) {
	return m.DeleteWithOptions(namespace, name, nil)
}

func (m *Helper) DeleteWithOptions(namespace, name string, options *metav1.DeleteOptions) (runtime.Object, error) {
	return m.RESTClient.Delete().
		NamespaceIfScoped(namespace, m.NamespaceScoped).
		Resource(m.Resource).
		Name(name).
		Body(options).
		Do().
		Get()
}

func (m *Helper) Create(namespace string, modify bool, obj runtime.Object, options *metav1.CreateOptions) (runtime.Object, error) {
	if options == nil {
		options = &metav1.CreateOptions{}
	}
	if modify {
		// Attempt to version the object based on client logic.
		version, err := metadataAccessor.ResourceVersion(obj)
		if err != nil {
			// We don't know how to clear the version on this object, so send it to the server as is
			return m.createResource(m.RESTClient, m.Resource, namespace, obj, options)
		}
		if version != "" {
			if err := metadataAccessor.SetResourceVersion(obj, ""); err != nil {
				return nil, err
			}
		}
	}

	return m.createResource(m.RESTClient, m.Resource, namespace, obj, options)
}

func (m *Helper) createResource(c RESTClient, resource, namespace string, obj runtime.Object, options *metav1.CreateOptions) (runtime.Object, error) {
	return c.Post().
		NamespaceIfScoped(namespace, m.NamespaceScoped).
		Resource(resource).
		VersionedParams(options, metav1.ParameterCodec).
		Body(obj).
		Do().
		Get()
}
func (m *Helper) Patch(namespace, name string, pt types.PatchType, data []byte, options *metav1.PatchOptions) (runtime.Object, error) {
	if options == nil {
		options = &metav1.PatchOptions{}
	}
	return m.RESTClient.Patch(pt).
		NamespaceIfScoped(namespace, m.NamespaceScoped).
		Resource(m.Resource).
		Name(name).
		VersionedParams(options, metav1.ParameterCodec).
		Body(data).
		Do().
		Get()
}

func (m *Helper) Replace(namespace, name string, overwrite bool, obj runtime.Object) (runtime.Object, error) {
	c := m.RESTClient

	// Attempt to version the object based on client logic.
	version, err := metadataAccessor.ResourceVersion(obj)
	if err != nil {
		// We don't know how to version this object, so send it to the server as is
		return m.replaceResource(c, m.Resource, namespace, name, obj)
	}
	if version == "" && overwrite {
		// Retrieve the current version of the object to overwrite the server object
		serverObj, err := c.Get().NamespaceIfScoped(namespace, m.NamespaceScoped).Resource(m.Resource).Name(name).Do().Get()
		if err != nil {
			// The object does not exist, but we want it to be created
			return m.replaceResource(c, m.Resource, namespace, name, obj)
		}
		serverVersion, err := metadataAccessor.ResourceVersion(serverObj)
		if err != nil {
			return nil, err
		}
		if err := metadataAccessor.SetResourceVersion(obj, serverVersion); err != nil {
			return nil, err
		}
	}

	return m.replaceResource(c, m.Resource, namespace, name, obj)
}

func (m *Helper) replaceResource(c RESTClient, resource, namespace, name string, obj runtime.Object) (runtime.Object, error) {
	return c.Put().NamespaceIfScoped(namespace, m.NamespaceScoped).Resource(resource).Name(name).Body(obj).Do().Get()
}

func (m *Helper) GetWithMultiTenancy(tenant, namespace, name string, export bool) (runtime.Object, error) {
	req := m.RESTClient.Get().
		TenantIfScoped(tenant, m.TenantScoped).
		NamespaceIfScoped(namespace, m.NamespaceScoped).
		Resource(m.Resource).
		Name(name)
	if export {
		// TODO: I should be part of GetOptions
		req.Param("export", strconv.FormatBool(export))
	}
	return req.Do().Get()
}

func (m *Helper) ListWithMultiTenancy(tenant, namespace, apiVersion string, export bool, options *metav1.ListOptions) (runtime.Object, error) {
	req := m.RESTClient.Get().
		TenantIfScoped(tenant, m.TenantScoped).
		NamespaceIfScoped(namespace, m.NamespaceScoped).
		Resource(m.Resource).
		VersionedParams(options, metav1.ParameterCodec)
	if export {
		// TODO: I should be part of ListOptions
		req.Param("export", strconv.FormatBool(export))
	}
	return req.Do().Get()
}

func (m *Helper) WatchWithMultiTenancy(tenant, namespace, apiVersion string, options *metav1.ListOptions) (watch.Interface, error) {
	options.Watch = true
	aggWatch := watch.NewAggregatedWatcher()
	for _, client := range m.RESTClients {
		aggWatch.AddWatchInterface(client.Get().
			TenantIfScoped(tenant, m.TenantScoped).
			NamespaceIfScoped(namespace, m.NamespaceScoped).
			Resource(m.Resource).
			VersionedParams(options, metav1.ParameterCodec).
			Watch())
	}
	return aggWatch, aggWatch.GetErrors()
}

func (m *Helper) WatchSingleWithMultiTenancy(tenant, namespace, name, resourceVersion string) (watch.Interface, error) {
	aggWatch := watch.NewAggregatedWatcher()
	for _, client := range m.RESTClients {
		aggWatch.AddWatchInterface(client.Get().
			TenantIfScoped(tenant, m.TenantScoped).
			NamespaceIfScoped(namespace, m.NamespaceScoped).
			Resource(m.Resource).
			VersionedParams(&metav1.ListOptions{
				ResourceVersion: resourceVersion,
				Watch:           true,
				FieldSelector:   fields.OneTermEqualSelector("metadata.name", name).String(),
			}, metav1.ParameterCodec).
			Watch())
	}
	return aggWatch, aggWatch.GetErrors()
}

func (m *Helper) DeleteWithMultiTenancy(tenant, namespace, name string) (runtime.Object, error) {
	return m.DeleteWithOptionsWithMultiTenancy(tenant, namespace, name, nil)
}

func (m *Helper) DeleteWithOptionsWithMultiTenancy(tenant, namespace, name string, options *metav1.DeleteOptions) (runtime.Object, error) {
	return m.RESTClient.Delete().
		TenantIfScoped(tenant, m.TenantScoped).
		NamespaceIfScoped(namespace, m.NamespaceScoped).
		Resource(m.Resource).
		Name(name).
		Body(options).
		Do().
		Get()
}

func (m *Helper) CreateWithMultiTenancy(tenant, namespace string, modify bool, obj runtime.Object, options *metav1.CreateOptions) (runtime.Object, error) {
	if options == nil {
		options = &metav1.CreateOptions{}
	}
	if modify {
		// Attempt to version the object based on client logic.
		version, err := metadataAccessor.ResourceVersion(obj)
		if err != nil {
			// We don't know how to clear the version on this object, so send it to the server as is
			return m.createResourceWithMultiTenancy(m.RESTClient, m.Resource, tenant, namespace, obj, options)
		}
		if version != "" {
			if err := metadataAccessor.SetResourceVersion(obj, ""); err != nil {
				return nil, err
			}
		}
	}

	return m.createResourceWithMultiTenancy(m.RESTClient, m.Resource, tenant, namespace, obj, options)
}

func (m *Helper) createResourceWithMultiTenancy(c RESTClient, resource, tenant, namespace string, obj runtime.Object, options *metav1.CreateOptions) (runtime.Object, error) {
	return c.Post().
		TenantIfScoped(tenant, m.TenantScoped).
		NamespaceIfScoped(namespace, m.NamespaceScoped).
		Resource(resource).
		VersionedParams(options, metav1.ParameterCodec).
		Body(obj).
		Do().
		Get()
}
func (m *Helper) PatchWithMultiTenancy(tenant, namespace, name string, pt types.PatchType, data []byte, options *metav1.PatchOptions) (runtime.Object, error) {
	if options == nil {
		options = &metav1.PatchOptions{}
	}
	return m.RESTClient.Patch(pt).
		TenantIfScoped(tenant, m.TenantScoped).
		NamespaceIfScoped(namespace, m.NamespaceScoped).
		Resource(m.Resource).
		Name(name).
		VersionedParams(options, metav1.ParameterCodec).
		Body(data).
		Do().
		Get()
}

func (m *Helper) ReplaceWithMultiTenancy(tenant, namespace, name string, overwrite bool, obj runtime.Object) (runtime.Object, error) {
	c := m.RESTClient

	// Attempt to version the object based on client logic.
	version, err := metadataAccessor.ResourceVersion(obj)
	if err != nil {
		// We don't know how to version this object, so send it to the server as is
		return m.replaceResourceWithMultiTenancy(c, m.Resource, tenant, namespace, name, obj)
	}
	if version == "" && overwrite {
		// Retrieve the current version of the object to overwrite the server object
		serverObj, err := c.Get().
			TenantIfScoped(tenant, m.TenantScoped).
			NamespaceIfScoped(namespace, m.NamespaceScoped).
			Resource(m.Resource).
			Name(name).
			Do().
			Get()
		if err != nil {
			// The object does not exist, but we want it to be created
			return m.replaceResourceWithMultiTenancy(c, m.Resource, tenant, namespace, name, obj)
		}
		serverVersion, err := metadataAccessor.ResourceVersion(serverObj)
		if err != nil {
			return nil, err
		}
		if err := metadataAccessor.SetResourceVersion(obj, serverVersion); err != nil {
			return nil, err
		}
	}

	return m.replaceResourceWithMultiTenancy(c, m.Resource, tenant, namespace, name, obj)
}

func (m *Helper) replaceResourceWithMultiTenancy(c RESTClient, resource, tenant, namespace, name string, obj runtime.Object) (runtime.Object, error) {
	return c.Put().
		TenantIfScoped(tenant, m.TenantScoped).
		NamespaceIfScoped(namespace, m.NamespaceScoped).
		Resource(resource).
		Name(name).
		Body(obj).
		Do().
		Get()
}
