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

package apiserver

import (
	"fmt"
	"github.com/pborman/uuid"
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	clientset "k8s.io/client-go/kubernetes"
	clientv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/integration/framework"
)

const (
	tenant1 = "tenant1"
	tenant2 = "tenant2"
	tenant3 = "tenant3"

	podDataParitionModel = "/%s/pods/, %s, %s\n"
	rsDataParitionModel  = "/%s/replicasets/, %s, %s\n"

	masterAddr1 = "192.168.10.6"
	masterAddr2 = "192.168.10.8"

	kubernetesServiceName = "kubernetes"
)

func setUpTwoApiservers(t *testing.T) (*httptest.Server, framework.CloseFunc, clientset.Interface, string, *httptest.Server, framework.CloseFunc, clientset.Interface, string) {
	prefix, configFilename1, configFilename2 := createTwoApiServersPartitionFiles(t)
	masterConfig1 := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename1, masterAddr1, "0")
	masterConfig2 := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename2, masterAddr2, "1")
	_, s1, closeFn1 := framework.RunAMaster(masterConfig1)

	// TODO - temporary change for current api server support - need to change after multiple api servers partition code is in place
	kubeConfig1 := restclient.KubeConfig{Host: s1.URL}
	clientSet1, err := clientset.NewForConfig(restclient.NewAggregatedConfig(&kubeConfig1))
	if err != nil {
		t.Fatalf("Error in create clientset: %v", err)
	}

	_, s2, closeFn2 := framework.RunAMaster(masterConfig2)

	kubeConfig2 := restclient.KubeConfig{Host: s2.URL}
	clientSet2, err := clientset.NewForConfig(restclient.NewAggregatedConfig(&kubeConfig2))
	if err != nil {
		t.Fatalf("Error in create clientset: %v", err)
	}

	return s1, closeFn1, clientSet1, configFilename1, s2, closeFn2, clientSet2, configFilename2
}

func TestSetupMultipleApiServers(t *testing.T) {
	s1, closeFn1, clientset1, configFilename1, s2, closeFn2, clientset2, configFilename2 := setUpTwoApiservers(t)
	defer deleteSinglePartitionConfigFile(t, configFilename1)
	defer deleteSinglePartitionConfigFile(t, configFilename2)
	defer closeFn1()
	defer closeFn2()
	t.Logf("server 1 %+v, clientset 2 %+v", s1, clientset1)
	t.Logf("server 2 %+v, clientset 2 %+v", s2, clientset2)

	assert.NotNil(t, s1)
	assert.NotNil(t, s2)
	assert.NotNil(t, clientset1)
	assert.NotNil(t, clientset2)
	assert.NotEqual(t, s1.URL, s2.URL)
}

func labelMap() map[string]string {
	return map[string]string{"foo": "bar"}
}

func generateRSData(tenant, namespace, rsName string, replicas int) *apps.ReplicaSet {
	replicasCopy := int32(replicas)
	return &apps.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Tenant:    tenant,
			Namespace: namespace,
			Name:      rsName,
		},
		Spec: apps.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelMap(),
			},
			Replicas: &replicasCopy,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelMap(),
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "fake-name",
							Image: "fakeimage",
						},
					},
				},
			},
		},
	}
}

func generatePod(tenant, namespace, podName string) *v1.Pod {
	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Tenant:    tenant,
			Name:      podName,
			Namespace: namespace,
			Labels:    labelMap(),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "fake-name",
					Image: "fakeimage",
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}
}

func createRS(t *testing.T, clientset clientset.Interface, tenant, namespace, rsName string, replicas int) *apps.ReplicaSet {
	rs := generateRSData(tenant, namespace, rsName, replicas)
	createdRS, err := clientset.AppsV1().ReplicaSetsWithMultiTenancy(rs.Namespace, rs.Tenant).Create(rs)
	if err != nil {
		t.Fatalf("Failed to create replica set %s/%s/%s: %v", rs.Tenant, rs.Namespace, rs.Name, err)
	}

	return createdRS
}

func getRS(t *testing.T, clientset clientset.Interface, tenant, namespace, rsName string) *apps.ReplicaSet {
	readRS, err := clientset.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant).Get(rsName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to read replicaset %s/%s/%s: %v", tenant, namespace, rsName, err)
	}

	return readRS
}

func deleteRS(t *testing.T, clientset clientset.Interface, rs *apps.ReplicaSet) {
	err := clientset.AppsV1().ReplicaSetsWithMultiTenancy(rs.Namespace, rs.Tenant).Delete(rs.Name, nil)
	if err != nil {
		t.Fatalf("Failed to delete replicaset %s/%s/%s: %v", rs.Tenant, rs.Namespace, rs.Name, err)
	}
}

func createPod(t *testing.T, clientset clientset.Interface, tenant, namespace, podName string) *v1.Pod {
	pod := generatePod(tenant, namespace, podName)
	createdPod, err := clientset.CoreV1().PodsWithMultiTenancy(namespace, tenant).Create(pod)
	if err != nil {
		t.Fatalf("Failed to create pod %s/%s/%s: %v", pod.Tenant, pod.Namespace, pod.Name, err)
	}

	return createdPod
}

func getPod(t *testing.T, clientset clientset.Interface, tenant, namespace, podName string) *v1.Pod {
	readPod, err := clientset.CoreV1().PodsWithMultiTenancy(namespace, tenant).Get(podName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to read pod %s/%s/%s: %v", tenant, namespace, podName, err)
	}

	return readPod
}

func deletePod(t *testing.T, clientset clientset.Interface, pod *v1.Pod) {
	err := clientset.CoreV1().PodsWithMultiTenancy(pod.Namespace, pod.Tenant).Delete(pod.Name, nil)
	if err != nil {
		t.Fatalf("Failed to delete pod %s/%s/%s: %v", pod.Tenant, pod.Namespace, pod.Name, err)
	}
}

func updatePod(t *testing.T, clientset clientset.Interface, pod *v1.Pod) (*v1.Pod, error) {
	return clientset.CoreV1().PodsWithMultiTenancy(pod.Namespace, pod.Tenant).Update(pod)
}

func checkPodEquality(t *testing.T, expectedPod, pod *v1.Pod) {
	assert.NotNil(t, expectedPod)
	assert.NotNil(t, pod)
	assert.Equal(t, expectedPod.UID, pod.UID)
	assert.Equal(t, expectedPod.Name, pod.Name)
	assert.Equal(t, expectedPod.Namespace, pod.Namespace)
	assert.Equal(t, expectedPod.Tenant, pod.Tenant)
	assert.Equal(t, expectedPod.HashKey, pod.HashKey)
}

func checkRSEquality(t *testing.T, expectedRS, rs *apps.ReplicaSet) {
	assert.NotNil(t, expectedRS)
	assert.NotNil(t, rs)
	assert.Equal(t, expectedRS.UID, rs.UID)
	assert.Equal(t, expectedRS.Name, rs.Name)
	assert.Equal(t, expectedRS.Namespace, rs.Namespace)
	assert.Equal(t, expectedRS.Tenant, rs.Tenant)
	assert.Equal(t, expectedRS.HashKey, rs.HashKey)
}

// Search entire list because previous data might not being cleared
func checkPodExistence(existingPods []v1.Pod, searchPods ...*v1.Pod) bool {
	for _, podToSearch := range searchPods {
		isFound := false
		for _, pod := range existingPods {
			if pod.HashKey == podToSearch.HashKey && pod.UID == podToSearch.UID &&
				pod.Name == podToSearch.Name && pod.Namespace == podToSearch.Namespace && pod.Tenant == podToSearch.Tenant {
				isFound = true
				break
			}
		}

		if !isFound {
			return false
		}
	}

	return true
}

func checkReplicaSetExistence(existingRSs []apps.ReplicaSet, searchRSs ...*apps.ReplicaSet) bool {
	for _, rsToSearch := range searchRSs {
		isFound := false
		for _, rs := range existingRSs {
			if rs.HashKey == rsToSearch.HashKey && rs.UID == rsToSearch.UID &&
				rs.Tenant == rsToSearch.Tenant && rs.Namespace == rsToSearch.Namespace && rs.Name == rsToSearch.Name {
				isFound = true
				break
			}
		}

		if !isFound {
			return false
		}
	}

	return true
}

func checkReplicaSetExistence2(existingRSs []interface{}, searchRSs ...*apps.ReplicaSet) bool {
	for _, rsToSearch := range searchRSs {
		isFound := false
		for _, rsInterface := range existingRSs {
			switch t := rsInterface.(type) {
			case *apps.ReplicaSet:
				rs, _ := rsInterface.(*apps.ReplicaSet)
				if rs.HashKey == rsToSearch.HashKey && rs.UID == rsToSearch.UID &&
					rs.Tenant == rsToSearch.Tenant && rs.Namespace == rsToSearch.Namespace && rs.Name == rsToSearch.Name {
					isFound = true
					break
				}
			default:
				klog.Infof("Unknown type %v, rsInterface [%+v]", t, rsInterface)
			}

		}

		if !isFound {
			klog.Infof("Not found!!! Search rs [%+v] in list [%+v]", rsToSearch, existingRSs)
			return false
		}
	}

	return true
}

func checkReplicaSetExistence3(existingRSs []*apps.ReplicaSet, searchRSs ...*apps.ReplicaSet) bool {
	for _, rsToSearch := range searchRSs {
		isFound := false
		for _, rs := range existingRSs {
			if rs.HashKey == rsToSearch.HashKey && rs.UID == rsToSearch.UID &&
				rs.Tenant == rsToSearch.Tenant && rs.Namespace == rsToSearch.Namespace && rs.Name == rsToSearch.Name {
				isFound = true
				break
			}
		}

		if !isFound {
			return false
		}
	}

	return true
}

func startEventBroadCaster(t *testing.T, cs clientset.Interface) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&clientv1core.EventSinkImpl{
		Interface: cs.CoreV1().Events(""),
	})
}

// Ideally, we should test all kinds - TODO - should be able to leverage generated test data
func TestGetCanGetAlldata(t *testing.T) {
	s1, closeFn1, clientset1, configFilename1, s2, closeFn2, clientset2, configFilename2 := setUpTwoApiservers(t)
	defer deleteSinglePartitionConfigFile(t, configFilename1)
	defer deleteSinglePartitionConfigFile(t, configFilename2)
	defer closeFn1()
	defer closeFn2()

	// create pods via 2 different api servers
	pod1 := createPod(t, clientset1, tenant1, "te", "pod1")
	defer framework.DeleteTestingTenant(tenant1, s1, t)
	pod2 := createPod(t, clientset2, tenant2, "te", "pod1")
	defer framework.DeleteTestingTenant(tenant2, s2, t)
	assert.NotNil(t, pod1)
	assert.NotNil(t, pod2)
	assert.NotEqual(t, pod1.UID, pod2.UID)

	// verify get pod with same api server
	readPod1, err := clientset1.CoreV1().PodsWithMultiTenancy(pod1.Namespace, pod1.Tenant).Get(pod1.Name, metav1.GetOptions{})
	assert.Nil(t, err, "Failed to get pod 1 from same clientset")
	assert.NotNil(t, readPod1)
	readPod2, err := clientset2.CoreV1().PodsWithMultiTenancy(pod2.Namespace, pod2.Tenant).Get(pod2.Name, metav1.GetOptions{})
	assert.Nil(t, err, "Failed to get pod 2 from same clientset")
	assert.NotNil(t, readPod2)

	// verify get pod through different api server
	readPod1, err = clientset2.CoreV1().PodsWithMultiTenancy(pod1.Namespace, pod1.Tenant).Get(pod1.Name, metav1.GetOptions{})
	assert.Nil(t, err, "Failed to get pod 1 from different clientset")
	if err == nil {
		checkPodEquality(t, pod1, readPod1)
	}
	readPod2, err = clientset1.CoreV1().PodsWithMultiTenancy(pod2.Namespace, pod2.Tenant).Get(pod2.Name, metav1.GetOptions{})
	assert.Nil(t, err, "Failed to get pod 2 from different clientset")
	if err == nil {
		checkPodEquality(t, pod2, readPod2)
	}

	// create replicaset via 2 different api servers
	rs1 := createRS(t, clientset1, tenant1, "rs1", "default", 1)
	rs2 := createRS(t, clientset2, tenant2, "rs2", "default", 1)
	assert.NotNil(t, rs1)
	assert.NotNil(t, rs2)
	assert.NotEqual(t, rs1.UID, rs2.UID)

	// verify get rs through different api server
	readRs1, err := clientset2.AppsV1().ReplicaSetsWithMultiTenancy(rs1.Namespace, rs1.Tenant).Get(rs1.Name, metav1.GetOptions{})
	assert.Nil(t, err, "Failed to get rs 1 from different clientset")
	if err == nil {
		checkRSEquality(t, rs1, readRs1)
	}
	readRs2, err := clientset1.AppsV1().ReplicaSetsWithMultiTenancy(rs2.Namespace, rs2.Tenant).Get(rs2.Name, metav1.GetOptions{})
	assert.Nil(t, err, "Failed to get rs 2 from different clientset")
	if err == nil {
		checkRSEquality(t, rs2, readRs2)
	}

	// tear down
	deletePod(t, clientset1, pod1)
	deletePod(t, clientset1, pod2)
	deleteRS(t, clientset2, rs1)
	deleteRS(t, clientset2, rs2)
}

func TestListCanGetAlldata(t *testing.T) {
	s1, closeFn1, clientset1, configFilename1, _, closeFn2, clientset2, configFilename2 := setUpTwoApiservers(t)
	defer deleteSinglePartitionConfigFile(t, configFilename1)
	defer deleteSinglePartitionConfigFile(t, configFilename2)
	defer closeFn1()
	defer closeFn2()

	// create 2 pods in same tenant and namespace via different api server
	namespace := "te"
	pod1 := createPod(t, clientset1, tenant1, namespace, "pod1")
	defer framework.DeleteTestingTenant(tenant1, s1, t)
	pod2 := createPod(t, clientset2, tenant1, namespace, "pod2")
	assert.NotNil(t, pod1)
	assert.NotNil(t, pod2)
	assert.NotEqual(t, pod1.UID, pod2.UID)

	// verify list pod through different api server
	podlist, err := clientset1.CoreV1().PodsWithMultiTenancy(namespace, tenant1).List(metav1.ListOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, podlist)
	assert.True(t, len(podlist.Items) >= 2)
	assert.True(t, checkPodExistence(podlist.Items, pod1, pod2))

	podlist2, err := clientset2.CoreV1().PodsWithMultiTenancy(namespace, tenant1).List(metav1.ListOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, podlist2)
	assert.True(t, len(podlist2.Items) >= 2)
	assert.True(t, checkPodExistence(podlist.Items, pod1, pod2))

	// create 2 replicaset in same tenant and namespace via different api server
	rs1 := createRS(t, clientset1, tenant1, namespace, "rs1", 1)
	rs2 := createRS(t, clientset2, tenant1, namespace, "rs2", 1)
	assert.NotNil(t, rs1)
	assert.NotNil(t, rs2)
	assert.NotEqual(t, rs1.UID, rs2.UID)

	// verify replicasets can be get from different api server
	readRs1, err := clientset2.AppsV1().ReplicaSetsWithMultiTenancy(rs1.Namespace, rs1.Tenant).Get(rs1.Name, metav1.GetOptions{})
	assert.Nil(t, err, "Failed to get rs 1 from different clientset")
	if err == nil {
		checkRSEquality(t, rs1, readRs1)
	}

	readRs2, err := clientset1.AppsV1().ReplicaSetsWithMultiTenancy(rs2.Namespace, rs2.Tenant).Get(rs2.Name, metav1.GetOptions{})
	assert.Nil(t, err, "Failed to get rs 2 from different clientset")
	if err == nil {
		checkRSEquality(t, rs2, readRs2)
	}

	// verify list rs through different api server
	rslist, err := clientset1.AppsV1().ReplicaSetsWithMultiTenancy(rs1.Namespace, rs1.Tenant).List(metav1.ListOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, rslist, "replicaset list should not be nil")
	assert.True(t, len(rslist.Items) >= 2)
	assert.True(t, checkReplicaSetExistence(rslist.Items, rs1, rs2))

	rslist2, err := clientset2.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant1).List(metav1.ListOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, rslist2)
	assert.True(t, len(rslist2.Items) >= 2)
	assert.True(t, checkReplicaSetExistence(rslist2.Items, rs1, rs2))

	// tear down
	deletePod(t, clientset1, pod1)
	deletePod(t, clientset1, pod2)
	deleteRS(t, clientset2, rs1)
	deleteRS(t, clientset2, rs2)
}

func TestPostCanUpdateAlldata(t *testing.T) {
	s1, closeFn1, clientset1, configFilename1, s2, closeFn2, clientset2, configFilename2 := setUpTwoApiservers(t)
	defer deleteSinglePartitionConfigFile(t, configFilename1)
	defer deleteSinglePartitionConfigFile(t, configFilename2)
	defer closeFn1()
	defer closeFn2()

	// create pods via 2 different api servers
	pod1 := createPod(t, clientset1, tenant1, "te", "pod1")
	defer framework.DeleteTestingTenant(tenant1, s1, t)
	pod2 := createPod(t, clientset2, tenant2, "te", "pod1")
	defer framework.DeleteTestingTenant(tenant2, s2, t)
	assert.NotNil(t, pod1)
	assert.NotNil(t, pod2)
	assert.NotEqual(t, pod1.UID, pod2.UID)

	// verify update pod via different api server
	pod1ToUpdate := pod1.DeepCopy()
	key := "1"
	value := "2"
	if pod1ToUpdate.Annotations == nil {
		pod1ToUpdate.Annotations = make(map[string]string)
	}
	pod1ToUpdate.Annotations[key] = value

	// update via server 2
	newPod1, err := updatePod(t, clientset2, pod1ToUpdate)
	assert.Nil(t, err)
	assert.NotNil(t, newPod1)

	// read via server 1
	pod1Read := getPod(t, clientset1, pod1.Tenant, pod1.Namespace, pod1.Name)
	assert.NotNil(t, pod1Read)
	checkPodEquality(t, pod1, pod1Read)

	valueRead, isOK := pod1Read.Annotations[key]
	assert.True(t, isOK)
	assert.Equal(t, value, valueRead)

	// update via server 1
	pod1ToUpdate = pod1Read.DeepCopy()
	newValue := value + "2"
	pod1ToUpdate.Annotations[key] = newValue
	newPod1, err = updatePod(t, clientset1, pod1ToUpdate)
	assert.Nil(t, err)
	assert.NotNil(t, newPod1)

	// read via server 2
	pod1Read = getPod(t, clientset2, pod1.Tenant, pod1.Namespace, pod1.Name)
	assert.NotNil(t, pod1Read)
	checkPodEquality(t, pod1, pod1Read)

	valueRead2, isOK2 := pod1Read.Annotations[key]
	assert.True(t, isOK2)
	assert.Equal(t, newValue, valueRead2)

	// tear down
	deletePod(t, clientset1, pod1)
	deletePod(t, clientset1, pod2)
}

func TestWatchOnlyGetDataFromOneParition(t *testing.T) {
	_, closeFn1, clientset1, configFilename1, _, closeFn2, clientset2, configFilename2 := setUpTwoApiservers(t)
	defer deleteSinglePartitionConfigFile(t, configFilename1)
	defer deleteSinglePartitionConfigFile(t, configFilename2)
	defer closeFn1()
	defer closeFn2()

	// create informer 1 from server 1
	resyncPeriod := 12 * time.Hour
	informer1 := informers.NewSharedInformerFactory(clientset1, resyncPeriod)
	stopCh := make(chan struct{})
	informer1.Start(stopCh)
	defer close(stopCh)

	// create informer 2 from server 2
	informer2 := informers.NewSharedInformerFactory(clientset2, resyncPeriod)
	informer2.Start(stopCh)

	startEventBroadCaster(t, clientset1)
	startEventBroadCaster(t, clientset2)
	informer1.WaitForCacheSync(stopCh)
	informer2.WaitForCacheSync(stopCh)
	go informer1.Apps().V1().ReplicaSets().Informer().Run(stopCh)
	go informer2.Apps().V1().ReplicaSets().Informer().Run(stopCh)

	namespace := "ns1"
	rsClient1 := clientset1.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant1)
	w1, err := rsClient1.Watch(metav1.ListOptions{})
	defer w1.Stop()
	assert.Nil(t, err)

	rsClient2 := clientset2.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant2)
	w2, err := rsClient2.Watch(metav1.ListOptions{})
	defer w2.Stop()
	assert.Nil(t, err)

	// create rs via 2 different api servers
	rs1 := createRS(t, clientset1, tenant1, namespace, "rs1", 1)
	rs2 := createRS(t, clientset2, tenant2, namespace, "rs2", 1)
	assert.NotNil(t, rs1)
	assert.NotNil(t, rs2)
	assert.NotEqual(t, rs1.UID, rs2.UID)

	time.Sleep(10 * time.Second)

	// check data from different api servers
	rslist1 := informer1.Apps().V1().ReplicaSets().Informer().GetIndexer().List()
	assert.NotNil(t, rslist1)
	klog.Infof(" rs list 1 from informer 1 len %d, [%+v]", len(rslist1), rslist1)

	rslist2 := informer2.Apps().V1().ReplicaSets().Informer().GetIndexer().List()
	assert.NotNil(t, rslist2)
	klog.Infof(" rs list 2 from informer 2 len %d, [%+v]", len(rslist2), rslist2)

	// check rs1 in informer 1 list
	assert.True(t, checkReplicaSetExistence2(rslist1, rs1))
	// check rs2 not in informer 1 list
	assert.False(t, checkReplicaSetExistence2(rslist1, rs2))

	// check rs2 in informer 2 list
	assert.True(t, checkReplicaSetExistence2(rslist2, rs2))

	// check rs1 not in informer 2 list
	assert.False(t, checkReplicaSetExistence2(rslist2, rs1))

	// tear down
	deleteRS(t, clientset1, rs1)
	deleteRS(t, clientset1, rs2)
}

func TestAggregatedWatchInformerCanGetAllData(t *testing.T) {
	prefix, configFilename1, configFilename2 := createTwoApiServersPartitionFiles(t)
	masterConfig1 := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename1, masterAddr1, "0")
	masterConfig2 := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename2, masterAddr2, "1")
	_, s1, closeFn1 := framework.RunAMaster(masterConfig1)
	_, s2, closeFn2 := framework.RunAMaster(masterConfig2)
	kubeConfig1 := restclient.KubeConfig{Host: s1.URL}
	kubeConfig2 := restclient.KubeConfig{Host: s2.URL}
	aggConfig := restclient.NewAggregatedConfig(&kubeConfig1, &kubeConfig2)
	aggClientSet, err := clientset.NewForConfig(aggConfig)
	assert.Nil(t, err)

	defer deleteSinglePartitionConfigFile(t, configFilename1)
	defer deleteSinglePartitionConfigFile(t, configFilename2)
	defer closeFn1()
	defer closeFn2()

	// create informer 1 from aggregated clientset
	resyncPeriod := 12 * time.Hour
	informer1 := informers.NewSharedInformerFactory(aggClientSet, resyncPeriod)
	stopCh := make(chan struct{})
	informer1.Start(stopCh)
	defer close(stopCh)

	startEventBroadCaster(t, aggClientSet)
	informer1.WaitForCacheSync(stopCh)
	go informer1.Apps().V1().ReplicaSets().Informer().Run(stopCh)

	// create rs via 2 different api servers
	namespace := "ns1"
	rs1 := createRS(t, aggClientSet, tenant1, namespace, "rs1", 1)
	rs2 := createRS(t, aggClientSet, tenant2, namespace, "rs2", 1)
	klog.Infof("created replicaset 1 [%+v]", rs1)
	klog.Infof("created replicaset 2 [%+v]", rs2)
	assert.NotNil(t, rs1)
	assert.NotNil(t, rs2)
	assert.NotEqual(t, rs1.UID, rs2.UID)

	time.Sleep(10 * time.Second)

	// check data from different api servers
	rslist1 := informer1.Apps().V1().ReplicaSets().Informer().GetIndexer().List()
	assert.NotNil(t, rslist1)
	klog.Infof(" rs list 1 from informer 1 len %d, [%+v]", len(rslist1), rslist1)

	// check rs1 in informer 1 list
	assert.True(t, checkReplicaSetExistence2(rslist1, rs1))

	// check rs2 in informer 1 list
	assert.True(t, checkReplicaSetExistence2(rslist1, rs2))

	// tear down
	deleteRS(t, aggClientSet, rs1)
	deleteRS(t, aggClientSet, rs2)
}

// Test apiserver sync data like ["registry/replicasets/", "registry/replicasets/tenant2")
func TestPartitionWithLeftUnbounded(t *testing.T) {
	_, closeFn, clientset, configFilename := setUpSingleApiserver(t, "", tenant2, "0")
	defer deleteSinglePartitionConfigFile(t, configFilename)
	defer closeFn()

	// create informer 1 from server 1
	resyncPeriod := 12 * time.Hour
	informer := informers.NewSharedInformerFactory(clientset, resyncPeriod)
	stopCh := make(chan struct{})
	informer.Start(stopCh)
	defer close(stopCh)

	startEventBroadCaster(t, clientset)
	informer.WaitForCacheSync(stopCh)
	go informer.Apps().V1().ReplicaSets().Informer().Run(stopCh)

	namespace := "ns1"
	rsClient := clientset.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant1)
	w, err := rsClient.Watch(metav1.ListOptions{})
	defer w.Stop()
	assert.Nil(t, err)

	rs := createRS(t, clientset, tenant1, namespace, "rs1", 1)
	assert.NotNil(t, rs)
	deleteRS(t, clientset, rs)

	otherRs := createRS(t, clientset, tenant3, namespace, "rs1", 1)
	assert.NotNil(t, otherRs)
	assert.NotEqual(t, rs.UID, otherRs.UID)
	deleteRS(t, clientset, otherRs)

	pod := createPod(t, clientset, tenant1, namespace, "pod")
	assert.NotNil(t, pod)
	deletePod(t, clientset, pod)

	// check data from different api servers
	rsFound := false
	for {
		select {
		case event, ok := <-w.ResultChan():
			if !ok {
				t.Fatalf("Failed to get replicaset from watch api server")
			}
			if event.Type == watch.Error {
				t.Fatalf("Result channel get error event. %v", event)
			}
			meta, err := meta.Accessor(event.Object)
			if err != nil {
				t.Fatalf("Unable to understand watch event %#v", event)
			}

			if rs.UID == meta.GetUID() {
				rsFound = true
				assert.Equal(t, rs.UID, meta.GetUID())
				assert.Equal(t, rs.HashKey, meta.GetHashKey())
				assert.Equal(t, rs.Name, meta.GetName())
				assert.Equal(t, rs.Namespace, meta.GetNamespace())
				assert.Equal(t, rs.Tenant, meta.GetTenant())
			}
			if otherRs.UID == meta.GetUID() {
				t.Fatalf("The api server should not sync other replicaset data")
			}
			if pod.UID == meta.GetUID() {
				t.Fatalf("The api server should not sync other pods data")
			}

		case <-time.After(10 * time.Second):
			t.Fatalf("unable to get replicaset from watch api server")
		}

		if rsFound {
			break
		}
	}
	assert.True(t, rsFound)

}

// Test apiserver sync data like ["registry/replicasets/tenant2", "")
func TestPartitionRightUnbounded(t *testing.T) {
	_, closeFn, clientset, configFilename := setUpSingleApiserver(t, tenant2, "", "0")
	defer deleteSinglePartitionConfigFile(t, configFilename)
	defer closeFn()

	// create informer 1 from server 1
	resyncPeriod := 12 * time.Hour
	informer := informers.NewSharedInformerFactory(clientset, resyncPeriod)
	stopCh := make(chan struct{})
	informer.Start(stopCh)
	defer close(stopCh)

	startEventBroadCaster(t, clientset)
	informer.WaitForCacheSync(stopCh)
	go informer.Apps().V1().ReplicaSets().Informer().Run(stopCh)

	namespace := "ns1"
	rsClient := clientset.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant2)
	w, err := rsClient.Watch(metav1.ListOptions{})
	defer w.Stop()
	assert.Nil(t, err)

	rs := createRS(t, clientset, tenant2, namespace, "rs2", 1)
	assert.NotNil(t, rs)
	deleteRS(t, clientset, rs)

	otherRs := createRS(t, clientset, tenant1, namespace, "rs1", 1)
	assert.NotNil(t, otherRs)
	assert.NotEqual(t, rs.UID, otherRs.UID)
	deleteRS(t, clientset, otherRs)

	pod := createPod(t, clientset, tenant2, namespace, "pod")
	assert.NotNil(t, pod)
	deletePod(t, clientset, pod)

	// check data from different api servers
	rsFound := false
	for {
		select {
		case event, ok := <-w.ResultChan():
			if !ok {
				t.Fatalf("Failed to get replicaset from watch api server")
			}
			if event.Type == watch.Error {
				t.Fatalf("Result channel get error event. %v", event)
			}
			meta, err := meta.Accessor(event.Object)
			if err != nil {
				t.Fatalf("Unable to understand watch event %#v", event)
			}

			if rs.UID == meta.GetUID() {
				rsFound = true
				assert.Equal(t, rs.UID, meta.GetUID())
				assert.Equal(t, rs.HashKey, meta.GetHashKey())
				assert.Equal(t, rs.Name, meta.GetName())
				assert.Equal(t, rs.Namespace, meta.GetNamespace())
				assert.Equal(t, rs.Tenant, meta.GetTenant())
			}
			if otherRs.UID == meta.GetUID() {
				t.Fatalf("The api server should not sync other replicaset data")
			}
			if pod.UID == meta.GetUID() {
				t.Fatalf("The api server should not sync other pods data")
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("unable to get replicaset from watch api server")
		}

		if rsFound {
			break
		}
	}
	assert.True(t, rsFound)
}

// Test apiserver sync data like ["registry/pods/tenant2", "registry/pods/tenant3")
func TestPartitionLeftRightBounded(t *testing.T) {
	_, closeFn, clientset, configFilename := setUpSingleApiserver(t, tenant2, tenant3, "0")
	defer deleteSinglePartitionConfigFile(t, configFilename)
	defer closeFn()

	// create informer 1 from server 1
	resyncPeriod := 12 * time.Hour
	informer := informers.NewSharedInformerFactory(clientset, resyncPeriod)
	stopCh := make(chan struct{})
	informer.Start(stopCh)
	defer close(stopCh)

	startEventBroadCaster(t, clientset)
	informer.WaitForCacheSync(stopCh)
	go informer.Apps().V1().ReplicaSets().Informer().Run(stopCh)

	namespace := "ns1"
	rsClient := clientset.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant2)
	w, err := rsClient.Watch(metav1.ListOptions{})
	defer w.Stop()
	assert.Nil(t, err)

	rs := createRS(t, clientset, tenant2, namespace, "rs2", 1)
	assert.NotNil(t, rs)

	otherRs := createRS(t, clientset, tenant3, namespace, "rs3", 1)
	assert.NotNil(t, otherRs)
	assert.NotEqual(t, rs.UID, otherRs.UID)

	// check data from different api servers
	rsFound := false
	for {
		select {
		case event, ok := <-w.ResultChan():
			if !ok {
				t.Fatalf("Failed to get replicaset from watch api server")
			}
			if event.Type == watch.Error {
				t.Fatalf("Result channel get error event. %v", event)
			}
			meta, err := meta.Accessor(event.Object)
			if err != nil {
				t.Fatalf("Unable to understand watch event %#v", event)
			}

			if rs.UID == meta.GetUID() {
				rsFound = true
				assert.Equal(t, rs.UID, meta.GetUID())
				assert.Equal(t, rs.HashKey, meta.GetHashKey())
				assert.Equal(t, rs.Name, meta.GetName())
				assert.Equal(t, rs.Namespace, meta.GetNamespace())
				assert.Equal(t, rs.Tenant, meta.GetTenant())
			}
			if otherRs.UID == meta.GetUID() {
				t.Fatalf("The api server should not sync other replicaset data")
			}

		case <-time.After(10 * time.Second):
			t.Fatalf("unable to get replicaset from watch api server")
		}

		if rsFound {
			break
		}
	}
	assert.True(t, rsFound)

	// tear down
	deleteRS(t, clientset, rs)
}

// Test apiserver sync data like ["registry/replicasets/", "registry/replicasets/")
func TestPartitionUnBounded(t *testing.T) {
	_, closeFn, clientset, configFilename := setUpSingleApiserver(t, "", "", "0")
	defer deleteSinglePartitionConfigFile(t, configFilename)
	defer closeFn()

	// create informer 1 from server 1
	resyncPeriod := 12 * time.Hour
	informer := informers.NewSharedInformerFactory(clientset, resyncPeriod)
	stopCh := make(chan struct{})
	informer.Start(stopCh)
	defer close(stopCh)

	startEventBroadCaster(t, clientset)
	informer.WaitForCacheSync(stopCh)
	go informer.Apps().V1().ReplicaSets().Informer().Run(stopCh)

	namespace := "ns1"
	rsClient := clientset.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant2)
	w, err := rsClient.Watch(metav1.ListOptions{})
	defer w.Stop()
	assert.Nil(t, err)

	rs := createRS(t, clientset, tenant2, namespace, "rs2", 1)
	assert.NotNil(t, rs)
	deleteRS(t, clientset, rs)

	pod := createPod(t, clientset, tenant2, namespace, "pod")
	assert.NotNil(t, pod)
	deletePod(t, clientset, pod)

	// check data from different api servers
	rsFound := false
	for {
		select {
		case event, ok := <-w.ResultChan():
			if !ok {
				t.Fatalf("Failed to get replicaset from watch api server")
			}
			if event.Type == watch.Error {
				t.Fatalf("Result channel get error event. %v", event)
			}
			meta, err := meta.Accessor(event.Object)
			if err != nil {
				t.Fatalf("Unable to understand watch event %#v", event)
			}

			if rs.UID == meta.GetUID() {
				rsFound = true
				assert.Equal(t, rs.UID, meta.GetUID())
				assert.Equal(t, rs.HashKey, meta.GetHashKey())
				assert.Equal(t, rs.Name, meta.GetName())
				assert.Equal(t, rs.Namespace, meta.GetNamespace())
				assert.Equal(t, rs.Tenant, meta.GetTenant())
			}

			if pod.UID == meta.GetUID() {
				t.Fatalf("The api server should not sync other pods data")
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("unable to get replicaset from watch api server")
		}

		if rsFound {
			break
		}
	}
	assert.True(t, rsFound)
}

// Both use partition manager. Can only run sequentially as DP manager is singleton
func TestAPIServerPartitionWithPartitionManager(t *testing.T) {
	testDataPartitionReset(t)
	testOneApiServerCluster(t)
}

func testDataPartitionReset(t *testing.T) {
	// set up one api server
	serviceGroupId := "10"

	prefix, configFilename := createSingleApiServerPartitionFile(t, "A", "z")
	masterConfig := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename, "", serviceGroupId)
	_, s, closeFn := framework.RunAMasterWithDataPartition(masterConfig)
	stopCh := make(chan struct{})
	defer close(stopCh)
	go masterConfig.ExtraConfig.DataPartitionManager.Run(stopCh)
	config := restclient.NewAggregatedConfig(&restclient.KubeConfig{Host: s.URL})
	clientSet, err := clientset.NewForConfig(config)
	if err != nil {
		t.Fatalf("Error in create clientset: %v", err)
	}

	defer deleteSinglePartitionConfigFile(t, configFilename)
	defer closeFn()

	// create informer from server
	resyncPeriod := 20 * time.Second
	informerFactory := informers.NewSharedInformerFactory(clientSet, resyncPeriod)
	informerFactory.Start(stopCh)

	startEventBroadCaster(t, clientSet)
	informerFactory.WaitForCacheSync(stopCh)
	rsInformer := informerFactory.Apps().V1().ReplicaSets()
	rsLister := rsInformer.Lister()
	go rsInformer.Informer().Run(stopCh)

	informerFactory.WaitForCacheSync(stopCh)

	namespace := "ns1"
	rsClient1 := clientSet.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant1)
	w1, err := rsClient1.Watch(metav1.ListOptions{})
	defer w1.Stop()
	assert.Nil(t, err)

	// create rs with tenant in data partition
	rs1 := createRS(t, clientSet, tenant1, namespace, "rs1", 1)
	assert.NotNil(t, rs1)

	// create rs with tenant not in data partition
	rs2 := createRS(t, clientSet, "zzz", namespace, "rs2", 1)
	assert.NotNil(t, rs2)

	time.Sleep(5 * time.Second)

	// check data from api servers
	rslist1, err := rsLister.ReplicaSetsWithMultiTenancy(rs1.Namespace, rs1.Tenant).List(labels.Everything())
	assert.Nil(t, err)
	assert.NotNil(t, rslist1)

	rslist2, err := rsLister.ReplicaSetsWithMultiTenancy(rs2.Namespace, rs2.Tenant).List(labels.Everything())
	assert.Nil(t, err)
	assert.True(t, rslist2 == nil || len(rslist2) == 0)

	// check rs1 in informer list
	assert.True(t, checkReplicaSetExistence3(rslist1, rs1))

	// check rs2 NOT in informer 1 list
	assert.False(t, checkReplicaSetExistence3(rslist2, rs2))

	// update data partition for api server to [tenant2, zzzz)
	dpConfigData := &v1.DataPartitionConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "partition-10",
		},
		RangeStart:        "tenant2",
		IsRangeStartValid: true,
		RangeEnd:          "zzzz",
		IsRangeEndValid:   true,
		ServiceGroupId:    serviceGroupId,
	}
	dpConfig, err := clientSet.CoreV1().DataPartitionConfigs().Create(dpConfigData)
	assert.Nil(t, err)
	assert.Equal(t, dpConfig.Name, dpConfigData.Name)
	assert.Equal(t, dpConfig.ServiceGroupId, dpConfigData.ServiceGroupId)
	assert.Equal(t, dpConfig.RangeStart, dpConfigData.RangeStart)
	assert.Equal(t, dpConfig.RangeEnd, dpConfigData.RangeEnd)
	assert.Equal(t, dpConfig.IsRangeEndValid, dpConfigData.IsRangeEndValid)
	assert.Equal(t, dpConfig.ServiceGroupId, dpConfigData.ServiceGroupId)

	// wait for population as resync is 30 second
	time.Sleep(35 * time.Second)

	// Get list again
	rslist3, err := rsLister.ReplicaSetsWithMultiTenancy(rs2.Namespace, rs2.Tenant).List(labels.Everything())
	t.Logf("rs list 3 [%#v]", rslist3)

	// check rs1 NOT in informer 1 list - is already in cache.
	// No cache invalidation now - waiting for compact
	//assert.False(t, checkReplicaSetExistence(rslist3, rs1))

	// check rs2 in informer 1 list
	assert.True(t, checkReplicaSetExistence3(rslist3, rs2))
}

func testOneApiServerCluster(t *testing.T) {
	serviceGroupId := "10"
	masterCount := 2

	// 1. set up api server 1
	prefix, configFilename := createSingleApiServerPartitionFile(t, "A", "z")
	defer deleteSinglePartitionConfigFile(t, configFilename)
	masterConfig1 := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename, masterAddr1, serviceGroupId)
	masterConfig1.ExtraConfig.MasterCount = masterCount
	_, s1, closeFn1 := framework.RunAMaster(masterConfig1)
	defer closeFn1()
	stopCh1 := make(chan struct{})
	defer close(stopCh1)
	//go masterConfig1.ExtraConfig.DataPartitionManager.Run(stopCh1)
	config1 := restclient.NewAggregatedConfig(&restclient.KubeConfig{Host: s1.URL})
	clientSet1, err := clientset.NewForConfig(config1)
	if err != nil {
		t.Fatalf("Error in create clientset 1: %v", err)
	}

	// create informer from server 1
	resyncPeriod := 30 * time.Second
	informerFactory1 := informers.NewSharedInformerFactory(clientSet1, resyncPeriod)
	informerFactory1.Start(stopCh1)

	startEventBroadCaster(t, clientSet1)
	informerFactory1.WaitForCacheSync(stopCh1)
	rsInformer1 := informerFactory1.Apps().V1().ReplicaSets()
	rsLister1 := rsInformer1.Lister()
	go rsInformer1.Informer().Run(stopCh1)

	informerFactory1.WaitForCacheSync(stopCh1)

	// 2. set up api server 2 with different ip in same service group
	masterConfig2 := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename, masterAddr2, serviceGroupId)
	masterConfig2.ExtraConfig.MasterCount = masterCount
	_, s2, closeFn2 := framework.RunAMaster(masterConfig2)
	defer closeFn2()
	stopCh2 := make(chan struct{})
	defer close(stopCh2)
	config2 := restclient.NewAggregatedConfig(&restclient.KubeConfig{Host: s2.URL})
	clientSet2, err := clientset.NewForConfig(config2)
	if err != nil {
		t.Fatalf("Error in create clientset 2: %v", err)
	}

	// create informer from server 2
	informerFactory2 := informers.NewSharedInformerFactory(clientSet2, resyncPeriod)
	informerFactory2.Start(stopCh2)

	startEventBroadCaster(t, clientSet2)
	informerFactory2.WaitForCacheSync(stopCh2)
	rsInformer2 := informerFactory2.Apps().V1().ReplicaSets()
	rsLister2 := rsInformer2.Lister()
	go rsInformer2.Informer().Run(stopCh2)

	informerFactory2.WaitForCacheSync(stopCh2)

	// 3. create replicaset
	namespace := "ns1"
	rsClient1 := clientSet1.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant1)
	w1, err := rsClient1.Watch(metav1.ListOptions{})
	defer w1.Stop()
	assert.Nil(t, err)

	// create rs with tenant in data partition
	rs1 := createRS(t, clientSet1, tenant1, namespace, "rs1", 1)
	assert.NotNil(t, rs1)

	// create rs with tenant not in data partition
	rs2 := createRS(t, clientSet1, "zzz", namespace, "rs2", 1)
	assert.NotNil(t, rs2)

	time.Sleep(2 * time.Second)

	// 4.1. check data from api servers 1
	rslist11, err := rsLister1.ReplicaSetsWithMultiTenancy(rs1.Namespace, rs1.Tenant).List(labels.Everything())
	assert.Nil(t, err)
	assert.NotNil(t, rslist11)

	rslist12, err := rsLister1.ReplicaSetsWithMultiTenancy(rs2.Namespace, rs2.Tenant).List(labels.Everything())
	assert.Nil(t, err)
	assert.True(t, rslist12 == nil || len(rslist12) == 0)

	// check rs1 in informer list
	assert.True(t, checkReplicaSetExistence3(rslist11, rs1))

	// 4.2. check data from api servers 2
	rslist21, err := rsLister2.ReplicaSetsWithMultiTenancy(rs1.Namespace, rs1.Tenant).List(labels.Everything())
	assert.Nil(t, err)
	assert.NotNil(t, rslist21)

	rslist22, err := rsLister2.ReplicaSetsWithMultiTenancy(rs2.Namespace, rs2.Tenant).List(labels.Everything())
	assert.Nil(t, err)
	assert.True(t, rslist22 == nil || len(rslist22) == 0)

	// check rs1 in informer list
	assert.True(t, checkReplicaSetExistence3(rslist21, rs1))

	// 5. check master lease in storage
	endpointClient := clientv1core.NewForConfigOrDie(masterConfig1.GenericConfig.LoopbackClientConfig)
	e, err := endpointClient.Endpoints(v1.NamespaceDefault).Get(kubernetesServiceName, metav1.GetOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, e)
	assert.Equal(t, 1, len(e.Subsets))
	assert.Equal(t, serviceGroupId, e.Subsets[0].ServiceGroupId)
	assert.Equal(t, 2, len(e.Subsets[0].Addresses))
	assert.Equal(t, masterAddr1, e.Subsets[0].Addresses[0].IP)
	assert.Equal(t, masterAddr2, e.Subsets[0].Addresses[1].IP)

	// tear down
	deleteRS(t, clientSet1, rs1)
	deleteRS(t, clientSet1, rs2)
}

// Cannot test data partition as DP manager is singleton - needs to test in e2e tests
func TestTwoApiServerCluster(t *testing.T) {
	serviceGroup1Id := "1"
	serviceGroup2Id := "2"
	masterCount := 3

	t.Log("1. set up api server 1 with serviceGroup1Id")
	prefix, configFilename1 := createSingleApiServerPartitionFile(t, "A", "m")
	defer deleteSinglePartitionConfigFile(t, configFilename1)

	masterConfig1 := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename1, masterAddr1, serviceGroup1Id)
	masterConfig1.ExtraConfig.MasterCount = masterCount
	_, _, closeFn1 := framework.RunAMaster(masterConfig1)

	t.Log("2. set up api server 2 with serviceGroup2Id")
	masterConfig2 := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename1, masterAddr2, serviceGroup2Id)
	masterConfig2.ExtraConfig.MasterCount = masterCount
	_, _, closeFn2 := framework.RunAMaster(masterConfig2)
	defer closeFn2()

	t.Log("3. set up api server 3 with serviceGroup1Id")
	masterAddr3 := "172.10.10.1"
	masterConfig3 := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename1, masterAddr3, serviceGroup1Id)
	masterConfig3.ExtraConfig.MasterCount = masterCount
	_, _, closeFn3 := framework.RunAMaster(masterConfig3)

	t.Log("4. set up api server 4 with serviceGroup2Id")
	masterAddr4 := "100.1.1.10"
	masterConfig4 := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename1, masterAddr4, serviceGroup2Id)
	masterConfig4.ExtraConfig.MasterCount = masterCount
	_, _, closeFn4 := framework.RunAMaster(masterConfig4)
	defer closeFn4()

	t.Log("5. set up api server with serviceGroup2Id")
	masterAddr5 := "100.1.1.9"
	masterConfig5 := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename1, masterAddr5, serviceGroup2Id)
	masterConfig5.ExtraConfig.MasterCount = masterCount
	_, _, closeFn5 := framework.RunAMaster(masterConfig5)

	time.Sleep(5 * time.Second)

	t.Log("5.1 check master lease in storage")
	endpointClient := clientv1core.NewForConfigOrDie(masterConfig1.GenericConfig.LoopbackClientConfig)
	e, err := endpointClient.Endpoints(v1.NamespaceDefault).Get(kubernetesServiceName, metav1.GetOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, e)
	t.Logf("endpoints [%+v]", e.Subsets)
	assert.Equal(t, 2, len(e.Subsets))
	assert.Equal(t, serviceGroup1Id, e.Subsets[0].ServiceGroupId)
	assert.Equal(t, serviceGroup2Id, e.Subsets[1].ServiceGroupId)

	assert.Equal(t, 2, len(e.Subsets[0].Addresses))
	assert.Equal(t, masterAddr3, e.Subsets[0].Addresses[0].IP)
	assert.Equal(t, masterAddr1, e.Subsets[0].Addresses[1].IP)

	assert.Equal(t, 3, len(e.Subsets[1].Addresses))
	assert.Equal(t, masterAddr4, e.Subsets[1].Addresses[0].IP)
	assert.Equal(t, masterAddr5, e.Subsets[1].Addresses[1].IP)
	assert.Equal(t, masterAddr2, e.Subsets[1].Addresses[2].IP)

	t.Logf("6. master 5 died")
	closeFn5()

	// master lease expires in 10 seconds
	time.Sleep(11 * time.Second)

	e, err = endpointClient.Endpoints(v1.NamespaceDefault).Get(kubernetesServiceName, metav1.GetOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, e)
	assert.Equal(t, 2, len(e.Subsets))
	assert.Equal(t, serviceGroup1Id, e.Subsets[0].ServiceGroupId)
	assert.Equal(t, serviceGroup2Id, e.Subsets[1].ServiceGroupId)

	assert.Equal(t, 2, len(e.Subsets[0].Addresses))
	assert.Equal(t, masterAddr3, e.Subsets[0].Addresses[0].IP)
	assert.Equal(t, masterAddr1, e.Subsets[0].Addresses[1].IP)

	assert.Equal(t, 2, len(e.Subsets[1].Addresses))
	assert.Equal(t, masterAddr4, e.Subsets[1].Addresses[0].IP)
	assert.Equal(t, masterAddr2, e.Subsets[1].Addresses[1].IP)

	t.Log("7. master 1 and 3 died - simulate all server in one service group died")
	closeFn1()
	closeFn3()
	time.Sleep(11 * time.Second)
	e, err = endpointClient.Endpoints(v1.NamespaceDefault).Get(kubernetesServiceName, metav1.GetOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, e)
	assert.Equal(t, 1, len(e.Subsets))
	assert.Equal(t, serviceGroup2Id, e.Subsets[0].ServiceGroupId)

	assert.Equal(t, 2, len(e.Subsets[0].Addresses))
	assert.Equal(t, masterAddr4, e.Subsets[0].Addresses[0].IP)
	assert.Equal(t, masterAddr2, e.Subsets[0].Addresses[1].IP)
}

func setUpSingleApiserver(t *testing.T, begin, end string, serviceGroupId string) (*httptest.Server, framework.CloseFunc, clientset.Interface, string) {
	prefix, configFilename := createSingleApiServerPartitionFile(t, begin, end)
	masterConfig := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename, "", serviceGroupId)
	_, s, closeFn := framework.RunAMaster(masterConfig)
	config := restclient.NewAggregatedConfig(&restclient.KubeConfig{Host: s.URL})
	clientSet, err := clientset.NewForConfig(config)
	if err != nil {
		t.Fatalf("Error in create clientset: %v", err)
	}
	return s, closeFn, clientSet, configFilename
}

func generatedPrefix() string {
	return path.Join(uuid.New(), "registry")
}

func createSinglePartitionConfig(t *testing.T, fileSuffix int, prefix, begin, end string) (configFilename string) {
	podData := fmt.Sprintf(podDataParitionModel, prefix, begin, end)
	rsData := fmt.Sprintf(rsDataParitionModel, prefix, begin, end)

	// const does not work here
	configFilename = fmt.Sprintf("apiserver-%d.config", fileSuffix)
	configFile, err := os.Create(configFilename)
	if err != nil {
		t.Fatalf("Unable to create api server partition file. error %v", err)
	}
	_, err = configFile.WriteString(podData)
	if err != nil {
		t.Fatalf("Unable to write api server partition file. error %v", err)
	}

	_, err = configFile.WriteString(rsData)
	if err != nil {
		t.Fatalf("Unable to write api server partition file. error %v", err)
	}

	return configFilename
}

func createSingleApiServerPartitionFile(t *testing.T, begin, end string) (prefix, configFilename string) {
	prefix = generatedPrefix()
	return prefix, createSinglePartitionConfig(t, 0, prefix, begin, end)
}

func createTwoApiServersPartitionFiles(t *testing.T) (prefix, configFilename1, configFilename2 string) {
	prefix = generatedPrefix()
	configFilename1 = createSinglePartitionConfig(t, 0, prefix, tenant1, tenant2)
	configFilename2 = createSinglePartitionConfig(t, 1, prefix, tenant2, tenant3)
	return prefix, configFilename1, configFilename2
}

func deleteSinglePartitionConfigFile(t *testing.T, configFilename string) {
	err := os.Remove(configFilename)
	if err != nil {
		t.Fatalf("Unable to delete api server partition file %s. error %v", configFilename, err)
	}
}
