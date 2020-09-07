package options

import (
	"fmt"
	"path"

	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
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

	validHTTPSPort := utilvalidation.IsValidPortNum(int(config.HTTPS.Port))
	validWPort := utilvalidation.IsValidPortNum(int(config.WebSocket.Port))
	validAddress := utilvalidation.IsValidIP(config.WebSocket.Address)
	validQPort := utilvalidation.IsValidPortNum(int(config.Quic.Port))
	validQAddress := utilvalidation.IsValidIP(config.Quic.Address)

	if len(validHTTPSPort) > 0 {
		for _, m := range validHTTPSPort {
			errs = append(errs, field.Invalid(field.NewPath("port"), config.HTTPS.Port, m))
		}
	}
	if len(validWPort) > 0 {
		for _, m := range validWPort {
			errs = append(errs, field.Invalid(field.NewPath("port"), config.WebSocket.Port, m))
		}
	}
	if len(validAddress) > 0 {
		for _, m := range validAddress {
			errs = append(errs, field.Invalid(field.NewPath("Address"), config.WebSocket.Address, m))
		}
	}
	if len(validQPort) > 0 {
		for _, m := range validQPort {
			errs = append(errs, field.Invalid(field.NewPath("port"), config.Quic.Port, m))
		}
	}
	if len(validQAddress) > 0 {
		for _, m := range validQAddress {
			errs = append(errs, field.Invalid(field.NewPath("Address"), config.Quic.Address, m))
		}
	}
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
