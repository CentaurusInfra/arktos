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

package options

import (
	"github.com/spf13/pflag"

	tenantconfig "k8s.io/kubernetes/pkg/controller/tenant/config"
)

// TenantControllerOptions holds the TenantController options.
type TenantControllerOptions struct {
	*tenantconfig.TenantControllerConfiguration
}

// AddFlags adds flags related to TenantController for controller manager to the specified FlagSet.
func (o *TenantControllerOptions) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}

	fs.DurationVar(&o.TenantSyncPeriod.Duration, "tenant-sync-period", o.TenantSyncPeriod.Duration, "The period for syncing tenant life-cycle updates")
	fs.Int32Var(&o.ConcurrentTenantSyncs, "concurrent-tenant-syncs", o.ConcurrentTenantSyncs, "The number of tenant objects that are allowed to sync concurrently. Larger number = more responsive tenant termination, but more CPU (and network) load")
	fs.StringVar(&o.DefaultNetworkTemplatePath, "default-network-template-path", o.DefaultNetworkTemplatePath, "The path to the template file for the default network spec in JSON of tenants. Empty means system won't create  the default network for new tenants")
}

// ApplyTo fills up TenantController config with options.
func (o *TenantControllerOptions) ApplyTo(cfg *tenantconfig.TenantControllerConfiguration) error {
	if o == nil {
		return nil
	}

	cfg.TenantSyncPeriod = o.TenantSyncPeriod
	cfg.ConcurrentTenantSyncs = o.ConcurrentTenantSyncs
	cfg.DefaultNetworkTemplatePath = o.DefaultNetworkTemplatePath

	return nil
}

// Validate checks validation of TenantControllerOptions.
func (o *TenantControllerOptions) Validate() []error {
	if o == nil {
		return nil
	}

	errs := []error{}
	return errs
}
