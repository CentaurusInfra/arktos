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

package defaultpodnetworkreadiness

import (
	"io"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	arktosv1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	api "k8s.io/kubernetes/pkg/apis/core"
)

const (
	// PluginName indicates name of admission plugin.
	PluginName = "DefaultPodNetworkReadiness"
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return new(), nil
	})
}

var _ admission.MutationInterface = &plugin{}

type plugin struct {
	*admission.Handler
}

// Admit makes an admission decision based on the request attributes
func (p plugin) Admit(attributes admission.Attributes, o admission.ObjectInterfaces) (err error) {
	if shouldIgnore(attributes) {
		return nil
	}

	pod, ok := attributes.GetObject().(*api.Pod)
	if !ok {
		return apierrors.NewBadRequest("Resource was marked with kind Pod but was unable to be converted")
	}

	if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.HostNetwork {
		return nil
	}

	if _, readinessExists := pod.Annotations[arktosv1.NetworkReadiness]; readinessExists {
		return nil
	}

	// no network-readiness yet; ensure annotation network-readiness=false is set by default
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = map[string]string{}
	}
	pod.ObjectMeta.Annotations[arktosv1.NetworkReadiness] = "false"
	return nil
}

func shouldIgnore(attributes admission.Attributes) bool {
	// ignore all calls to subresources or resources other than pods.
	if len(attributes.GetSubresource()) != 0 || attributes.GetResource().GroupResource() != api.Resource("pods") {
		return true
	}

	return false
}

func new() *plugin {
	return &plugin{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}
