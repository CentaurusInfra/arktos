/*
Copyright 2016 The Kubernetes Authors.
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

package predicates

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/klog"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/scheduler/algorithm"
	priorityutil "k8s.io/kubernetes/pkg/scheduler/algorithm/priorities/util"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
	schedutil "k8s.io/kubernetes/pkg/scheduler/util"
)

// PredicateMetadata interface represents anything that can access a predicate metadata.
type PredicateMetadata interface {
	ShallowCopy() PredicateMetadata
	AddPod(addedPod *v1.Pod, nodeInfo *schedulernodeinfo.NodeInfo) error
	RemovePod(deletedPod *v1.Pod, node *v1.Node) error
}

// PredicateMetadataProducer is a function that computes predicate metadata for a given pod.
type PredicateMetadataProducer func(pod *v1.Pod, nodeNameToInfo map[string]*schedulernodeinfo.NodeInfo) PredicateMetadata

// PredicateMetadataFactory defines a factory of predicate metadata.
type PredicateMetadataFactory struct {
	podLister algorithm.PodLister
}

// AntiAffinityTerm's topology key value used in predicate metadata
type topologyPair struct {
	key   string
	value string
}

// TODO(Huang-Wei): It might be possible to use "make(map[topologyPair]*int64)" so that ￼
// we can do atomic additions instead of using a global mutext, however we need to consider
type topologyToMatchedTermCount map[topologyPair]int64


// NOTE: When new fields are added/removed or logic is changed, please make sure that
// RemovePod, AddPod, and ShallowCopy functions are updated to work with the new changes.
type predicateMetadata struct {
	pod           *v1.Pod
	podBestEffort bool
	podRequest    *schedulernodeinfo.Resource
	podPorts      []*v1.ContainerPort

	podAffinityMetadata *podAffinityMetadata

	serviceAffinityInUse                   bool
	serviceAffinityMatchingPodList         []*v1.Pod
	serviceAffinityMatchingPodServices     []*v1.Service
	// ignoredExtendedResources is a set of extended resource names that will
	// be ignored in the PodFitsResources predicate.
	//
	// They can be scheduler extender managed resources, the consumption of
	// which should be accounted only by the extenders. This set is synthesized
	// from scheduler extender configuration and does not change per pod.
	ignoredExtendedResources sets.String
}

type podAffinityMetadata struct {
	// A map of topology pairs to the number of existing pods that has
	// anti-affinity terms that match the "pod".
	topologyToMatchedExistingAntiAffinityTerms topologyToMatchedTermCount
	// A map of topology pairs to the number of existing pods that match
	// the affinity terms of the "pod".
	topologyToMatchedAffinityTerms topologyToMatchedTermCount
	// A map of topology pairs to the number of existing pods that match
	// the anti-affinity terms of the "pod".
	topologyToMatchedAntiAffinityTerms topologyToMatchedTermCount
}

// updateWithAffinityTerms updates the topologyToMatchedTermCount map with the specified value
// for each affinity term if "targetPod" matches ALL terms.
func (m topologyToMatchedTermCount) updateWithAffinityTerms(targetPod *v1.Pod, targetPodNode *v1.Node, affinityTerms []*affinityTermProperties, value int64) {
	if podMatchesAllAffinityTermProperties(targetPod, affinityTerms) {
		for _, t := range affinityTerms {
			if topologyValue, ok := targetPodNode.Labels[t.topologyKey]; ok {
				pair := topologyPair{key: t.topologyKey, value: topologyValue}
				m[pair] += value
				// value could be a negative value, hence we delete the entry if
				// the entry is down to zero.
				if m[pair] == 0 {
					delete(m, pair)
				}
			}
		}
	}
}

// updateAntiAffinityTerms updates the topologyToMatchedTermCount map with the specified value
// for each anti-affinity term matched the target pod.
func (m topologyToMatchedTermCount) updateWithAntiAffinityTerms(targetPod *v1.Pod, targetPodNode *v1.Node, antiAffinityTerms []*affinityTermProperties, value int64) {
	// Check anti-affinity properties.
	for _, a := range antiAffinityTerms {
		if priorityutil.PodMatchesTermsNamespaceAndSelector(targetPod, a.namespaces, a.selector) {
			if topologyValue, ok := targetPodNode.Labels[a.topologyKey]; ok {
				pair := topologyPair{key: a.topologyKey, value: topologyValue}
				m[pair] += value
				// value could be a negative value, hence we delete the entry if
				// the entry is down to zero.
				if m[pair] == 0 {
					delete(m, pair)
				}
			}
		}
	}
}

func (m *podAffinityMetadata) updatePod(updatedPod, pod *v1.Pod, node *v1.Node, multiplier int64) error {
	if m == nil {
		return nil
	}

	// Update matching existing anti-affinity terms.
	updatedPodAffinity := updatedPod.Spec.Affinity

	if updatedPodAffinity != nil && updatedPodAffinity.PodAntiAffinity != nil {
		antiAffinityProperties, err := getAffinityTermProperties(pod, GetPodAntiAffinityTerms(updatedPodAffinity.PodAntiAffinity))
		if err != nil {
			klog.Errorf("error in getting anti-affinity properties of Pod %v", updatedPod.Name)
			return err
		}
		m.topologyToMatchedExistingAntiAffinityTerms.updateWithAntiAffinityTerms(pod, node, antiAffinityProperties, multiplier)
	}

	// Update matching incoming pod (anti)affinity terms.
	affinity := pod.Spec.Affinity
	podNodeName := updatedPod.Spec.NodeName
	if affinity != nil && len(podNodeName) > 0 {
		if affinity.PodAffinity == nil {
			affinityProperties, err := getAffinityTermProperties(pod, GetPodAffinityTerms(affinity.PodAffinity))
			if err != nil {
				klog.Errorf("error in getting affinity properties of Pod %v", pod.Name)
				return err
			}
			m.topologyToMatchedAffinityTerms.updateWithAffinityTerms(updatedPod, node, affinityProperties, multiplier)
		}
		if affinity.PodAntiAffinity != nil {
			antiAffinityProperties, err := getAffinityTermProperties(pod, GetPodAntiAffinityTerms(affinity.PodAntiAffinity))
			if err != nil {
				klog.Errorf("error in getting anti-affinity properties of Pod %v", pod.Name)
				return err
			}
			m.topologyToMatchedAntiAffinityTerms.updateWithAntiAffinityTerms(updatedPod, node, antiAffinityProperties, multiplier)
		}
	}
	return nil
}

func (m *podAffinityMetadata) clone() *podAffinityMetadata {
	if m == nil {
		return nil
	}

	copy := podAffinityMetadata{}
	copy.topologyToMatchedAffinityTerms = m.topologyToMatchedAffinityTerms.clone()
	copy.topologyToMatchedAntiAffinityTerms = m.topologyToMatchedAntiAffinityTerms.clone()
	copy.topologyToMatchedExistingAntiAffinityTerms = m.topologyToMatchedExistingAntiAffinityTerms.clone()

	return &copy
}

func (m topologyToMatchedTermCount) appendMaps(toAppend topologyToMatchedTermCount) {
	for pair := range toAppend {
		m[pair] += toAppend[pair]
	}
}

func (m topologyToMatchedTermCount) clone() topologyToMatchedTermCount {
	copy := make(topologyToMatchedTermCount, len(m))
	copy.appendMaps(m)
	return copy
}

// Ensure that predicateMetadata implements algorithm.PredicateMetadata.
var _ PredicateMetadata = &predicateMetadata{}

// predicateMetadataProducer function produces predicate metadata. It is stored in a global variable below
// and used to modify the return values of PredicateMetadataProducer
type predicateMetadataProducer func(pm *predicateMetadata)

var predicateMetadataProducers = make(map[string]predicateMetadataProducer)

// RegisterPredicateMetadataProducer registers a PredicateMetadataProducer.
func RegisterPredicateMetadataProducer(predicateName string, precomp predicateMetadataProducer) {
	predicateMetadataProducers[predicateName] = precomp
}

// EmptyPredicateMetadataProducer returns a no-op MetadataProducer type.
func EmptyPredicateMetadataProducer(pod *v1.Pod, nodeNameToInfo map[string]*schedulernodeinfo.NodeInfo) PredicateMetadata {
	return nil
}

// RegisterPredicateMetadataProducerWithExtendedResourceOptions registers a
// PredicateMetadataProducer that creates predicate metadata with the provided
// options for extended resources.
//
// See the comments in "predicateMetadata" for the explanation of the options.
func RegisterPredicateMetadataProducerWithExtendedResourceOptions(ignoredExtendedResources sets.String) {
	RegisterPredicateMetadataProducer("PredicateWithExtendedResourceOptions", func(pm *predicateMetadata) {
		pm.ignoredExtendedResources = ignoredExtendedResources
	})
}

// NewPredicateMetadataFactory creates a PredicateMetadataFactory.
func NewPredicateMetadataFactory(podLister algorithm.PodLister) PredicateMetadataProducer {
	factory := &PredicateMetadataFactory{
		podLister,
	}
	return factory.GetMetadata
}

// GetMetadata returns the predicateMetadata used which will be used by various predicates.
func (pfactory *PredicateMetadataFactory) GetMetadata(pod *v1.Pod, nodeNameToInfo map[string]*schedulernodeinfo.NodeInfo) PredicateMetadata {
	// If we cannot compute metadata, just return nil
	if pod == nil {
		return nil
	}
	// existingPodAntiAffinityMap will be used later for efficient check on existing pods' anti-affinity
	existingPodAntiAffinityMap, err := getTPMapMatchingExistingAntiAffinity(pod, nodeNameToInfo)
	if err != nil {
		return nil
	}
	// incomingPodAffinityMap will be used later for efficient check on incoming pod's affinity
	// incomingPodAntiAffinityMap will be used later for efficient check on incoming pod's anti-affinity
	incomingPodAffinityMap, incomingPodAntiAffinityMap, err := getTPMapMatchingIncomingAffinityAntiAffinity(pod, nodeNameToInfo)
	if err != nil {
		klog.Errorf("[predicate meta data generation] error finding pods that match affinity terms: %v", err)
		return nil
	}
	predicateMetadata := &predicateMetadata{
		pod:                                    pod,
		podBestEffort:                          isPodBestEffort(pod),
		podRequest:                             GetResourceRequest(pod),
		podPorts:                               schedutil.GetContainerPorts(pod),
		podAffinityMetadata:     				&podAffinityMetadata{
			topologyToMatchedExistingAntiAffinityTerms: existingPodAntiAffinityMap,
			topologyToMatchedAntiAffinityTerms: incomingPodAntiAffinityMap,
			topologyToMatchedAffinityTerms: incomingPodAffinityMap,
		},
	}
	for predicateName, precomputeFunc := range predicateMetadataProducers {
		klog.V(10).Infof("Precompute: %v", predicateName)
		precomputeFunc(predicateMetadata)
	}
	return predicateMetadata
}

// RemovePod changes predicateMetadata assuming that the given `deletedPod` is
// deleted from the system.
func (meta *predicateMetadata) RemovePod(deletedPod *v1.Pod, node *v1.Node) error {
	deletedPodFullName := schedutil.GetPodFullName(deletedPod)
	if deletedPodFullName == schedutil.GetPodFullName(meta.pod) {
		return fmt.Errorf("deletedPod and meta.pod must not be the same")
	}

	if err := meta.podAffinityMetadata.updatePod(deletedPod, meta.pod, node, -1); err != nil {
		return err
	}

	// All pods in the serviceAffinityMatchingPodList are in the same namespace.
	// So, if the namespace of the first one is not the same as the namespace of the
	// deletedPod, we don't need to check the list, as deletedPod isn't in the list.
	if meta.serviceAffinityInUse &&
		len(meta.serviceAffinityMatchingPodList) > 0 &&
		deletedPod.Namespace == meta.serviceAffinityMatchingPodList[0].Namespace {
		for i, pod := range meta.serviceAffinityMatchingPodList {
			if schedutil.GetPodFullName(pod) == deletedPodFullName {
				meta.serviceAffinityMatchingPodList = append(
					meta.serviceAffinityMatchingPodList[:i],
					meta.serviceAffinityMatchingPodList[i+1:]...)
				break
			}
		}
	}
	return nil
}

// AddPod changes predicateMetadata assuming that `newPod` is added to the
// system.
func (meta *predicateMetadata) AddPod(addedPod *v1.Pod, nodeInfo *schedulernodeinfo.NodeInfo) error {
	addedPodFullName := schedutil.GetPodFullName(addedPod)
	if addedPodFullName == schedutil.GetPodFullName(meta.pod) {
		return fmt.Errorf("addedPod and meta.pod must not be the same")
	}
	if nodeInfo.Node() == nil {
		return fmt.Errorf("invalid node in nodeInfo")
	}
	if err := meta.podAffinityMetadata.updatePod(addedPod, meta.pod, nodeInfo.Node(), 1); err != nil {
		return err
	}
	// If addedPod is in the same namespace as the meta.pod, update the list
	// of matching pods if applicable.
	if meta.serviceAffinityInUse && addedPod.Namespace == meta.pod.Namespace {
		selector := CreateSelectorFromLabels(meta.pod.Labels)
		if selector.Matches(labels.Set(addedPod.Labels)) {
			meta.serviceAffinityMatchingPodList = append(meta.serviceAffinityMatchingPodList,
				addedPod)
		}
	}
	return nil
}

// ShallowCopy copies a metadata struct into a new struct and creates a copy of
// its maps and slices, but it does not copy the contents of pointer values.
func (meta *predicateMetadata) ShallowCopy() PredicateMetadata {
	newPredMeta := &predicateMetadata{
		pod:                      meta.pod,
		podBestEffort:            meta.podBestEffort,
		podRequest:               meta.podRequest,
		serviceAffinityInUse:     meta.serviceAffinityInUse,
		ignoredExtendedResources: meta.ignoredExtendedResources,
	}
	newPredMeta.podPorts = append([]*v1.ContainerPort(nil), meta.podPorts...)
	newPredMeta.podAffinityMetadata = meta.podAffinityMetadata.clone()
	newPredMeta.serviceAffinityMatchingPodServices = append([]*v1.Service(nil),
		meta.serviceAffinityMatchingPodServices...)
	newPredMeta.serviceAffinityMatchingPodList = append([]*v1.Pod(nil),
		meta.serviceAffinityMatchingPodList...)
	return (PredicateMetadata)(newPredMeta)
}

type affinityTermProperties struct {
	namespaces sets.String
	selector   labels.Selector
	topologyKey string
}

// getAffinityTermProperties receives a Pod and affinity terms and returns the namespaces and
// selectors of the terms.
func getAffinityTermProperties(pod *v1.Pod, terms []v1.PodAffinityTerm) (properties []*affinityTermProperties, err error) {
	if terms == nil {
		return properties, nil
	}

	for _, term := range terms {
		namespaces := priorityutil.GetNamespacesFromPodAffinityTerm(pod, &term)
		selector, err := metav1.LabelSelectorAsSelector(term.LabelSelector)
		if err != nil {
			return nil, err
		}
		properties = append(properties, &affinityTermProperties{namespaces: namespaces, selector: selector})
	}
	return properties, nil
}

// podMatchesAllAffinityTermProperties returns true IFF the given pod matches all the given properties.
func podMatchesAllAffinityTermProperties(pod *v1.Pod, properties []*affinityTermProperties) bool {
	if len(properties) == 0 {
		return false
	}
	for _, property := range properties {
		if !priorityutil.PodMatchesTermsNamespaceAndSelector(pod, property.namespaces, property.selector) {
			return false
		}
	}
	return true
}

// getTPMapMatchingExistingAntiAffinity calculates the following for each existing pod on each node:
// (1) Whether it has PodAntiAffinity
// (2) Whether any AffinityTerm matches the incoming pod
func getTPMapMatchingExistingAntiAffinity(pod *v1.Pod, nodeInfoMap map[string]*schedulernodeinfo.NodeInfo) (topologyToMatchedTermCount, error) {
	allNodeNames := make([]string, 0, len(nodeInfoMap))
	for name := range nodeInfoMap {
		allNodeNames = append(allNodeNames, name)
	}

	var lock sync.Mutex
	var firstError error

	topologyMap := make(topologyToMatchedTermCount)

	appendResult := func(toAppend topologyToMatchedTermCount) {
		lock.Lock()
		defer lock.Unlock()
		topologyMap.appendMaps(toAppend)
	}
	catchError := func(err error) {
		lock.Lock()
		defer lock.Unlock()
		if firstError == nil {
			firstError = err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	processNode := func(i int) {
		nodeInfo := nodeInfoMap[allNodeNames[i]]
		node:= nodeInfo.Node()
		if node == nil {
			catchError(fmt.Errorf("node not found"))
			return
		}
		existingPods := nodeInfo.PodsWithAffinity()
		klog.V(6).Infof("Node %v, Pods with Affinity: %v", nodeInfo.Node().Name, len(existingPods))
		for _, existingPod := range existingPods {
			existingPodTopologyMaps, err := getMatchingAntiAffinityTopologyPairsOfPod(pod, existingPod, node)
			if err != nil {
				catchError(err)
				cancel()
				return
			}
			appendResult(existingPodTopologyMaps)
		}
	}

	klog.V(6).Infof("Number of nodes: %v", len(allNodeNames))
	// TODO: make this configurable so we can test perf impact when increasing or decreasing concurrency
	//       on a 96 core machine, it can be much more than 16 concurrent threads to run the processNode function
	workqueue.ParallelizeUntil(ctx, 16, len(allNodeNames), processNode)
	return topologyMap, firstError
}

// getTPMapMatchingIncomingAffinityAntiAffinity finds existing Pods that match affinity terms of the given "pod".
// It returns a topologyPairsMaps that are checked later by the affinity
// predicate. With this topologyPairsMaps available, the affinity predicate does not
// need to check all the pods in the cluster.
func getTPMapMatchingIncomingAffinityAntiAffinity(pod *v1.Pod, nodeInfoMap map[string]*schedulernodeinfo.NodeInfo) (topologyPairsAffinityPodsMaps, topologyPairsAntiAffinityPodsMaps topologyToMatchedTermCount, err error) {
	affinity := pod.Spec.Affinity
	if affinity == nil || (affinity.PodAffinity == nil && affinity.PodAntiAffinity == nil) {
		return nil, nil, nil
	}

	allNodeNames := make([]string, 0, len(nodeInfoMap))
	for name := range nodeInfoMap {
		allNodeNames = append(allNodeNames, name)
	}

	var lock sync.Mutex
	var firstError error
	topologyPairsAffinityPodsMaps = make(topologyToMatchedTermCount)
	topologyPairsAntiAffinityPodsMaps = make(topologyToMatchedTermCount)
	appendResult := func(nodeName string, nodeTopologyPairsAffinityPodsMaps, nodeTopologyPairsAntiAffinityPodsMaps topologyToMatchedTermCount) {
		lock.Lock()
		defer lock.Unlock()
		if len(nodeTopologyPairsAffinityPodsMaps) > 0 {
			topologyPairsAffinityPodsMaps.appendMaps(nodeTopologyPairsAffinityPodsMaps)
		}
		if len(nodeTopologyPairsAntiAffinityPodsMaps) > 0 {
			topologyPairsAntiAffinityPodsMaps.appendMaps(nodeTopologyPairsAntiAffinityPodsMaps)
		}
	}

	catchError := func(err error) {
		lock.Lock()
		defer lock.Unlock()
		if firstError == nil {
			firstError = err
		}
	}

	affinityTerms := GetPodAffinityTerms(affinity.PodAffinity)
	affinityProperties, err := getAffinityTermProperties(pod, affinityTerms)
	if err != nil {
		return nil, nil, err
	}
	antiAffinityTerms := GetPodAntiAffinityTerms(affinity.PodAntiAffinity)
	antiAffinityProperties, err := getAffinityTermProperties(pod, antiAffinityTerms)
	if err != nil {
		return nil, nil, err
	}

	processNode := func(i int) {
		nodeInfo := nodeInfoMap[allNodeNames[i]]
		node := nodeInfo.Node()
		if node == nil {
			catchError(fmt.Errorf("nodeInfo.Node is nil"))
			return
		}
		nodeTopologyPairsAffinityPodsMaps := make(topologyToMatchedTermCount)
		nodeTopologyPairsAntiAffinityPodsMaps := make(topologyToMatchedTermCount)
		for _, existingPod := range nodeInfo.Pods() {
			// Check affinity properties.
			if podMatchesAllAffinityTermProperties(existingPod, affinityProperties) {
				for _, term := range affinityTerms {
					if topologyValue, ok := node.Labels[term.TopologyKey]; ok {
						pair := topologyPair{key: term.TopologyKey, value: topologyValue}
						nodeTopologyPairsAffinityPodsMaps[pair]++
					}
				}
			}
			// Check anti-affinity properties.
			for i, term := range antiAffinityTerms {
				p := antiAffinityProperties[i]
				if priorityutil.PodMatchesTermsNamespaceAndSelector(existingPod, p.namespaces, p.selector) {
					if topologyValue, ok := node.Labels[term.TopologyKey]; ok {
						pair := topologyPair{key: term.TopologyKey, value: topologyValue}
						nodeTopologyPairsAntiAffinityPodsMaps[pair]++
					}
				}
			}
		}
		if len(nodeTopologyPairsAffinityPodsMaps) > 0 || len(nodeTopologyPairsAntiAffinityPodsMaps) > 0 {
			appendResult(node.Name, nodeTopologyPairsAffinityPodsMaps, nodeTopologyPairsAntiAffinityPodsMaps)
		}
	}

	klog.V(6).Infof("DEBUG: should not see this log in our current perf runs. Number of nodes: %v", len(allNodeNames))
	// TODO: make this configurable so we can test perf impact when increasing or decreasing concurrency
	//       on a 96 core machine, it can be much more than 16 concurrent threads to run the processNode function
	workqueue.ParallelizeUntil(context.Background(), 16, len(allNodeNames), processNode)

	return topologyPairsAffinityPodsMaps, topologyPairsAntiAffinityPodsMaps, firstError
}

// targetPodMatchesAffinityOfPod returns true if "targetPod" matches ALL affinity terms of
// "pod". This function does not check topology.
// So, whether the targetPod actually matches or not needs further checks for a specific
// node.
func targetPodMatchesAffinityOfPod(pod, targetPod *v1.Pod) bool {
	affinity := pod.Spec.Affinity
	if affinity == nil || affinity.PodAffinity == nil {
		return false
	}
	affinityProperties, err := getAffinityTermProperties(pod, GetPodAffinityTerms(affinity.PodAffinity))
	if err != nil {
		klog.Errorf("error in getting affinity properties of Pod %v", pod.Name)
		return false
	}
	return podMatchesAllAffinityTermProperties(targetPod, affinityProperties)
}
