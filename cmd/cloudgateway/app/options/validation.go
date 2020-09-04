package options

import (
	"fmt"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	"path"
)

// Validate checks Config and return a slice of found errs
func Validate(config *v1.CloudGatewayConfig) []error {
	var errs []error
	errs = append(errs, ValidateKubeAPIConfig(config.KubeAPIConfig)...)
	errs = append(errs, ValidateModuleCloudHub(config.Modules.CloudHub)...)

	return errs
}

// ValidateModuleCloudHub validates and return a slice of found errs
func ValidateModuleCloudHub(config *v1.CloudHub) []error {
	var errs []error
	if !config.Enable {
		return errs
	}

	// TODO(liuzongbao): add cloudhub config validate
	return errs
}

// Validate kubeAPIConfig and return a slice of found errs
func ValidateKubeAPIConfig(config *v1.KubeAPIConfig) []error {
	var errs []error
	if config.KubeConfig != "" && !path.IsAbs(config.KubeConfig) {
		errs = append(errs, fmt.Errorf("kubeconfig need abs path"))
	}

	if config.Master == "" {
		errs = append(errs, fmt.Errorf("master for arktos api server url is missing"))
	}

	return errs
}
