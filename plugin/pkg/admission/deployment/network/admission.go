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

package network

import (
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	arktosv1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	apps "k8s.io/kubernetes/pkg/apis/apps"
)

// PluginName indicates name of admission plugin.
const PluginName = "DeploymentNetwork"

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

	d, ok := attributes.GetObject().(*apps.Deployment)
	if !ok {
		return apierrors.NewBadRequest("Resource was marked with kind Deployment but was unable to be converted")
	}

	// deployment and pod template shall have consistent network labels
	networkOfDeploy := d.ObjectMeta.Labels[arktosv1.NetworkLabel]
	networkInTemplate := d.Spec.Template.Labels[arktosv1.NetworkLabel]
	if networkOfDeploy != networkInTemplate {
		return apierrors.NewBadRequest("invalid deployment with conflicting network label")
	}

	return nil
}

func shouldIgnore(attributes admission.Attributes) bool {
	// ignore all calls to subresources or resources other than deployments.
	if len(attributes.GetSubresource()) != 0 || attributes.GetResource().GroupResource() != apps.Resource("deployments") {
		return true
	}

	return false
}

func new() *plugin {
	return &plugin{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}
