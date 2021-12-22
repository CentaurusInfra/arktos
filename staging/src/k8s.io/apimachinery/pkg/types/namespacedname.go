/*
Copyright 2015 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

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

package types

import (
	"fmt"
)

// NamespacedName comprises a resource name, with a mandatory namespace,
// rendered as "<namespace>/<name>".  Being a type captures intent and
// helps make sure that UIDs, namespaced names and non-namespaced names
// do not get conflated in code.  For most use cases, namespace and name
// will already have been format validated at the API entry point, so we
// don't do that here.  Where that's not the case (e.g. in testing),
// consider using NamespacedNameOrDie() in testing.go in this package.

type NamespacedName struct {
	Tenant    string
	Namespace string
	Name      string
}

const (
	Separator = '/'
)

// String returns the general purpose string representation
func (n NamespacedName) String() string {
	return fmt.Sprintf("%s%c%s%c%s", n.Tenant, Separator, n.Namespace, Separator, n.Name)
}

// NamespacednameWithTenantSource is used to identify a service and endpoint in scale out architecture
type NamespacednameWithTenantSource struct {
	TenantPartitionId int
	Tenant            string
	Namespace         string
	Name              string
}

// String returns the general purpose string representation
func (n NamespacednameWithTenantSource) String() string {
	return fmt.Sprintf("%s_%d/%s/%s", n.Tenant, n.TenantPartitionId, n.Namespace, n.Name)
}
