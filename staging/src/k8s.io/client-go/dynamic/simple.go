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

package dynamic

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

type dynamicClient struct {
	clients []*rest.RESTClient
}

var _ Interface = &dynamicClient{}

// ConfigFor returns a copy of the provided config with the
// appropriate dynamic client defaults set.
func ConfigFor(inConfigs *rest.Config) *rest.Config {
	if inConfigs == nil || len(inConfigs.GetAllConfigs()) == 0 {
		return inConfigs
	}

	allConfigs := rest.NewAggregatedConfig()
	for _, inConfig := range inConfigs.GetAllConfigs() {
		config := rest.CopyConfig(inConfig)
		config.AcceptContentTypes = "application/json"
		config.ContentType = "application/json"
		config.NegotiatedSerializer = basicNegotiatedSerializer{} // this gets used for discovery and error handling types
		if config.UserAgent == "" {
			config.UserAgent = rest.DefaultKubernetesUserAgent()
		}
		allConfigs.AddConfig(config)
	}

	return allConfigs
}

// NewForConfigOrDie creates a new Interface for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) Interface {
	ret, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return ret
}

// NewForConfig creates a new dynamic client or returns an error.
func NewForConfig(inConfigs *rest.Config) (Interface, error) {
	if inConfigs == nil || len(inConfigs.GetAllConfigs()) == 0 {
		return nil, errors.New("Input was empty")
	}

	configs := ConfigFor(inConfigs)
	restClients := make([]*rest.RESTClient, len(configs.GetAllConfigs()))
	for i, config := range configs.GetAllConfigs() {
		// for serializing the options
		config.GroupVersion = &schema.GroupVersion{}
		config.APIPath = "/if-you-see-this-search-for-the-break"

		restClient, err := rest.RESTClientFor(config)
		if err != nil {
			return nil, err
		}
		restClients[i] = restClient
	}

	return &dynamicClient{clients: restClients}, nil
}

type dynamicResourceClient struct {
	client    *dynamicClient
	tenant    string
	namespace string
	resource  schema.GroupVersionResource
}

func (c *dynamicClient) Resource(resource schema.GroupVersionResource) NamespaceableResourceInterface {
	return &dynamicResourceClient{client: c, resource: resource}
}

func (c *dynamicResourceClient) Namespace(ns string) ResourceInterface {
	return c.NamespaceWithMultiTenancy(ns, metav1.TenantSystem)
}

func (c *dynamicResourceClient) NamespaceWithMultiTenancy(ns string, tenant string) ResourceInterface {
	ret := *c
	ret.namespace = ns
	ret.tenant = tenant
	return &ret
}

func (c *dynamicResourceClient) getRestClient() *rest.RESTClient {
	max := len(c.client.clients)
	switch max {
	case 0:
		return nil
	case 1:
		return c.client.clients[0]
	default:
		rand.Seed(time.Now().UnixNano())
		ran := rand.Intn(max)
		return c.client.clients[ran]
	}
}

func (c *dynamicResourceClient) Create(obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	outBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
	if err != nil {
		return nil, err
	}

	name := ""
	if len(subresources) > 0 {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		name = accessor.GetName()
		if len(name) == 0 {
			return nil, fmt.Errorf("name is required")
		}
	}

	result := c.getRestClient().
		Post().
		AbsPath(append(c.makeURLSegments(name), subresources...)...).
		Body(outBytes).
		SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
		Do()

	if err := result.Error(); err != nil {
		return nil, err
	}

	retBytes, err := result.Raw()
	if err != nil {
		return nil, err
	}

	uncastObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, retBytes)
	if err != nil {
		return nil, err
	}

	return uncastObj.(*unstructured.Unstructured), nil
}

func (c *dynamicResourceClient) Update(obj *unstructured.Unstructured, opts metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	name := accessor.GetName()
	if len(name) == 0 {
		return nil, fmt.Errorf("name is required")
	}
	outBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
	if err != nil {
		return nil, err
	}

	result := c.getRestClient().
		Put().
		AbsPath(append(c.makeURLSegments(name), subresources...)...).
		Body(outBytes).
		SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
		Do()
	if err := result.Error(); err != nil {
		return nil, err
	}

	retBytes, err := result.Raw()
	if err != nil {
		return nil, err
	}
	uncastObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, retBytes)
	if err != nil {
		return nil, err
	}
	return uncastObj.(*unstructured.Unstructured), nil
}

func (c *dynamicResourceClient) UpdateStatus(obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	name := accessor.GetName()
	if len(name) == 0 {
		return nil, fmt.Errorf("name is required")
	}

	outBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
	if err != nil {
		return nil, err
	}

	result := c.getRestClient().
		Put().
		AbsPath(append(c.makeURLSegments(name), "status")...).
		Body(outBytes).
		SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
		Do()
	if err := result.Error(); err != nil {
		return nil, err
	}

	retBytes, err := result.Raw()
	if err != nil {
		return nil, err
	}
	uncastObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, retBytes)
	if err != nil {
		return nil, err
	}
	return uncastObj.(*unstructured.Unstructured), nil
}

func (c *dynamicResourceClient) Delete(name string, opts *metav1.DeleteOptions, subresources ...string) error {
	if len(name) == 0 {
		return fmt.Errorf("name is required")
	}
	if opts == nil {
		opts = &metav1.DeleteOptions{}
	}
	deleteOptionsByte, err := runtime.Encode(deleteOptionsCodec.LegacyCodec(schema.GroupVersion{Version: "v1"}), opts)
	if err != nil {
		return err
	}

	result := c.getRestClient().
		Delete().
		AbsPath(append(c.makeURLSegments(name), subresources...)...).
		Body(deleteOptionsByte).
		Do()
	return result.Error()
}

func (c *dynamicResourceClient) DeleteCollection(opts *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	if opts == nil {
		opts = &metav1.DeleteOptions{}
	}
	deleteOptionsByte, err := runtime.Encode(deleteOptionsCodec.LegacyCodec(schema.GroupVersion{Version: "v1"}), opts)
	if err != nil {
		return err
	}

	result := c.getRestClient().
		Delete().
		AbsPath(c.makeURLSegments("")...).
		Body(deleteOptionsByte).
		SpecificallyVersionedParams(&listOptions, dynamicParameterCodec, versionV1).
		Do()
	return result.Error()
}

func (c *dynamicResourceClient) Get(name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if len(name) == 0 {
		return nil, fmt.Errorf("name is required")
	}
	result := c.getRestClient().Get().AbsPath(append(c.makeURLSegments(name), subresources...)...).SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).Do()
	if err := result.Error(); err != nil {
		return nil, err
	}
	retBytes, err := result.Raw()
	if err != nil {
		return nil, err
	}
	uncastObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, retBytes)
	if err != nil {
		return nil, err
	}
	return uncastObj.(*unstructured.Unstructured), nil
}

func (c *dynamicResourceClient) List(opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	// When resource version is not empty, it reads from api server local cache
	// Need to check all api server partitions
	searchClients := make([]*rest.RESTClient, 1)
	if opts.ResourceVersion != "" && len(c.client.clients) > 1 {
		searchClients = c.client.clients
	} else {
		searchClients[0] = c.getRestClient()
	}

	var listLock sync.Mutex

	var wg sync.WaitGroup
	results := make(map[int]*unstructured.UnstructuredList)
	errs := make(map[int]error)
	for i, client := range searchClients {
		wg.Add(1)

		go func(ci rest.Interface, opts metav1.ListOptions, lock *sync.Mutex, pos int, resultMap map[int]*unstructured.UnstructuredList, errMap map[int]error) {
			defer wg.Done()

			r := ci.Get().AbsPath(c.makeURLSegments("")...).SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).Do()
			if err := r.Error(); err != nil {
				setResult(resultMap, errMap, lock, pos, nil, err)
				return
			}

			retBytes, err := r.Raw()
			if err != nil {
				setResult(resultMap, errMap, lock, pos, nil, err)
				return
			}

			uncastObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, retBytes)
			if err != nil {
				setResult(resultMap, errMap, lock, pos, nil, err)
				return
			}

			if list, ok := uncastObj.(*unstructured.UnstructuredList); ok {
				setResult(resultMap, errMap, lock, pos, list, nil)
				return
			}

			list, err := uncastObj.(*unstructured.Unstructured).ToList()
			if err != nil {
				setResult(resultMap, errMap, lock, pos, nil, err)
				return
			} else {
				setResult(resultMap, errMap, lock, pos, list, nil)
			}

		}(client, opts, &listLock, i, results, errs)
	}
	wg.Wait()

	// consolidate list result
	var aggList *unstructured.UnstructuredList
	rv := ""
	for i := 0; i < len(searchClients); i++ {
		if errs[i] != nil {
			return nil, errs[i]
		}

		if rv == "" {
			rv = results[i].GetResourceVersion()
			aggList = results[i]
		} else {
			isNewer, errCompare := diff.RevisionStrIsNewer(results[i].GetResourceVersion(), rv)
			if errCompare != nil {
				err := apierrors.NewInternalError(fmt.Errorf("Invalid resource version [%v]", errCompare))
				return nil, err
			}

			if isNewer {
				rv := results[i].GetResourceVersion()
				aggList.SetResourceVersion(rv)
			}

			aggList.Items = append(aggList.Items, results[i].Items...)
			aggList.Object["items"] = aggList.Items
		}
	}

	return aggList, nil
}

func setResult(resultMap map[int]*unstructured.UnstructuredList, errMap map[int]error, lock *sync.Mutex, pos int, result *unstructured.UnstructuredList, err error) {
	lock.Lock()
	resultMap[pos] = result
	errMap[pos] = err
	lock.Unlock()
}

func (c *dynamicResourceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	aggWatch := watch.NewAggregatedWatcher()
	for _, client := range c.client.clients {
		watcher, err := client.Get().AbsPath(c.makeURLSegments("")...).
			SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
			Watch()
		aggWatch.AddWatchInterface(watcher, err)
	}
	return aggWatch, aggWatch.GetErrors()
}

func (c *dynamicResourceClient) Patch(name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if len(name) == 0 {
		return nil, fmt.Errorf("name is required")
	}
	result := c.getRestClient().
		Patch(pt).
		AbsPath(append(c.makeURLSegments(name), subresources...)...).
		Body(data).
		SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
		Do()
	if err := result.Error(); err != nil {
		return nil, err
	}
	retBytes, err := result.Raw()
	if err != nil {
		return nil, err
	}
	uncastObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, retBytes)
	if err != nil {
		return nil, err
	}
	return uncastObj.(*unstructured.Unstructured), nil
}

func (c *dynamicResourceClient) makeURLSegments(name string) []string {
	url := []string{}
	if len(c.resource.Group) == 0 {
		url = append(url, "api")
	} else {
		url = append(url, "apis", c.resource.Group)
	}
	url = append(url, c.resource.Version)

	if len(c.tenant) > 0 {
		url = append(url, "tenants", c.tenant)
	}

	if len(c.namespace) > 0 {
		url = append(url, "namespaces", c.namespace)
	}
	url = append(url, c.resource.Resource)

	if len(name) > 0 {
		url = append(url, name)
	}

	return url
}
