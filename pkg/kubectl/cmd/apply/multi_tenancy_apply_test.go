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

package apply

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	clienttesting "k8s.io/client-go/testing"
	cmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
	utilpointer "k8s.io/utils/pointer"
)

const (
	filenameWidgetClientsideWithMultiTenancy = "../../../../test/fixtures/pkg/kubectl/cmd/apply/widget-clientside-multi-tenancy.yaml"
	filenameWidgetServersideWithMultiTenancy = "../../../../test/fixtures/pkg/kubectl/cmd/apply/widget-serverside-multi-tenancy.yaml"
)

func TestRunApplyPrintsValidObjectListWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	configMapList := readConfigMapList(t, filenameCM)
	pathCM := "/tenants/test-te/namespaces/test/configmaps"

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case strings.HasPrefix(p, pathCM) && m == "GET":
				fallthrough
			case strings.HasPrefix(p, pathCM) && m == "PATCH":
				var body io.ReadCloser

				switch p {
				case pathCM + "/test0":
					body = ioutil.NopCloser(bytes.NewReader(configMapList[0]))
				case pathCM + "/test1":
					body = ioutil.NopCloser(bytes.NewReader(configMapList[1]))
				default:
					t.Errorf("unexpected request to %s", p)
				}

				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: body}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

	ioStreams, _, buf, _ := genericclioptions.NewTestIOStreams()
	cmd := NewCmdApply("kubectl", tf, ioStreams)
	cmd.Flags().Set("filename", filenameCM)
	cmd.Flags().Set("output", "json")
	cmd.Flags().Set("dry-run", "true")
	cmd.Run(cmd, []string{})

	// ensure that returned list can be unmarshaled back into a configmap list
	cmList := corev1.List{}
	if err := runtime.DecodeInto(codec, buf.Bytes(), &cmList); err != nil {
		t.Fatal(err)
	}

	if len(cmList.Items) != 2 {
		t.Fatalf("Expected 2 items in the result; got %d", len(cmList.Items))
	}
	if !strings.Contains(string(cmList.Items[0].Raw), "key1") {
		t.Fatalf("Did not get first ConfigMap at the first position")
	}
	if !strings.Contains(string(cmList.Items[1].Raw), "key2") {
		t.Fatalf("Did not get second ConfigMap at the second position")
	}
}

func TestApplyObjectWithoutAnnotationWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	nameRC, rcBytes := readReplicationController(t, filenameRC)
	pathRC := "/tenants/test-te/namespaces/test/replicationcontrollers/" + nameRC

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == pathRC && m == "GET":
				bodyRC := ioutil.NopCloser(bytes.NewReader(rcBytes))
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
			case p == pathRC && m == "PATCH":
				bodyRC := ioutil.NopCloser(bytes.NewReader(rcBytes))
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

	ioStreams, _, buf, errBuf := genericclioptions.NewTestIOStreams()
	cmd := NewCmdApply("kubectl", tf, ioStreams)
	cmd.Flags().Set("filename", filenameRC)
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{})

	// uses the name from the file, not the response
	expectRC := "replicationcontroller/" + nameRC + "\n"
	expectWarning := fmt.Sprintf(warningNoLastAppliedConfigAnnotation, "kubectl")
	if errBuf.String() != expectWarning {
		t.Fatalf("unexpected non-warning: %s\nexpected: %s", errBuf.String(), expectWarning)
	}
	if buf.String() != expectRC {
		t.Fatalf("unexpected output: %s\nexpected: %s", buf.String(), expectRC)
	}
}

func TestApplyObjectWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	nameRC, currentRC := readAndAnnotateReplicationController(t, filenameRC)
	pathRC := "/tenants/test-te/namespaces/test/replicationcontrollers/" + nameRC

	for _, fn := range testingOpenAPISchemaFns {
		t.Run("test apply when a local object is specified", func(t *testing.T) {
			tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
			defer tf.Cleanup()

			tf.UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					switch p, m := req.URL.Path, req.Method; {
					case p == pathRC && m == "GET":
						bodyRC := ioutil.NopCloser(bytes.NewReader(currentRC))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
					case p == pathRC && m == "PATCH":
						validatePatchApplication(t, req)
						bodyRC := ioutil.NopCloser(bytes.NewReader(currentRC))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
					default:
						t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
						return nil, nil
					}
				}),
			}
			tf.OpenAPISchemaFunc = fn
			tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

			ioStreams, _, buf, errBuf := genericclioptions.NewTestIOStreams()
			cmd := NewCmdApply("kubectl", tf, ioStreams)
			cmd.Flags().Set("filename", filenameRC)
			cmd.Flags().Set("output", "name")
			cmd.Run(cmd, []string{})

			// uses the name from the file, not the response
			expectRC := "replicationcontroller/" + nameRC + "\n"
			if buf.String() != expectRC {
				t.Fatalf("unexpected output: %s\nexpected: %s", buf.String(), expectRC)
			}
			if errBuf.String() != "" {
				t.Fatalf("unexpected error output: %s", errBuf.String())
			}
		})
	}
}

func TestApplyObjectOutputWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	nameRC, currentRC := readAndAnnotateReplicationController(t, filenameRC)
	pathRC := "/tenants/test-te/namespaces/test/replicationcontrollers/" + nameRC

	// Add some extra data to the post-patch object
	postPatchObj := &unstructured.Unstructured{}
	if err := json.Unmarshal(currentRC, &postPatchObj.Object); err != nil {
		t.Fatal(err)
	}
	postPatchLabels := postPatchObj.GetLabels()
	if postPatchLabels == nil {
		postPatchLabels = map[string]string{}
	}
	postPatchLabels["post-patch"] = "value"
	postPatchObj.SetLabels(postPatchLabels)
	postPatchData, err := json.Marshal(postPatchObj)
	if err != nil {
		t.Fatal(err)
	}

	for _, fn := range testingOpenAPISchemaFns {
		t.Run("test apply returns correct output", func(t *testing.T) {
			tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
			defer tf.Cleanup()

			tf.UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					switch p, m := req.URL.Path, req.Method; {
					case p == pathRC && m == "GET":
						bodyRC := ioutil.NopCloser(bytes.NewReader(currentRC))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
					case p == pathRC && m == "PATCH":
						validatePatchApplication(t, req)
						bodyRC := ioutil.NopCloser(bytes.NewReader(postPatchData))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
					default:
						t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
						return nil, nil
					}
				}),
			}
			tf.OpenAPISchemaFunc = fn
			tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

			ioStreams, _, buf, errBuf := genericclioptions.NewTestIOStreams()
			cmd := NewCmdApply("kubectl", tf, ioStreams)
			cmd.Flags().Set("filename", filenameRC)
			cmd.Flags().Set("output", "yaml")
			cmd.Run(cmd, []string{})

			if !strings.Contains(buf.String(), "test-rc") {
				t.Fatalf("unexpected output: %s\nexpected to contain: %s", buf.String(), "test-rc")
			}
			if !strings.Contains(buf.String(), "post-patch: value") {
				t.Fatalf("unexpected output: %s\nexpected to contain: %s", buf.String(), "post-patch: value")
			}
			if errBuf.String() != "" {
				t.Fatalf("unexpected error output: %s", errBuf.String())
			}
		})
	}
}

func TestApplyRetryWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	nameRC, currentRC := readAndAnnotateReplicationController(t, filenameRC)
	pathRC := "/tenants/test-te/namespaces/test/replicationcontrollers/" + nameRC

	for _, fn := range testingOpenAPISchemaFns {
		t.Run("test apply retries on conflict error", func(t *testing.T) {
			firstPatch := true
			retry := false
			getCount := 0
			tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
			defer tf.Cleanup()

			tf.UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					switch p, m := req.URL.Path, req.Method; {
					case p == pathRC && m == "GET":
						getCount++
						bodyRC := ioutil.NopCloser(bytes.NewReader(currentRC))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
					case p == pathRC && m == "PATCH":
						if firstPatch {
							firstPatch = false
							statusErr := kubeerr.NewConflict(schema.GroupResource{Group: "", Resource: "rc"}, "test-rc", fmt.Errorf("the object has been modified. Please apply at first"))
							bodyBytes, _ := json.Marshal(statusErr)
							bodyErr := ioutil.NopCloser(bytes.NewReader(bodyBytes))
							return &http.Response{StatusCode: http.StatusConflict, Header: cmdtesting.DefaultHeader(), Body: bodyErr}, nil
						}
						retry = true
						validatePatchApplication(t, req)
						bodyRC := ioutil.NopCloser(bytes.NewReader(currentRC))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
					default:
						t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
						return nil, nil
					}
				}),
			}
			tf.OpenAPISchemaFunc = fn
			tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

			ioStreams, _, buf, errBuf := genericclioptions.NewTestIOStreams()
			cmd := NewCmdApply("kubectl", tf, ioStreams)
			cmd.Flags().Set("filename", filenameRC)
			cmd.Flags().Set("output", "name")
			cmd.Run(cmd, []string{})

			if !retry || getCount != 2 {
				t.Fatalf("apply didn't retry when get conflict error")
			}

			// uses the name from the file, not the response
			expectRC := "replicationcontroller/" + nameRC + "\n"
			if buf.String() != expectRC {
				t.Fatalf("unexpected output: %s\nexpected: %s", buf.String(), expectRC)
			}
			if errBuf.String() != "" {
				t.Fatalf("unexpected error output: %s", errBuf.String())
			}
		})
	}
}

func TestApplyNonExistObjectWithMultiTenancy(t *testing.T) {
	nameRC, currentRC := readAndAnnotateReplicationController(t, filenameRC)
	pathRC := "/tenants/test-te/namespaces/test/replicationcontrollers"
	pathNameRC := pathRC + "/" + nameRC

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/api/v1/tenants/test-te/namespaces/test" && m == "GET":
				return &http.Response{StatusCode: 404, Header: cmdtesting.DefaultHeader(), Body: ioutil.NopCloser(bytes.NewReader(nil))}, nil
			case p == pathNameRC && m == "GET":
				return &http.Response{StatusCode: 404, Header: cmdtesting.DefaultHeader(), Body: ioutil.NopCloser(bytes.NewReader(nil))}, nil
			case p == pathRC && m == "POST":
				bodyRC := ioutil.NopCloser(bytes.NewReader(currentRC))
				return &http.Response{StatusCode: 201, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

	ioStreams, _, buf, _ := genericclioptions.NewTestIOStreams()
	cmd := NewCmdApply("kubectl", tf, ioStreams)
	cmd.Flags().Set("filename", filenameRC)
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{})

	// uses the name from the file, not the response
	expectRC := "replicationcontroller/" + nameRC + "\n"
	if buf.String() != expectRC {
		t.Errorf("unexpected output: %s\nexpected: %s", buf.String(), expectRC)
	}
}

func TestApplyEmptyPatchWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	nameRC, _ := readAndAnnotateReplicationController(t, filenameRC)
	pathRC := "/tenants/test-te/namespaces/test/replicationcontrollers"
	pathNameRC := pathRC + "/" + nameRC

	verifyPost := false

	var body []byte

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	tf.UnstructuredClient = &fake.RESTClient{
		GroupVersion:         schema.GroupVersion{Version: "v1"},
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/api/v1/tenants/test-te/namespaces/test" && m == "GET":
				return &http.Response{StatusCode: 404, Header: cmdtesting.DefaultHeader(), Body: ioutil.NopCloser(bytes.NewReader(nil))}, nil
			case p == pathNameRC && m == "GET":
				if body == nil {
					return &http.Response{StatusCode: 404, Header: cmdtesting.DefaultHeader(), Body: ioutil.NopCloser(bytes.NewReader(nil))}, nil
				}
				bodyRC := ioutil.NopCloser(bytes.NewReader(body))
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
			case p == pathRC && m == "POST":
				body, _ = ioutil.ReadAll(req.Body)
				verifyPost = true
				bodyRC := ioutil.NopCloser(bytes.NewReader(body))
				return &http.Response{StatusCode: 201, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

	// 1. apply non exist object
	ioStreams, _, buf, _ := genericclioptions.NewTestIOStreams()
	cmd := NewCmdApply("kubectl", tf, ioStreams)
	cmd.Flags().Set("filename", filenameRC)
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{})

	expectRC := "replicationcontroller/" + nameRC + "\n"
	if buf.String() != expectRC {
		t.Fatalf("unexpected output: %s\nexpected: %s", buf.String(), expectRC)
	}
	if !verifyPost {
		t.Fatal("No server-side post call detected")
	}

	// 2. test apply already exist object, will not send empty patch request
	ioStreams, _, buf, _ = genericclioptions.NewTestIOStreams()
	cmd = NewCmdApply("kubectl", tf, ioStreams)
	cmd.Flags().Set("filename", filenameRC)
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{})

	if buf.String() != expectRC {
		t.Fatalf("unexpected output: %s\nexpected: %s", buf.String(), expectRC)
	}
}

func TestApplyMultipleObjectsAsListWithMultiTenancy(t *testing.T) {
	testApplyMultipleObjectsWithMultiTenancy(t, true)
}

func TestApplyMultipleObjectsAsFilesWithMultiTenancy(t *testing.T) {
	testApplyMultipleObjectsWithMultiTenancy(t, false)
}

func testApplyMultipleObjectsWithMultiTenancy(t *testing.T, asList bool) {
	nameRC, currentRC := readAndAnnotateReplicationController(t, filenameRC)
	pathRC := "/tenants/test-te/namespaces/test/replicationcontrollers/" + nameRC

	nameSVC, currentSVC := readAndAnnotateService(t, filenameSVC)
	pathSVC := "/tenants/test-te/namespaces/test/services/" + nameSVC

	for _, fn := range testingOpenAPISchemaFns {
		t.Run("test apply on multiple objects", func(t *testing.T) {
			tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
			defer tf.Cleanup()

			tf.UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					switch p, m := req.URL.Path, req.Method; {
					case p == pathRC && m == "GET":
						bodyRC := ioutil.NopCloser(bytes.NewReader(currentRC))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
					case p == pathRC && m == "PATCH":
						validatePatchApplication(t, req)
						bodyRC := ioutil.NopCloser(bytes.NewReader(currentRC))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
					case p == pathSVC && m == "GET":
						bodySVC := ioutil.NopCloser(bytes.NewReader(currentSVC))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodySVC}, nil
					case p == pathSVC && m == "PATCH":
						validatePatchApplication(t, req)
						bodySVC := ioutil.NopCloser(bytes.NewReader(currentSVC))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodySVC}, nil
					default:
						t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
						return nil, nil
					}
				}),
			}
			tf.OpenAPISchemaFunc = fn
			tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

			ioStreams, _, buf, errBuf := genericclioptions.NewTestIOStreams()
			cmd := NewCmdApply("kubectl", tf, ioStreams)
			if asList {
				cmd.Flags().Set("filename", filenameRCSVC)
			} else {
				cmd.Flags().Set("filename", filenameRC)
				cmd.Flags().Set("filename", filenameSVC)
			}
			cmd.Flags().Set("output", "name")

			cmd.Run(cmd, []string{})

			// Names should come from the REST response, NOT the files
			expectRC := "replicationcontroller/" + nameRC + "\n"
			expectSVC := "service/" + nameSVC + "\n"
			// Test both possible orders since output is non-deterministic.
			expectOne := expectRC + expectSVC
			expectTwo := expectSVC + expectRC
			if buf.String() != expectOne && buf.String() != expectTwo {
				t.Fatalf("unexpected output: %s\nexpected: %s OR %s", buf.String(), expectOne, expectTwo)
			}
			if errBuf.String() != "" {
				t.Fatalf("unexpected error output: %s", errBuf.String())
			}
		})
	}
}

func TestApplyNULLPreservationWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	deploymentName := "nginx-deployment"
	deploymentPath := "/tenants/test-te/namespaces/test/deployments/" + deploymentName

	verifiedPatch := false
	deploymentBytes := readDeploymentFromFile(t, filenameDeployObjServerside)

	for _, fn := range testingOpenAPISchemaFns {
		t.Run("test apply preserves NULL fields", func(t *testing.T) {
			tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
			defer tf.Cleanup()

			tf.UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					switch p, m := req.URL.Path, req.Method; {
					case p == deploymentPath && m == "GET":
						body := ioutil.NopCloser(bytes.NewReader(deploymentBytes))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: body}, nil
					case p == deploymentPath && m == "PATCH":
						patch, err := ioutil.ReadAll(req.Body)
						if err != nil {
							t.Fatal(err)
						}

						patchMap := map[string]interface{}{}
						if err := json.Unmarshal(patch, &patchMap); err != nil {
							t.Fatal(err)
						}
						annotationMap := walkMapPath(t, patchMap, []string{"metadata", "annotations"})
						if _, ok := annotationMap[corev1.LastAppliedConfigAnnotation]; !ok {
							t.Fatalf("patch does not contain annotation:\n%s\n", patch)
						}
						strategy := walkMapPath(t, patchMap, []string{"spec", "strategy"})
						if value, ok := strategy["rollingUpdate"]; !ok || value != nil {
							t.Fatalf("patch did not retain null value in key: rollingUpdate:\n%s\n", patch)
						}
						verifiedPatch = true

						// The real API server would had returned the patched object but Kubectl
						// is ignoring the actual return object.
						// TODO: Make this match actual server behavior by returning the patched object.
						body := ioutil.NopCloser(bytes.NewReader(deploymentBytes))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: body}, nil
					default:
						t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
						return nil, nil
					}
				}),
			}
			tf.OpenAPISchemaFunc = fn
			tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

			ioStreams, _, buf, errBuf := genericclioptions.NewTestIOStreams()
			cmd := NewCmdApply("kubectl", tf, ioStreams)
			cmd.Flags().Set("filename", filenameDeployObjClientside)
			cmd.Flags().Set("output", "name")

			cmd.Run(cmd, []string{})

			expected := "deployment.apps/" + deploymentName + "\n"
			if buf.String() != expected {
				t.Fatalf("unexpected output: %s\nexpected: %s", buf.String(), expected)
			}
			if errBuf.String() != "" {
				t.Fatalf("unexpected error output: %s", errBuf.String())
			}
			if !verifiedPatch {
				t.Fatal("No server-side patch call detected")
			}
		})
	}
}

// TestUnstructuredApply checks apply operations on an unstructured object
func TestUnstructuredApplyWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	name, curr := readAndAnnotateUnstructured(t, filenameWidgetClientsideWithMultiTenancy)
	path := "/tenants/test-te/namespaces/test/widgets/" + name

	verifiedPatch := false

	for _, fn := range testingOpenAPISchemaFns {
		t.Run("test apply works correctly with unstructured objects", func(t *testing.T) {
			tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
			defer tf.Cleanup()

			tf.UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					switch p, m := req.URL.Path, req.Method; {
					case p == path && m == "GET":
						body := ioutil.NopCloser(bytes.NewReader(curr))
						return &http.Response{
							StatusCode: 200,
							Header:     cmdtesting.DefaultHeader(),
							Body:       body}, nil
					case p == path && m == "PATCH":
						contentType := req.Header.Get("Content-Type")
						if contentType != "application/merge-patch+json" {
							t.Fatalf("Unexpected Content-Type: %s", contentType)
						}
						validatePatchApplication(t, req)
						verifiedPatch = true

						body := ioutil.NopCloser(bytes.NewReader(curr))
						return &http.Response{
							StatusCode: 200,
							Header:     cmdtesting.DefaultHeader(),
							Body:       body}, nil
					default:
						t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
						return nil, nil
					}
				}),
			}
			tf.OpenAPISchemaFunc = fn
			tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

			ioStreams, _, buf, errBuf := genericclioptions.NewTestIOStreams()
			cmd := NewCmdApply("kubectl", tf, ioStreams)
			cmd.Flags().Set("filename", filenameWidgetClientsideWithMultiTenancy)
			cmd.Flags().Set("output", "name")
			cmd.Run(cmd, []string{})

			expected := "widget.unit-test.test.com/" + name + "\n"
			if buf.String() != expected {
				t.Fatalf("unexpected output: %s\nexpected: %s", buf.String(), expected)
			}
			if errBuf.String() != "" {
				t.Fatalf("unexpected error output: %s", errBuf.String())
			}
			if !verifiedPatch {
				t.Fatal("No server-side patch call detected")
			}
		})
	}
}

// TestUnstructuredIdempotentApply checks repeated apply operation on an unstructured object
func TestUnstructuredIdempotentApplyWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)

	serversideObject := readUnstructuredFromFile(t, filenameWidgetServersideWithMultiTenancy)
	serversideData, err := runtime.Encode(unstructured.NewJSONFallbackEncoder(codec), serversideObject)
	if err != nil {
		t.Fatal(err)
	}
	path := "/tenants/test-te/namespaces/test/widgets/widget"

	for _, fn := range testingOpenAPISchemaFns {
		t.Run("test repeated apply operations on an unstructured object", func(t *testing.T) {
			tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
			defer tf.Cleanup()

			tf.UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					switch p, m := req.URL.Path, req.Method; {
					case p == path && m == "GET":
						body := ioutil.NopCloser(bytes.NewReader(serversideData))
						return &http.Response{
							StatusCode: 200,
							Header:     cmdtesting.DefaultHeader(),
							Body:       body}, nil
					case p == path && m == "PATCH":
						// In idempotent updates, kubectl will resolve to an empty patch and not send anything to the server
						// Thus, if we reach this branch, kubectl is unnecessarily sending a patch.

						patch, err := ioutil.ReadAll(req.Body)
						if err != nil {
							t.Fatal(err)
						}
						t.Fatalf("Unexpected Patch: %s", patch)
						return nil, nil
					default:
						t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
						return nil, nil
					}
				}),
			}
			tf.OpenAPISchemaFunc = fn
			tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

			ioStreams, _, buf, errBuf := genericclioptions.NewTestIOStreams()
			cmd := NewCmdApply("kubectl", tf, ioStreams)
			cmd.Flags().Set("filename", filenameWidgetClientsideWithMultiTenancy)
			cmd.Flags().Set("output", "name")
			cmd.Run(cmd, []string{})

			expected := "widget.unit-test.test.com/widget\n"
			if buf.String() != expected {
				t.Fatalf("unexpected output: %s\nexpected: %s", buf.String(), expected)
			}
			if errBuf.String() != "" {
				t.Fatalf("unexpected error output: %s", errBuf.String())
			}
		})
	}
}

func TestForceApplyWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	scheme := runtime.NewScheme()
	nameRC, currentRC := readAndAnnotateReplicationController(t, filenameRC)
	pathRC := "/tenants/test-te/namespaces/test/replicationcontrollers/" + nameRC
	pathRCList := "/tenants/test-te/namespaces/test/replicationcontrollers"
	expected := map[string]int{
		"getOk":       6,
		"getNotFound": 1,
		"getList":     0,
		"patch":       6,
		"delete":      1,
		"post":        1,
	}

	for _, fn := range testingOpenAPISchemaFns {
		t.Run("test apply with --force", func(t *testing.T) {
			deleted := false
			isScaledDownToZero := false
			counts := map[string]int{}
			tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
			defer tf.Cleanup()

			tf.ClientConfigVal = cmdtesting.DefaultClientConfig()
			tf.UnstructuredClient = &fake.RESTClient{
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					switch p, m := req.URL.Path, req.Method; {
					case strings.HasSuffix(p, pathRC) && m == "GET":
						if deleted {
							counts["getNotFound"]++
							return &http.Response{StatusCode: 404, Header: cmdtesting.DefaultHeader(), Body: ioutil.NopCloser(bytes.NewReader([]byte{}))}, nil
						}
						counts["getOk"]++
						var bodyRC io.ReadCloser
						if isScaledDownToZero {
							rcObj := readReplicationControllerFromFile(t, filenameRC)
							rcObj.Spec.Replicas = utilpointer.Int32Ptr(0)
							rcBytes, err := runtime.Encode(codec, rcObj)
							if err != nil {
								t.Fatal(err)
							}
							bodyRC = ioutil.NopCloser(bytes.NewReader(rcBytes))
						} else {
							bodyRC = ioutil.NopCloser(bytes.NewReader(currentRC))
						}
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
					case strings.HasSuffix(p, pathRCList) && m == "GET":
						counts["getList"]++
						rcObj := readUnstructuredFromFile(t, filenameRC)
						list := &unstructured.UnstructuredList{
							Object: map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "ReplicationControllerList",
							},
							Items: []unstructured.Unstructured{*rcObj},
						}
						listBytes, err := runtime.Encode(codec, list)
						if err != nil {
							t.Fatal(err)
						}
						bodyRCList := ioutil.NopCloser(bytes.NewReader(listBytes))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRCList}, nil
					case strings.HasSuffix(p, pathRC) && m == "PATCH":
						counts["patch"]++
						if counts["patch"] <= 6 {
							statusErr := kubeerr.NewConflict(schema.GroupResource{Group: "", Resource: "rc"}, "test-rc", fmt.Errorf("the object has been modified. Please apply at first"))
							bodyBytes, _ := json.Marshal(statusErr)
							bodyErr := ioutil.NopCloser(bytes.NewReader(bodyBytes))
							return &http.Response{StatusCode: http.StatusConflict, Header: cmdtesting.DefaultHeader(), Body: bodyErr}, nil
						}
						t.Fatalf("unexpected request: %#v after %v tries\n%#v", req.URL, counts["patch"], req)
						return nil, nil
					case strings.HasSuffix(p, pathRC) && m == "PUT":
						counts["put"]++
						bodyRC := ioutil.NopCloser(bytes.NewReader(currentRC))
						isScaledDownToZero = true
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
					case strings.HasSuffix(p, pathRCList) && m == "POST":
						counts["post"]++
						deleted = false
						isScaledDownToZero = false
						bodyRC := ioutil.NopCloser(bytes.NewReader(currentRC))
						return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: bodyRC}, nil
					default:
						t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
						return nil, nil
					}
				}),
			}
			fakeDynamicClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)
			fakeDynamicClient.PrependReactor("delete", "replicationcontrollers", func(action clienttesting.Action) (bool, runtime.Object, error) {
				if deleteAction, ok := action.(clienttesting.DeleteAction); ok {
					if deleteAction.GetName() == nameRC {
						counts["delete"]++
						deleted = true
						return true, nil, nil
					}
				}
				return false, nil, nil
			})
			tf.FakeDynamicClient = fakeDynamicClient
			tf.OpenAPISchemaFunc = fn
			tf.Client = tf.UnstructuredClient
			tf.ClientConfigVal = restclient.CreateEmptyConfig()

			ioStreams, _, buf, errBuf := genericclioptions.NewTestIOStreams()
			cmd := NewCmdApply("kubectl", tf, ioStreams)
			cmd.Flags().Set("filename", filenameRC)
			cmd.Flags().Set("output", "name")
			cmd.Flags().Set("force", "true")
			cmd.Run(cmd, []string{})

			for method, exp := range expected {
				if exp != counts[method] {
					t.Errorf("Unexpected amount of %q API calls, wanted %v got %v", method, exp, counts[method])
				}
			}

			if expected := "replicationcontroller/" + nameRC + "\n"; buf.String() != expected {
				t.Fatalf("unexpected output: %s\nexpected: %s", buf.String(), expected)
			}
			if errBuf.String() != "" {
				t.Fatalf("unexpected error output: %s", errBuf.String())
			}
		})
	}
}
