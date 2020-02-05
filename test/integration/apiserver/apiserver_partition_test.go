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

	podDataParitionModel = "/%s/pods/, %s, %s\n"
	rsDataParitionModel  = "/%s/replicasets/, %s, %s\n"

	configFilename1 = "apiserver-0.config"
	configFilename2 = "apiserver-1.config"
)

func setUpApiservers(t *testing.T) (*httptest.Server, framework.CloseFunc, clientset.Interface, *httptest.Server, framework.CloseFunc, clientset.Interface) {
	prefix, configFilename1, configFilename2 := createApiServerDataPartitionFile(t)
	masterConfig1, masterConfig2 := framework.NewIntegrationTestMasterConfigParition(prefix, configFilename1, configFilename2)
	_, s1, closeFn1 := framework.RunAMaster(masterConfig1)

	config1 := restclient.Config{Host: s1.URL}
	clientSet1, err := clientset.NewForConfig(&config1)
	if err != nil {
		t.Fatalf("Error in create clientset: %v", err)
	}

	_, s2, closeFn2 := framework.RunAMaster(masterConfig2)

	config2 := restclient.Config{Host: s2.URL}
	clientSet2, err := clientset.NewForConfig(&config2)
	if err != nil {
		t.Fatalf("Error in create clientset: %v", err)
	}

	return s1, closeFn1, clientSet1, s2, closeFn2, clientSet2
}

func TestSetupMultipleApiServers(t *testing.T) {
	s1, closeFn1, clientset1, s2, closeFn2, clientset2 := setUpApiservers(t)
	defer closeFn1()
	defer closeFn2()
	t.Logf("server 1 %+v, clientset 2 %+v", s1, clientset1)
	t.Logf("server 2 %+v, clientset 2 %+v", s2, clientset2)

	assert.NotNil(t, s1)
	assert.NotNil(t, s2)
	assert.NotNil(t, clientset1)
	assert.NotNil(t, clientset2)
	assert.NotEqual(t, s1.URL, s2.URL)
	deleteApiServerDataPartitionFile(t)
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

func startEventBroadCaster(t *testing.T, cs clientset.Interface) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&clientv1core.EventSinkImpl{
		Interface: cs.CoreV1().Events(""),
	})
}

func createApiServerDataPartitionFile(t *testing.T) (prefix, configFilename1, configFilename2 string) {
	prefix = path.Join(uuid.New(), "registry")

	// partition 1
	podData1 := fmt.Sprintf(podDataParitionModel, prefix, tenant1, tenant2)
	rsData1 := fmt.Sprintf(rsDataParitionModel, prefix, tenant1, tenant2)

	// const does not work here
	file1, err := os.Create("apiserver-0.config")
	if err != nil {
		t.Fatalf("Unable to create api server partition file. error %v", err)
	}
	_, err = file1.WriteString(podData1)
	if err != nil {
		t.Fatalf("Unable to write api server partition file. error %v", err)
	}

	_, err = file1.WriteString(rsData1)
	if err != nil {
		t.Fatalf("Unable to write api server partition file. error %v", err)
	}

	// parition 2
	podData2 := fmt.Sprintf(podDataParitionModel, prefix, tenant2, "tenant3")
	rsData2 := fmt.Sprintf(rsDataParitionModel, prefix, tenant2, "tenant3")
	file2, err := os.Create("apiserver-1.config")
	if err != nil {
		t.Fatalf("Unable to create api server partition file. error %v", err)
	}
	_, err = file2.WriteString(podData2)
	if err != nil {
		t.Fatalf("Unable to write api server partition file. error %v", err)
	}

	_, err = file2.WriteString(rsData2)
	if err != nil {
		t.Fatalf("Unable to write api server partition file. error %v", err)
	}

	return prefix, configFilename1, configFilename2
}

func deleteApiServerDataPartitionFile(t *testing.T) {
	err := os.Remove(configFilename1)
	if err != nil {
		t.Fatalf("Unable to delete api server partition file. error %v", err)
	}
	err = os.Remove(configFilename2)
	if err != nil {
		t.Fatalf("Unable to delete api server partition file. error %v", err)
	}
}

// Ideally, we should test all kinds - TODO - should be able to leverage generated test data
func TestGetCanGetAlldata(t *testing.T) {
	s1, closeFn1, clientset1, s2, _, clientset2 := setUpApiservers(t)
	//_, closeFn1, clientset1, _, closeFn2, clientset2 := setUpApiservers(t)
	defer closeFn1()
	//defer closeFn2()

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
	deleteApiServerDataPartitionFile(t)
}

func TestListCanGetAlldata(t *testing.T) {
	s1, closeFn1, clientset1, _, _, clientset2 := setUpApiservers(t)
	defer closeFn1()
	//defer closeFn2()

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
	deleteApiServerDataPartitionFile(t)
}

func TestPostCanUpdateAlldata(t *testing.T) {
	s1, closeFn1, clientset1, s2, _, clientset2 := setUpApiservers(t)
	defer closeFn1()
	//defer closeFn2()

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
	deleteApiServerDataPartitionFile(t)
}

func TestWatchOnlyGetDataFromOneParition(t *testing.T) {
	_, closeFn1, clientset1, _, _, clientset2 := setUpApiservers(t)
	defer closeFn1()
	//defer closeFn2()

	// create informer 1 from server 1
	resyncPeriod := 12 * time.Hour
	informer1 := informers.NewSharedInformerFactory(clientset1, resyncPeriod)
	stopCh := make(chan struct{})
	informer1.Start(stopCh)
	defer close(stopCh)

	startEventBroadCaster(t, clientset1)
	informer1.WaitForCacheSync(stopCh)
	go informer1.Apps().V1().ReplicaSets().Informer().Run(stopCh)

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

	// check data from different api servers
	rs1Found := false
	rs2Found := false
	for {
		select {
		case event, ok := <-w1.ResultChan():
			if !ok {
				t.Fatalf("Failed to get replicaset from watch api server 1")
			}
			if event.Type == watch.Error {
				t.Fatalf("Result channel get error event. %v", event)
			}
			meta, err := meta.Accessor(event.Object)
			if err != nil {
				t.Fatalf("Unable to understand watch event %#v", event)
			}
			assert.Equal(t, rs1.UID, meta.GetUID())
			assert.Equal(t, rs1.HashKey, meta.GetHashKey())
			assert.Equal(t, rs1.Name, meta.GetName())
			assert.Equal(t, rs1.Namespace, meta.GetNamespace())
			assert.Equal(t, rs1.Tenant, meta.GetTenant())
			rs1Found = true

		case event, ok := <-w2.ResultChan():
			if !ok {
				t.Fatalf("Failed to get replicaset from watch api server 2")
			}
			if event.Type == watch.Error {
				t.Fatalf("Result channel get error event. %v", event)
			}
			meta, err := meta.Accessor(event.Object)
			if err != nil {
				t.Fatalf("Unable to understand watch event %#v", event)
			}
			assert.Equal(t, rs2.UID, meta.GetUID())
			assert.Equal(t, rs2.HashKey, meta.GetHashKey())
			assert.Equal(t, rs2.Name, meta.GetName())
			assert.Equal(t, rs2.Namespace, meta.GetNamespace())
			assert.Equal(t, rs2.Tenant, meta.GetTenant())
			rs2Found = true

		case <-time.After(10 * time.Second):
			t.Fatalf("unable to get replicaset from watch api server 1")
		}

		if rs1Found && rs2Found {
			break
		}
	}

	assert.True(t, rs1Found)
	assert.True(t, rs2Found)

	// TODO - add more tests

	// tear down
	deleteRS(t, clientset1, rs1)
	deleteRS(t, clientset1, rs2)
	deleteApiServerDataPartitionFile(t)
}

// TODO - Update and re-enable after informer data is merged
func _TestInformerCanGetAllData(t *testing.T) {
	time.Sleep(10 * time.Second)
	_, closeFn1, clientset1, _, _, clientset2 := setUpApiservers(t)
	defer closeFn1()
	//defer closeFn2()

	// create informer 1 from server 1
	resyncPeriod := 12 * time.Hour
	informer1 := informers.NewSharedInformerFactory(clientset1, resyncPeriod)
	stopCh := make(chan struct{})
	informer1.Start(stopCh)
	defer close(stopCh)

	startEventBroadCaster(t, clientset1)
	informer1.WaitForCacheSync(stopCh)
	go informer1.Apps().V1().ReplicaSets().Informer().Run(stopCh)

	// create informer 2 from server 2
	informer2 := informers.NewSharedInformerFactory(clientset2, resyncPeriod)
	informer2.Start(stopCh)
	startEventBroadCaster(t, clientset2)
	informer2.WaitForCacheSync(stopCh)
	go informer2.Apps().V1().ReplicaSets().Informer().Run(stopCh)

	// create rs via 2 different api servers
	namespace := "ns1"
	rs1 := createRS(t, clientset1, tenant1, namespace, "rs1", 1)
	rs2 := createRS(t, clientset2, tenant2, namespace, "rs2", 1)
	klog.Infof("created replicaset 1 [%+v]", rs1)
	klog.Infof("created replicaset 2 [%+v]", rs2)
	assert.NotNil(t, rs1)
	assert.NotNil(t, rs2)
	assert.NotEqual(t, rs1.UID, rs2.UID)

	//time.Sleep(10 * time.Second)

	// check data from different api servers
	rslist1 := informer1.Apps().V1().ReplicaSets().Informer().GetIndexer().List()
	assert.NotNil(t, rslist1)
	klog.Infof(" rs list 1 from informer 1 len %d, [%+v]", len(rslist1), rslist1)

	rslist2 := informer2.Apps().V1().ReplicaSets().Informer().GetIndexer().List()
	assert.NotNil(t, rslist2)
	klog.Infof(" rs list 2 from informer 2 len %d, [%+v]", len(rslist2), rslist2)

	// check rs1 in informer 1 list
	assert.True(t, checkReplicaSetExistence2(rslist1, rs1))

	// TODO - uncomment after informer data merged
	// check rs2 in informer 1 list
	//assert.True(t, checkReplicaSetExistence2(rslist1, rs2))

	// check rs2 in informer 2 list
	assert.True(t, checkReplicaSetExistence2(rslist2, rs2))

	// TODO - uncomment after informer data merged
	// check rs1 in informer 2 list
	//assert.True(t, checkReplicaSetExistence2(rslist2, rs1))

	// tear down
	deleteRS(t, clientset1, rs1)
	deleteRS(t, clientset1, rs2)
	deleteApiServerDataPartitionFile(t)
}

// Test apiserver sync data like ["registry/pods/", "registry/pods/tenant2")
func TestPartitionWithLeftUnbounded(t *testing.T) {
	_, closeFn, clientset:= setUpApiserver(t, "", tenant2 )
	defer closeFn()
	//defer closeFn2()

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

	otherRs :=  createRS(t, clientset, "tenant3", namespace, "rs1", 1)
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
	deletePartitionConfigFile(t)
}

// Test apiserver sync data like ["registry/pods/tenant2", "")
func TestPartitionRightUnbounded(t *testing.T) {
	_, closeFn, clientset:= setUpApiserver(t, tenant2, "" )
	defer closeFn()
	//defer closeFn2()

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

	otherRs :=  createRS(t, clientset, tenant1, namespace, "rs1", 1)
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
	deletePartitionConfigFile(t)
}

// Test apiserver sync data like ["registry/pods/tenant2", "registry/pods/tenant3")
func TestPartitionLeftRightBounded(t *testing.T) {
	_, closeFn, clientset:= setUpApiserver(t, tenant2,  "tenant3" )
	defer closeFn()
	//defer closeFn2()

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

	otherRs :=  createRS(t, clientset, "tenant3", namespace, "rs3", 1)
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
	deletePartitionConfigFile(t)
}

func setUpApiserver(t *testing.T, begin, end string) (*httptest.Server, framework.CloseFunc, clientset.Interface) {
	prefix, configFilename1 := createPartitionConfig(t, begin, end)
	masterConfig := framework.NewIntegrationServerWithPartitionConfig(prefix, configFilename1)
	_, s, closeFn := framework.RunAMaster(masterConfig)

	config := restclient.Config{Host: s.URL}
	clientSet, err := clientset.NewForConfig(&config)
	if err != nil {
		t.Fatalf("Error in create clientset: %v", err)
	}
	return s, closeFn, clientSet
}

func createPartitionConfig(t *testing.T, begin, end string) (prefix, configFilename string) {
	prefix = path.Join(uuid.New(), "registry")

	podData := fmt.Sprintf(podDataParitionModel, prefix, begin, end)
	rsData := fmt.Sprintf(rsDataParitionModel, prefix, begin, end)

	// const does not work here
	configFile, err := os.Create("apiserver-0.config")
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

	return prefix, configFilename
}

func deletePartitionConfigFile(t *testing.T) {
	err := os.Remove(configFilename1)
	if err != nil {
		t.Fatalf("Unable to delete api server partition file. error %v", err)
	}
}


