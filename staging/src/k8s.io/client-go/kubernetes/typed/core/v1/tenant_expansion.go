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

package v1

import v1 "k8s.io/api/core/v1"

// The TenantExpansion interface allows manually adding extra methods to the TenantInterface.
type TenantExpansion interface {
	Finalize(item *v1.Tenant) (*v1.Tenant, error)
}

// Finalize takes the representation of a tenant to update.  Returns the server's representation of the tenant, and an error, if it occurs.
func (c *tenants) Finalize(tenant *v1.Tenant) (result *v1.Tenant, err error) {
	result = &v1.Tenant{}
	err = c.client.Put().Resource("tenants").Name(tenant.Name).SubResource("finalize").Body(tenant).Do().Into(result)
	return
}
