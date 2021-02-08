/*
Copyright 2017 The Kubernetes Authors.

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

package apimachinery

import (
	"fmt"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	crdclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edeploy "k8s.io/kubernetes/test/e2e/framework/deployment"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/utils/crd"
	imageutils "k8s.io/kubernetes/test/utils/image"
	"k8s.io/utils/pointer"
	"reflect"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	// ensure libs have a chance to initialize
	_ "github.com/stretchr/testify/assert"
)

const (
	secretName      = "sample-webhook-secret"
	deploymentName  = "sample-webhook-deployment"
	serviceName     = "e2e-test-webhook"
	servicePort     = 8443
	roleBindingName = "webhook-auth-reader"

	// The webhook configuration names should not be reused between test instances.
	crWebhookConfigName                    = "e2e-test-webhook-config-cr"
	webhookConfigName                      = "e2e-test-webhook-config"
	attachingPodWebhookConfigName          = "e2e-test-webhook-config-attaching-pod"
	mutatingWebhookConfigName              = "e2e-test-mutating-webhook-config"
	podMutatingWebhookConfigName           = "e2e-test-mutating-webhook-pod"
	webhookFailClosedConfigName            = "e2e-test-webhook-fail-closed"
	validatingWebhookForWebhooksConfigName = "e2e-test-validating-webhook-for-webhooks-config"
	mutatingWebhookForWebhooksConfigName   = "e2e-test-mutating-webhook-for-webhooks-config"
	dummyValidatingWebhookConfigName       = "e2e-test-dummy-validating-webhook-config"
	dummyMutatingWebhookConfigName         = "e2e-test-dummy-mutating-webhook-config"
	crdWebhookConfigName                   = "e2e-test-webhook-config-crd"
	slowWebhookConfigName                  = "e2e-test-webhook-config-slow"

	skipNamespaceLabelKey     = "skip-webhook-admission"
	skipNamespaceLabelValue   = "yes"
	skippedNamespaceName      = "exempted-namesapce"
	disallowedPodName         = "disallowed-pod"
	toBeAttachedPodName       = "to-be-attached-pod"
	hangingPodName            = "hanging-pod"
	disallowedConfigMapName   = "disallowed-configmap"
	nonDeletableConfigmapName = "nondeletable-configmap"
	allowedConfigMapName      = "allowed-configmap"
	failNamespaceLabelKey     = "fail-closed-webhook"
	failNamespaceLabelValue   = "yes"
	failNamespaceName         = "fail-closed-namesapce"
	addedLabelKey             = "added-label"
	addedLabelValue           = "yes"
)

var serverWebhookVersion = utilversion.MustParseSemantic("v1.8.0")

var _ = SIGDescribe("AdmissionWebhook", func() {
	var context *certContext
	f := framework.NewDefaultFramework("webhook")

	var client clientset.Interface
	var namespaceName string

	ginkgo.BeforeEach(func() {
		client = f.ClientSet
		namespaceName = f.Namespace.Name

		// Make sure the relevant provider supports admission webhook
		framework.SkipUnlessServerVersionGTE(serverWebhookVersion, f.ClientSet.Discovery())
		framework.SkipUnlessProviderIs("gce", "gke", "local")

		_, err := f.ClientSet.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().List(metav1.ListOptions{})
		if errors.IsNotFound(err) {
			framework.Skipf("dynamic configuration of webhooks requires the admissionregistration.k8s.io group to be enabled")
		}

		ginkgo.By("Setting up server cert")
		context = setupServerCert(namespaceName, serviceName)
		createAuthReaderRoleBinding(f, namespaceName)

		// Note that in 1.9 we will have backwards incompatible change to
		// admission webhooks, so the image will be updated to 1.9 sometime in
		// the development 1.9 cycle.
		deployWebhookAndService(f, imageutils.GetE2EImage(imageutils.AdmissionWebhook), context)
	})

	ginkgo.AfterEach(func() {
		cleanWebhookTest(client, namespaceName)
	})

	ginkgo.It("Should be able to deny pod and configmap creation", func() {
		webhookCleanup := registerWebhook(f, context)
		defer webhookCleanup()
		testWebhook(f)
	})

	ginkgo.It("Should be able to deny attaching pod", func() {
		webhookCleanup := registerWebhookForAttachingPod(f, context)
		defer webhookCleanup()
		testAttachingPodWebhook(f)
	})

	ginkgo.It("Should be able to deny custom resource creation and deletion", func() {
		testcrd, err := crd.CreateTestCRD(f)
		if err != nil {
			return
		}
		defer testcrd.CleanUp()
		webhookCleanup := registerWebhookForCustomResource(f, context, testcrd)
		defer webhookCleanup()
		testCustomResourceWebhook(f, testcrd.Crd, testcrd.DynamicClients["v1"])
		testBlockingCustomResourceDeletion(f, testcrd.Crd, testcrd.DynamicClients["v1"])
	})

	ginkgo.It("Should unconditionally reject operations on fail closed webhook", func() {
		webhookCleanup := registerFailClosedWebhook(f, context)
		defer webhookCleanup()
		testFailClosedWebhook(f)
	})

	ginkgo.It("Should mutate configmap", func() {
		webhookCleanup := registerMutatingWebhookForConfigMap(f, context)
		defer webhookCleanup()
		testMutatingConfigMapWebhook(f)
	})

	ginkgo.It("Should mutate pod and apply defaults after mutation", func() {
		webhookCleanup := registerMutatingWebhookForPod(f, context)
		defer webhookCleanup()
		testMutatingPodWebhook(f)
	})

	ginkgo.It("Should not be able to mutate or prevent deletion of webhook configuration objects", func() {
		validatingWebhookCleanup := registerValidatingWebhookForWebhookConfigurations(f, context)
		defer validatingWebhookCleanup()
		mutatingWebhookCleanup := registerMutatingWebhookForWebhookConfigurations(f, context)
		defer mutatingWebhookCleanup()
		testWebhooksForWebhookConfigurations(f)
	})

	ginkgo.It("Should mutate custom resource", func() {
		testcrd, err := crd.CreateTestCRD(f)
		if err != nil {
			return
		}
		defer testcrd.CleanUp()
		webhookCleanup := registerMutatingWebhookForCustomResource(f, context, testcrd)
		defer webhookCleanup()
		testMutatingCustomResourceWebhook(f, testcrd.Crd, testcrd.DynamicClients["v1"], false)
	})

	ginkgo.It("Should deny crd creation", func() {
		crdWebhookCleanup := registerValidatingWebhookForCRD(f, context)
		defer crdWebhookCleanup()

		testCRDDenyWebhook(f)
	})

	ginkgo.It("Should mutate custom resource with different stored version", func() {
		testcrd, err := createAdmissionWebhookMultiVersionTestCRDWithV1Storage(f)
		if err != nil {
			return
		}
		defer testcrd.CleanUp()
		webhookCleanup := registerMutatingWebhookForCustomResource(f, context, testcrd)
		defer webhookCleanup()
		testMultiVersionCustomResourceWebhook(f, testcrd)
	})

	ginkgo.It("Should mutate custom resource with pruning", func() {
		const prune = true
		testcrd, err := createAdmissionWebhookMultiVersionTestCRDWithV1Storage(f, func(crd *apiextensionsv1beta1.CustomResourceDefinition) {
			crd.Spec.PreserveUnknownFields = pointer.BoolPtr(false)
			crd.Spec.Validation = &apiextensionsv1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &apiextensionsv1beta1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]apiextensionsv1beta1.JSONSchemaProps{
						"data": {
							Type: "object",
							Properties: map[string]apiextensionsv1beta1.JSONSchemaProps{
								"mutation-start":   {Type: "string"},
								"mutation-stage-1": {Type: "string"},
								// mutation-stage-2 is intentionally missing such that it is pruned
							},
						},
					},
				},
			}
		})
		if err != nil {
			return
		}
		defer testcrd.CleanUp()
		webhookCleanup := registerMutatingWebhookForCustomResource(f, context, testcrd)
		defer webhookCleanup()
		testMutatingCustomResourceWebhook(f, testcrd.Crd, testcrd.DynamicClients["v1"], prune)
	})

	ginkgo.It("Should honor timeout", func() {
		policyFail := admissionregistrationv1beta1.Fail
		policyIgnore := admissionregistrationv1beta1.Ignore

		ginkgo.By("Setting timeout (1s) shorter than webhook latency (5s)")
		slowWebhookCleanup := registerSlowWebhook(f, context, &policyFail, pointer.Int32Ptr(1))
		testSlowWebhookTimeoutFailEarly(f)
		slowWebhookCleanup()

		ginkgo.By("Having no error when timeout is shorter than webhook latency and failure policy is ignore")
		slowWebhookCleanup = registerSlowWebhook(f, context, &policyIgnore, pointer.Int32Ptr(1))
		testSlowWebhookTimeoutNoError(f)
		slowWebhookCleanup()

		ginkgo.By("Having no error when timeout is longer than webhook latency")
		slowWebhookCleanup = registerSlowWebhook(f, context, &policyFail, pointer.Int32Ptr(10))
		testSlowWebhookTimeoutNoError(f)
		slowWebhookCleanup()

		ginkgo.By("Having no error when timeout is empty (defaulted to 10s in v1beta1)")
		slowWebhookCleanup = registerSlowWebhook(f, context, &policyFail, nil)
		testSlowWebhookTimeoutNoError(f)
		slowWebhookCleanup()
	})

	// TODO: add more e2e tests for mutating webhooks
	// 1. mutating webhook that mutates pod
	// 2. mutating webhook that sends empty patch
	//   2.1 and sets status.allowed=true
	//   2.2 and sets status.allowed=false
	// 3. mutating webhook that sends patch, but also sets status.allowed=false
	// 4. mutating webhook that fail-open v.s. fail-closed
})

func createAuthReaderRoleBinding(f *framework.Framework, namespace string) {
	ginkgo.By("Create role binding to let webhook read extension-apiserver-authentication")
	client := f.ClientSet
	// Create the role binding to allow the webhook read the extension-apiserver-authentication configmap
	_, err := client.RbacV1().RoleBindings("kube-system").Create(&rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleBindingName,
			Annotations: map[string]string{
				rbacv1.AutoUpdateAnnotationKey: "true",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "",
			Kind:     "Role",
			Name:     "extension-apiserver-authentication-reader",
		},
		// Webhook uses the default service account.
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: namespace,
			},
		},
	})
	if err != nil && errors.IsAlreadyExists(err) {
		e2elog.Logf("role binding %s already exists", roleBindingName)
	} else {
		framework.ExpectNoError(err, "creating role binding %s:webhook to access configMap", namespace)
	}
}

func deployWebhookAndService(f *framework.Framework, image string, context *certContext) {
	ginkgo.By("Deploying the webhook pod")
	client := f.ClientSet

	// Creating the secret that contains the webhook's cert.
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tls.crt": context.cert,
			"tls.key": context.key,
		},
	}
	namespace := f.Namespace.Name
	_, err := client.CoreV1().Secrets(namespace).Create(secret)
	framework.ExpectNoError(err, "creating secret %q in namespace %q", secretName, namespace)

	// Create the deployment of the webhook
	podLabels := map[string]string{"app": "sample-webhook", "webhook": "true"}
	replicas := int32(1)
	zero := int64(0)
	mounts := []v1.VolumeMount{
		{
			Name:      "webhook-certs",
			ReadOnly:  true,
			MountPath: "/webhook.local.config/certificates",
		},
	}
	volumes := []v1.Volume{
		{
			Name: "webhook-certs",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{SecretName: secretName},
			},
		},
	}
	containers := []v1.Container{
		{
			Name:         "sample-webhook",
			VolumeMounts: mounts,
			Args: []string{
				"--tls-cert-file=/webhook.local.config/certificates/tls.crt",
				"--tls-private-key-file=/webhook.local.config/certificates/tls.key",
				"--alsologtostderr",
				"-v=4",
				"2>&1",
			},
			Image: image,
		},
	}
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   deploymentName,
			Labels: podLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: &zero,
					Containers:                    containers,
					Volumes:                       volumes,
				},
			},
		},
	}
	deployment, err := client.AppsV1().Deployments(namespace).Create(d)
	framework.ExpectNoError(err, "creating deployment %s in namespace %s", deploymentName, namespace)
	ginkgo.By("Wait for the deployment to be ready")
	err = e2edeploy.WaitForDeploymentRevisionAndImage(client, namespace, deploymentName, "1", image)
	framework.ExpectNoError(err, "waiting for the deployment of image %s in %s in %s to complete", image, deploymentName, namespace)
	err = e2edeploy.WaitForDeploymentComplete(client, deployment)
	framework.ExpectNoError(err, "waiting for the deployment status valid", image, deploymentName, namespace)

	ginkgo.By("Deploying the webhook service")

	serviceLabels := map[string]string{"webhook": "true"}
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      serviceName,
			Labels:    map[string]string{"test": "webhook"},
		},
		Spec: v1.ServiceSpec{
			Selector: serviceLabels,
			Ports: []v1.ServicePort{
				{
					Protocol:   "TCP",
					Port:       servicePort,
					TargetPort: intstr.FromInt(443),
				},
			},
		},
	}
	_, err = client.CoreV1().Services(namespace).Create(service)
	framework.ExpectNoError(err, "creating service %s in namespace %s", serviceName, namespace)

	ginkgo.By("Verifying the service has paired with the endpoint")
	err = framework.WaitForServiceEndpointsNum(client, namespace, serviceName, 1, 1*time.Second, 30*time.Second)
	framework.ExpectNoError(err, "waiting for service %s/%s have %d endpoint", namespace, serviceName, 1)
}

func strPtr(s string) *string { return &s }

func registerWebhook(f *framework.Framework, context *certContext) func() {
	client := f.ClientSet
	ginkgo.By("Registering the webhook via the AdmissionRegistration API")

	namespace := f.Namespace.Name
	configName := webhookConfigName
	// A webhook that cannot talk to server, with fail-open policy
	failOpenHook := failingWebhook(namespace, "fail-open.k8s.io")
	policyIgnore := admissionregistrationv1beta1.Ignore
	failOpenHook.FailurePolicy = &policyIgnore

	_, err := client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Create(&admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
			{
				Name: "deny-unwanted-pod-container-name-and-label.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/pods"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
			},
			{
				Name: "deny-unwanted-configmap-data.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create, admissionregistrationv1beta1.Update, admissionregistrationv1beta1.Delete},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"configmaps"},
					},
				}},
				// The webhook skips the namespace that has label "skip-webhook-admission":"yes"
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      skipNamespaceLabelKey,
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{skipNamespaceLabelValue},
						},
					},
				},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/configmaps"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
			},
			// Server cannot talk to this webhook, so it always fails.
			// Because this webhook is configured fail-open, request should be admitted after the call fails.
			failOpenHook,
		},
	})
	framework.ExpectNoError(err, "registering webhook config %s with namespace %s", configName, namespace)

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)

	return func() {
		client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(configName, nil)
	}
}

func registerWebhookForAttachingPod(f *framework.Framework, context *certContext) func() {
	client := f.ClientSet
	ginkgo.By("Registering the webhook via the AdmissionRegistration API")

	namespace := f.Namespace.Name
	configName := attachingPodWebhookConfigName
	// A webhook that cannot talk to server, with fail-open policy
	failOpenHook := failingWebhook(namespace, "fail-open.k8s.io")
	policyIgnore := admissionregistrationv1beta1.Ignore
	failOpenHook.FailurePolicy = &policyIgnore

	_, err := client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Create(&admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
			{
				Name: "deny-attaching-pod.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Connect},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods/attach"},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/pods/attach"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
			},
		},
	})
	framework.ExpectNoError(err, "registering webhook config %s with namespace %s", configName, namespace)

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)

	return func() {
		client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(configName, nil)
	}
}

func registerMutatingWebhookForConfigMap(f *framework.Framework, context *certContext) func() {
	client := f.ClientSet
	ginkgo.By("Registering the mutating configmap webhook via the AdmissionRegistration API")

	namespace := f.Namespace.Name
	configName := mutatingWebhookConfigName

	_, err := client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Create(&admissionregistrationv1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Webhooks: []admissionregistrationv1beta1.MutatingWebhook{
			{
				Name: "adding-configmap-data-stage-1.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"configmaps"},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/mutating-configmaps"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
			},
			{
				Name: "adding-configmap-data-stage-2.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"configmaps"},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/mutating-configmaps"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
			},
		},
	})
	framework.ExpectNoError(err, "registering mutating webhook config %s with namespace %s", configName, namespace)

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)
	return func() { client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Delete(configName, nil) }
}

func testMutatingConfigMapWebhook(f *framework.Framework) {
	ginkgo.By("create a configmap that should be updated by the webhook")
	client := f.ClientSet
	configMap := toBeMutatedConfigMap(f)
	mutatedConfigMap, err := client.CoreV1().ConfigMaps(f.Namespace.Name).Create(configMap)
	gomega.Expect(err).To(gomega.BeNil())
	expectedConfigMapData := map[string]string{
		"mutation-start":   "yes",
		"mutation-stage-1": "yes",
		"mutation-stage-2": "yes",
	}
	if !reflect.DeepEqual(expectedConfigMapData, mutatedConfigMap.Data) {
		framework.Failf("\nexpected %#v\n, got %#v\n", expectedConfigMapData, mutatedConfigMap.Data)
	}
}

func registerMutatingWebhookForPod(f *framework.Framework, context *certContext) func() {
	client := f.ClientSet
	ginkgo.By("Registering the mutating pod webhook via the AdmissionRegistration API")

	namespace := f.Namespace.Name
	configName := podMutatingWebhookConfigName

	_, err := client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Create(&admissionregistrationv1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Webhooks: []admissionregistrationv1beta1.MutatingWebhook{
			{
				Name: "adding-init-container.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/mutating-pods"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
			},
		},
	})
	framework.ExpectNoError(err, "registering mutating webhook config %s with namespace %s", configName, namespace)

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)

	return func() { client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Delete(configName, nil) }
}

func testMutatingPodWebhook(f *framework.Framework) {
	ginkgo.By("create a pod that should be updated by the webhook")
	client := f.ClientSet
	configMap := toBeMutatedPod(f)
	mutatedPod, err := client.CoreV1().Pods(f.Namespace.Name).Create(configMap)
	gomega.Expect(err).To(gomega.BeNil())
	if len(mutatedPod.Spec.InitContainers) != 1 {
		framework.Failf("expect pod to have 1 init container, got %#v", mutatedPod.Spec.InitContainers)
	}
	if got, expected := mutatedPod.Spec.InitContainers[0].Name, "webhook-added-init-container"; got != expected {
		framework.Failf("expect the init container name to be %q, got %q", expected, got)
	}
	if got, expected := mutatedPod.Spec.InitContainers[0].TerminationMessagePolicy, v1.TerminationMessageReadFile; got != expected {
		framework.Failf("expect the init terminationMessagePolicy to be default to %q, got %q", expected, got)
	}
}

func toBeMutatedPod(f *framework.Framework) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "webhook-to-be-mutated",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "example",
					Image: imageutils.GetPauseImageName(),
				},
			},
		},
	}
}

func testWebhook(f *framework.Framework) {
	ginkgo.By("create a pod that should be denied by the webhook")
	client := f.ClientSet
	// Creating the pod, the request should be rejected
	pod := nonCompliantPod(f)
	_, err := client.CoreV1().Pods(f.Namespace.Name).Create(pod)
	framework.ExpectError(err, "create pod %s in namespace %s should have been denied by webhook", pod.Name, f.Namespace.Name)
	expectedErrMsg1 := "the pod contains unwanted container name"
	if !strings.Contains(err.Error(), expectedErrMsg1) {
		framework.Failf("expect error contains %q, got %q", expectedErrMsg1, err.Error())
	}
	expectedErrMsg2 := "the pod contains unwanted label"
	if !strings.Contains(err.Error(), expectedErrMsg2) {
		framework.Failf("expect error contains %q, got %q", expectedErrMsg2, err.Error())
	}

	ginkgo.By("create a pod that causes the webhook to hang")
	client = f.ClientSet
	// Creating the pod, the request should be rejected
	pod = hangingPod(f)
	_, err = client.CoreV1().Pods(f.Namespace.Name).Create(pod)
	framework.ExpectError(err, "create pod %s in namespace %s should have caused webhook to hang", pod.Name, f.Namespace.Name)
	expectedTimeoutErr := "request did not complete within"
	if !strings.Contains(err.Error(), expectedTimeoutErr) {
		framework.Failf("expect timeout error %q, got %q", expectedTimeoutErr, err.Error())
	}

	ginkgo.By("create a configmap that should be denied by the webhook")
	// Creating the configmap, the request should be rejected
	configmap := nonCompliantConfigMap(f)
	_, err = client.CoreV1().ConfigMaps(f.Namespace.Name).Create(configmap)
	framework.ExpectError(err, "create configmap %s in namespace %s should have been denied by the webhook", configmap.Name, f.Namespace.Name)
	expectedErrMsg := "the configmap contains unwanted key and value"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		framework.Failf("expect error contains %q, got %q", expectedErrMsg, err.Error())
	}

	ginkgo.By("create a configmap that should be admitted by the webhook")
	// Creating the configmap, the request should be admitted
	configmap = &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: allowedConfigMapName,
		},
		Data: map[string]string{
			"admit": "this",
		},
	}
	_, err = client.CoreV1().ConfigMaps(f.Namespace.Name).Create(configmap)
	framework.ExpectNoError(err, "failed to create configmap %s in namespace: %s", configmap.Name, f.Namespace.Name)

	ginkgo.By("update (PUT) the admitted configmap to a non-compliant one should be rejected by the webhook")
	toNonCompliantFn := func(cm *v1.ConfigMap) {
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data["webhook-e2e-test"] = "webhook-disallow"
	}
	_, err = updateConfigMap(client, f.Namespace.Name, allowedConfigMapName, toNonCompliantFn)
	framework.ExpectError(err, "update (PUT) admitted configmap %s in namespace %s to a non-compliant one should be rejected by webhook", allowedConfigMapName, f.Namespace.Name)
	if !strings.Contains(err.Error(), expectedErrMsg) {
		framework.Failf("expect error contains %q, got %q", expectedErrMsg, err.Error())
	}

	ginkgo.By("update (PATCH) the admitted configmap to a non-compliant one should be rejected by the webhook")
	patch := nonCompliantConfigMapPatch()
	_, err = client.CoreV1().ConfigMaps(f.Namespace.Name).Patch(allowedConfigMapName, types.StrategicMergePatchType, []byte(patch))
	framework.ExpectError(err, "update admitted configmap %s in namespace %s by strategic merge patch to a non-compliant one should be rejected by webhook. Patch: %+v", allowedConfigMapName, f.Namespace.Name, patch)
	if !strings.Contains(err.Error(), expectedErrMsg) {
		framework.Failf("expect error contains %q, got %q", expectedErrMsg, err.Error())
	}

	ginkgo.By("create a namespace that bypass the webhook")
	err = createNamespace(f, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: skippedNamespaceName,
		Labels: map[string]string{
			skipNamespaceLabelKey: skipNamespaceLabelValue,
		},
	}})
	framework.ExpectNoError(err, "creating namespace %q", skippedNamespaceName)
	// clean up the namespace
	defer client.CoreV1().Namespaces().Delete(skippedNamespaceName, nil)

	ginkgo.By("create a configmap that violates the webhook policy but is in a whitelisted namespace")
	configmap = nonCompliantConfigMap(f)
	_, err = client.CoreV1().ConfigMaps(skippedNamespaceName).Create(configmap)
	framework.ExpectNoError(err, "failed to create configmap %s in namespace: %s", configmap.Name, skippedNamespaceName)
}

func testBlockingConfigmapDeletion(f *framework.Framework) {
	ginkgo.By("create a configmap that should be denied by the webhook when deleting")
	client := f.ClientSet
	configmap := nonDeletableConfigmap(f)
	_, err := client.CoreV1().ConfigMaps(f.Namespace.Name).Create(configmap)
	framework.ExpectNoError(err, "failed to create configmap %s in namespace: %s", configmap.Name, f.Namespace.Name)

	ginkgo.By("deleting the configmap should be denied by the webhook")
	err = client.CoreV1().ConfigMaps(f.Namespace.Name).Delete(configmap.Name, &metav1.DeleteOptions{})
	framework.ExpectError(err, "deleting configmap %s in namespace: %s should be denied", configmap.Name, f.Namespace.Name)
	expectedErrMsg1 := "the configmap cannot be deleted because it contains unwanted key and value"
	if !strings.Contains(err.Error(), expectedErrMsg1) {
		framework.Failf("expect error contains %q, got %q", expectedErrMsg1, err.Error())
	}

	ginkgo.By("remove the offending key and value from the configmap data")
	toCompliantFn := func(cm *v1.ConfigMap) {
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data["webhook-e2e-test"] = "webhook-allow"
	}
	_, err = updateConfigMap(client, f.Namespace.Name, configmap.Name, toCompliantFn)
	framework.ExpectNoError(err, "failed to update configmap %s in namespace: %s", configmap.Name, f.Namespace.Name)

	ginkgo.By("deleting the updated configmap should be successful")
	err = client.CoreV1().ConfigMaps(f.Namespace.Name).Delete(configmap.Name, &metav1.DeleteOptions{})
	framework.ExpectNoError(err, "failed to delete configmap %s in namespace: %s", configmap.Name, f.Namespace.Name)
}

func testAttachingPodWebhook(f *framework.Framework) {
	ginkgo.By("create a pod")
	client := f.ClientSet
	pod := toBeAttachedPod(f)
	_, err := client.CoreV1().Pods(f.Namespace.Name).Create(pod)
	framework.ExpectNoError(err, "failed to create pod %s in namespace: %s", pod.Name, f.Namespace.Name)
	err = e2epod.WaitForPodNameRunningInNamespace(client, pod.Name, f.Namespace.Name)
	framework.ExpectNoError(err, "error while waiting for pod %s to go to Running phase in namespace: %s", pod.Name, f.Namespace.Name)

	ginkgo.By("'kubectl attach' the pod, should be denied by the webhook")
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	_, err = framework.NewKubectlCommand("attach", fmt.Sprintf("--namespace=%v", f.Namespace.Name), pod.Name, "-i", "-c=container1").WithTimeout(timer.C).Exec()
	framework.ExpectError(err, "'kubectl attach' the pod, should be denied by the webhook")
	if e, a := "attaching to pod 'to-be-attached-pod' is not allowed", err.Error(); !strings.Contains(a, e) {
		framework.Failf("unexpected 'kubectl attach' error message. expected to contain %q, got %q", e, a)
	}
}

// failingWebhook returns a webhook with rule of create configmaps,
// but with an invalid client config so that server cannot communicate with it
func failingWebhook(namespace, name string) admissionregistrationv1beta1.ValidatingWebhook {
	return admissionregistrationv1beta1.ValidatingWebhook{
		Name: name,
		Rules: []admissionregistrationv1beta1.RuleWithOperations{{
			Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
			Rule: admissionregistrationv1beta1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"configmaps"},
			},
		}},
		ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
			Service: &admissionregistrationv1beta1.ServiceReference{
				Namespace: namespace,
				Name:      serviceName,
				Path:      strPtr("/configmaps"),
				Port:      pointer.Int32Ptr(servicePort),
			},
			// Without CA bundle, the call to webhook always fails
			CABundle: nil,
		},
	}
}

func registerFailClosedWebhook(f *framework.Framework, context *certContext) func() {
	client := f.ClientSet
	ginkgo.By("Registering a webhook that server cannot talk to, with fail closed policy, via the AdmissionRegistration API")

	namespace := f.Namespace.Name
	configName := webhookFailClosedConfigName
	// A webhook that cannot talk to server, with fail-closed policy
	policyFail := admissionregistrationv1beta1.Fail
	hook := failingWebhook(namespace, "fail-closed.k8s.io")
	hook.FailurePolicy = &policyFail
	hook.NamespaceSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      failNamespaceLabelKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{failNamespaceLabelValue},
			},
		},
	}

	_, err := client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Create(&admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
			// Server cannot talk to this webhook, so it always fails.
			// Because this webhook is configured fail-closed, request should be rejected after the call fails.
			hook,
		},
	})
	framework.ExpectNoError(err, "registering webhook config %s with namespace %s", configName, namespace)

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)
	return func() {
		f.ClientSet.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(configName, nil)
	}
}

func testFailClosedWebhook(f *framework.Framework) {
	client := f.ClientSet
	ginkgo.By("create a namespace for the webhook")
	err := createNamespace(f, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: failNamespaceName,
		Labels: map[string]string{
			failNamespaceLabelKey: failNamespaceLabelValue,
		},
	}})
	framework.ExpectNoError(err, "creating namespace %q", failNamespaceName)
	defer client.CoreV1().Namespaces().Delete(failNamespaceName, nil)

	ginkgo.By("create a configmap should be unconditionally rejected by the webhook")
	configmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	_, err = client.CoreV1().ConfigMaps(failNamespaceName).Create(configmap)
	framework.ExpectError(err, "create configmap in namespace: %s should be unconditionally rejected by the webhook", failNamespaceName)
	if !errors.IsInternalError(err) {
		framework.Failf("expect an internal error, got %#v", err)
	}
}

func registerValidatingWebhookForWebhookConfigurations(f *framework.Framework, context *certContext) func() {
	var err error
	client := f.ClientSet
	ginkgo.By("Registering a validating webhook on ValidatingWebhookConfiguration and MutatingWebhookConfiguration objects, via the AdmissionRegistration API")

	namespace := f.Namespace.Name
	configName := validatingWebhookForWebhooksConfigName
	failurePolicy := admissionregistrationv1beta1.Fail

	// This webhook denies all requests to Delete validating webhook configuration and
	// mutating webhook configuration objects. It should never be called, however, because
	// dynamic admission webhooks should not be called on requests involving webhook configuration objects.
	_, err = client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Create(&admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
			{
				Name: "deny-webhook-configuration-deletions.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Delete},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{"admissionregistration.k8s.io"},
						APIVersions: []string{"*"},
						Resources: []string{
							"validatingwebhookconfigurations",
							"mutatingwebhookconfigurations",
						},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/always-deny"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
				FailurePolicy: &failurePolicy,
			},
		},
	})
	framework.ExpectNoError(err, "registering webhook config %s with namespace %s", configName, namespace)

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)
	return func() {
		err := client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(configName, nil)
		framework.ExpectNoError(err, "deleting webhook config %s with namespace %s", configName, namespace)
	}
}

func registerMutatingWebhookForWebhookConfigurations(f *framework.Framework, context *certContext) func() {
	var err error
	client := f.ClientSet
	ginkgo.By("Registering a mutating webhook on ValidatingWebhookConfiguration and MutatingWebhookConfiguration objects, via the AdmissionRegistration API")

	namespace := f.Namespace.Name
	configName := mutatingWebhookForWebhooksConfigName
	failurePolicy := admissionregistrationv1beta1.Fail

	// This webhook adds a label to all requests create to validating webhook configuration and
	// mutating webhook configuration objects. It should never be called, however, because
	// dynamic admission webhooks should not be called on requests involving webhook configuration objects.
	_, err = client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Create(&admissionregistrationv1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Webhooks: []admissionregistrationv1beta1.MutatingWebhook{
			{
				Name: "add-label-to-webhook-configurations.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{"admissionregistration.k8s.io"},
						APIVersions: []string{"*"},
						Resources: []string{
							"validatingwebhookconfigurations",
							"mutatingwebhookconfigurations",
						},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/add-label"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
				FailurePolicy: &failurePolicy,
			},
		},
	})
	framework.ExpectNoError(err, "registering webhook config %s with namespace %s", configName, namespace)

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)
	return func() {
		err := client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Delete(configName, nil)
		framework.ExpectNoError(err, "deleting webhook config %s with namespace %s", configName, namespace)
	}
}

// This test assumes that the deletion-rejecting webhook defined in
// registerValidatingWebhookForWebhookConfigurations and the webhook-config-mutating
// webhook defined in registerMutatingWebhookForWebhookConfigurations already exist.
func testWebhooksForWebhookConfigurations(f *framework.Framework) {
	var err error
	client := f.ClientSet
	ginkgo.By("Creating a dummy validating-webhook-configuration object")

	namespace := f.Namespace.Name
	failurePolicy := admissionregistrationv1beta1.Ignore

	mutatedValidatingWebhookConfiguration, err := client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Create(&admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: dummyValidatingWebhookConfigName,
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
			{
				Name: "dummy-validating-webhook.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
					// This will not match any real resources so this webhook should never be called.
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"invalid"},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						// This path not recognized by the webhook service,
						// so the call to this webhook will always fail,
						// but because the failure policy is ignore, it will
						// have no effect on admission requests.
						Path: strPtr(""),
						Port: pointer.Int32Ptr(servicePort),
					},
					CABundle: nil,
				},
				FailurePolicy: &failurePolicy,
			},
		},
	})
	framework.ExpectNoError(err, "registering webhook config %s with namespace %s", dummyValidatingWebhookConfigName, namespace)
	if mutatedValidatingWebhookConfiguration.ObjectMeta.Labels != nil && mutatedValidatingWebhookConfiguration.ObjectMeta.Labels[addedLabelKey] == addedLabelValue {
		framework.Failf("expected %s not to be mutated by mutating webhooks but it was", dummyValidatingWebhookConfigName)
	}

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)

	ginkgo.By("Deleting the validating-webhook-configuration, which should be possible to remove")

	err = client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(dummyValidatingWebhookConfigName, nil)
	framework.ExpectNoError(err, "deleting webhook config %s with namespace %s", dummyValidatingWebhookConfigName, namespace)

	ginkgo.By("Creating a dummy mutating-webhook-configuration object")

	mutatedMutatingWebhookConfiguration, err := client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Create(&admissionregistrationv1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: dummyMutatingWebhookConfigName,
		},
		Webhooks: []admissionregistrationv1beta1.MutatingWebhook{
			{
				Name: "dummy-mutating-webhook.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
					// This will not match any real resources so this webhook should never be called.
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"invalid"},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						// This path not recognized by the webhook service,
						// so the call to this webhook will always fail,
						// but because the failure policy is ignore, it will
						// have no effect on admission requests.
						Path: strPtr(""),
						Port: pointer.Int32Ptr(servicePort),
					},
					CABundle: nil,
				},
				FailurePolicy: &failurePolicy,
			},
		},
	})
	framework.ExpectNoError(err, "registering webhook config %s with namespace %s", dummyMutatingWebhookConfigName, namespace)
	if mutatedMutatingWebhookConfiguration.ObjectMeta.Labels != nil && mutatedMutatingWebhookConfiguration.ObjectMeta.Labels[addedLabelKey] == addedLabelValue {
		framework.Failf("expected %s not to be mutated by mutating webhooks but it was", dummyMutatingWebhookConfigName)
	}

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)

	ginkgo.By("Deleting the mutating-webhook-configuration, which should be possible to remove")

	err = client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Delete(dummyMutatingWebhookConfigName, nil)
	framework.ExpectNoError(err, "deleting webhook config %s with namespace %s", dummyMutatingWebhookConfigName, namespace)
}

func createNamespace(f *framework.Framework, ns *v1.Namespace) error {
	return wait.PollImmediate(100*time.Millisecond, 30*time.Second, func() (bool, error) {
		_, err := f.ClientSet.CoreV1().Namespaces().Create(ns)
		if err != nil {
			if strings.HasPrefix(err.Error(), "object is being deleted:") {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}

func nonCompliantPod(f *framework.Framework) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: disallowedPodName,
			Labels: map[string]string{
				"webhook-e2e-test": "webhook-disallow",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "webhook-disallow",
					Image: imageutils.GetPauseImageName(),
				},
			},
		},
	}
}

func hangingPod(f *framework.Framework) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: hangingPodName,
			Labels: map[string]string{
				"webhook-e2e-test": "wait-forever",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "wait-forever",
					Image: imageutils.GetPauseImageName(),
				},
			},
		},
	}
}

func toBeAttachedPod(f *framework.Framework) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: toBeAttachedPodName,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "container1",
					Image: imageutils.GetPauseImageName(),
				},
			},
		},
	}
}

func nonCompliantConfigMap(f *framework.Framework) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: disallowedConfigMapName,
		},
		Data: map[string]string{
			"webhook-e2e-test": "webhook-disallow",
		},
	}
}

func nonDeletableConfigmap(f *framework.Framework) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: nonDeletableConfigmapName,
		},
		Data: map[string]string{
			"webhook-e2e-test": "webhook-nondeletable",
		},
	}
}

func toBeMutatedConfigMap(f *framework.Framework) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "to-be-mutated",
		},
		Data: map[string]string{
			"mutation-start": "yes",
		},
	}
}

func nonCompliantConfigMapPatch() string {
	return fmt.Sprint(`{"data":{"webhook-e2e-test":"webhook-disallow"}}`)
}

type updateConfigMapFn func(cm *v1.ConfigMap)

func updateConfigMap(c clientset.Interface, ns, name string, update updateConfigMapFn) (*v1.ConfigMap, error) {
	var cm *v1.ConfigMap
	pollErr := wait.PollImmediate(2*time.Second, 1*time.Minute, func() (bool, error) {
		var err error
		if cm, err = c.CoreV1().ConfigMaps(ns).Get(name, metav1.GetOptions{}); err != nil {
			return false, err
		}
		update(cm)
		if cm, err = c.CoreV1().ConfigMaps(ns).Update(cm); err == nil {
			return true, nil
		}
		// Only retry update on conflict
		if !errors.IsConflict(err) {
			return false, err
		}
		return false, nil
	})
	return cm, pollErr
}

type updateCustomResourceFn func(cm *unstructured.Unstructured)

func updateCustomResource(c dynamic.ResourceInterface, ns, name string, update updateCustomResourceFn) (*unstructured.Unstructured, error) {
	var cr *unstructured.Unstructured
	pollErr := wait.PollImmediate(2*time.Second, 1*time.Minute, func() (bool, error) {
		var err error
		if cr, err = c.Get(name, metav1.GetOptions{}); err != nil {
			return false, err
		}
		update(cr)
		if cr, err = c.Update(cr, metav1.UpdateOptions{}); err == nil {
			return true, nil
		}
		// Only retry update on conflict
		if !errors.IsConflict(err) {
			return false, err
		}
		return false, nil
	})
	return cr, pollErr
}

func cleanWebhookTest(client clientset.Interface, namespaceName string) {
	_ = client.CoreV1().Services(namespaceName).Delete(serviceName, nil)
	_ = client.AppsV1().Deployments(namespaceName).Delete(deploymentName, nil)
	_ = client.CoreV1().Secrets(namespaceName).Delete(secretName, nil)
	_ = client.RbacV1().RoleBindings("kube-system").Delete(roleBindingName, nil)
}

func registerWebhookForCustomResource(f *framework.Framework, context *certContext, testcrd *crd.TestCrd) func() {
	client := f.ClientSet
	ginkgo.By("Registering the custom resource webhook via the AdmissionRegistration API")

	namespace := f.Namespace.Name
	configName := crWebhookConfigName
	_, err := client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Create(&admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
			{
				Name: "deny-unwanted-custom-resource-data.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create, admissionregistrationv1beta1.Update, admissionregistrationv1beta1.Delete},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{testcrd.Crd.Spec.Group},
						APIVersions: servedAPIVersions(testcrd.Crd),
						Resources:   []string{testcrd.Crd.Spec.Names.Plural},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/custom-resource"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
			},
		},
	})
	framework.ExpectNoError(err, "registering custom resource webhook config %s with namespace %s", configName, namespace)

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)
	return func() {
		client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(configName, nil)
	}
}

func registerMutatingWebhookForCustomResource(f *framework.Framework, context *certContext, testcrd *crd.TestCrd) func() {
	client := f.ClientSet
	ginkgo.By(fmt.Sprintf("Registering the mutating webhook for custom resource %s via the AdmissionRegistration API", testcrd.Crd.Name))

	namespace := f.Namespace.Name
	configName := f.UniqueName
	_, err := client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Create(&admissionregistrationv1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Webhooks: []admissionregistrationv1beta1.MutatingWebhook{
			{
				Name: "mutate-custom-resource-data-stage-1.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create, admissionregistrationv1beta1.Update},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{testcrd.Crd.Spec.Group},
						APIVersions: servedAPIVersions(testcrd.Crd),
						Resources:   []string{testcrd.Crd.Spec.Names.Plural},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/mutating-custom-resource"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
			},
			{
				Name: "mutate-custom-resource-data-stage-2.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{testcrd.Crd.Spec.Group},
						APIVersions: servedAPIVersions(testcrd.Crd),
						Resources:   []string{testcrd.Crd.Spec.Names.Plural},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/mutating-custom-resource"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
			},
		},
	})
	framework.ExpectNoError(err, "registering custom resource webhook config %s with namespace %s", configName, namespace)

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)

	return func() { client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Delete(configName, nil) }
}

func testCustomResourceWebhook(f *framework.Framework, crd *apiextensionsv1beta1.CustomResourceDefinition, customResourceClient dynamic.ResourceInterface) {
	ginkgo.By("Creating a custom resource that should be denied by the webhook")
	crInstanceName := "cr-instance-1"
	crInstance := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       crd.Spec.Names.Kind,
			"apiVersion": crd.Spec.Group + "/" + crd.Spec.Version,
			"metadata": map[string]interface{}{
				"name":      crInstanceName,
				"namespace": f.Namespace.Name,
			},
			"data": map[string]interface{}{
				"webhook-e2e-test": "webhook-disallow",
			},
		},
	}
	_, err := customResourceClient.Create(crInstance, metav1.CreateOptions{})
	framework.ExpectError(err, "create custom resource %s in namespace %s should be denied by webhook", crInstanceName, f.Namespace.Name)
	expectedErrMsg := "the custom resource contains unwanted data"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		framework.Failf("expect error contains %q, got %q", expectedErrMsg, err.Error())
	}
}

func testBlockingCustomResourceDeletion(f *framework.Framework, crd *apiextensionsv1beta1.CustomResourceDefinition, customResourceClient dynamic.ResourceInterface) {
	ginkgo.By("Creating a custom resource whose deletion would be denied by the webhook")
	crInstanceName := "cr-instance-2"
	crInstance := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       crd.Spec.Names.Kind,
			"apiVersion": crd.Spec.Group + "/" + crd.Spec.Version,
			"metadata": map[string]interface{}{
				"name":      crInstanceName,
				"namespace": f.Namespace.Name,
			},
			"data": map[string]interface{}{
				"webhook-e2e-test": "webhook-nondeletable",
			},
		},
	}
	_, err := customResourceClient.Create(crInstance, metav1.CreateOptions{})
	framework.ExpectNoError(err, "failed to create custom resource %s in namespace: %s", crInstanceName, f.Namespace.Name)

	ginkgo.By("Deleting the custom resource should be denied")
	err = customResourceClient.Delete(crInstanceName, &metav1.DeleteOptions{})
	framework.ExpectError(err, "deleting custom resource %s in namespace: %s should be denied", crInstanceName, f.Namespace.Name)
	expectedErrMsg1 := "the custom resource cannot be deleted because it contains unwanted key and value"
	if !strings.Contains(err.Error(), expectedErrMsg1) {
		framework.Failf("expect error contains %q, got %q", expectedErrMsg1, err.Error())
	}

	ginkgo.By("Remove the offending key and value from the custom resource data")
	toCompliantFn := func(cr *unstructured.Unstructured) {
		if _, ok := cr.Object["data"]; !ok {
			cr.Object["data"] = map[string]interface{}{}
		}
		data := cr.Object["data"].(map[string]interface{})
		data["webhook-e2e-test"] = "webhook-allow"
	}
	_, err = updateCustomResource(customResourceClient, f.Namespace.Name, crInstanceName, toCompliantFn)
	framework.ExpectNoError(err, "failed to update custom resource %s in namespace: %s", crInstanceName, f.Namespace.Name)

	ginkgo.By("Deleting the updated custom resource should be successful")
	err = customResourceClient.Delete(crInstanceName, &metav1.DeleteOptions{})
	framework.ExpectNoError(err, "failed to delete custom resource %s in namespace: %s", crInstanceName, f.Namespace.Name)

}

func testMutatingCustomResourceWebhook(f *framework.Framework, crd *apiextensionsv1beta1.CustomResourceDefinition, customResourceClient dynamic.ResourceInterface, prune bool) {
	ginkgo.By("Creating a custom resource that should be mutated by the webhook")
	crName := "cr-instance-1"
	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       crd.Spec.Names.Kind,
			"apiVersion": crd.Spec.Group + "/" + crd.Spec.Version,
			"metadata": map[string]interface{}{
				"name":      crName,
				"namespace": f.Namespace.Name,
			},
			"data": map[string]interface{}{
				"mutation-start": "yes",
			},
		},
	}
	mutatedCR, err := customResourceClient.Create(cr, metav1.CreateOptions{})
	framework.ExpectNoError(err, "failed to create custom resource %s in namespace: %s", crName, f.Namespace.Name)
	expectedCRData := map[string]interface{}{
		"mutation-start":   "yes",
		"mutation-stage-1": "yes",
	}
	if !prune {
		expectedCRData["mutation-stage-2"] = "yes"
	}
	if !reflect.DeepEqual(expectedCRData, mutatedCR.Object["data"]) {
		framework.Failf("\nexpected %#v\n, got %#v\n", expectedCRData, mutatedCR.Object["data"])
	}
}

func testMultiVersionCustomResourceWebhook(f *framework.Framework, testcrd *crd.TestCrd) {
	customResourceClient := testcrd.DynamicClients["v1"]
	ginkgo.By("Creating a custom resource while v1 is storage version")
	crName := "cr-instance-1"
	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       testcrd.Crd.Spec.Names.Kind,
			"apiVersion": testcrd.Crd.Spec.Group + "/" + testcrd.Crd.Spec.Version,
			"metadata": map[string]interface{}{
				"name":      crName,
				"namespace": f.Namespace.Name,
			},
			"data": map[string]interface{}{
				"mutation-start": "yes",
			},
		},
	}
	_, err := customResourceClient.Create(cr, metav1.CreateOptions{})
	framework.ExpectNoError(err, "failed to create custom resource %s in namespace: %s", crName, f.Namespace.Name)

	ginkgo.By("Patching Custom Resource Definition to set v2 as storage")
	apiVersionWithV2StoragePatch := fmt.Sprint(`{"spec": {"versions": [{"name": "v1", "storage": false, "served": true},{"name": "v2", "storage": true, "served": true}]}}`)
	_, err = testcrd.APIExtensionClient.ApiextensionsV1beta1().CustomResourceDefinitions().Patch(testcrd.Crd.Name, types.StrategicMergePatchType, []byte(apiVersionWithV2StoragePatch))
	framework.ExpectNoError(err, "failed to patch custom resource definition %s in namespace: %s", testcrd.Crd.Name, f.Namespace.Name)

	ginkgo.By("Patching the custom resource while v2 is storage version")
	crDummyPatch := fmt.Sprint(`[{ "op": "add", "path": "/dummy", "value": "test" }]`)
	_, err = testcrd.DynamicClients["v2"].Patch(crName, types.JSONPatchType, []byte(crDummyPatch), metav1.PatchOptions{})
	framework.ExpectNoError(err, "failed to patch custom resource %s in namespace: %s", crName, f.Namespace.Name)
}

func registerValidatingWebhookForCRD(f *framework.Framework, context *certContext) func() {
	client := f.ClientSet
	ginkgo.By("Registering the crd webhook via the AdmissionRegistration API")

	namespace := f.Namespace.Name
	configName := crdWebhookConfigName

	// This webhook will deny the creation of CustomResourceDefinitions which have the
	// label "webhook-e2e-test":"webhook-disallow"
	// NOTE: Because tests are run in parallel and in an unpredictable order, it is critical
	// that no other test attempts to create CRD with that label.
	_, err := client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Create(&admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
			{
				Name: "deny-crd-with-unwanted-label.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{"apiextensions.k8s.io"},
						APIVersions: []string{"*"},
						Resources:   []string{"customresourcedefinitions"},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/crd"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
			},
		},
	})
	framework.ExpectNoError(err, "registering crd webhook config %s with namespace %s", configName, namespace)

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)
	return func() {
		client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(configName, nil)
	}
}

func testCRDDenyWebhook(f *framework.Framework) {
	ginkgo.By("Creating a custom resource definition that should be denied by the webhook")
	name := fmt.Sprintf("e2e-test-%s-%s-crd", f.BaseName, "deny")
	kind := fmt.Sprintf("E2e-test-%s-%s-crd", f.BaseName, "deny")
	group := fmt.Sprintf("%s-crd-test.k8s.io", f.BaseName)
	apiVersions := []apiextensionsv1beta1.CustomResourceDefinitionVersion{
		{
			Name:    "v1",
			Served:  true,
			Storage: true,
		},
	}

	// Creating a custom resource definition for use by assorted tests.
	config, err := framework.LoadConfig()
	if err != nil {
		framework.Failf("failed to load config: %v", err)
		return
	}
	apiExtensionClient, err := crdclientset.NewForConfig(config)
	if err != nil {
		framework.Failf("failed to initialize apiExtensionClient: %v", err)
		return
	}
	crd := &apiextensionsv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "s." + group,
			Labels: map[string]string{
				"webhook-e2e-test": "webhook-disallow",
			},
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:    group,
			Versions: apiVersions,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Singular: name,
				Kind:     kind,
				ListKind: kind + "List",
				Plural:   name + "s",
			},
			Scope: apiextensionsv1beta1.NamespaceScoped,
		},
	}

	// create CRD
	_, err = apiExtensionClient.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	framework.ExpectError(err, "create custom resource definition %s should be denied by webhook", crd.Name)
	expectedErrMsg := "the crd contains unwanted label"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		framework.Failf("expect error contains %q, got %q", expectedErrMsg, err.Error())
	}
}

func registerSlowWebhook(f *framework.Framework, context *certContext, policy *admissionregistrationv1beta1.FailurePolicyType, timeout *int32) func() {
	client := f.ClientSet
	ginkgo.By("Registering slow webhook via the AdmissionRegistration API")

	namespace := f.Namespace.Name
	configName := slowWebhookConfigName

	// Add a unique label to the namespace
	ns, err := client.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	framework.ExpectNoError(err, "error getting namespace %s", namespace)
	if ns.Labels == nil {
		ns.Labels = map[string]string{}
	}
	ns.Labels[slowWebhookConfigName] = namespace
	_, err = client.CoreV1().Namespaces().Update(ns)
	framework.ExpectNoError(err, "error labeling namespace %s", namespace)

	_, err = client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Create(&admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
			{
				Name: "allow-configmap-with-delay-webhook.k8s.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"configmaps"},
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      strPtr("/always-allow-delay-5s"),
						Port:      pointer.Int32Ptr(servicePort),
					},
					CABundle: context.signingCert,
				},
				// Scope the webhook to just this namespace
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: ns.Labels,
				},
				FailurePolicy:  policy,
				TimeoutSeconds: timeout,
			},
		},
	})
	framework.ExpectNoError(err, "registering slow webhook config %s with namespace %s", configName, namespace)

	// The webhook configuration is honored in 10s.
	time.Sleep(10 * time.Second)

	return func() {
		client.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Delete(configName, nil)
	}
}

func testSlowWebhookTimeoutFailEarly(f *framework.Framework) {
	ginkgo.By("Request fails when timeout (1s) is shorter than slow webhook latency (5s)")
	client := f.ClientSet
	name := "e2e-test-slow-webhook-configmap"
	_, err := client.CoreV1().ConfigMaps(f.Namespace.Name).Create(&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name}})
	framework.ExpectError(err, "create configmap in namespace %s should have timed-out reaching slow webhook", f.Namespace.Name)
	expectedErrMsg := `/always-allow-delay-5s?timeout=1s: context deadline exceeded`
	if !strings.Contains(err.Error(), expectedErrMsg) {
		framework.Failf("expect error contains %q, got %q", expectedErrMsg, err.Error())
	}
}

func testSlowWebhookTimeoutNoError(f *framework.Framework) {
	client := f.ClientSet
	name := "e2e-test-slow-webhook-configmap"
	_, err := client.CoreV1().ConfigMaps(f.Namespace.Name).Create(&v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name}})
	gomega.Expect(err).To(gomega.BeNil())
	err = client.CoreV1().ConfigMaps(f.Namespace.Name).Delete(name, &metav1.DeleteOptions{})
	gomega.Expect(err).To(gomega.BeNil())
}

// createAdmissionWebhookMultiVersionTestCRDWithV1Storage creates a new CRD specifically
// for the admissin webhook calling test.
func createAdmissionWebhookMultiVersionTestCRDWithV1Storage(f *framework.Framework, opts ...crd.Option) (*crd.TestCrd, error) {
	group := fmt.Sprintf("%s-multiversion-crd-test.k8s.io", f.BaseName)
	return crd.CreateMultiVersionTestCRD(f, group, append([]crd.Option{func(crd *apiextensionsv1beta1.CustomResourceDefinition) {
		crd.Spec.Versions = []apiextensionsv1beta1.CustomResourceDefinitionVersion{
			{
				Name:    "v1",
				Served:  true,
				Storage: true,
			},
			{
				Name:    "v2",
				Served:  true,
				Storage: false,
			},
		}
	}}, opts...)...)
}

// servedAPIVersions returns the API versions served by the CRD.
func servedAPIVersions(crd *apiextensionsv1beta1.CustomResourceDefinition) []string {
	ret := []string{}
	for _, v := range crd.Spec.Versions {
		if v.Served {
			ret = append(ret, v.Name)
		}
	}
	return ret
}
