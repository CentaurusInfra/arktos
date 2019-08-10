package options

import (
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
)

// KubeControllerManagerOptions is the main context object for the kube-controller manager.
type CloudFabricControllerManagerOptions struct {
	/*
		Generic           *cmoptions.GenericControllerManagerConfigurationOptions
		KubeCloudShared   *cmoptions.KubeCloudSharedOptions
		ServiceController *cmoptions.ServiceControllerOptions
	*/

	SecureServing *apiserveroptions.SecureServingOptionsWithLoopback
	// TODO: remove insecure serving mode
	InsecureServing *apiserveroptions.DeprecatedInsecureServingOptionsWithLoopback
	Authentication  *apiserveroptions.DelegatingAuthenticationOptions
	Authorization   *apiserveroptions.DelegatingAuthorizationOptions

	Master     string
	Kubeconfig string
}
