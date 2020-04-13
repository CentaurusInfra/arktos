/*
Copyright 2017 The Kubernetes Authors.
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

package reconcilers

/*
Original Source:
https://github.com/openshift/origin/blob/bb340c5dd5ff72718be86fb194dedc0faed7f4c7/pkg/cmd/server/election/lease_endpoint_reconciler_test.go
*/

import (
	"net"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeLeaseEntry struct {
	updated        bool
	serviceGroupId string
	ip             string
}

type fakeLeases struct {
	keys map[string]*fakeLeaseEntry
}

var _ Leases = &fakeLeases{}

func newFakeLeases() *fakeLeases {
	return &fakeLeases{keys: make(map[string]*fakeLeaseEntry)}
}

func (f *fakeLeases) ListLeases() (map[string][]string, error) {
	serviceGroupIdToIps := make(map[string][]string)

	for ip, leaseEntry := range f.keys {
		existedIps, isOK := serviceGroupIdToIps[leaseEntry.serviceGroupId]
		if !isOK {
			serviceGroupIdToIps[leaseEntry.serviceGroupId] = []string{ip}
		} else {
			serviceGroupIdToIps[leaseEntry.serviceGroupId] = append(existedIps, ip)
		}

	}

	return serviceGroupIdToIps, nil
}

func (f *fakeLeases) UpdateLease(ip string, serviceGroupId string) error {
	_, isOK := f.keys[ip]
	if !isOK {
		f.keys[ip] = &fakeLeaseEntry{
			updated:        true,
			serviceGroupId: serviceGroupId,
			ip:             ip,
		}
	} else {
		f.keys[ip].updated = true
	}
	return nil
}

func (f *fakeLeases) RemoveLease(ip string) error {
	delete(f.keys, ip)
	return nil
}

func (f *fakeLeases) SetKeys(keys map[string]string) {
	for ip := range keys {
		f.keys[ip] = &fakeLeaseEntry{
			updated:        false,
			serviceGroupId: keys[ip],
			ip:             ip,
		}
	}
}

func (f *fakeLeases) GetUpdatedKeys() []string {
	res := []string{}
	for ip, entry := range f.keys {
		if entry.updated {
			res = append(res, ip)
		}
	}
	return res
}

func convertIpToMap(endpointsKeys []string, serviceGroupId string) map[string]string {
	m := make(map[string]string)
	for _, ip := range endpointsKeys {
		m[ip] = serviceGroupId
	}

	return m
}

func TestLeaseEndpointReconciler(t *testing.T) {
	ns := corev1.NamespaceDefault
	om := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Tenant: metav1.TenantDefault, Namespace: ns, Name: name}
	}
	reconcileTests := []struct {
		testName      string
		serviceName   string
		ip            string
		endpointPorts []corev1.EndpointPort
		endpointKeys  []string
		endpoints     *corev1.EndpointsList
		expectUpdate  *corev1.Endpoints // nil means none expected
	}{
		{
			testName:      "no existing endpoints",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpoints:     nil,
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
					Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:      "existing endpoints satisfy",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
						Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
		},
		{
			testName:      "existing endpoints satisfy + refresh existing key",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:  []string{"1.2.3.4"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
						Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
		},
		{
			testName:      "existing endpoints satisfy but too many",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}, {IP: "4.3.2.1"}},
						Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
					Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:      "existing endpoints satisfy but too many + extra masters",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:  []string{"1.2.3.4", "4.3.2.2", "4.3.2.3", "4.3.2.4"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{
							{IP: "1.2.3.4"},
							{IP: "4.3.2.1"},
							{IP: "4.3.2.2"},
							{IP: "4.3.2.3"},
							{IP: "4.3.2.4"},
						},
						Ports: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{
						{IP: "1.2.3.4"},
						{IP: "4.3.2.2"},
						{IP: "4.3.2.3"},
						{IP: "4.3.2.4"},
					},
					Ports: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:      "existing endpoints satisfy but too many + extra masters + delete first",
			serviceName:   "foo",
			ip:            "4.3.2.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:  []string{"4.3.2.1", "4.3.2.2", "4.3.2.3", "4.3.2.4"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{
							{IP: "1.2.3.4"},
							{IP: "4.3.2.1"},
							{IP: "4.3.2.2"},
							{IP: "4.3.2.3"},
							{IP: "4.3.2.4"},
						},
						Ports: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{
						{IP: "4.3.2.1"},
						{IP: "4.3.2.2"},
						{IP: "4.3.2.3"},
						{IP: "4.3.2.4"},
					},
					Ports: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:      "existing endpoints current IP missing",
			serviceName:   "foo",
			ip:            "4.3.2.2",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:  []string{"4.3.2.1"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.1"},
						},
						Ports: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{
						{IP: "4.3.2.1"},
						{IP: "4.3.2.2"},
					},
					Ports: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:      "existing endpoints wrong name",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("bar"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
						Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
					Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:      "existing endpoints wrong IP",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "4.3.2.1"}},
						Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
					Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:      "existing endpoints wrong port",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
						Ports:     []corev1.EndpointPort{{Name: "foo", Port: 9090, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
					Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:      "existing endpoints wrong protocol",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
						Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "UDP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
					Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:      "existing endpoints wrong port name",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "baz", Port: 8080, Protocol: "TCP"}},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
						Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
					Ports:     []corev1.EndpointPort{{Name: "baz", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:    "existing endpoints extra service ports satisfy",
			serviceName: "foo",
			ip:          "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{
				{Name: "foo", Port: 8080, Protocol: "TCP"},
				{Name: "bar", Port: 1000, Protocol: "TCP"},
				{Name: "baz", Port: 1010, Protocol: "TCP"},
			},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
						Ports: []corev1.EndpointPort{
							{Name: "foo", Port: 8080, Protocol: "TCP"},
							{Name: "bar", Port: 1000, Protocol: "TCP"},
							{Name: "baz", Port: 1010, Protocol: "TCP"},
						},
					}},
				}},
			},
		},
		{
			testName:    "existing endpoints extra service ports missing port",
			serviceName: "foo",
			ip:          "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{
				{Name: "foo", Port: 8080, Protocol: "TCP"},
				{Name: "bar", Port: 1000, Protocol: "TCP"},
			},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
						Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
					Ports: []corev1.EndpointPort{
						{Name: "foo", Port: 8080, Protocol: "TCP"},
						{Name: "bar", Port: 1000, Protocol: "TCP"},
					},
				}},
			},
		},
	}
	for _, test := range reconcileTests {
		fakeLeases := newFakeLeases()
		fakeLeases.SetKeys(convertIpToMap(test.endpointKeys, ""))
		clientset := fake.NewSimpleClientset()
		if test.endpoints != nil {
			for _, ep := range test.endpoints.Items {
				if _, err := clientset.CoreV1().Endpoints(ep.Namespace).Create(&ep); err != nil {
					t.Errorf("case %q: unexpected error: %v", test.testName, err)
					continue
				}
			}
		}
		r := NewLeaseEndpointReconciler(clientset.CoreV1(), fakeLeases)
		err := r.ReconcileEndpoints(test.serviceName, "", net.ParseIP(test.ip), test.endpointPorts, true)
		if err != nil {
			t.Errorf("case %q: unexpected error: %v", test.testName, err)
		}
		actualEndpoints, err := clientset.CoreV1().Endpoints(corev1.NamespaceDefault).Get(test.serviceName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("case %q: unexpected error: %v", test.testName, err)
		}
		if test.expectUpdate != nil {
			if e, a := test.expectUpdate, actualEndpoints; !reflect.DeepEqual(e, a) {
				t.Errorf("case %q: expected update:\n%#v\ngot:\n%#v\n", test.testName, e, a)
			}
		}
		if updatedKeys := fakeLeases.GetUpdatedKeys(); len(updatedKeys) != 1 || updatedKeys[0] != test.ip {
			t.Errorf("case %q: expected the master's IP to be refreshed, but the following IPs were refreshed instead: %v", test.testName, updatedKeys)
		}
	}
}

func TestLeaseEndpointNonReconcile(t *testing.T) {
	ns := corev1.NamespaceDefault
	om := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Tenant: metav1.TenantDefault, Namespace: ns, Name: name}
	}

	nonReconcileTests := []struct {
		testName      string
		serviceName   string
		ip            string
		endpointPorts []corev1.EndpointPort
		endpointKeys  []string
		endpoints     *corev1.EndpointsList
		expectUpdate  *corev1.Endpoints // nil means none expected
	}{
		{
			testName:    "existing endpoints extra service ports missing port no update",
			serviceName: "foo",
			ip:          "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{
				{Name: "foo", Port: 8080, Protocol: "TCP"},
				{Name: "bar", Port: 1000, Protocol: "TCP"},
			},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
						Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: nil,
		},
		{
			testName:    "existing endpoints extra service ports, wrong ports, wrong IP",
			serviceName: "foo",
			ip:          "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{
				{Name: "foo", Port: 8080, Protocol: "TCP"},
				{Name: "bar", Port: 1000, Protocol: "TCP"},
			},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{{IP: "4.3.2.1"}},
						Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
					Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:      "no existing endpoints",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpoints:     nil,
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
					Ports:     []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
	}
	for _, test := range nonReconcileTests {
		t.Run(test.testName, func(t *testing.T) {
			fakeLeases := newFakeLeases()
			fakeLeases.SetKeys(convertIpToMap(test.endpointKeys, ""))
			clientset := fake.NewSimpleClientset()
			if test.endpoints != nil {
				for _, ep := range test.endpoints.Items {
					if _, err := clientset.CoreV1().Endpoints(ep.Namespace).Create(&ep); err != nil {
						t.Errorf("case %q: unexpected error: %v", test.testName, err)
						continue
					}
				}
			}
			r := NewLeaseEndpointReconciler(clientset.CoreV1(), fakeLeases)
			err := r.ReconcileEndpoints(test.serviceName, "", net.ParseIP(test.ip), test.endpointPorts, false)
			if err != nil {
				t.Errorf("case %q: unexpected error: %v", test.testName, err)
			}
			actualEndpoints, err := clientset.CoreV1().Endpoints(corev1.NamespaceDefault).Get(test.serviceName, metav1.GetOptions{})
			if err != nil {
				t.Errorf("case %q: unexpected error: %v", test.testName, err)
			}
			if test.expectUpdate != nil {
				if e, a := test.expectUpdate, actualEndpoints; !reflect.DeepEqual(e, a) {
					t.Errorf("case %q: expected update:\n%#v\ngot:\n%#v\n", test.testName, e, a)
				}
			}
			if updatedKeys := fakeLeases.GetUpdatedKeys(); len(updatedKeys) != 1 || updatedKeys[0] != test.ip {
				t.Errorf("case %q: expected the master's IP to be refreshed, but the following IPs were refreshed instead: %v", test.testName, updatedKeys)
			}
		})
	}
}

func TestMultipleEndpointSubsetsReconcile(t *testing.T) {
	ns := corev1.NamespaceDefault
	om := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Tenant: metav1.TenantDefault, Namespace: ns, Name: name}
	}

	reconcileTests := []struct {
		testName       string
		serviceName    string
		ip             string // current master ip
		serviceGroupId string // service group id of current master instance
		endpointPorts  []corev1.EndpointPort
		endpointKeys   map[string]string     // all master leases
		endpoints      *corev1.EndpointsList // endpoints in registry
		expectUpdate   *corev1.Endpoints     // nil means none expected
	}{
		{
			testName:       "no existing endpoints",
			serviceName:    "foo",
			ip:             "1.2.3.4",
			serviceGroupId: "1",
			endpointPorts:  []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpoints:      nil,
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses:      []corev1.EndpointAddress{{IP: "1.2.3.4"}},
					Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					ServiceGroupId: "1",
				}},
			},
		},
		{
			testName:       "existing endpoints satisfy",
			serviceName:    "foo",
			ip:             "1.2.3.4",
			serviceGroupId: "2",
			endpointPorts:  []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses:      []corev1.EndpointAddress{{IP: "1.2.3.4"}},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "2",
					}},
				}},
			},
		},
		{
			testName:       "add a new instance to existing service group",
			serviceName:    "bar",
			ip:             "2.1.1.2",
			serviceGroupId: "1",
			endpointPorts:  []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:   map[string]string{"2.1.1.2": "1", "4.3.2.1": "1"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("bar"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.1"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "1",
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("bar"),
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "2.1.1.2"},
							{IP: "4.3.2.1"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "1",
					},
				},
			},
		},
		{
			testName:       "add a new instance for new service group",
			serviceName:    "bar",
			ip:             "2.1.1.2",
			serviceGroupId: "2",
			endpointPorts:  []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:   map[string]string{"2.1.1.2": "2", "4.3.2.1": "1"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("bar"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.1"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "1",
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("bar"),
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.1"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "1",
					},
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "2.1.1.2"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "2",
					},
				},
			},
		},
		{
			testName:       "add a new instance for existing service group",
			serviceName:    "bar",
			ip:             "2.1.1.2",
			serviceGroupId: "2",
			endpointPorts:  []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:   map[string]string{"2.1.1.2": "2", "4.3.2.1": "1", "4.3.2.4": "2"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("bar"),
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "4.3.2.1"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "1",
						},
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "4.3.2.4"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "2",
						},
					},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("bar"),
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.1"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "1",
					},
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "2.1.1.2"},
							{IP: "4.3.2.4"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "2",
					},
				},
			},
		},
		{
			testName:       "remove an instance from service group that has another instance - update from different service group",
			serviceName:    "bar",
			ip:             "4.3.2.1",
			serviceGroupId: "1",
			endpointPorts:  []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:   map[string]string{"4.3.2.1": "1", "4.3.2.4": "2"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("bar"),
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "4.3.2.1"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "1",
						},
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "4.3.2.4"},
								{IP: "2.1.2.4"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "2",
						},
					},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("bar"),
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.1"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "1",
					},
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.4"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "2",
					},
				},
			},
		},
		{
			testName:       "remove an instance from service group that has another instance - update from same service group",
			serviceName:    "bar",
			ip:             "4.3.2.4",
			serviceGroupId: "2",
			endpointPorts:  []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:   map[string]string{"4.3.2.1": "1", "4.3.2.4": "2"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("bar"),
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "4.3.2.1"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "1",
						},
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "4.3.2.4"},
								{IP: "2.1.2.4"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "2",
						},
					},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("bar"),
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.1"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "1",
					},
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.4"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "2",
					},
				},
			},
		},
		{
			testName:       "remove an instance from service group that has no other instance - update from another service group",
			serviceName:    "bar",
			ip:             "4.3.2.1",
			serviceGroupId: "1",
			endpointPorts:  []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:   map[string]string{"4.3.2.1": "1"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("bar"),
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "4.3.2.1"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "1",
						},
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "2.1.2.4"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "2",
						},
					},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("bar"),
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.1"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "1",
					},
				},
			},
		},
		{
			testName:       "remove two instances from service group that has no other instance - update from another service group",
			serviceName:    "bar",
			ip:             "4.3.2.1",
			serviceGroupId: "1",
			endpointPorts:  []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:   map[string]string{"4.3.2.1": "1"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("bar"),
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "4.3.2.1"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "1",
						},
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "2.1.2.4"},
								{IP: "2.1.2.5"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "2",
						},
					},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("bar"),
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.1"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "1",
					},
				},
			},
		},
		{
			testName:       "remove multiple instances from multiple service groups that has no other instance - update from another service group",
			serviceName:    "bar",
			ip:             "4.3.2.1",
			serviceGroupId: "1",
			endpointPorts:  []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:   map[string]string{"4.3.2.1": "1"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("bar"),
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "4.3.2.1"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "1",
						},
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "2.1.2.4"},
								{IP: "2.1.2.5"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "2",
						},
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "3.1.2.4"},
								{IP: "3.1.2.5"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "3",
						},
					},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("bar"),
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.1"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "1",
					},
				},
			},
		},
		{
			testName:       "remove multiple instances from multiple service groups that has no other instance and missing instance from same service group",
			serviceName:    "bar",
			ip:             "4.3.2.1",
			serviceGroupId: "1",
			endpointPorts:  []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:   map[string]string{"4.3.2.1": "1"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("bar"),
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "4.3.2.1"},
								{IP: "4.3.2.2"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "1",
						},
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "2.1.2.4"},
								{IP: "2.1.2.5"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "2",
						},
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "3.1.2.4"},
								{IP: "3.1.2.5"},
							},
							Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
							ServiceGroupId: "3",
						},
					},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("bar"),
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "4.3.2.1"},
						},
						Ports:          []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
						ServiceGroupId: "1",
					},
				},
			},
		},
	}

	for _, test := range reconcileTests {
		fakeLeases := newFakeLeases()
		fakeLeases.SetKeys(test.endpointKeys)
		clientset := fake.NewSimpleClientset()
		if test.endpoints != nil {
			for _, ep := range test.endpoints.Items {
				if _, err := clientset.CoreV1().Endpoints(ep.Namespace).Create(&ep); err != nil {
					t.Errorf("case %q: unexpected error: %v", test.testName, err)
					continue
				}
			}
		}
		r := NewLeaseEndpointReconciler(clientset.CoreV1(), fakeLeases)
		err := r.ReconcileEndpoints(test.serviceName, test.serviceGroupId, net.ParseIP(test.ip), test.endpointPorts, true)
		if err != nil {
			t.Errorf("case %q: unexpected error: %v", test.testName, err)
		}
		actualEndpoints, err := clientset.CoreV1().Endpoints(corev1.NamespaceDefault).Get(test.serviceName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("case %q: unexpected error: %v", test.testName, err)
		}
		if test.expectUpdate != nil {
			if e, a := test.expectUpdate, actualEndpoints; !reflect.DeepEqual(e, a) {
				t.Errorf("case %q: expected update:\n%#v\ngot:\n%#v\n", test.testName, e, a)
			}
		}
		if updatedKeys := fakeLeases.GetUpdatedKeys(); len(updatedKeys) != 1 || updatedKeys[0] != test.ip {
			t.Errorf("case %q: expected the master's IP to be refreshed, but the following IPs were refreshed instead: %v", test.testName, updatedKeys)
		}
	}
}

func TestLeaseRemoveEndpoints(t *testing.T) {
	ns := corev1.NamespaceDefault
	om := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Tenant: metav1.TenantDefault, Namespace: ns, Name: name}
	}
	stopTests := []struct {
		testName      string
		serviceName   string
		ip            string
		endpointPorts []corev1.EndpointPort
		endpointKeys  []string
		endpoints     *corev1.EndpointsList
		expectUpdate  *corev1.Endpoints // nil means none expected
	}{
		{
			testName:      "successful stop reconciling",
			serviceName:   "foo",
			ip:            "1.2.3.4",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:  []string{"1.2.3.4", "4.3.2.2", "4.3.2.3", "4.3.2.4"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{
							{IP: "1.2.3.4"},
							{IP: "4.3.2.2"},
							{IP: "4.3.2.3"},
							{IP: "4.3.2.4"},
						},
						Ports: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
			expectUpdate: &corev1.Endpoints{
				ObjectMeta: om("foo"),
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{
						{IP: "4.3.2.2"},
						{IP: "4.3.2.3"},
						{IP: "4.3.2.4"},
					},
					Ports: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
				}},
			},
		},
		{
			testName:      "stop reconciling with ip not in endpoint ip list",
			serviceName:   "foo",
			ip:            "5.6.7.8",
			endpointPorts: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
			endpointKeys:  []string{"1.2.3.4", "4.3.2.2", "4.3.2.3", "4.3.2.4"},
			endpoints: &corev1.EndpointsList{
				Items: []corev1.Endpoints{{
					ObjectMeta: om("foo"),
					Subsets: []corev1.EndpointSubset{{
						Addresses: []corev1.EndpointAddress{
							{IP: "1.2.3.4"},
							{IP: "4.3.2.2"},
							{IP: "4.3.2.3"},
							{IP: "4.3.2.4"},
						},
						Ports: []corev1.EndpointPort{{Name: "foo", Port: 8080, Protocol: "TCP"}},
					}},
				}},
			},
		},
	}
	for _, test := range stopTests {
		t.Run(test.testName, func(t *testing.T) {
			fakeLeases := newFakeLeases()
			fakeLeases.SetKeys(convertIpToMap(test.endpointKeys, ""))
			clientset := fake.NewSimpleClientset()
			for _, ep := range test.endpoints.Items {
				if _, err := clientset.CoreV1().Endpoints(ep.Namespace).Create(&ep); err != nil {
					t.Errorf("case %q: unexpected error: %v", test.testName, err)
					continue
				}
			}
			r := NewLeaseEndpointReconciler(clientset.CoreV1(), fakeLeases)
			err := r.RemoveEndpoints(test.serviceName, "", net.ParseIP(test.ip), test.endpointPorts)
			if err != nil {
				t.Errorf("case %q: unexpected error: %v", test.testName, err)
			}
			actualEndpoints, err := clientset.CoreV1().Endpoints(corev1.NamespaceDefault).Get(test.serviceName, metav1.GetOptions{})
			if err != nil {
				t.Errorf("case %q: unexpected error: %v", test.testName, err)
			}
			if test.expectUpdate != nil {
				if e, a := test.expectUpdate, actualEndpoints; !reflect.DeepEqual(e, a) {
					t.Errorf("case %q: expected update:\n%#v\ngot:\n%#v\n", test.testName, e, a)
				}
			}
			for _, key := range fakeLeases.GetUpdatedKeys() {
				if key == test.ip {
					t.Errorf("case %q: Found ip %s in leases but shouldn't be there", test.testName, key)
				}
			}
		})
	}
}
