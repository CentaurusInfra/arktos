package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/api/types/v1alpha1"
)

// DefaultV1Alpha1Interface interface for default controller managers
type DefaultV1Alpha1Interface interface {
	ControllerManagers(namespace string) ControllerManagerInterface
}

// DefaultV1Alpha1Client client for default controller managers
type DefaultV1Alpha1Client struct {
	restClient rest.Interface
}

// NewForConfig new config for default controller managers
func NewForConfig(c *rest.Config) (*DefaultV1Alpha1Client, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: v1alpha1.GroupName, Version: v1alpha1.GroupVersion}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &DefaultV1Alpha1Client{restClient: client}, nil
}

// ControllerManagers interface for default controller managers
func (c *DefaultV1Alpha1Client) ControllerManagers(namespace string) ControllerManagerInterface {
	return &controllerManagerClient{
		restClient: c.restClient,
		ns:         namespace,
	}
}
