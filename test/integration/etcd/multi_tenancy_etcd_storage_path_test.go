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

package etcd

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/cmd/kube-apiserver/app/options"
)

// TestEtcdStoragePath tests to make sure that all objects are stored in an expected location in etcd.
// It will start failing when a new type is added to ensure that all future types are added to this test.
// It will also fail when a type gets moved to a different location. Be very careful in this situation because
// it essentially means that you will be break old clusters unless you create some migration path for the old data.
func TestEtcdStoragePathWithMultiTenancy(t *testing.T) {
	master := StartRealMasterOrDie(t, func(opts *options.ServerRunOptions) {
		// force enable all resources so we can check storage.
		// TODO: drop these once we stop allowing them to be served.
		opts.APIEnablement.RuntimeConfig["extensions/v1beta1/deployments"] = "true"
		opts.APIEnablement.RuntimeConfig["extensions/v1beta1/daemonsets"] = "true"
		opts.APIEnablement.RuntimeConfig["extensions/v1beta1/replicasets"] = "true"
		opts.APIEnablement.RuntimeConfig["extensions/v1beta1/podsecuritypolicies"] = "true"
		opts.APIEnablement.RuntimeConfig["extensions/v1beta1/networkpolicies"] = "true"
	})
	defer master.Cleanup()
	defer dumpEtcdKVOnFailure(t, master.KV)

	client := &allClient{dynamicClient: master.Dynamic}

	if _, err := master.Client.CoreV1().NamespacesWithMultiTenancy(testTenant).Create(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace, Tenant: testTenant}}); err != nil {
		t.Fatal(err)
	}

	etcdStorageData := GetEtcdStorageDataWithMultiTenancy()
	// daemonset is not allowed in user tenant; exclude from test expectations
	// todo: separate test data for system tenant case and for user tenant case
	delete(etcdStorageData, gvr("extensions", "v1beta1", "daemonsets"))
	delete(etcdStorageData, gvr("apps", "v1", "daemonsets"))
	delete(etcdStorageData, gvr("apps", "v1beta2", "daemonsets"))

	kindSeen := sets.NewString()
	pathSeen := map[string][]schema.GroupVersionResource{}
	etcdSeen := map[schema.GroupVersionResource]empty{}
	cohabitatingResources := map[string]map[schema.GroupVersionKind]empty{}

	for _, resourceToPersist := range master.Resources {
		t.Run(resourceToPersist.Mapping.Resource.String(), func(t *testing.T) {
			mapping := resourceToPersist.Mapping
			gvk := resourceToPersist.Mapping.GroupVersionKind
			gvResource := resourceToPersist.Mapping.Resource
			kind := gvk.Kind

			if kindWhiteList.Has(kind) {
				kindSeen.Insert(kind)
				t.Skip("whitelisted")
			}

			if kind == "DaemonSet" {
				t.Skip("daemonset in user tenant is known not allowed")
			}

			etcdSeen[gvResource] = empty{}
			testData, hasTest := etcdStorageData[gvResource]

			if !hasTest {
				t.Fatalf("no test data for %s.  Please add a test for your new type to GetEtcdStorageDataForNamespace() and GetEtcdStorageDataForNamespaceWithMultiTenancy().", gvResource)
			}

			if len(testData.ExpectedEtcdPath) == 0 {
				t.Fatalf("empty test data for %s", gvResource)
			}

			shouldCreate := len(testData.Stub) != 0 // try to create only if we have a stub

			var (
				input *metaObject
				err   error
			)
			if shouldCreate {
				if input, err = jsonToMetaObject([]byte(testData.Stub)); err != nil || input.isEmpty() {
					t.Fatalf("invalid test data for %s: %v", gvResource, err)
				}
				// unset type meta fields - we only set these in the CRD test data and it makes
				// any CRD test with an expectedGVK override fail the DeepDerivative test
				input.Kind = ""
				input.APIVersion = ""
			}

			all := &[]cleanupData{}
			defer func() {
				if !t.Failed() { // do not cleanup if test has already failed since we may need things in the etcd dump
					if err := client.cleanupWithMultiTenancy(all); err != nil {
						t.Fatalf("failed to clean up etcd: %#v", err)
					}
				}
			}()

			if err := client.createPrerequisitesWithMultiTenancy(master.Mapper, testTenant, testNamespace, testData.Prerequisites, all); err != nil {
				t.Fatalf("failed to create prerequisites for %s: %#v", gvResource, err)
			}

			if shouldCreate { // do not try to create items with no stub
				if err := client.createWithMultiTenancy(testData.Stub, testTenant, testNamespace, mapping, all); err != nil {
					t.Fatalf("failed to create stub for %s: %#v", gvResource, err)
				}
			}

			output, err := getFromEtcd(master.KV, testData.ExpectedEtcdPath)
			if err != nil {
				t.Fatalf("failed to get from etcd for %s: %#v", gvResource, err)
			}

			expectedGVK := gvk
			if testData.ExpectedGVK != nil {
				if gvk == *testData.ExpectedGVK {
					t.Errorf("GVK override %s for %s is unnecessary or something was changed incorrectly", testData.ExpectedGVK, gvk)
				}
				expectedGVK = *testData.ExpectedGVK
			}

			actualGVK := output.getGVK()
			if actualGVK != expectedGVK {
				t.Errorf("GVK for %s does not match, expected %s got %s", kind, expectedGVK, actualGVK)
			}

			if !apiequality.Semantic.DeepDerivative(input, output) {
				t.Errorf("Test stub for %s does not match: %s", kind, diff.ObjectGoPrintDiff(input, output))
			}

			addGVKToEtcdBucket(cohabitatingResources, actualGVK, getEtcdBucket(testData.ExpectedEtcdPath))
			pathSeen[testData.ExpectedEtcdPath] = append(pathSeen[testData.ExpectedEtcdPath], mapping.Resource)
		})
	}

	if inEtcdData, inEtcdSeen := diffMaps(etcdStorageData, etcdSeen); len(inEtcdData) != 0 || len(inEtcdSeen) != 0 {
		t.Errorf("etcd data does not match the types we saw:\nin etcd data but not seen:\n%s\nseen but not in etcd data:\n%s", inEtcdData, inEtcdSeen)
	}
	if inKindData, inKindSeen := diffMaps(kindWhiteList, kindSeen); len(inKindData) != 0 || len(inKindSeen) != 0 {
		t.Errorf("kind whitelist data does not match the types we saw:\nin kind whitelist but not seen:\n%s\nseen but not in kind whitelist:\n%s", inKindData, inKindSeen)
	}

	for bucket, gvks := range cohabitatingResources {
		if len(gvks) != 1 {
			gvkStrings := []string{}
			for key := range gvks {
				gvkStrings = append(gvkStrings, keyStringer(key))
			}
			t.Errorf("cohabitating resources in etcd bucket %s have inconsistent GVKs\nyou may need to use DefaultStorageFactory.AddCohabitatingResources to sync the GVK of these resources:\n%s", bucket, gvkStrings)
		}
	}

	for path, gvrs := range pathSeen {
		if len(gvrs) != 1 {
			gvrStrings := []string{}
			for _, key := range gvrs {
				gvrStrings = append(gvrStrings, keyStringer(key))
			}
			t.Errorf("invalid test data, please ensure all expectedEtcdPath are unique, path %s has duplicate GVRs:\n%s", path, gvrStrings)
		}
	}
}

func (c *allClient) createWithMultiTenancy(stub, te string, ns string, mapping *meta.RESTMapping, all *[]cleanupData) error {
	resourceClient, obj, err := JSONToUnstructuredWithMultiTenancy(stub, te, ns, mapping, c.dynamicClient)
	if err != nil {
		return err
	}

	actual, err := resourceClient.Create(obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	*all = append(*all, cleanupData{obj: actual, resource: mapping.Resource})

	return nil
}

func (c *allClient) cleanupWithMultiTenancy(all *[]cleanupData) error {
	for i := len(*all) - 1; i >= 0; i-- { // delete in reverse order in case creation order mattered
		obj := (*all)[i].obj
		gvr := (*all)[i].resource

		if err := c.dynamicClient.Resource(gvr).NamespaceWithMultiTenancy(obj.GetNamespace(), obj.GetTenant()).Delete(obj.GetName(), nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *allClient) createPrerequisitesWithMultiTenancy(mapper meta.RESTMapper, te string, ns string, prerequisites []Prerequisite, all *[]cleanupData) error {
	for _, prerequisite := range prerequisites {
		gvk, err := mapper.KindFor(prerequisite.GvrData)
		if err != nil {
			return err
		}
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}
		if err := c.createWithMultiTenancy(prerequisite.Stub, te, ns, mapping, all); err != nil {
			return err
		}
	}
	return nil
}
