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

package exists

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	genericadmissioninitializer "k8s.io/apiserver/pkg/admission/initializer"
	informers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// PluginName indicates name of admission plugin.
const PluginName = "TenantExists"

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return NewExists(), nil
	})
}

// Exists is an implementation of admission.Interface.
// It rejects all incoming requests if the tenant does not exist.

type Exists struct {
	*admission.Handler
	client       kubernetes.Interface
	tenantLister corev1listers.TenantLister
}

var _ admission.ValidationInterface = &Exists{}
var _ = genericadmissioninitializer.WantsExternalKubeInformerFactory(&Exists{})
var _ = genericadmissioninitializer.WantsExternalKubeClientSet(&Exists{})

// Validate makes an admission decision based on the request attributes
func (e *Exists) Validate(a admission.Attributes, o admission.ObjectInterfaces) error {
	// if we're here, then the API server has found a route, which means that if we have an empty tenant
	// it is a resource above any tenants.
	if len(a.GetTenant()) == 0 {
		return nil
	}

	// system tenant is inherently supported. It always exist even if it is not created explicitly
	if a.GetTenant() == metav1.TenantSystem {
		return nil
	}
	// we need to wait for our caches to warm
	if !e.WaitForReady() {
		return admission.NewForbidden(a, fmt.Errorf("not yet ready to handle request"))
	}
	_, err := e.tenantLister.Get(a.GetTenant())
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return errors.NewInternalError(err)
	}

	// in case of latency in our caches, make a call direct to storage to verify that it truly exists or not
	_, err = e.client.CoreV1().Tenants().Get(a.GetTenant(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return err
		}
		return errors.NewInternalError(err)
	}

	return nil
}

// NewExists creates a new tenant exists admission control handler
func NewExists() *Exists {
	return &Exists{
		Handler: admission.NewHandler(admission.Create, admission.Update, admission.Delete),
	}
}

// SetExternalKubeClientSet implements the WantsExternalKubeClientSet interface.
func (e *Exists) SetExternalKubeClientSet(client kubernetes.Interface) {
	e.client = client
}

// SetExternalKubeInformerFactory implements the WantsExternalKubeInformerFactory interface.
func (e *Exists) SetExternalKubeInformerFactory(f informers.SharedInformerFactory) {
	tenantInformer := f.Core().V1().Tenants()
	e.tenantLister = tenantInformer.Lister()
	e.SetReadyFunc(tenantInformer.Informer().HasSynced)
}

// ValidateInitialization implements the InitializationValidator interface.
func (e *Exists) ValidateInitialization() error {
	if e.tenantLister == nil {
		return fmt.Errorf("missing tenantLister")
	}
	if e.client == nil {
		return fmt.Errorf("missing client")
	}
	return nil
}
