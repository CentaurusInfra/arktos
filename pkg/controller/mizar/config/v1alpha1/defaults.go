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

package v1alpha1

import (
	mizararktosnetworkconfigv1alpha1 "k8s.io/kube-controller-manager/config/v1alpha1"
)

// RecommendedDefaultTenantControllerConfiguration sets the default for tenantController configurations
func RecommendedDefaultTenantControllerConfiguration(obj *mizararktosnetworkconfigv1alpha1.MizarArktosNetworkControllerConfiguration) {
	// default to [11-99]
	if obj.VPCRangeStart == 0 {
		obj.VPCRangeStart = 11
	}
	if obj.VPCRangeEnd == 0 {
		obj.VPCRangeEnd = 99
	}
}
