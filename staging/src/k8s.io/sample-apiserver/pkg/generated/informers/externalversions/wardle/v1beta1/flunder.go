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

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
	wardlev1beta1 "k8s.io/sample-apiserver/pkg/apis/wardle/v1beta1"
	versioned "k8s.io/sample-apiserver/pkg/generated/clientset/versioned"
	internalinterfaces "k8s.io/sample-apiserver/pkg/generated/informers/externalversions/internalinterfaces"
	v1beta1 "k8s.io/sample-apiserver/pkg/generated/listers/wardle/v1beta1"
)

// FlunderInformer provides access to a shared informer and lister for
// Flunders.
type FlunderInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1beta1.FlunderLister
}

type flunderInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
	tenant           string
}

// NewFlunderInformer constructs a new informer for Flunder type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFlunderInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredFlunderInformer(client, namespace, resyncPeriod, indexers, nil)
}

func NewFlunderInformerWithMultiTenancy(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tenant string) cache.SharedIndexInformer {
	return NewFilteredFlunderInformerWithMultiTenancy(client, namespace, resyncPeriod, indexers, nil, tenant)
}

// NewFilteredFlunderInformer constructs a new informer for Flunder type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredFlunderInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return NewFilteredFlunderInformerWithMultiTenancy(client, namespace, resyncPeriod, indexers, tweakListOptions, "")
}

func NewFilteredFlunderInformerWithMultiTenancy(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc, tenant string) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.WardleV1beta1().FlundersWithMultiTenancy(namespace, tenant).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.WardleV1beta1().FlundersWithMultiTenancy(namespace, tenant).Watch(options)
			},
		},
		&wardlev1beta1.Flunder{},
		resyncPeriod,
		indexers,
	)
}

func (f *flunderInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredFlunderInformerWithMultiTenancy(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions, f.tenant)
}

func (f *flunderInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&wardlev1beta1.Flunder{}, f.defaultInformer)
}

func (f *flunderInformer) Lister() v1beta1.FlunderLister {
	return v1beta1.NewFlunderLister(f.Informer().GetIndexer())
}
