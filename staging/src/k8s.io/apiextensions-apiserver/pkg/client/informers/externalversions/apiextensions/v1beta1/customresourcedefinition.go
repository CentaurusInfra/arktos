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

// Code generated by informer-gen. DO NOT EDIT.

package v1beta1

import (
	time "time"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	internalinterfaces "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/internalinterfaces"
	v1beta1 "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// CustomResourceDefinitionInformer provides access to a shared informer and lister for
// CustomResourceDefinitions.
type CustomResourceDefinitionInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1beta1.CustomResourceDefinitionLister
}

type customResourceDefinitionInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	tenant           string
}

// NewCustomResourceDefinitionInformer constructs a new informer for CustomResourceDefinition type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCustomResourceDefinitionInformer(client clientset.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCustomResourceDefinitionInformer(client, resyncPeriod, indexers, nil)
}

func NewCustomResourceDefinitionInformerWithMultiTenancy(client clientset.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tenant string) cache.SharedIndexInformer {
	return NewFilteredCustomResourceDefinitionInformerWithMultiTenancy(client, resyncPeriod, indexers, nil, tenant)
}

// NewFilteredCustomResourceDefinitionInformer constructs a new informer for CustomResourceDefinition type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCustomResourceDefinitionInformer(client clientset.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return NewFilteredCustomResourceDefinitionInformerWithMultiTenancy(client, resyncPeriod, indexers, tweakListOptions, "")
}

func NewFilteredCustomResourceDefinitionInformerWithMultiTenancy(client clientset.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc, tenant string) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ApiextensionsV1beta1().CustomResourceDefinitionsWithMultiTenancy(tenant).List(options)
			},
			WatchFunc: func(options v1.ListOptions) watch.AggregatedWatchInterface {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ApiextensionsV1beta1().CustomResourceDefinitionsWithMultiTenancy(tenant).Watch(options)
			},
		},
		&apiextensionsv1beta1.CustomResourceDefinition{},
		resyncPeriod,
		indexers,
	)
}

func (f *customResourceDefinitionInformer) defaultInformer(client clientset.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredCustomResourceDefinitionInformerWithMultiTenancy(client, resyncPeriod, cache.Indexers{cache.TenantIndex: cache.MetaTenantIndexFunc}, f.tweakListOptions, f.tenant)
}

func (f *customResourceDefinitionInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&apiextensionsv1beta1.CustomResourceDefinition{}, f.defaultInformer)
}

func (f *customResourceDefinitionInformer) Lister() v1beta1.CustomResourceDefinitionLister {
	return v1beta1.NewCustomResourceDefinitionLister(f.Informer().GetIndexer())
}
