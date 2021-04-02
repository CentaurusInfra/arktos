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

package v1

import (
	internalinterfaces "k8s.io/client-go/informers/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// Actions returns a ActionInformer.
	Actions() ActionInformer
	// ComponentStatuses returns a ComponentStatusInformer.
	ComponentStatuses() ComponentStatusInformer
	// ConfigMaps returns a ConfigMapInformer.
	ConfigMaps() ConfigMapInformer
	// ControllerInstances returns a ControllerInstanceInformer.
	ControllerInstances() ControllerInstanceInformer
	// DataPartitionConfigs returns a DataPartitionConfigInformer.
	DataPartitionConfigs() DataPartitionConfigInformer
	// Endpoints returns a EndpointsInformer.
	Endpoints() EndpointsInformer
	// Events returns a EventInformer.
	Events() EventInformer
	// LimitRanges returns a LimitRangeInformer.
	LimitRanges() LimitRangeInformer
	// Namespaces returns a NamespaceInformer.
	Namespaces() NamespaceInformer
	// Nodes returns a NodeInformer.
	Nodes() NodeInformer
	// PersistentVolumes returns a PersistentVolumeInformer.
	PersistentVolumes() PersistentVolumeInformer
	// PersistentVolumeClaims returns a PersistentVolumeClaimInformer.
	PersistentVolumeClaims() PersistentVolumeClaimInformer
	// Pods returns a PodInformer.
	Pods() PodInformer
	// PodTemplates returns a PodTemplateInformer.
	PodTemplates() PodTemplateInformer
	// ReplicationControllers returns a ReplicationControllerInformer.
	ReplicationControllers() ReplicationControllerInformer
	// ResourceQuotas returns a ResourceQuotaInformer.
	ResourceQuotas() ResourceQuotaInformer
	// Secrets returns a SecretInformer.
	Secrets() SecretInformer
	// Services returns a ServiceInformer.
	Services() ServiceInformer
	// ServiceAccounts returns a ServiceAccountInformer.
	ServiceAccounts() ServiceAccountInformer
	// StorageClusters returns a StorageClusterInformer.
	StorageClusters() StorageClusterInformer
	// Tenants returns a TenantInformer.
	Tenants() TenantInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	tenant           string
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, tenant: "all", namespace: namespace, tweakListOptions: tweakListOptions}
}

func NewWithMultiTenancy(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc, tenant string) Interface {

	return &version{factory: f, tenant: tenant, namespace: namespace, tweakListOptions: tweakListOptions}
}

// Actions returns a ActionInformer.
func (v *version) Actions() ActionInformer {
	return &actionInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// ComponentStatuses returns a ComponentStatusInformer.
func (v *version) ComponentStatuses() ComponentStatusInformer {
	return &componentStatusInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// ConfigMaps returns a ConfigMapInformer.
func (v *version) ConfigMaps() ConfigMapInformer {
	return &configMapInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// ControllerInstances returns a ControllerInstanceInformer.
func (v *version) ControllerInstances() ControllerInstanceInformer {
	return &controllerInstanceInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// DataPartitionConfigs returns a DataPartitionConfigInformer.
func (v *version) DataPartitionConfigs() DataPartitionConfigInformer {
	return &dataPartitionConfigInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// Endpoints returns a EndpointsInformer.
func (v *version) Endpoints() EndpointsInformer {
	return &endpointsInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// Events returns a EventInformer.
func (v *version) Events() EventInformer {
	return &eventInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// LimitRanges returns a LimitRangeInformer.
func (v *version) LimitRanges() LimitRangeInformer {
	return &limitRangeInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// Namespaces returns a NamespaceInformer.
func (v *version) Namespaces() NamespaceInformer {
	return &namespaceInformer{factory: v.factory, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// Nodes returns a NodeInformer.
func (v *version) Nodes() NodeInformer {
	return &nodeInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// PersistentVolumes returns a PersistentVolumeInformer.
func (v *version) PersistentVolumes() PersistentVolumeInformer {
	return &persistentVolumeInformer{factory: v.factory, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// PersistentVolumeClaims returns a PersistentVolumeClaimInformer.
func (v *version) PersistentVolumeClaims() PersistentVolumeClaimInformer {
	return &persistentVolumeClaimInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// Pods returns a PodInformer.
func (v *version) Pods() PodInformer {
	return &podInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// PodTemplates returns a PodTemplateInformer.
func (v *version) PodTemplates() PodTemplateInformer {
	return &podTemplateInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// ReplicationControllers returns a ReplicationControllerInformer.
func (v *version) ReplicationControllers() ReplicationControllerInformer {
	return &replicationControllerInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// ResourceQuotas returns a ResourceQuotaInformer.
func (v *version) ResourceQuotas() ResourceQuotaInformer {
	return &resourceQuotaInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// Secrets returns a SecretInformer.
func (v *version) Secrets() SecretInformer {
	return &secretInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// Services returns a ServiceInformer.
func (v *version) Services() ServiceInformer {
	return &serviceInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// ServiceAccounts returns a ServiceAccountInformer.
func (v *version) ServiceAccounts() ServiceAccountInformer {
	return &serviceAccountInformer{factory: v.factory, namespace: v.namespace, tenant: v.tenant, tweakListOptions: v.tweakListOptions}
}

// StorageClusters returns a StorageClusterInformer.
func (v *version) StorageClusters() StorageClusterInformer {
	return &storageClusterInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// Tenants returns a TenantInformer.
func (v *version) Tenants() TenantInformer {
	return &tenantInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}
