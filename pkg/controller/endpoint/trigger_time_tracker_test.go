/*
Copyright 2019 The Kubernetes Authors.
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

package endpoint

import (
	"runtime"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	t0 = time.Date(2019, 01, 01, 0, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Second)
	t2 = t1.Add(time.Second)
	t3 = t2.Add(time.Second)
	t4 = t3.Add(time.Second)
	t5 = t4.Add(time.Second)

	ns   = "ns1"
	name = "my-service"
)

func TestNewService_NoPods(t *testing.T) {
	testNewService_NoPods(t, metav1.TenantSystem)
}

func TestNewService_NoPodsWithMultiTenancy(t *testing.T) {
	testNewService_NoPods(t, "test-te")
}

func testNewService_NoPods(t *testing.T, tenant string) {
	tester := newTester(t)

	service := createService(tenant, ns, name, t2)
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service).expect(t2)
}

func TestNewService_ExistingPods(t *testing.T) {
	testNewService_ExistingPods(t, metav1.TenantSystem)
}

func TestNewService_ExistingPodsWithMultiTenancy(t *testing.T) {
	testNewService_ExistingPods(t, "test-te")
}

func testNewService_ExistingPods(t *testing.T, tenant string) {
	tester := newTester(t)

	service := createService(tenant, ns, name, t3)
	pod1 := createPod(tenant, ns, "pod1", t0)
	pod2 := createPod(tenant, ns, "pod2", t1)
	pod3 := createPod(tenant, ns, "pod3", t5)
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2, pod3).
		// Pods were created before service, but trigger time is the time when service was created.
		expect(t3)
}

func TestPodsAdded(t *testing.T) {
	testPodsAdded(t, metav1.TenantSystem)
}

func TestPodsAddedWithMultiTenancy(t *testing.T) {
	testPodsAdded(t, "test-te")
}

func testPodsAdded(t *testing.T, tenant string) {
	tester := newTester(t)

	service := createService(tenant, ns, name, t0)
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service).expect(t0)

	pod1 := createPod(tenant, ns, "pod1", t2)
	pod2 := createPod(tenant, ns, "pod2", t1)
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2).expect(t1)
}

func TestPodsUpdated(t *testing.T) {
	testPodsUpdated(t, metav1.TenantSystem)
}

func TestPodsUpdatedWithMultiTenancy(t *testing.T) {
	testPodsUpdated(t, "test-te")
}

func testPodsUpdated(t *testing.T, tenant string) {
	tester := newTester(t)

	service := createService(tenant, ns, name, t0)
	pod1 := createPod(tenant, ns, "pod1", t1)
	pod2 := createPod(tenant, ns, "pod2", t2)
	pod3 := createPod(tenant, ns, "pod3", t3)
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2, pod3).expect(t0)

	pod1 = createPod(tenant, ns, "pod1", t5)
	pod2 = createPod(tenant, ns, "pod2", t4)
	// pod3 doesn't change.
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2, pod3).expect(t4)
}

func TestPodsUpdated_NoOp(t *testing.T) {
	testPodsUpdated_NoOp(t, metav1.TenantSystem)
}

func TestPodsUpdated_NoOpWithMultiTenancy(t *testing.T) {
	testPodsUpdated_NoOp(t, "test-te")
}

func testPodsUpdated_NoOp(t *testing.T, tenant string) {
	tester := newTester(t)

	service := createService(tenant, ns, name, t0)
	pod1 := createPod(tenant, ns, "pod1", t1)
	pod2 := createPod(tenant, ns, "pod2", t2)
	pod3 := createPod(tenant, ns, "pod3", t3)
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2, pod3).expect(t0)

	// Nothing has changed.
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2, pod3).expectNil()
}

func TestPodDeletedThenAdded(t *testing.T) {
	testPodDeletedThenAdded(t, metav1.TenantSystem)
}

func TestPodDeletedThenAddedWithMultiTenancy(t *testing.T) {
	testPodDeletedThenAdded(t, "test-te")
}

func testPodDeletedThenAdded(t *testing.T, tenant string) {
	tester := newTester(t)

	service := createService(tenant, ns, name, t0)
	pod1 := createPod(tenant, ns, "pod1", t1)
	pod2 := createPod(tenant, ns, "pod2", t2)
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2).expect(t0)

	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1).expectNil()

	pod2 = createPod(tenant, ns, "pod2", t4)
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2).expect(t4)
}

func TestServiceDeletedThenAdded(t *testing.T) {
	testServiceDeletedThenAdded(t, metav1.TenantSystem)
}

func TestServiceDeletedThenAddedWithMultiTenancy(t *testing.T) {
	testServiceDeletedThenAdded(t, "test-te")
}

func testServiceDeletedThenAdded(t *testing.T, tenant string) {
	tester := newTester(t)

	service := createService(tenant, ns, name, t0)
	pod1 := createPod(tenant, ns, "pod1", t1)
	pod2 := createPod(tenant, ns, "pod2", t2)
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2).expect(t0)

	tester.DeleteEndpoints(tenant, ns, name)

	service = createService(tenant, ns, name, t3)
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2).expect(t3)
}

func TestServiceUpdated_NoPodChange(t *testing.T) {
	testServiceUpdated_NoPodChange(t, metav1.TenantSystem)
}

func TestServiceUpdated_NoPodChangeWithMultiTenancy(t *testing.T) {
	testServiceUpdated_NoPodChange(t, "test-te")
}

func testServiceUpdated_NoPodChange(t *testing.T, tenant string) {
	tester := newTester(t)

	service := createService(tenant, ns, name, t0)
	pod1 := createPod(tenant, ns, "pod1", t1)
	pod2 := createPod(tenant, ns, "pod2", t2)
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2).expect(t0)

	// service's ports have changed.
	service.Spec = v1.ServiceSpec{
		Selector: map[string]string{},
		Ports:    []v1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080), Protocol: "TCP"}},
	}

	// Currently we're not able to calculate trigger time for service updates, hence the returned
	// value is a nil time.
	tester.whenComputeEndpointsLastChangeTriggerTime(tenant, ns, name, service, pod1, pod2).expectNil()
}

// ------- Test Utils -------

type tester struct {
	*TriggerTimeTracker
	t *testing.T
}

func newTester(t *testing.T) *tester {
	return &tester{NewTriggerTimeTracker(), t}
}

func (t *tester) whenComputeEndpointsLastChangeTriggerTime(
	tenant, namespace, name string, service *v1.Service, pods ...*v1.Pod) subject {
	return subject{t.ComputeEndpointsLastChangeTriggerTime(tenant, namespace, name, service, pods), t.t}
}

type subject struct {
	got time.Time
	t   *testing.T
}

func (s subject) expect(expected time.Time) {
	s.doExpect(expected)
}

func (s subject) expectNil() {
	s.doExpect(time.Time{})
}

func (s subject) doExpect(expected time.Time) {
	if s.got != expected {
		_, fn, line, _ := runtime.Caller(2)
		s.t.Errorf("Wrong trigger time in %s:%d expected %s, got %s", fn, line, expected, s.got)
	}
}

func createPod(tenant, namespace, name string, readyTime time.Time) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Tenant: tenant, Namespace: namespace, Name: name},
		Status: v1.PodStatus{Conditions: []v1.PodCondition{
			{
				Type:               v1.PodReady,
				Status:             v1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(readyTime),
			},
		},
		},
	}
}

func createService(tenant, namespace, name string, creationTime time.Time) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Tenant:            tenant,
			Namespace:         namespace,
			Name:              name,
			CreationTimestamp: metav1.NewTime(creationTime),
		},
	}
}
