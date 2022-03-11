/*
Copyright 2022 Authors of Arktos.

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

package options

import (
	"github.com/spf13/pflag"
	mizararktosnetworkconfig "k8s.io/kubernetes/pkg/controller/mizar/config"
)

// MizarArktosNetworkControllerOptions holds the MizarArktosNetworkController options
type MizarArktosNetworkControllerOptions struct {
	*mizararktosnetworkconfig.MizarArktosNetworkControllerConfiguration
}

// AddFlags adds flags related to MizarArktosNetworkController for controller manager to the specified FlagSet.
func (o *MizarArktosNetworkControllerOptions) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}

	fs.Int32Var(&o.VPCRangeStart, "vpc-range-start", o.VPCRangeStart, "Current tenant partition Mizar cniplugin VPC Class A IP start at")
	fs.Int32Var(&o.VPCRangeEnd, "vpc-range-end", o.VPCRangeEnd, "Current tenant partition Mizar cniplugin VPC Class A IP end at")
}

// ApplyTo fills up MizarArktosNetworkController config with options.
func (o *MizarArktosNetworkControllerOptions) ApplyTo(cfg *mizararktosnetworkconfig.MizarArktosNetworkControllerConfiguration) error {
	if o == nil {
		return nil
	}

	cfg.VPCRangeStart = o.VPCRangeStart
	cfg.VPCRangeEnd = o.VPCRangeEnd

	return nil
}

// Validate checks validation of MizarArktosNetworkControllerOptions.
func (o *MizarArktosNetworkControllerOptions) Validate() []error {
	if o == nil {
		return nil
	}

	errs := []error{}
	return errs
}
