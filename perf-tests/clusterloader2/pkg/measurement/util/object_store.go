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

/*
This file is copy of https://github.com/kubernetes/kubernetes/blob/master/test/utils/pod_store.go
with slight changes regarding labelSelector and flagSelector applied.
*/

package util

import (
	"fmt"
	"k8s.io/klog"
	"reflect"
	"strings"
	"sync"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/perf-tests/clusterloader2/pkg/util"
)

// ObjectStore is a convenient wrapper around cache.Store.
type ObjectStore struct {
	cache.Store
	stopCh    chan struct{}
	Reflector *cache.Reflector
}

// newObjectStore creates ObjectStore based on given object selector.
func newObjectStore(obj runtime.Object, lw *cache.ListWatch, selector *ObjectSelector, indexers cache.Indexers) (*ObjectStore, error) {
	var store cache.Store
	if indexers == nil {
		store = cache.NewStore(cache.MetaNamespaceKeyFunc)
	} else {
		store = cache.NewIndexer(cache.MetaNamespaceKeyFunc, indexers)
	}
	stopCh := make(chan struct{})
	name := fmt.Sprintf("%sStore", reflect.TypeOf(obj).String())
	if selector != nil {
		name = name + ": " + selector.String()
	}
	reflector := cache.NewNamedReflector(name, lw, obj, store, 0, false)
	go reflector.Run(stopCh)
	if err := wait.PollImmediate(50*time.Millisecond, 2*time.Minute, func() (bool, error) {
		if len(reflector.LastSyncResourceVersion()) != 0 {
			return true, nil
		}
		return false, nil
	}); err != nil {
		close(stopCh)
		return nil, fmt.Errorf("couldn't initialize %s: %v", name, err)
	}
	return &ObjectStore{
		Store:     store,
		stopCh:    stopCh,
		Reflector: reflector,
	}, nil
}

// Stop stops ObjectStore watch.
func (s *ObjectStore) Stop() {
	close(s.stopCh)
}

// PodStore is a convenient wrapper around cache.Store.
type PodStore struct {
	*ObjectStore
	listener      int
	podListerFunc func(labelKey string, labelValue string, ns string) ([]*v1.Pod, error)
}

const (
	labelNameKeyIndex  = "label.name"
	labelGroupKeyIndex = "label.group"

	labelNameKey  = "name"
	labelGroupKey = "group"
)

var podStore *PodStore = nil
var initPodStoreLock sync.Mutex

// NewPodStore creates PodStore based on given object selector.
func NewPodStore(c clientset.Interface, selector *ObjectSelector) (*PodStore, error) {
	initPodStoreLock.Lock()
	defer initPodStoreLock.Unlock()

	if podStore != nil {
		podStore.listener++
		klog.V(4).Infof("ADD PodStore listener, total %v", podStore.listener)
		return podStore, nil
	}

	var err error
	podStore, err = initPodStore(c)
	return podStore, err
}

func initPodStore(c clientset.Interface) (*PodStore, error) {
	klog.V(4).Infof("initPodStore")
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return c.CoreV1().PodsWithMultiTenancy(metav1.NamespaceAll, util.GetTenant()).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return c.CoreV1().PodsWithMultiTenancy(metav1.NamespaceAll, util.GetTenant()).Watch(options)
		},
	}
	labelIndexers := cache.Indexers{
		labelNameKeyIndex: func(obj interface{}) ([]string, error) {
			pod, ok := obj.(*v1.Pod)
			if !ok {
				return []string{}, nil
			}
			if nameLabel, isOK := pod.Labels[labelNameKey]; isOK {
				return []string{fmt.Sprintf("%s/%s", pod.Namespace, nameLabel)}, nil
			}
			return []string{}, nil
		},
		labelGroupKeyIndex: func(obj interface{}) ([]string, error) {
			pod, ok := obj.(*v1.Pod)
			if !ok {
				return []string{}, nil
			}
			if groupLabel, isOK := pod.Labels[labelGroupKey]; isOK {
				return []string{groupLabel}, nil
			}
			return []string{}, nil
		},
	}

	objectStore, err := newObjectStore(&v1.Pod{}, lw, nil, labelIndexers)
	if err != nil {
		return nil, err
	}

	index, _ := objectStore.Store.(cache.Indexer)
	podListerFunc := func(labelKey string, labelValue string, ns string) ([]*v1.Pod, error) {
		var err error
		var objs []interface{}
		switch labelKey {
		case labelNameKey:
			objs, err = index.ByIndex(labelNameKeyIndex, fmt.Sprintf("%s/%s", ns, labelValue))
		case labelGroupKey:
			objs, err = index.ByIndex(labelGroupKeyIndex, labelValue)
		default:
			err = fmt.Errorf("Not supported index [%v]", labelKey)
		}
		if err != nil {
			return nil, err
		}
		pods := make([]*v1.Pod, 0, len(objs))
		for _, obj := range objs {
			pod, ok := obj.(*v1.Pod)
			if !ok {
				continue
			}
			pods = append(pods, pod)
		}
		return pods, nil
	}

	return &PodStore{ObjectStore: objectStore, listener: 1, podListerFunc: podListerFunc}, nil
}

// List returns list of pods (that satisfy conditions provided to NewPodStore).
func (s *PodStore) List() []*v1.Pod {
	objects := s.Store.List()
	pods := make([]*v1.Pod, 0, len(objects))
	for _, o := range objects {
		pods = append(pods, o.(*v1.Pod))
	}
	return pods
}

func (s *PodStore) Stop() {
	initPodStoreLock.Lock()
	defer initPodStoreLock.Unlock()
	if podStore == nil {
		return
	}
	if podStore.listener == 1 {
		klog.V(4).Infof("Stop PodStore")
		close(s.stopCh)
		podStore = nil
	} else {
		podStore.listener--
		klog.V(4).Infof("REMOVE PodStore listener, total %v", podStore.listener)
	}
}

func FilterPods(ps *PodStore, selector *ObjectSelector) []*v1.Pod {
	filteredPods := make([]*v1.Pod, 0)

	// if has name label, use podLister func, otherwise, check every label
	var selectorMap map[string]string
	if selector.LabelSelector != "" {
		selectorMap = getLabelSelectorMapFromString(selector.LabelSelector)
	}
	if ps.podListerFunc != nil {
		labelKeyName := ""
		labelKeyValue := ""
		if name, isOK := selectorMap[labelNameKey]; isOK {
			labelKeyName = labelNameKey
			labelKeyValue = name
		} else if group, isOK := selectorMap[labelGroupKey]; isOK {
			labelKeyName = labelGroupKey
			labelKeyValue = group
		}
		if labelKeyName != "" && labelKeyValue != "" {
			pods, err := ps.podListerFunc(labelKeyName, labelKeyValue, selector.Namespace)
			if err != nil {
				return filteredPods
			}
			return pods
		}
	}

	// Keep the log here. It should be ok to have a few. Need to take a look if there is a lot of such message as
	// 	it is an indication that index was not used and test is heavily loaded in below logic.
	klog.Infof("Label did not match, search all pods by label")

	pods := ps.List()
	for _, pod := range pods {
		if selector.Namespace != "" && pod.Namespace != selector.Namespace {
			continue
		}
		if selector.LabelSelector != "" {
			if !isLabelMatch(selectorMap, pod.Labels) {
				continue
			}
		}
		// TODO - FieldSelector
		if selector.FieldSelector != "" {
			klog.Warningf("FieldSelector not supported. selector FS [%s]", selector.FieldSelector)
		}
		filteredPods = append(filteredPods, pod)
	}

	return filteredPods
}

func isLabelMatch(targetLS map[string]string, objSelector map[string]string) bool {
	if len(targetLS) == 0 {
		return true
	}

	for k, v := range targetLS {
		if value, isOK := objSelector[k]; !isOK || value != v {
			return false
		}
	}

	return true
}

func getLabelSelectorMapFromString(ls string) map[string]string {
	separator := ";" // assume label selectors are separated by ;
	labels := strings.Split(ls, separator)
	lsMap := make(map[string]string, len(labels))
	for _, label := range labels {
		values := strings.Split(label, "=")
		if len(values) != 2 {
			// currently only handle k = v case
			continue
		}
		lsMap[strings.TrimSpace(values[0])] = strings.TrimSpace(values[1])
	}
	return lsMap
}

// PVCStore is a convenient wrapper around cache.Store.
type PVCStore struct {
	*ObjectStore
}

// NewPVCStore creates PVCStore based on a given object selector.
func NewPVCStore(c clientset.Interface, selector *ObjectSelector) (*PVCStore, error) {
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.LabelSelector = selector.LabelSelector
			options.FieldSelector = selector.FieldSelector
			return c.CoreV1().PersistentVolumeClaimsWithMultiTenancy(selector.Namespace, util.GetTenant()).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = selector.LabelSelector
			options.FieldSelector = selector.FieldSelector
			return c.CoreV1().PersistentVolumeClaimsWithMultiTenancy(selector.Namespace, util.GetTenant()).Watch(options)
		},
	}
	objectStore, err := newObjectStore(&v1.PersistentVolumeClaim{}, lw, selector, nil)
	if err != nil {
		return nil, err
	}
	return &PVCStore{ObjectStore: objectStore}, nil
}

// List returns list of pvcs (that satisfy conditions provided to NewPVCStore).
func (s *PVCStore) List() []*v1.PersistentVolumeClaim {
	objects := s.Store.List()
	pvcs := make([]*v1.PersistentVolumeClaim, 0, len(objects))
	for _, o := range objects {
		pvcs = append(pvcs, o.(*v1.PersistentVolumeClaim))
	}
	return pvcs
}

// PVStore is a convenient wrapper around cache.Store.
type PVStore struct {
	*ObjectStore
}

// NewPVStore creates PVStore based on a given object selector.
func NewPVStore(c clientset.Interface, selector *ObjectSelector) (*PVStore, error) {
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.LabelSelector = selector.LabelSelector
			options.FieldSelector = selector.FieldSelector
			return c.CoreV1().PersistentVolumesWithMultiTenancy(util.GetTenant()).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = selector.LabelSelector
			options.FieldSelector = selector.FieldSelector
			return c.CoreV1().PersistentVolumesWithMultiTenancy(util.GetTenant()).Watch(options)
		},
	}
	objectStore, err := newObjectStore(&v1.PersistentVolume{}, lw, selector, nil)
	if err != nil {
		return nil, err
	}
	return &PVStore{ObjectStore: objectStore}, nil
}

// List returns list of pvs (that satisfy conditions provided to NewPVStore).
func (s *PVStore) List() []*v1.PersistentVolume {
	objects := s.Store.List()
	pvs := make([]*v1.PersistentVolume, 0, len(objects))
	for _, o := range objects {
		pvs = append(pvs, o.(*v1.PersistentVolume))
	}
	return pvs
}

// NodeStore is a convenient wrapper around cache.Store.
type NodeStore struct {
	*ObjectStore
}

// NewNodeStore creates NodeStore based on a given object selector.
func NewNodeStore(c clientset.Interface, selector *ObjectSelector) (*NodeStore, error) {
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.LabelSelector = selector.LabelSelector
			options.FieldSelector = selector.FieldSelector
			return c.CoreV1().Nodes().List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = selector.LabelSelector
			options.FieldSelector = selector.FieldSelector
			return c.CoreV1().Nodes().Watch(options)
		},
	}
	objectStore, err := newObjectStore(&v1.Node{}, lw, selector, nil)
	if err != nil {
		return nil, err
	}
	return &NodeStore{ObjectStore: objectStore}, nil
}

// List returns list of nodes that satisfy conditions provided to NewNodeStore.
func (s *NodeStore) List() []*v1.Node {
	objects := s.Store.List()
	nodes := make([]*v1.Node, 0, len(objects))
	for _, o := range objects {
		nodes = append(nodes, o.(*v1.Node))
	}
	return nodes
}
