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

package fake

import (
	v1 "k8s.io/api/core/v1"
	core "k8s.io/client-go/testing"
)

func (c *FakeTenants) Finalize(tenant *v1.Tenant) (*v1.Tenant, error) {
	action := core.CreateActionImpl{}
	action.Verb = "create"
	action.Resource = tenantsResource
	action.Subresource = "finalize"
	action.Object = tenant

	obj, err := c.Fake.Invokes(action, tenant)
	if obj == nil {
		return nil, err
	}

	return obj.(*v1.Tenant), err
}
