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

	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	internalinterfaces "k8s.io/client-go/informers/internalinterfaces"
	kubernetes "k8s.io/client-go/kubernetes"
	v1beta1 "k8s.io/client-go/listers/batch/v1beta1"
	cache "k8s.io/client-go/tools/cache"
)

// CronJobInformer provides access to a shared informer and lister for
// CronJobs.
type CronJobInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1beta1.CronJobLister
}

type cronJobInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
	tenant           string
}

// NewCronJobInformer constructs a new informer for CronJob type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCronJobInformer(client kubernetes.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCronJobInformer(client, namespace, resyncPeriod, indexers, nil)
}

func NewCronJobInformerWithMultiTenancy(client kubernetes.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tenant string) cache.SharedIndexInformer {
	return NewFilteredCronJobInformerWithMultiTenancy(client, namespace, resyncPeriod, indexers, nil, tenant)
}

// NewFilteredCronJobInformer constructs a new informer for CronJob type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCronJobInformer(client kubernetes.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return NewFilteredCronJobInformerWithMultiTenancy(client, namespace, resyncPeriod, indexers, tweakListOptions, "")
}

func NewFilteredCronJobInformerWithMultiTenancy(client kubernetes.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc, tenant string) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.BatchV1beta1().CronJobsWithMultiTenancy(namespace, tenant).List(options)
			},
			WatchFunc: func(options v1.ListOptions) watch.AggregatedWatchInterface {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.BatchV1beta1().CronJobsWithMultiTenancy(namespace, tenant).Watch(options)
			},
		},
		&batchv1beta1.CronJob{},
		resyncPeriod,
		indexers,
	)
}

func (f *cronJobInformer) defaultInformer(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredCronJobInformerWithMultiTenancy(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions, f.tenant)
}

func (f *cronJobInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&batchv1beta1.CronJob{}, f.defaultInformer)
}

func (f *cronJobInformer) Lister() v1beta1.CronJobLister {
	return v1beta1.NewCronJobLister(f.Informer().GetIndexer())
}
