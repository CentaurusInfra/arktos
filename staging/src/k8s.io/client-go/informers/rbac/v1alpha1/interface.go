/*
Copyright The Kubernetes Authors.
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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	internalinterfaces "k8s.io/client-go/informers/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// ClusterRoles returns a ClusterRoleInformer.
	ClusterRoles() ClusterRoleInformer
	// ClusterRoleBindings returns a ClusterRoleBindingInformer.
	ClusterRoleBindings() ClusterRoleBindingInformer
	// Roles returns a RoleInformer.
	Roles() RoleInformer
	// RoleBindings returns a RoleBindingInformer.
	RoleBindings() RoleBindingInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	tenant           string
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, tenant: "", namespace: namespace, tweakListOptions: tweakListOptions}
}

func NewWithMultiTenancy(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc, tenant string) Interface {

	return &version{factory: f, tenant: tenant, namespace: namespace, tweakListOptions: tweakListOptions}
}

// ClusterRoles returns a ClusterRoleInformer.
func (v *version) ClusterRoles() ClusterRoleInformer {
	return &clusterRoleInformer{factory: v.factory, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// ClusterRoleBindings returns a ClusterRoleBindingInformer.
func (v *version) ClusterRoleBindings() ClusterRoleBindingInformer {
	return &clusterRoleBindingInformer{factory: v.factory, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// Roles returns a RoleInformer.
func (v *version) Roles() RoleInformer {
	return &roleInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// RoleBindings returns a RoleBindingInformer.
func (v *version) RoleBindings() RoleBindingInformer {
	return &roleBindingInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}
