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

package factory

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/kubernetes/fake"
	fakeV1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
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

func TestBindWithMultiTenancy(t *testing.T) {
	table := []struct {
		name    string
		binding *v1.Binding
	}{
		{
			name: "binding can bind and validate request",
			binding: &v1.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Tenant:    testTenant,
					Name:      "foo",
				},
				Target: v1.ObjectReference{
					Name: "foohost.kubernetes.mydomain.com",
				},
			},
		},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			testBindWithMultiTenancy(test.binding, t)
		})
	}
}

func testBindWithMultiTenancy(binding *v1.Binding, t *testing.T) {
	testPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: binding.GetName(), Namespace: metav1.NamespaceDefault, Tenant: testTenant},
		Spec:       apitesting.V1DeepEqualSafePodSpec(),
	}
	client := fake.NewSimpleClientset(&v1.PodList{Items: []v1.Pod{*testPod}})

	b := binder{client}

	if err := b.Bind(binding); err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	pod := client.CoreV1().PodsWithMultiTenancy(metav1.NamespaceDefault, testTenant).(*fakeV1.FakePods)

	actualBinding, err := pod.GetBinding(binding.GetName())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
		return
	}
	if !reflect.DeepEqual(binding, actualBinding) {
		t.Errorf("Binding did not match expectation, expected: %v, actual: %v", binding, actualBinding)
	}
}

func TestDefaultErrorFuncWithMultiTenancy(t *testing.T) {
	testPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar", Tenant: testTenant},
		Spec:       apitesting.V1DeepEqualSafePodSpec(),
	}
	client := fake.NewSimpleClientset(&v1.PodList{Items: []v1.Pod{*testPod}})
	stopCh := make(chan struct{})
	defer close(stopCh)

	timestamp := time.Now()
	queue := internalqueue.NewPriorityQueueWithClock(nil, clock.NewFakeClock(timestamp), nil)
	schedulerCache := internalcache.New(30*time.Second, stopCh)
	errFunc := MakeDefaultErrorFunc(client, queue, schedulerCache, stopCh)

	// Trigger error handling again to put the pod in unschedulable queue
	errFunc(testPod, nil)

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
	queue.MoveAllToActiveQueue()

	// Trigger error handling again to put the pod in backoff queue
	errFunc(testPod, nil)

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
