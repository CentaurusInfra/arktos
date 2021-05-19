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

package scheduler

import (
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	"reflect"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	internalcache "k8s.io/kubernetes/pkg/scheduler/internal/cache"
	internalqueue "k8s.io/kubernetes/pkg/scheduler/internal/queue"
)

var testTenant = "test-te"

// testClientGetPodRequest function provides a routine used by TestDefaultErrorFunc test.
// It tests whether the fake client can receive request and correctly "get" the  tenant, namespace
// and name of the error pod.
func testClientGetPodRequestWithMultiTenancy(client *fake.Clientset, t *testing.T, podTenant string, podNs string, podName string) {
	requestReceived := false
	actions := client.Actions()
	for _, a := range actions {
		if a.GetVerb() == "get" {
			getAction, ok := a.(clienttesting.GetAction)
			if !ok {
				t.Errorf("Can't cast action object to GetAction interface")
				break
			}
			name := getAction.GetName()
			ns := a.GetNamespace()
			tenant := a.GetTenant()
			if name != podName || ns != podNs || tenant != podTenant {
				t.Errorf("Expected name %s namespace %s tenant %s, got %s %s %s",
					podName, podNs, podTenant, tenant, name, ns)
			}
			requestReceived = true
		}
	}
	if !requestReceived {
		t.Errorf("Get pod request not received")
	}
}

func TestDefaultErrorFuncWithMultiTenancy(t *testing.T) {
	testPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar", Tenant: testTenant},
		Spec:       apitesting.V1DeepEqualSafePodSpec(),
	}
	testPodInfo := &framework.PodInfo{Pod: testPod}
	client := fake.NewSimpleClientset(&v1.PodList{Items: []v1.Pod{*testPod}})
	stopCh := make(chan struct{})
	defer close(stopCh)

	timestamp := time.Now()
	queue := internalqueue.NewPriorityQueue(nil, internalqueue.WithClock(clock.NewFakeClock(timestamp)))
	schedulerCache := internalcache.New(30*time.Second, stopCh)
	errFunc := MakeDefaultErrorFunc(client, queue, schedulerCache)

	// Trigger error handling again to put the pod in unschedulable queue
	errFunc(testPodInfo, nil)

	// Try up to a minute to retrieve the error pod from priority queue
	foundPodFlag := false
	maxIterations := 10 * 60
	for i := 0; i < maxIterations; i++ {
		time.Sleep(100 * time.Millisecond)
		got := getPodfromPriorityQueue(queue, testPod)
		if got == nil {
			continue
		}

		testClientGetPodRequestWithMultiTenancy(client, t, testPod.Tenant, testPod.Namespace, testPod.Name)

		if e, a := testPod, got; !reflect.DeepEqual(e, a) {
			t.Errorf("Expected %v, got %v", e, a)
		}

		foundPodFlag = true
		break
	}

	if !foundPodFlag {
		t.Errorf("Failed to get pod from the unschedulable queue after waiting for a minute: %v", testPod)
	}

	// Remove the pod from priority queue to test putting error
	// pod in backoff queue.
	queue.Delete(testPod)

	// Trigger a move request
	queue.MoveAllToActiveOrBackoffQueue("test")

	// Trigger error handling again to put the pod in backoff queue
	errFunc(testPodInfo, nil)

	foundPodFlag = false
	for i := 0; i < maxIterations; i++ {
		time.Sleep(100 * time.Millisecond)
		// The pod should be found from backoff queue at this time
		got := getPodfromPriorityQueue(queue, testPod)
		if got == nil {
			continue
		}

		testClientGetPodRequestWithMultiTenancy(client, t, testPod.Tenant, testPod.Namespace, testPod.Name)

		if e, a := testPod, got; !reflect.DeepEqual(e, a) {
			t.Errorf("Expected %v, got %v", e, a)
		}

		foundPodFlag = true
		break
	}

	if !foundPodFlag {
		t.Errorf("Failed to get pod from the backoff queue after waiting for a minute: %v", testPod)
	}
}
