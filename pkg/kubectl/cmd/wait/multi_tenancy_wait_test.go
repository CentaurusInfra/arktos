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

package wait

import (
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func newUnstructuredWithMultiTenancy(apiVersion, kind, namespace, name string, tenant string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"tenant":    tenant,
				"namespace": namespace,
				"name":      name,
				"uid":       "some-UID-value",
			},
		},
	}
}

func TestWaitForDeletionWithMultiTenancy(t *testing.T) {
	scheme := runtime.NewScheme()

	tests := []struct {
		name       string
		infos      []*resource.Info
		fakeClient func() *dynamicfakeclient.FakeDynamicClient
		timeout    time.Duration
		uidMap     UIDMap

		expectedErr     string
		validateActions func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name: "missing on get",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme)
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name:  "handles no infos",
			infos: []*resource.Info{},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme)
			},
			timeout:     10 * time.Second,
			expectedErr: errNoMatchingResources.Error(),

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
		{
			name: "uid conflict on get",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te")), nil
				})
				count := 0
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					if count == 0 {
						count++
						fakeWatch := watch.NewRaceFreeFake()
						go func() {
							time.Sleep(100 * time.Millisecond)
							fakeWatch.Stop()
						}()
						return true, fakeWatch, nil
					}
					fakeWatch := watch.NewRaceFreeFake()
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,
			uidMap: UIDMap{
				ResourceLocation{Tenant: "test-te", Namespace: "ns-foo", Name: "name-foo"}:                                                                               types.UID("some-UID-value"),
				ResourceLocation{GroupResource: schema.GroupResource{Group: "group", Resource: "theresource"}, Tenant: "test-te", Namespace: "ns-foo", Name: "name-foo"}: types.UID("some-nonmatching-UID-value"),
			},

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "times out",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te")), nil
				})
				return fakeClient
			},
			timeout: 1 * time.Second,

			expectedErr: "timed out waiting for the condition on theresource/name-foo",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch close out",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					unstructuredObj := newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te")
					unstructuredObj.SetResourceVersion("123")
					unstructuredList := newUnstructuredList(unstructuredObj)
					unstructuredList.SetResourceVersion("234")
					return true, unstructuredList, nil
				})
				count := 0
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					if count == 0 {
						count++
						fakeWatch := watch.NewRaceFreeFake()
						go func() {
							time.Sleep(100 * time.Millisecond)
							fakeWatch.Stop()
						}()
						return true, fakeWatch, nil
					}
					fakeWatch := watch.NewRaceFreeFake()
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 3 * time.Second,

			expectedErr: "timed out waiting for the condition on theresource/name-foo",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 4 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") || actions[1].(clienttesting.WatchAction).GetWatchRestrictions().ResourceVersion != "234" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[2].Matches("list", "theresource") || actions[2].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[3].Matches("watch", "theresource") || actions[3].(clienttesting.WatchAction).GetWatchRestrictions().ResourceVersion != "234" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch delete",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te")), nil
				})
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					fakeWatch.Action(watch.Deleted, newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te"))
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch delete multiple",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource-1"},
					},
					Name:      "name-foo-1",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource-2"},
					},
					Name:      "name-foo-2",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("get", "theresource-1", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo-1", "test-te"), nil
				})
				fakeClient.PrependReactor("get", "theresource-2", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo-2", "test-te"), nil
				})
				fakeClient.PrependWatchReactor("theresource-1", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					fakeWatch.Action(watch.Deleted, newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo-1", "test-te"))
					return true, fakeWatch, nil
				})
				fakeClient.PrependWatchReactor("theresource-2", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					fakeWatch.Action(watch.Deleted, newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo-2", "test-te"))
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource-1") {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("list", "theresource-2") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "ignores watch error",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te")), nil
				})
				count := 0
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					if count == 0 {
						fakeWatch.Error(newUnstructuredStatus(&metav1.Status{
							TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
							Status:   "Failure",
							Code:     500,
							Message:  "Bad",
						}))
						fakeWatch.Stop()
					} else {
						fakeWatch.Action(watch.Deleted, newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te"))
					}
					count++
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 4 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
				if !actions[2].Matches("list", "theresource") || actions[2].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[3].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := test.fakeClient()
			o := &WaitOptions{
				ResourceFinder: genericclioptions.NewSimpleFakeResourceFinder(test.infos...),
				UIDMap:         test.uidMap,
				DynamicClient:  fakeClient,
				Timeout:        test.timeout,

				Printer:     printers.NewDiscardingPrinter(),
				ConditionFn: IsDeleted,
				IOStreams:   genericclioptions.NewTestIOStreamsDiscard(),
			}
			err := o.RunWait()
			switch {
			case err == nil && len(test.expectedErr) == 0:
			case err != nil && len(test.expectedErr) == 0:
				t.Fatal(err)
			case err == nil && len(test.expectedErr) != 0:
				t.Fatalf("missing: %q", test.expectedErr)
			case err != nil && len(test.expectedErr) != 0:
				if !strings.Contains(err.Error(), test.expectedErr) {
					t.Fatalf("expected %q, got %q", test.expectedErr, err.Error())
				}
			}

			test.validateActions(t, fakeClient.Actions())
		})
	}
}

func TestWaitForConditionWithMultiTenancy(t *testing.T) {
	scheme := runtime.NewScheme()

	tests := []struct {
		name       string
		infos      []*resource.Info
		fakeClient func() *dynamicfakeclient.FakeDynamicClient
		timeout    time.Duration

		expectedErr     string
		validateActions func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name: "present on get",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(addCondition(
						newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te"),
						"the-condition", "status-value",
					)), nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name:  "handles no infos",
			infos: []*resource.Info{},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme)
			},
			timeout:     10 * time.Second,
			expectedErr: errNoMatchingResources.Error(),

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles empty object name",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				return dynamicfakeclient.NewSimpleDynamicClient(scheme)
			},
			timeout:     10 * time.Second,
			expectedErr: "resource name must be provided",

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
		{
			name: "times out",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, addCondition(
						newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te"),
						"some-other-condition", "status-value",
					), nil
				})
				return fakeClient
			},
			timeout: 1 * time.Second,

			expectedErr: "timed out waiting for the condition on theresource/name-foo",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch close out",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					unstructuredObj := newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te")
					unstructuredObj.SetResourceVersion("123")
					unstructuredList := newUnstructuredList(unstructuredObj)
					unstructuredList.SetResourceVersion("234")
					return true, unstructuredList, nil
				})
				count := 0
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					if count == 0 {
						count++
						fakeWatch := watch.NewRaceFreeFake()
						go func() {
							time.Sleep(100 * time.Millisecond)
							fakeWatch.Stop()
						}()
						return true, fakeWatch, nil
					}
					fakeWatch := watch.NewRaceFreeFake()
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 3 * time.Second,

			expectedErr: "timed out waiting for the condition on theresource/name-foo",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 4 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") || actions[1].(clienttesting.WatchAction).GetWatchRestrictions().ResourceVersion != "234" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[2].Matches("list", "theresource") || actions[2].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[3].Matches("watch", "theresource") || actions[3].(clienttesting.WatchAction).GetWatchRestrictions().ResourceVersion != "234" {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch condition change",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te")), nil
				})
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					fakeWatch.Action(watch.Modified, addCondition(
						newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te"),
						"the-condition", "status-value",
					))
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "handles watch created",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					fakeWatch.Action(watch.Added, addCondition(
						newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te"),
						"the-condition", "status-value",
					))
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
		{
			name: "ignores watch error",
			infos: []*resource.Info{
				{
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{Group: "group", Version: "version", Resource: "theresource"},
					},
					Name:      "name-foo",
					Namespace: "ns-foo",
					Tenant:    "test-te",
				},
			},
			fakeClient: func() *dynamicfakeclient.FakeDynamicClient {
				fakeClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
				fakeClient.PrependReactor("list", "theresource", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, newUnstructuredList(newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te")), nil
				})
				count := 0
				fakeClient.PrependWatchReactor("theresource", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
					fakeWatch := watch.NewRaceFreeFake()
					if count == 0 {
						fakeWatch.Error(newUnstructuredStatus(&metav1.Status{
							TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
							Status:   "Failure",
							Code:     500,
							Message:  "Bad",
						}))
						fakeWatch.Stop()
					} else {
						fakeWatch.Action(watch.Modified, addCondition(
							newUnstructuredWithMultiTenancy("group/version", "TheKind", "ns-foo", "name-foo", "test-te"),
							"the-condition", "status-value",
						))
					}
					count++
					return true, fakeWatch, nil
				})
				return fakeClient
			},
			timeout: 10 * time.Second,

			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 4 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("list", "theresource") || actions[0].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
				if !actions[2].Matches("list", "theresource") || actions[2].(clienttesting.ListAction).GetListRestrictions().Fields.String() != "metadata.name=name-foo" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[3].Matches("watch", "theresource") {
					t.Error(spew.Sdump(actions))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := test.fakeClient()
			o := &WaitOptions{
				ResourceFinder: genericclioptions.NewSimpleFakeResourceFinder(test.infos...),
				DynamicClient:  fakeClient,
				Timeout:        test.timeout,

				Printer:     printers.NewDiscardingPrinter(),
				ConditionFn: ConditionalWait{conditionName: "the-condition", conditionStatus: "status-value", errOut: ioutil.Discard}.IsConditionMet,
				IOStreams:   genericclioptions.NewTestIOStreamsDiscard(),
			}
			err := o.RunWait()
			switch {
			case err == nil && len(test.expectedErr) == 0:
			case err != nil && len(test.expectedErr) == 0:
				t.Fatal(err)
			case err == nil && len(test.expectedErr) != 0:
				t.Fatalf("missing: %q", test.expectedErr)
			case err != nil && len(test.expectedErr) != 0:
				if !strings.Contains(err.Error(), test.expectedErr) {
					t.Fatalf("expected %q, got %q", test.expectedErr, err.Error())
				}
			}

			test.validateActions(t, fakeClient.Actions())
		})
	}
}
