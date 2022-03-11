// +build !ignore_autogenerated

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

// Code generated by conversion-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	v1alpha1 "k8s.io/kube-controller-manager/config/v1alpha1"
	config "k8s.io/kubernetes/pkg/controller/mizar/config"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*v1alpha1.GroupResource)(nil), (*v1.GroupResource)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_GroupResource_To_v1_GroupResource(a.(*v1alpha1.GroupResource), b.(*v1.GroupResource), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1.GroupResource)(nil), (*v1alpha1.GroupResource)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1_GroupResource_To_v1alpha1_GroupResource(a.(*v1.GroupResource), b.(*v1alpha1.GroupResource), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha1.MizarArktosNetworkControllerConfiguration)(nil), (*config.MizarArktosNetworkControllerConfiguration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_MizarArktosNetworkControllerConfiguration_To_config_MizarArktosNetworkControllerConfiguration(a.(*v1alpha1.MizarArktosNetworkControllerConfiguration), b.(*config.MizarArktosNetworkControllerConfiguration), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.MizarArktosNetworkControllerConfiguration)(nil), (*v1alpha1.MizarArktosNetworkControllerConfiguration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_MizarArktosNetworkControllerConfiguration_To_v1alpha1_MizarArktosNetworkControllerConfiguration(a.(*config.MizarArktosNetworkControllerConfiguration), b.(*v1alpha1.MizarArktosNetworkControllerConfiguration), scope)
	}); err != nil {
		return err
	}
	if err := s.AddConversionFunc((*config.MizarArktosNetworkControllerConfiguration)(nil), (*v1alpha1.MizarArktosNetworkControllerConfiguration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_MizarArktosNetworkControllerConfiguration_To_v1alpha1_MizarArktosNetworkControllerConfiguration(a.(*config.MizarArktosNetworkControllerConfiguration), b.(*v1alpha1.MizarArktosNetworkControllerConfiguration), scope)
	}); err != nil {
		return err
	}
	if err := s.AddConversionFunc((*v1alpha1.MizarArktosNetworkControllerConfiguration)(nil), (*config.MizarArktosNetworkControllerConfiguration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_MizarArktosNetworkControllerConfiguration_To_config_MizarArktosNetworkControllerConfiguration(a.(*v1alpha1.MizarArktosNetworkControllerConfiguration), b.(*config.MizarArktosNetworkControllerConfiguration), scope)
	}); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1alpha1_GroupResource_To_v1_GroupResource(in *v1alpha1.GroupResource, out *v1.GroupResource, s conversion.Scope) error {
	out.Group = in.Group
	out.Resource = in.Resource
	return nil
}

// Convert_v1alpha1_GroupResource_To_v1_GroupResource is an autogenerated conversion function.
func Convert_v1alpha1_GroupResource_To_v1_GroupResource(in *v1alpha1.GroupResource, out *v1.GroupResource, s conversion.Scope) error {
	return autoConvert_v1alpha1_GroupResource_To_v1_GroupResource(in, out, s)
}

func autoConvert_v1_GroupResource_To_v1alpha1_GroupResource(in *v1.GroupResource, out *v1alpha1.GroupResource, s conversion.Scope) error {
	out.Group = in.Group
	out.Resource = in.Resource
	return nil
}

// Convert_v1_GroupResource_To_v1alpha1_GroupResource is an autogenerated conversion function.
func Convert_v1_GroupResource_To_v1alpha1_GroupResource(in *v1.GroupResource, out *v1alpha1.GroupResource, s conversion.Scope) error {
	return autoConvert_v1_GroupResource_To_v1alpha1_GroupResource(in, out, s)
}

func autoConvert_v1alpha1_MizarArktosNetworkControllerConfiguration_To_config_MizarArktosNetworkControllerConfiguration(in *v1alpha1.MizarArktosNetworkControllerConfiguration, out *config.MizarArktosNetworkControllerConfiguration, s conversion.Scope) error {
	out.VPCRangeStart = in.VPCRangeStart
	out.VPCRangeEnd = in.VPCRangeEnd
	return nil
}

func autoConvert_config_MizarArktosNetworkControllerConfiguration_To_v1alpha1_MizarArktosNetworkControllerConfiguration(in *config.MizarArktosNetworkControllerConfiguration, out *v1alpha1.MizarArktosNetworkControllerConfiguration, s conversion.Scope) error {
	out.VPCRangeStart = in.VPCRangeStart
	out.VPCRangeEnd = in.VPCRangeEnd
	return nil
}
