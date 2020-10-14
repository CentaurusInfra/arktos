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

package apimachinery

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/onsi/ginkgo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
)

var serverAggregatorVersion = utilversion.MustParseSemantic("v1.10.0")

const (
	aggregatorServicePort = 7443
)

var _ = SIGDescribe("Aggregator", func() {
	var ns string
	var c clientset.Interface
	var aggrclient *aggregatorclient.Clientset

	// BeforeEachs run in LIFO order, AfterEachs run in FIFO order.
	// We want cleanTest to happen before the namespace cleanup AfterEach
	// inserted by NewDefaultFramework, so we put this AfterEach in front
	// of NewDefaultFramework.
	ginkgo.AfterEach(func() {
		cleanTest(c, aggrclient, ns)
	})

	f := framework.NewDefaultFramework("aggregator")

	// We want namespace initialization BeforeEach inserted by
	// NewDefaultFramework to happen before this, so we put this BeforeEach
	// after NewDefaultFramework.
	ginkgo.BeforeEach(func() {
		c = f.ClientSet
		ns = f.Namespace.Name

		if aggrclient == nil {
			config, err := framework.LoadConfig()
			if err != nil {
				framework.Failf("could not load config: %v", err)
			}
			aggrclient, err = aggregatorclient.NewForConfig(config)
			if err != nil {
				framework.Failf("could not create aggregator client: %v", err)
			}
		}
	})
})

func cleanTest(client clientset.Interface, aggrclient *aggregatorclient.Clientset, namespace string) {
	// delete the APIService first to avoid causing discovery errors
	_ = aggrclient.ApiregistrationV1().APIServices().Delete("v1alpha1.wardle.k8s.io", nil)

	_ = client.AppsV1().Deployments(namespace).Delete("sample-apiserver-deployment", nil)
	_ = client.CoreV1().Secrets(namespace).Delete("sample-apiserver-secret", nil)
	_ = client.CoreV1().Services(namespace).Delete("sample-api", nil)
	_ = client.CoreV1().ServiceAccounts(namespace).Delete("sample-apiserver", nil)
	_ = client.RbacV1().RoleBindings("kube-system").Delete("wardler-auth-reader", nil)
	_ = client.RbacV1().ClusterRoleBindings().Delete("wardler:"+namespace+":auth-delegator", nil)
	_ = client.RbacV1().ClusterRoles().Delete("sample-apiserver-reader", nil)
	_ = client.RbacV1().ClusterRoleBindings().Delete("wardler:"+namespace+":sample-apiserver-reader", nil)
}

// pollTimed will call Poll but time how long Poll actually took.
// It will then e2elog.Logf the msg with the duration of the Poll.
// It is assumed that msg will contain one %s for the elapsed time.
func pollTimed(interval, timeout time.Duration, condition wait.ConditionFunc, msg string) error {
	defer func(start time.Time, msg string) {
		elapsed := time.Since(start)
		e2elog.Logf(msg, elapsed)
	}(time.Now(), msg)
	return wait.Poll(interval, timeout, condition)
}

func validateErrorWithDebugInfo(f *framework.Framework, err error, pods *v1.PodList, msg string, fields ...interface{}) {
	if err != nil {
		namespace := f.Namespace.Name
		msg := fmt.Sprintf(msg, fields...)
		msg += fmt.Sprintf(" but received unexpected error:\n%v", err)
		client := f.ClientSet
		ep, err := client.CoreV1().Endpoints(namespace).Get("sample-api", metav1.GetOptions{})
		if err == nil {
			msg += fmt.Sprintf("\nFound endpoints for sample-api:\n%v", ep)
		}
		pds, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{})
		if err == nil {
			msg += fmt.Sprintf("\nFound pods in %s:\n%v", namespace, pds)
			msg += fmt.Sprintf("\nOriginal pods in %s:\n%v", namespace, pods)
		}

		framework.Failf(msg)
	}
}

func generateFlunderName(base string) string {
	id, err := rand.Int(rand.Reader, big.NewInt(2147483647))
	if err != nil {
		return base
	}
	return fmt.Sprintf("%s-%d", base, id)
}
