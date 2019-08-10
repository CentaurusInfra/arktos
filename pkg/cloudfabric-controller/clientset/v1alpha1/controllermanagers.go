package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/api/types/v1alpha1"
)

// ControllerManagerInterface Controller Manager functions
type ControllerManagerInterface interface {
	List(opts metav1.ListOptions) (*v1alpha1.ControllerManagerList, error)
	Get(name string, options metav1.GetOptions) (*v1alpha1.ControllerManager, error)
	Create(*v1alpha1.ControllerManager) (*v1alpha1.ControllerManager, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
}

type controllerManagerClient struct {
	restClient rest.Interface
	ns         string
}

func (c *controllerManagerClient) List(opts metav1.ListOptions) (*v1alpha1.ControllerManagerList, error) {
	result := v1alpha1.ControllerManagerList{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource("controllermanagers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)

	return &result, err
}

func (c *controllerManagerClient) Get(name string, opts metav1.GetOptions) (*v1alpha1.ControllerManager, error) {
	result := v1alpha1.ControllerManager{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource("controllermanagers").
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)

	return &result, err
}

func (c *controllerManagerClient) Create(controllermanager *v1alpha1.ControllerManager) (*v1alpha1.ControllerManager, error) {
	result := v1alpha1.ControllerManager{}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource("controllermanagers").
		Body(controllermanager).
		Do().
		Into(&result)

	return &result, err
}

func (c *controllerManagerClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.restClient.
		Get().
		Namespace(c.ns).
		Resource("controllermanagers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}
