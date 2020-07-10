/*
Copyright 2014 The Kubernetes Authors.
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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	utiltesting "k8s.io/client-go/util/testing"
	"k8s.io/kubernetes/pkg/api/testapi"
	endptspkg "k8s.io/kubernetes/pkg/api/v1/endpoints"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/controller"
)

var alwaysReady = func() bool { return true }
var neverReady = func() bool { return false }
var emptyNodeName string
var triggerTime = time.Date(2018, 01, 01, 0, 0, 0, 0, time.UTC)
var triggerTimeString = triggerTime.Format(time.RFC3339Nano)
var oldTriggerTimeString = triggerTime.Add(-time.Hour).Format(time.RFC3339Nano)

func addPods(store cache.Store, tenant, namespace string, nPods int, nPorts int, nNotReady int) {
	for i := 0; i < nPods+nNotReady; i++ {
		p := &v1.Pod{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Tenant:    tenant,
				Namespace: namespace,
				Name:      fmt.Sprintf("pod%d", i),
				Labels:    map[string]string{"foo": "bar"},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{{Ports: []v1.ContainerPort{}}},
			},
			Status: v1.PodStatus{
				PodIP: fmt.Sprintf("1.2.3.%d", 4+i),
				Conditions: []v1.PodCondition{
					{
						Type:   v1.PodReady,
						Status: v1.ConditionTrue,
					},
				},
			},
		}
		if i >= nPods {
			p.Status.Conditions[0].Status = v1.ConditionFalse
		}
		for j := 0; j < nPorts; j++ {
			p.Spec.Containers[0].Ports = append(p.Spec.Containers[0].Ports,
				v1.ContainerPort{Name: fmt.Sprintf("port%d", i), ContainerPort: int32(8080 + j)})
		}
		store.Add(p)
	}
}

func addNotReadyPodsWithSpecifiedRestartPolicyAndPhase(store cache.Store, tenant, namespace string, nPods int, nPorts int, restartPolicy v1.RestartPolicy, podPhase v1.PodPhase) {
	for i := 0; i < nPods; i++ {
		p := &v1.Pod{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Tenant:    tenant,
				Namespace: namespace,
				Name:      fmt.Sprintf("pod%d", i),
				Labels:    map[string]string{"foo": "bar"},
			},
			Spec: v1.PodSpec{
				RestartPolicy: restartPolicy,
				Containers:    []v1.Container{{Ports: []v1.ContainerPort{}}},
			},
			Status: v1.PodStatus{
				PodIP: fmt.Sprintf("1.2.3.%d", 4+i),
				Phase: podPhase,
				Conditions: []v1.PodCondition{
					{
						Type:   v1.PodReady,
						Status: v1.ConditionFalse,
					},
				},
			},
		}
		for j := 0; j < nPorts; j++ {
			p.Spec.Containers[0].Ports = append(p.Spec.Containers[0].Ports,
				v1.ContainerPort{Name: fmt.Sprintf("port%d", i), ContainerPort: int32(8080 + j)})
		}
		store.Add(p)
	}
}

func makeEndpointsResourcePath(tenant, namespace, name string) string {

	return testapi.Default.ResourcePathWithMultiTenancy("endpoints", tenant, namespace, name)
}

func makeTestServer(t *testing.T, tenant, namespace string) (*httptest.Server, *utiltesting.FakeHandler) {
	fakeEndpointsHandler := utiltesting.FakeHandler{
		StatusCode:   http.StatusOK,
		ResponseBody: runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{}),
	}
	mux := http.NewServeMux()
	mux.Handle(testapi.Default.ResourcePathWithMultiTenancy("endpoints", tenant, namespace, ""), &fakeEndpointsHandler)
	mux.Handle(testapi.Default.ResourcePathWithMultiTenancy("endpoints/", tenant, namespace, ""), &fakeEndpointsHandler)

	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		t.Errorf("unexpected request: %v", req.RequestURI)
		http.Error(res, "", http.StatusNotFound)
	})
	return httptest.NewServer(mux), &fakeEndpointsHandler
}

func makeTenantNamespaceKey(tenant, namespace, name string) string {
	if tenant == "" || tenant == metav1.TenantSystem {
		return fmt.Sprintf("%s/%s", namespace, name)
	}

	return fmt.Sprintf("%s/%s/%s", tenant, namespace, name)
}

type endpointController struct {
	*EndpointController
	podStore       cache.Store
	serviceStore   cache.Store
	endpointsStore cache.Store
}

func newController(url string) *endpointController {
	kubeConfig := &restclient.KubeConfig{Host: url, ContentConfig: restclient.ContentConfig{GroupVersion: &schema.GroupVersion{Group: "", Version: "v1"}}}
	configs := restclient.NewAggregatedConfig(kubeConfig)
	client := clientset.NewForConfigOrDie(configs)
	informerFactory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
	endpoints := NewEndpointController(informerFactory.Core().V1().Pods(), informerFactory.Core().V1().Services(),
		informerFactory.Core().V1().Endpoints(), client)
	endpoints.podsSynced = alwaysReady
	endpoints.servicesSynced = alwaysReady
	endpoints.endpointsSynced = alwaysReady
	return &endpointController{
		endpoints,
		informerFactory.Core().V1().Pods().Informer().GetStore(),
		informerFactory.Core().V1().Services().Informer().GetStore(),
		informerFactory.Core().V1().Endpoints().Informer().GetStore(),
	}
}

func TestSyncEndpointsItemsPreserveNoSelector(t *testing.T) {
	testSyncEndpointsItemsPreserveNoSelector(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsPreserveNoSelectorWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItemsPreserveNoSelector(t, "test-te")
}

func testSyncEndpointsItemsPreserveNoSelector(t *testing.T, tenant string) {
	ns := metav1.NamespaceDefault
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "6.7.8.9", NodeName: &emptyNodeName}},
			Ports:     []v1.EndpointPort{{Port: 1000}},
		}},
	})
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec:       v1.ServiceSpec{Ports: []v1.ServicePort{{Port: 80}}},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))
	endpointsHandler.ValidateRequestCount(t, 0)
}

func TestSyncEndpointsExistingNilSubsets(t *testing.T) {
	testSyncEndpointsExistingNilSubsets(t, metav1.TenantSystem)
}

func TestSyncEndpointsExistingNilSubsetsWithMultiTenancy(t *testing.T) {
	testSyncEndpointsExistingNilSubsets(t, "test-te")
}

func testSyncEndpointsExistingNilSubsets(t *testing.T, tenant string) {
	ns := metav1.NamespaceDefault
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: nil,
	})
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"},
			Ports:    []v1.ServicePort{{Port: 80}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))
	endpointsHandler.ValidateRequestCount(t, 0)
}

func TestSyncEndpointsExistingEmptySubsets(t *testing.T) {
	testSyncEndpointsExistingEmptySubsets(t, metav1.TenantSystem)
}

func TestSyncEndpointsExistingEmptySubsetsWithMultiTenancy(t *testing.T) {
	testSyncEndpointsExistingEmptySubsets(t, "test-te")
}

func testSyncEndpointsExistingEmptySubsets(t *testing.T, tenant string) {
	ns := metav1.NamespaceDefault
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{},
	})
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"},
			Ports:    []v1.ServicePort{{Port: 80}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))
	endpointsHandler.ValidateRequestCount(t, 0)
}

func TestSyncEndpointsNewNoSubsets(t *testing.T) {
	testSyncEndpointsNewNoSubsets(t, metav1.TenantSystem)
}

func TestSyncEndpointsNewNoSubsetsWithMultiTenancy(t *testing.T) {
	testSyncEndpointsNewNoSubsets(t, "test-te")
}

func testSyncEndpointsNewNoSubsets(t *testing.T, tenant string) {
	ns := metav1.NamespaceDefault
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"},
			Ports:    []v1.ServicePort{{Port: 80}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))
	endpointsHandler.ValidateRequestCount(t, 1)
}

func TestCheckLeftoverEndpoints(t *testing.T) {
	testCheckLeftoverEndpoints(t, metav1.TenantSystem)
}

func TestCheckLeftoverEndpointsWithMultiTenancy(t *testing.T) {
	testCheckLeftoverEndpoints(t, "test-te")
}

func testCheckLeftoverEndpoints(t *testing.T, tenant string) {
	ns := metav1.NamespaceDefault
	testServer, _ := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "6.7.8.9", NodeName: &emptyNodeName}},
			Ports:     []v1.EndpointPort{{Port: 1000}},
		}},
	})
	endpoints.checkLeftoverEndpoints()
	if e, a := 1, endpoints.queue.Len(); e != a {
		t.Fatalf("Expected %v, got %v", e, a)
	}
	got, _ := endpoints.queue.Get()

	if e, a := makeTenantNamespaceKey(tenant, ns, "foo"), got; e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestSyncEndpointsProtocolTCP(t *testing.T) {
	testSyncEndpointsProtocolTCP(t, metav1.TenantSystem)
}

func TestSyncEndpointsProtocolTCPWithMultiTenancy(t *testing.T) {
	testSyncEndpointsProtocolTCP(t, "test-te")
}

func testSyncEndpointsProtocolTCP(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "6.7.8.9", NodeName: &emptyNodeName}},
			Ports:     []v1.EndpointPort{{Port: 1000, Protocol: "TCP"}},
		}},
	})
	addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
			Ports:    []v1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080), Protocol: "TCP"}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	endpointsHandler.ValidateRequestCount(t, 1)
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     []v1.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestSyncEndpointsProtocolUDP(t *testing.T) {
	testSyncEndpointsProtocolUDP(t, metav1.TenantSystem)
}

func TestSyncEndpointsProtocolUDPWithMultiTenancy(t *testing.T) {
	testSyncEndpointsProtocolUDP(t, "test-te")
}

func testSyncEndpointsProtocolUDP(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "6.7.8.9", NodeName: &emptyNodeName}},
			Ports:     []v1.EndpointPort{{Port: 1000, Protocol: "UDP"}},
		}},
	})
	addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
			Ports:    []v1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080), Protocol: "UDP"}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	endpointsHandler.ValidateRequestCount(t, 1)
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     []v1.EndpointPort{{Port: 8080, Protocol: "UDP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestSyncEndpointsProtocolSCTP(t *testing.T) {
	testSyncEndpointsProtocolSCTP(t, metav1.TenantSystem)
}

func TestSyncEndpointsProtocolSCTPWithMultiTenancy(t *testing.T) {
	testSyncEndpointsProtocolSCTP(t, "test-te")
}

func testSyncEndpointsProtocolSCTP(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "6.7.8.9", NodeName: &emptyNodeName}},
			Ports:     []v1.EndpointPort{{Port: 1000, Protocol: "SCTP"}},
		}},
	})
	addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
			Ports:    []v1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080), Protocol: "SCTP"}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	endpointsHandler.ValidateRequestCount(t, 1)
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     []v1.EndpointPort{{Port: 8080, Protocol: "SCTP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestSyncEndpointsItemsEmptySelectorSelectsAll(t *testing.T) {
	testSyncEndpointsItemsEmptySelectorSelectsAll(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsEmptySelectorSelectsAllWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItemsEmptySelectorSelectsAll(t, "test-te")
}

func testSyncEndpointsItemsEmptySelectorSelectsAll(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{},
	})
	addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
			Ports:    []v1.ServicePort{{Port: 80, Protocol: "TCP", TargetPort: intstr.FromInt(8080)}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     []v1.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestSyncEndpointsItemsEmptySelectorSelectsAllNotReady(t *testing.T) {
	testSyncEndpointsItemsEmptySelectorSelectsAllNotReady(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsEmptySelectorSelectsAllNotReadyWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItemsEmptySelectorSelectsAllNotReady(t, "test-te")
}

func testSyncEndpointsItemsEmptySelectorSelectsAllNotReady(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{},
	})
	addPods(endpoints.podStore, tenant, ns, 0, 1, 1)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
			Ports:    []v1.ServicePort{{Port: 80, Protocol: "TCP", TargetPort: intstr.FromInt(8080)}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			NotReadyAddresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:             []v1.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestSyncEndpointsItemsEmptySelectorSelectsAllMixed(t *testing.T) {
	testSyncEndpointsItemsEmptySelectorSelectsAllMixed(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsEmptySelectorSelectsAllMixedWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItemsEmptySelectorSelectsAllMixed(t, "test-te")
}

func testSyncEndpointsItemsEmptySelectorSelectsAllMixed(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{},
	})
	addPods(endpoints.podStore, tenant, ns, 1, 1, 1)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
			Ports:    []v1.ServicePort{{Port: 80, Protocol: "TCP", TargetPort: intstr.FromInt(8080)}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses:         []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			NotReadyAddresses: []v1.EndpointAddress{{IP: "1.2.3.5", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod1", Namespace: ns, Tenant: tenant}}},
			Ports:             []v1.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestSyncEndpointsItemsPreexisting(t *testing.T) {
	testSyncEndpointsItemsPreexisting(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsPreexistingWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItemsPreexisting(t, "test-te")
}

func testSyncEndpointsItemsPreexisting(t *testing.T, tenant string) {
	ns := "bar"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "6.7.8.9", NodeName: &emptyNodeName}},
			Ports:     []v1.EndpointPort{{Port: 1000}},
		}},
	})
	addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"},
			Ports:    []v1.ServicePort{{Port: 80, Protocol: "TCP", TargetPort: intstr.FromInt(8080)}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     []v1.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestSyncEndpointsItemsPreexistingIdentical(t *testing.T) {
	testSyncEndpointsItemsPreexistingIdentical(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsPreexistingIdenticalWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItemsPreexistingIdentical(t, "test-te")
}

func testSyncEndpointsItemsPreexistingIdentical(t *testing.T, tenant string) {
	ns := metav1.NamespaceDefault
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "1",
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     []v1.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	addPods(endpoints.podStore, tenant, metav1.NamespaceDefault, 1, 1, 0)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: metav1.NamespaceDefault, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"},
			Ports:    []v1.ServicePort{{Port: 80, Protocol: "TCP", TargetPort: intstr.FromInt(8080)}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))
	endpointsHandler.ValidateRequestCount(t, 0)
}

func TestSyncEndpointsItems(t *testing.T) {
	testSyncEndpointsItems(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItems(t, "test-te")
}

func testSyncEndpointsItems(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	addPods(endpoints.podStore, tenant, ns, 3, 2, 0)
	addPods(endpoints.podStore, tenant, "blah", 5, 2, 0) // make sure these aren't found!
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"},
			Ports: []v1.ServicePort{
				{Name: "port0", Port: 80, Protocol: "TCP", TargetPort: intstr.FromInt(8080)},
				{Name: "port1", Port: 88, Protocol: "TCP", TargetPort: intstr.FromInt(8088)},
			},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, "other", "foo"))

	expectedSubsets := []v1.EndpointSubset{{
		Addresses: []v1.EndpointAddress{
			{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}},
			{IP: "1.2.3.5", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod1", Namespace: ns, Tenant: tenant}},
			{IP: "1.2.3.6", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod2", Namespace: ns, Tenant: tenant}},
		},
		Ports: []v1.EndpointPort{
			{Name: "port0", Port: 8080, Protocol: "TCP"},
			{Name: "port1", Port: 8088, Protocol: "TCP"},
		},
	}}
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "",
			Name:            "foo",
		},
		Subsets: endptspkg.SortSubsets(expectedSubsets),
	})
	endpointsHandler.ValidateRequestCount(t, 1)
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, ""), "POST", &data)
}

func TestSyncEndpointsItemsWithLabels(t *testing.T) {
	testSyncEndpointsItemsWithLabels(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsWithLabelsWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItemsWithLabels(t, "test-te")
}

func testSyncEndpointsItemsWithLabels(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	addPods(endpoints.podStore, tenant, ns, 3, 2, 0)
	serviceLabels := map[string]string{"foo": "bar"}
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Tenant:    tenant,
			Namespace: ns,
			Labels:    serviceLabels,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"},
			Ports: []v1.ServicePort{
				{Name: "port0", Port: 80, Protocol: "TCP", TargetPort: intstr.FromInt(8080)},
				{Name: "port1", Port: 88, Protocol: "TCP", TargetPort: intstr.FromInt(8088)},
			},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	expectedSubsets := []v1.EndpointSubset{{
		Addresses: []v1.EndpointAddress{
			{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}},
			{IP: "1.2.3.5", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod1", Namespace: ns, Tenant: tenant}},
			{IP: "1.2.3.6", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod2", Namespace: ns, Tenant: tenant}},
		},
		Ports: []v1.EndpointPort{
			{Name: "port0", Port: 8080, Protocol: "TCP"},
			{Name: "port1", Port: 8088, Protocol: "TCP"},
		},
	}}
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "",
			Name:            "foo",
			Labels:          serviceLabels,
		},
		Subsets: endptspkg.SortSubsets(expectedSubsets),
	})
	endpointsHandler.ValidateRequestCount(t, 1)
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, ""), "POST", &data)
}

func TestSyncEndpointsItemsPreexistingLabelsChange(t *testing.T) {
	testSyncEndpointsItemsPreexistingLabelsChange(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsPreexistingLabelsChangeWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItemsPreexistingLabelsChange(t, "test-te")
}

func testSyncEndpointsItemsPreexistingLabelsChange(t *testing.T, tenant string) {
	ns := "bar"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "6.7.8.9", NodeName: &emptyNodeName}},
			Ports:     []v1.EndpointPort{{Port: 1000}},
		}},
	})
	addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
	serviceLabels := map[string]string{"baz": "blah"}
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Tenant:    tenant,
			Namespace: ns,
			Labels:    serviceLabels,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"},
			Ports:    []v1.ServicePort{{Port: 80, Protocol: "TCP", TargetPort: intstr.FromInt(8080)}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
			Labels:          serviceLabels,
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     []v1.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestWaitsForAllInformersToBeSynced2(t *testing.T) {
	testWaitsForAllInformersToBeSynced2(t, metav1.TenantSystem)
}

func TestWaitsForAllInformersToBeSynced2WithMultiTenancy(t *testing.T) {
	testWaitsForAllInformersToBeSynced2(t, "test-te")
}

func testWaitsForAllInformersToBeSynced2(t *testing.T, tenant string) {
	var tests = []struct {
		podsSynced            func() bool
		servicesSynced        func() bool
		endpointsSynced       func() bool
		shouldUpdateEndpoints bool
	}{
		{neverReady, alwaysReady, alwaysReady, false},
		{alwaysReady, neverReady, alwaysReady, false},
		{alwaysReady, alwaysReady, neverReady, false},
		{alwaysReady, alwaysReady, alwaysReady, true},
	}

	for _, test := range tests {
		func() {
			ns := "other"
			testServer, endpointsHandler := makeTestServer(t, tenant, ns)
			defer testServer.Close()
			endpoints := newController(testServer.URL)
			addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
			service := &v1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{},
					Ports:    []v1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080), Protocol: "TCP"}},
				},
			}
			endpoints.serviceStore.Add(service)
			endpoints.enqueueService(service)
			endpoints.podsSynced = test.podsSynced
			endpoints.servicesSynced = test.servicesSynced
			endpoints.endpointsSynced = test.endpointsSynced
			endpoints.workerLoopPeriod = 10 * time.Millisecond
			stopCh := make(chan struct{})
			defer close(stopCh)
			go endpoints.Run(1, stopCh)

			// cache.WaitForCacheSync has a 100ms poll period, and the endpoints worker has a 10ms period.
			// To ensure we get all updates, including unexpected ones, we need to wait at least as long as
			// a single cache sync period and worker period, with some fudge room.
			time.Sleep(150 * time.Millisecond)
			if test.shouldUpdateEndpoints {
				// Ensure the work queue has been processed by looping for up to a second to prevent flakes.
				wait.PollImmediate(50*time.Millisecond, 1*time.Second, func() (bool, error) {
					return endpoints.queue.Len() == 0, nil
				})
				endpointsHandler.ValidateRequestCount(t, 1)
			} else {
				endpointsHandler.ValidateRequestCount(t, 0)
			}
		}()
	}
}

func TestSyncEndpointsHeadlessService(t *testing.T) {
	testSyncEndpointsHeadlessService(t, metav1.TenantSystem)
}

func TestSyncEndpointsHeadlessServiceWithMultiTenancy(t *testing.T) {
	testSyncEndpointsHeadlessService(t, "test-te")
}

func testSyncEndpointsHeadlessService(t *testing.T, tenant string) {
	ns := "headless"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "6.7.8.9", NodeName: &emptyNodeName}},
			Ports:     []v1.EndpointPort{{Port: 1000, Protocol: "TCP"}},
		}},
	})
	addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector:  map[string]string{},
			ClusterIP: api.ClusterIPNone,
			Ports:     []v1.ServicePort{},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     []v1.EndpointPort{},
		}},
	})
	endpointsHandler.ValidateRequestCount(t, 1)
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyNeverAndPhaseFailed(t *testing.T) {
	testSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyNeverAndPhaseFailed(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyNeverAndPhaseFailedWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyNeverAndPhaseFailed(t, "test-te")
}

func testSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyNeverAndPhaseFailed(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Subsets: []v1.EndpointSubset{},
	})
	addNotReadyPodsWithSpecifiedRestartPolicyAndPhase(endpoints.podStore, tenant, ns, 1, 1, v1.RestartPolicyNever, v1.PodFailed)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"},
			Ports:    []v1.ServicePort{{Port: 80, Protocol: "TCP", TargetPort: intstr.FromInt(8080)}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyNeverAndPhaseSucceeded(t *testing.T) {
	testSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyNeverAndPhaseSucceeded(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyNeverAndPhaseSucceededWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyNeverAndPhaseSucceeded(t, "test-te")
}

func testSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyNeverAndPhaseSucceeded(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Subsets: []v1.EndpointSubset{},
	})
	addNotReadyPodsWithSpecifiedRestartPolicyAndPhase(endpoints.podStore, tenant, ns, 1, 1, v1.RestartPolicyNever, v1.PodSucceeded)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"},
			Ports:    []v1.ServicePort{{Port: 80, Protocol: "TCP", TargetPort: intstr.FromInt(8080)}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyOnFailureAndPhaseSucceeded(t *testing.T) {
	testSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyOnFailureAndPhaseSucceeded(t, metav1.TenantSystem)
}

func TestSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyOnFailureAndPhaseSucceededWithMultiTenancy(t *testing.T) {
	testSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyOnFailureAndPhaseSucceeded(t, "test-te")
}

func testSyncEndpointsItemsExcludeNotReadyPodsWithRestartPolicyOnFailureAndPhaseSucceeded(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Subsets: []v1.EndpointSubset{},
	})
	addNotReadyPodsWithSpecifiedRestartPolicyAndPhase(endpoints.podStore, tenant, ns, 1, 1, v1.RestartPolicyOnFailure, v1.PodSucceeded)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"},
			Ports:    []v1.ServicePort{{Port: 80, Protocol: "TCP", TargetPort: intstr.FromInt(8080)}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestSyncEndpointsHeadlessWithoutPort(t *testing.T) {
	testSyncEndpointsHeadlessWithoutPort(t, metav1.TenantSystem)
}

func TestSyncEndpointsHeadlessWithoutPortWithMultiTenancy(t *testing.T) {
	testSyncEndpointsHeadlessWithoutPort(t, "test-te")
}

func testSyncEndpointsHeadlessWithoutPort(t *testing.T, tenant string) {
	ns := metav1.NamespaceDefault
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector:  map[string]string{"foo": "bar"},
			ClusterIP: "None",
			Ports:     nil,
		},
	})
	addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))
	endpointsHandler.ValidateRequestCount(t, 1)
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     nil,
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, ""), "POST", &data)
}

// There are 3*5 possibilities(3 types of RestartPolicy by 5 types of PodPhase). Not list them all here.
// Just list all of the 3 false cases and 3 of the 12 true cases.
func TestShouldPodBeInEndpoints(t *testing.T) {
	testShouldPodBeInEndpoints(t, metav1.TenantSystem)
}

func TestShouldPodBeInEndpointsWithMultiTenancy(t *testing.T) {
	testShouldPodBeInEndpoints(t, "test-te")
}

func testShouldPodBeInEndpoints(t *testing.T, tenant string) {
	testCases := []struct {
		name     string
		pod      *v1.Pod
		expected bool
	}{
		// Pod should not be in endpoints cases:
		{
			name: "Failed pod with Never RestartPolicy",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
				},
				Status: v1.PodStatus{
					Phase: v1.PodFailed,
				},
			},
			expected: false,
		},
		{
			name: "Succeeded pod with Never RestartPolicy",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
				},
				Status: v1.PodStatus{
					Phase: v1.PodSucceeded,
				},
			},
			expected: false,
		},
		{
			name: "Succeeded pod with OnFailure RestartPolicy",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyOnFailure,
				},
				Status: v1.PodStatus{
					Phase: v1.PodSucceeded,
				},
			},
			expected: false,
		},
		// Pod should be in endpoints cases:
		{
			name: "Failed pod with Always RestartPolicy",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyAlways,
				},
				Status: v1.PodStatus{
					Phase: v1.PodFailed,
				},
			},
			expected: true,
		},
		{
			name: "Pending pod with Never RestartPolicy",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
				},
			},
			expected: true,
		},
		{
			name: "Unknown pod with OnFailure RestartPolicy",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyOnFailure,
				},
				Status: v1.PodStatus{
					Phase: v1.PodUnknown,
				},
			},
			expected: true,
		},
	}
	for _, test := range testCases {
		result := shouldPodBeInEndpoints(test.pod)
		if result != test.expected {
			t.Errorf("%s: expected : %t, got: %t", test.name, test.expected, result)
		}
	}
}

func TestPodToEndpointAddress(t *testing.T) {
	testPodToEndpointAddress(t, metav1.TenantSystem)
}

func TestPodToEndpointAddressWithMultiTenancy(t *testing.T) {
	testPodToEndpointAddress(t, "test-te")
}

func testPodToEndpointAddress(t *testing.T, tenant string) {
	podStore := cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc)
	ns := "test"
	addPods(podStore, tenant, ns, 1, 1, 0)
	pods := podStore.List()
	if len(pods) != 1 {
		t.Errorf("podStore size: expected: %d, got: %d", 1, len(pods))
		return
	}
	pod := pods[0].(*v1.Pod)
	epa := podToEndpointAddress(pod)
	if epa.IP != pod.Status.PodIP {
		t.Errorf("IP: expected: %s, got: %s", pod.Status.PodIP, epa.IP)
	}
	if *(epa.NodeName) != pod.Spec.NodeName {
		t.Errorf("NodeName: expected: %s, got: %s", pod.Spec.NodeName, *(epa.NodeName))
	}
	if epa.TargetRef.Kind != "Pod" {
		t.Errorf("TargetRef.Kind: expected: %s, got: %s", "Pod", epa.TargetRef.Kind)
	}
	if epa.TargetRef.Namespace != pod.ObjectMeta.Namespace {
		t.Errorf("TargetRef.Kind: expected: %s, got: %s", pod.ObjectMeta.Namespace, epa.TargetRef.Namespace)
	}
	if epa.TargetRef.Name != pod.ObjectMeta.Name {
		t.Errorf("TargetRef.Kind: expected: %s, got: %s", pod.ObjectMeta.Name, epa.TargetRef.Name)
	}
	if epa.TargetRef.UID != pod.ObjectMeta.UID {
		t.Errorf("TargetRef.Kind: expected: %s, got: %s", pod.ObjectMeta.UID, epa.TargetRef.UID)
	}
	if epa.TargetRef.ResourceVersion != pod.ObjectMeta.ResourceVersion {
		t.Errorf("TargetRef.Kind: expected: %s, got: %s", pod.ObjectMeta.ResourceVersion, epa.TargetRef.ResourceVersion)
	}
}

func TestPodChanged(t *testing.T) {
	testPodChanged(t, metav1.TenantSystem)
}

func TestPodChangedWithMultiTenancy(t *testing.T) {
	testPodChanged(t, "test-te")
}

func testPodChanged(t *testing.T, tenant string) {
	podStore := cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc)
	ns := "test"
	addPods(podStore, tenant, ns, 1, 1, 0)
	pods := podStore.List()
	if len(pods) != 1 {
		t.Errorf("podStore size: expected: %d, got: %d", 1, len(pods))
		return
	}
	oldPod := pods[0].(*v1.Pod)
	newPod := oldPod.DeepCopy()

	if podChanged(oldPod, newPod) {
		t.Errorf("Expected pod to be unchanged for copied pod")
	}

	newPod.Spec.NodeName = "changed"
	if !podChanged(oldPod, newPod) {
		t.Errorf("Expected pod to be changed for pod with NodeName changed")
	}
	newPod.Spec.NodeName = oldPod.Spec.NodeName

	newPod.ObjectMeta.ResourceVersion = "changed"
	if podChanged(oldPod, newPod) {
		t.Errorf("Expected pod to be unchanged for pod with only ResourceVersion changed")
	}
	newPod.ObjectMeta.ResourceVersion = oldPod.ObjectMeta.ResourceVersion

	newPod.Status.PodIP = "1.2.3.1"
	if !podChanged(oldPod, newPod) {
		t.Errorf("Expected pod to be changed with pod IP address change")
	}
	newPod.Status.PodIP = oldPod.Status.PodIP

	newPod.ObjectMeta.Name = "wrong-name"
	if !podChanged(oldPod, newPod) {
		t.Errorf("Expected pod to be changed with pod name change")
	}
	newPod.ObjectMeta.Name = oldPod.ObjectMeta.Name

	saveConditions := oldPod.Status.Conditions
	oldPod.Status.Conditions = nil
	if !podChanged(oldPod, newPod) {
		t.Errorf("Expected pod to be changed with pod readiness change")
	}
	oldPod.Status.Conditions = saveConditions

	now := metav1.NewTime(time.Now().UTC())
	newPod.ObjectMeta.DeletionTimestamp = &now
	if !podChanged(oldPod, newPod) {
		t.Errorf("Expected pod to be changed with DeletionTimestamp change")
	}
	newPod.ObjectMeta.DeletionTimestamp = oldPod.ObjectMeta.DeletionTimestamp.DeepCopy()
}

func TestDetermineNeededServiceUpdates(t *testing.T) {
	testCases := []struct {
		name  string
		a     sets.String
		b     sets.String
		union sets.String
		xor   sets.String
	}{
		{
			name:  "no services changed",
			a:     sets.NewString("a", "b", "c"),
			b:     sets.NewString("a", "b", "c"),
			xor:   sets.NewString(),
			union: sets.NewString("a", "b", "c"),
		},
		{
			name:  "all old services removed, new services added",
			a:     sets.NewString("a", "b", "c"),
			b:     sets.NewString("d", "e", "f"),
			xor:   sets.NewString("a", "b", "c", "d", "e", "f"),
			union: sets.NewString("a", "b", "c", "d", "e", "f"),
		},
		{
			name:  "all old services removed, no new services added",
			a:     sets.NewString("a", "b", "c"),
			b:     sets.NewString(),
			xor:   sets.NewString("a", "b", "c"),
			union: sets.NewString("a", "b", "c"),
		},
		{
			name:  "no old services, but new services added",
			a:     sets.NewString(),
			b:     sets.NewString("a", "b", "c"),
			xor:   sets.NewString("a", "b", "c"),
			union: sets.NewString("a", "b", "c"),
		},
		{
			name:  "one service removed, one service added, two unchanged",
			a:     sets.NewString("a", "b", "c"),
			b:     sets.NewString("b", "c", "d"),
			xor:   sets.NewString("a", "d"),
			union: sets.NewString("a", "b", "c", "d"),
		},
		{
			name:  "no services",
			a:     sets.NewString(),
			b:     sets.NewString(),
			xor:   sets.NewString(),
			union: sets.NewString(),
		},
	}
	for _, testCase := range testCases {
		retval := determineNeededServiceUpdates(testCase.a, testCase.b, false)
		if !retval.Equal(testCase.xor) {
			t.Errorf("%s (with podChanged=false): expected: %v  got: %v", testCase.name, testCase.xor.List(), retval.List())
		}

		retval = determineNeededServiceUpdates(testCase.a, testCase.b, true)
		if !retval.Equal(testCase.union) {
			t.Errorf("%s (with podChanged=true): expected: %v  got: %v", testCase.name, testCase.union.List(), retval.List())
		}
	}
}

func TestLastTriggerChangeTimeAnnotation(t *testing.T) {
	testLastTriggerChangeTimeAnnotation(t, metav1.TenantSystem)
}

func TestLastTriggerChangeTimeAnnotationWithMultiTenancy(t *testing.T) {
	testLastTriggerChangeTimeAnnotation(t, "test-te")
}

func testLastTriggerChangeTimeAnnotation(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "6.7.8.9", NodeName: &emptyNodeName}},
			Ports:     []v1.EndpointPort{{Port: 1000, Protocol: "TCP"}},
		}},
	})
	addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant, CreationTimestamp: metav1.NewTime(triggerTime)},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
			Ports:    []v1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080), Protocol: "TCP"}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	endpointsHandler.ValidateRequestCount(t, 1)
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
			Annotations: map[string]string{
				v1.EndpointsLastChangeTriggerTime: triggerTimeString,
			},
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     []v1.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestLastTriggerChangeTimeAnnotation_AnnotationOverridden(t *testing.T) {
	testLastTriggerChangeTimeAnnotation_AnnotationOverridden(t, metav1.TenantSystem)
}

func TestLastTriggerChangeTimeAnnotation_AnnotationOverriddenWithMultiTenancy(t *testing.T) {
	testLastTriggerChangeTimeAnnotation_AnnotationOverridden(t, "test-te")
}

func testLastTriggerChangeTimeAnnotation_AnnotationOverridden(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
			Annotations: map[string]string{
				v1.EndpointsLastChangeTriggerTime: oldTriggerTimeString,
			},
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "6.7.8.9", NodeName: &emptyNodeName}},
			Ports:     []v1.EndpointPort{{Port: 1000, Protocol: "TCP"}},
		}},
	})
	addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant, CreationTimestamp: metav1.NewTime(triggerTime)},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
			Ports:    []v1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080), Protocol: "TCP"}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	endpointsHandler.ValidateRequestCount(t, 1)
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
			Annotations: map[string]string{
				v1.EndpointsLastChangeTriggerTime: triggerTimeString,
			},
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     []v1.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}

func TestLastTriggerChangeTimeAnnotation_AnnotationCleared(t *testing.T) {
	testLastTriggerChangeTimeAnnotation_AnnotationCleared(t, metav1.TenantSystem)
}

func TestLastTriggerChangeTimeAnnotation_AnnotationClearedWithMultiTenancy(t *testing.T) {
	testLastTriggerChangeTimeAnnotation_AnnotationCleared(t, "test-te")
}

func testLastTriggerChangeTimeAnnotation_AnnotationCleared(t *testing.T, tenant string) {
	ns := "other"
	testServer, endpointsHandler := makeTestServer(t, tenant, ns)
	defer testServer.Close()
	endpoints := newController(testServer.URL)
	endpoints.endpointsStore.Add(&v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
			Annotations: map[string]string{
				v1.EndpointsLastChangeTriggerTime: triggerTimeString,
			},
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "6.7.8.9", NodeName: &emptyNodeName}},
			Ports:     []v1.EndpointPort{{Port: 1000, Protocol: "TCP"}},
		}},
	})
	// Neither pod nor service has trigger time, this should cause annotation to be cleared.
	addPods(endpoints.podStore, tenant, ns, 1, 1, 0)
	endpoints.serviceStore.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: ns, Tenant: tenant},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
			Ports:    []v1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080), Protocol: "TCP"}},
		},
	})
	endpoints.syncService(makeTenantNamespaceKey(tenant, ns, "foo"))

	endpointsHandler.ValidateRequestCount(t, 1)
	data := runtime.EncodeOrDie(testapi.Default.Codec(), &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Tenant:          tenant,
			Namespace:       ns,
			ResourceVersion: "1",
			Annotations:     map[string]string{}, // Annotation not set anymore.
		},
		Subsets: []v1.EndpointSubset{{
			Addresses: []v1.EndpointAddress{{IP: "1.2.3.4", NodeName: &emptyNodeName, TargetRef: &v1.ObjectReference{Kind: "Pod", Name: "pod0", Namespace: ns, Tenant: tenant}}},
			Ports:     []v1.EndpointPort{{Port: 8080, Protocol: "TCP"}},
		}},
	})
	endpointsHandler.ValidateRequest(t, makeEndpointsResourcePath(tenant, ns, "foo"), "PUT", &data)
}
