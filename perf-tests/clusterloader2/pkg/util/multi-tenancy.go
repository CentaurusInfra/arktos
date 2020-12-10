/*
Copyright 2019 The Kubernetes Authors.

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

package util

import (
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
)

func GetTenant() string {
	tenantName := os.Getenv("SCALEOUT_TEST_TENANT")
    if len(tenantName) == 0 {
        return metav1.TenantSystem
	}
	
	errs := apimachineryvalidation.ValidateTenantName(tenantName, false)
	if len(errs) > 0 {
		klog.Fatalf("Invalide tenant name %v: %v", tenantName, errs)
	}

    return strings.ToLower(tenantName)
}


