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

	// template and pod template shall have consistent network labels
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
