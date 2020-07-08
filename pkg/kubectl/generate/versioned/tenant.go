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

package versioned

import (
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/kubernetes/pkg/kubectl/generate"
)

// TenantGeneratorV1 supports stable generation of a Tenant
type TenantGeneratorV1 struct {
	// Name of Tenant
	Name string

	StorageClusterId string
}

// Ensure it supports the generator pattern that uses parameter injection
var _ generate.Generator = &TenantGeneratorV1{}

// Ensure it supports the generator pattern that uses parameters specified during construction
var _ generate.StructuredGenerator = &TenantGeneratorV1{}

// Generate returns a Tenant using the specified parameters
func (g TenantGeneratorV1) Generate(genericParams map[string]interface{}) (runtime.Object, error) {
	err := generate.ValidateParams(g.ParamNames(), genericParams)
	if err != nil {
		return nil, err
	}
	params := map[string]string{}
	for key, value := range genericParams {
		strVal, isString := value.(string)
		if !isString {
			return nil, fmt.Errorf("expected string, saw %v for '%s'", value, key)
		}
		params[key] = strVal
	}

	delegate := &TenantGeneratorV1{Name: params["name"], StorageClusterId: params["storagecluster"]}
	return delegate.StructuredGenerate()
}

// ParamNames returns the set of supported input parameters when using the parameter injection generator pattern
func (g TenantGeneratorV1) ParamNames() []generate.GeneratorParam {
	return []generate.GeneratorParam{
		{Name: "name", Required: true},
		{Name: "storagecluster", Required: true},
	}
}

// StructuredGenerate outputs a Tenant object using the configured fields
func (g *TenantGeneratorV1) StructuredGenerate() (runtime.Object, error) {
	if err := g.validate(); err != nil {
		return nil, err
	}
	Tenant := &v1.Tenant{}
	Tenant.Name = g.Name
	Tenant.Spec.StorageClusterId = g.StorageClusterId
	return Tenant, nil
}

// validate validates required fields are set to support structured generation
func (g *TenantGeneratorV1) validate() error {
	if len(g.Name) == 0 {
		return fmt.Errorf("name must be specified")
	}

	if _, err := diff.GetClusterIdFromString(g.StorageClusterId); err != nil {
		return fmt.Errorf("Invalid StorageClusterId: %v", err)
	}
	return nil
}
