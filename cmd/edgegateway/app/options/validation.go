package options

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "k8s.io/kubernetes/pkg/apis/edgegateway/v1"
)

// Validate checks Config and return a slice of found errs
func Validate(c *v1.EdgeGatewayConfig) []error {
	var errs []error
	errs = append(errs, ValidateModuleEdgeHub(*c.Modules.EdgeHub)...)
	return errs
}

// ValidateModuleEdgeHub validates and return a slice of found errs
func ValidateModuleEdgeHub(h v1.EdgeHub) []error {
	var errs []error
	if !h.Enable {
		return errs
	}

	if h.WebSocket.Enable == h.Quic.Enable {
		errs = append(errs, field.Invalid(field.NewPath("enable"),
			h.Quic.Enable, "websocket.enable and quic.enable cannot be true and false at the same time"))
	}

	return errs
}
