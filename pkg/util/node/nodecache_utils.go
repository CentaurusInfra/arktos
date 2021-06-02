/*
Copyright 2020 Authors of Arktos.

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

package node

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"sync"
	"time"
)

func GetNodeFromNodelisters(nodeListers map[string]corelisters.NodeLister, nodeName string) (*v1.Node, string, error) {
	for rpId, nodeLister := range nodeListers {
		node, err := nodeLister.Get(nodeName)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			klog.Errorf("Encountered error at GetNodeFromNodelisters, rpId %s. error %v", rpId, err)
			return nil, "", err
		}
		return node, rpId, nil
	}

	return nil, "", errors.NewNotFound(v1.Resource("node"), nodeName)
}

func GetNodeListersAndSyncedFromNodeInformers(nodeinformers map[string]coreinformers.NodeInformer) (nodeListers map[string]corelisters.NodeLister, nodeListersSynced map[string]cache.InformerSynced) {
	nodeListers = make(map[string]corelisters.NodeLister)
	nodeListersSynced = make(map[string]cache.InformerSynced)
	for rpId, nodeinformer := range nodeinformers {
		nodeListers[rpId] = nodeinformer.Lister()
		nodeListersSynced[rpId] = nodeinformer.Informer().HasSynced
	}

	return
}

func ListNodes(nodeListers map[string]corelisters.NodeLister, selector labels.Selector) (ret []*v1.Node, err error) {
	allNodes := make([]*v1.Node, 0)
	for _, nodeLister := range nodeListers {
		nodes, err := nodeLister.List(selector)
		if err != nil {
			//TODO - check error, allow skipping certain error such as client not initialized
			return nil, err
		}
		allNodes = append(allNodes, nodes...)
	}

	return allNodes, nil
}

// TODO - add timeout and return false
// TODO - consider unify implementation of WaitForNodeCacheSync with WaitForCacheSync
func WaitForNodeCacheSync(controllerName string, nodeListersSynced map[string]cache.InformerSynced) bool {
	klog.Infof("Waiting for caches to sync for %s controller", controllerName)

	var wg sync.WaitGroup
	wg.Add(len(nodeListersSynced))
	for key, nodeSynced := range nodeListersSynced {
		go func(rpId string, cacheSync cache.InformerSynced) {
			for {
				if cacheSync() {
					klog.Infof("Cache are synced for resource provider %s", rpId)
					wg.Done()
					break
				}
				klog.V(3).Infof("Wait for node sync from resource provider %s", rpId)
				time.Sleep(5 * time.Second)
			}
		}(key, nodeSynced)
	}
	wg.Wait()

	klog.Infof("Caches are synced for %s controller", controllerName)
	return true
}
