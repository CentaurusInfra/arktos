/*
Copyright 2018 The Kubernetes Authors.
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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// VolumeConfiguration contains *all* enumerated flags meant to configure all volume
// plugins. From this config, the controller-manager binary will create many instances of
// volume.VolumeConfig, each containing only the configuration needed for that plugin which
// are then passed to the appropriate plugin. The ControllerManager binary is the only part
// of the code which knows what plugins are supported and which flags correspond to each plugin.
type VolumeConfiguration struct {
	// enableHostPathProvisioning enables HostPath PV provisioning when running without a
	// cloud provider. This allows testing and development of provisioning features. HostPath
	// provisioning is not supported in any way, won't work in a multi-node cluster, and
	// should not be used for anything other than testing or development.
	EnableHostPathProvisioning *bool
	// enableDynamicProvisioning enables the provisioning of volumes when running within an environment
	// that supports dynamic provisioning. Defaults to true.
	EnableDynamicProvisioning *bool
	// volumePluginDir is the full path of the directory in which the flex
	// volume plugin should search for additional third party volume plugins
	FlexVolumePluginDir string
}

// GroupResource describes an group resource.
type GroupResource struct {
	// group is the group portion of the GroupResource.
	Group string
	// resource is the resource portion of the GroupResource.
	Resource string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubeControllerManagerConfiguration contains elements describing kube-controller manager.
type KubeControllerManagerConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// Generic holds configuration for a generic controller-manager
	Generic GenericControllerManagerConfiguration
	// KubeCloudSharedConfiguration holds configuration for shared related features
	// both in cloud controller manager and kube-controller manager.
	KubeCloudShared KubeCloudSharedConfiguration

	// CSRSigningControllerConfiguration holds configuration for
	// CSRSigningController related features.
	CSRSigningController CSRSigningControllerConfiguration
	// DeprecatedControllerConfiguration holds configuration for some deprecated
	// features.
	DeprecatedController DeprecatedControllerConfiguration
	// EndpointControllerConfiguration holds configuration for EndpointController
	// related features.
	EndpointController EndpointControllerConfiguration
	// GarbageCollectorControllerConfiguration holds configuration for
	// GarbageCollectorController related features.
	GarbageCollectorController GarbageCollectorControllerConfiguration
	// NamespaceControllerConfiguration holds configuration for NamespaceController
	// related features.
	NamespaceController NamespaceControllerConfiguration
	// PodGCControllerConfiguration holds configuration for PodGCController
	// related features.
	PodGCController PodGCControllerConfiguration
	// ReplicaSetControllerConfiguration holds configuration for ReplicaSet related features.
	ReplicaSetController ReplicaSetControllerConfiguration
	// SAControllerConfiguration holds configuration for ServiceAccountController
	// related features.
	SAController SAControllerConfiguration
	// TenantControllerConfiguration holds configuration for TenantController
	// related features.
	TenantController TenantControllerConfiguration `json:"tenantController,omitempty"`
}

// GenericControllerManagerConfiguration holds configuration for a generic controller-manager.
type GenericControllerManagerConfiguration struct {
	// port is the port that the controller-manager's http service runs on.
	Port int32
	// address is the IP address to serve on (set to 0.0.0.0 for all interfaces).
	Address string
	// minResyncPeriod is the resync period in reflectors; will be random between
	// minResyncPeriod and 2*minResyncPeriod.
	MinResyncPeriod metav1.Duration
	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	ClientConnection componentbaseconfigv1alpha1.ClientConnectionConfiguration
	// How long to wait between starting controller managers
	ControllerStartInterval metav1.Duration
	// leaderElection defines the configuration of leader election client.
	LeaderElection componentbaseconfigv1alpha1.LeaderElectionConfiguration
	// Controllers is the list of controllers to enable or disable
	// '*' means "all enabled by default controllers"
	// 'foo' means "enable 'foo'"
	// '-foo' means "disable 'foo'"
	// first item for a particular name wins
	Controllers []string
	// DebuggingConfiguration holds configuration for Debugging related features.
	Debugging componentbaseconfigv1alpha1.DebuggingConfiguration
}

// KubeCloudSharedConfiguration contains elements shared by both kube-controller manager
// and cloud-controller manager, but not genericconfig.
type KubeCloudSharedConfiguration struct {
	// CloudProviderConfiguration holds configuration for CloudProvider related features.
	CloudProvider CloudProviderConfiguration
	// externalCloudVolumePlugin specifies the plugin to use when cloudProvider is "external".
	// It is currently used by the in repo cloud providers to handle node and volume control in the KCM.
	ExternalCloudVolumePlugin string
	// useServiceAccountCredentials indicates whether controllers should be run with
	// individual service account credentials.
	UseServiceAccountCredentials bool
	// run with untagged cloud instances
	AllowUntaggedCloud bool
	// routeReconciliationPeriod is the period for reconciling routes created for Nodes by cloud provider..
	RouteReconciliationPeriod metav1.Duration
	// nodeMonitorPeriod is the period for syncing NodeStatus in NodeController.
	NodeMonitorPeriod metav1.Duration
	// clusterName is the instance prefix for the cluster.
	ClusterName string
	// clusterCIDR is CIDR Range for Pods in cluster.
	ClusterCIDR string
	// AllocateNodeCIDRs enables CIDRs for Pods to be allocated and, if
	// ConfigureCloudRoutes is true, to be set on the cloud provider.
	AllocateNodeCIDRs bool
	// CIDRAllocatorType determines what kind of pod CIDR allocator will be used.
	CIDRAllocatorType string
	// configureCloudRoutes enables CIDRs allocated with allocateNodeCIDRs
	// to be configured on the cloud provider.
	ConfigureCloudRoutes *bool
	// nodeSyncPeriod is the period for syncing nodes from cloudprovider. Longer
	// periods will result in fewer calls to cloud provider, but may delay addition
	// of new nodes to cluster.
	NodeSyncPeriod metav1.Duration
}

// CloudProviderConfiguration contains basically elements about cloud provider.
type CloudProviderConfiguration struct {
	// Name is the provider for cloud services.
	Name string
	// cloudConfigFile is the path to the cloud provider configuration file.
	CloudConfigFile string
}

// CSRSigningControllerConfiguration contains elements describing CSRSigningController.
type CSRSigningControllerConfiguration struct {
	// clusterSigningCertFile is the filename containing a PEM-encoded
	// X509 CA certificate used to issue cluster-scoped certificates
	ClusterSigningCertFile string
	// clusterSigningCertFile is the filename containing a PEM-encoded
	// RSA or ECDSA private key used to issue cluster-scoped certificates
	ClusterSigningKeyFile string
	// clusterSigningDuration is the length of duration signed certificates
	// will be given.
	ClusterSigningDuration metav1.Duration
}

// DeprecatedControllerConfiguration contains elements be deprecated.
type DeprecatedControllerConfiguration struct {
	// DEPRECATED: deletingPodsQps is the number of nodes per second on which pods are deleted in
	// case of node failure.
	DeletingPodsQPS float32
	// DEPRECATED: deletingPodsBurst is the number of nodes on which pods are bursty deleted in
	// case of node failure. For more details look into RateLimiter.
	DeletingPodsBurst int32
	// registerRetryCount is the number of retries for initial node registration.
	// Retry interval equals node-sync-period.
	RegisterRetryCount int32
}

// EndpointControllerConfiguration contains elements describing EndpointController.
type EndpointControllerConfiguration struct {
	// concurrentEndpointSyncs is the number of endpoint syncing operations
	// that will be done concurrently. Larger number = faster endpoint updating,
	// but more CPU (and network) load.
	ConcurrentEndpointSyncs int32
}

// GarbageCollectorControllerConfiguration contains elements describing GarbageCollectorController.
type GarbageCollectorControllerConfiguration struct {
	// enables the generic garbage collector. MUST be synced with the
	// corresponding flag of the kube-apiserver. WARNING: the generic garbage
	// collector is an alpha feature.
	EnableGarbageCollector *bool
	// concurrentGCSyncs is the number of garbage collector workers that are
	// allowed to sync concurrently.
	ConcurrentGCSyncs int32
	// gcIgnoredResources is the list of GroupResources that garbage collection should ignore.
	GCIgnoredResources []GroupResource
}

// NamespaceControllerConfiguration contains elements describing NamespaceController.
type NamespaceControllerConfiguration struct {
	// namespaceSyncPeriod is the period for syncing namespace life-cycle
	// updates.
	NamespaceSyncPeriod metav1.Duration
	// concurrentNamespaceSyncs is the number of namespace objects that are
	// allowed to sync concurrently.
	ConcurrentNamespaceSyncs int32
}

// PodGCControllerConfiguration contains elements describing PodGCController.
type PodGCControllerConfiguration struct {
	// terminatedPodGCThreshold is the number of terminated pods that can exist
	// before the terminated pod garbage collector starts deleting terminated pods.
	// If <= 0, the terminated pod garbage collector is disabled.
	TerminatedPodGCThreshold int32
}

// ReplicaSetControllerConfiguration contains elements describing ReplicaSetController.
type ReplicaSetControllerConfiguration struct {
	// concurrentRSSyncs is the number of replica sets that are  allowed to sync
	// concurrently. Larger number = more responsive replica  management, but more
	// CPU (and network) load.
	ConcurrentRSSyncs int32
}

// SAControllerConfiguration contains elements describing ServiceAccountController.
type SAControllerConfiguration struct {
	// serviceAccountKeyFile is the filename containing a PEM-encoded private RSA key
	// used to sign service account tokens.
	ServiceAccountKeyFile string
	// concurrentSATokenSyncs is the number of service account token syncing operations
	// that will be done concurrently.
	ConcurrentSATokenSyncs int32
	// rootCAFile is the root certificate authority will be included in service
	// account's token secret. This must be a valid PEM-encoded CA bundle.
	RootCAFile string
}

// TenantControllerConfiguration contains elements describing TenantController.
type TenantControllerConfiguration struct {
	// TenantSyncPeriod is the period for syncing tenant life-cycle
	// updates.
	TenantSyncPeriod metav1.Duration `json:"tenantSyncPeriod,omitempty"`
	// concurrentTenantSyncs is the number of tenant objects that are
	// allowed to sync concurrently.
	ConcurrentTenantSyncs int32 `json:"concurrentTenantSyncs,omitempty"`
	// DefaultNetworkTemplatePath is the path to json-formatted template file of default network in tenant space
	DefaultNetworkTemplatePath string `json:"defaultNetworkTemplatePath,omitempty"`
}
