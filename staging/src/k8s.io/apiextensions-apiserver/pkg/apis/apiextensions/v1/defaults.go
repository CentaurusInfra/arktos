/*
Copyright 2019 The Kubernetes Authors.
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

// File modified by cherrypick from kubernetes on 02/25/2021
package v1

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	utilpointer "k8s.io/utils/pointer"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_CustomResourceDefinition(obj *CustomResourceDefinition) {
	SetDefaults_CustomResourceDefinitionSpec(&obj.Spec)
	if len(obj.Status.StoredVersions) == 0 {
		for _, v := range obj.Spec.Versions {
			if v.Storage {
				obj.Status.StoredVersions = append(obj.Status.StoredVersions, v.Name)
				break
			}
		}
	}
}

func SetDefaults_CustomResourceDefinitionSpec(obj *CustomResourceDefinitionSpec) {
	if len(obj.Names.Singular) == 0 {
		obj.Names.Singular = strings.ToLower(obj.Names.Kind)
	}
	if len(obj.Names.ListKind) == 0 && len(obj.Names.Kind) > 0 {
		obj.Names.ListKind = obj.Names.Kind + "List"
	}
	if obj.Conversion == nil {
		obj.Conversion = &CustomResourceConversion{
			Strategy: NoneConverter,
		}
	}
}

// SetDefaults_ServiceReference sets defaults for Webhook's ServiceReference
func SetDefaults_ServiceReference(obj *ServiceReference) {
	if obj.Port == nil {
		obj.Port = utilpointer.Int32Ptr(443)
	}
}

// SetDefaults_JSONSchemaProps sets the defaults for JSONSchemaProps
func SetDefaults_JSONSchemaProps(obj *JSONSchemaProps) {
	if obj == nil {
		return
	}
	if obj.Type == "array" && obj.XListType == nil {
		obj.XListType = utilpointer.StringPtr("atomic")
	}
	if obj.Items != nil {
		SetDefaults_JSONSchemaProps(obj.Items.Schema)
		defaultJSONSchemaPropsArray(obj.Items.JSONSchemas)
	}
	defaultJSONSchemaPropsArray(obj.AllOf)
	defaultJSONSchemaPropsArray(obj.OneOf)
	defaultJSONSchemaPropsArray(obj.AnyOf)
	SetDefaults_JSONSchemaProps(obj.Not)
	defaultJSONSchemaPropsMap(obj.Properties)
	if obj.AdditionalProperties != nil {
		SetDefaults_JSONSchemaProps(obj.AdditionalProperties.Schema)
	}
	defaultJSONSchemaPropsMap(obj.PatternProperties)
	for i := range obj.Dependencies {
		SetDefaults_JSONSchemaProps(obj.Dependencies[i].Schema)
	}
	if obj.AdditionalItems != nil {
		SetDefaults_JSONSchemaProps(obj.AdditionalItems.Schema)
	}
	defaultJSONSchemaPropsMap(map[string]JSONSchemaProps(obj.Definitions))
}

func defaultJSONSchemaPropsArray(obj []JSONSchemaProps) {
	for i := range obj {
		SetDefaults_JSONSchemaProps(&obj[i])
	}
}

func defaultJSONSchemaPropsMap(obj map[string]JSONSchemaProps) {
	for i := range obj {
		props := obj[i]
		SetDefaults_JSONSchemaProps(&props)
		obj[i] = props
	}
}
