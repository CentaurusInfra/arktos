/*
Copyright 2017 The Kubernetes Authors.
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

package options

import (
	"net"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	componentbaseconfig "k8s.io/component-base/config"
	cmoptions "k8s.io/kubernetes/cmd/controller-manager/app/options"
	kubectrlmgrconfig "k8s.io/kubernetes/pkg/controller/apis/config"
	csrsigningconfig "k8s.io/kubernetes/pkg/controller/certificates/signer/config"
	endpointconfig "k8s.io/kubernetes/pkg/controller/endpoint/config"
	garbagecollectorconfig "k8s.io/kubernetes/pkg/controller/garbagecollector/config"
	namespaceconfig "k8s.io/kubernetes/pkg/controller/namespace/config"
	podgcconfig "k8s.io/kubernetes/pkg/controller/podgc/config"
	replicasetconfig "k8s.io/kubernetes/pkg/controller/replicaset/config"
	serviceaccountconfig "k8s.io/kubernetes/pkg/controller/serviceaccount/config"
	tenantconfig "k8s.io/kubernetes/pkg/controller/tenant/config"
)

func TestAddFlags(t *testing.T) {
	fs := pflag.NewFlagSet("addflagstest", pflag.ContinueOnError)
	s, _ := NewKubeControllerManagerOptions()
	for _, f := range s.Flags([]string{""}, []string{""}).FlagSets {
		fs.AddFlagSet(f)
	}

	args := []string{
		"--address=192.168.4.10",
		"--allocate-node-cidrs=true",
		"--cidr-allocator-type=CloudAllocator",
		"--cloud-config=/cloud-config",
		"--cloud-provider=gce",
		"--cluster-cidr=1.2.3.4/24",
		"--cluster-name=k8s",
		"--cluster-signing-cert-file=/cluster-signing-cert",
		"--cluster-signing-key-file=/cluster-signing-key",
		"--concurrent-endpoint-syncs=10",
		"--concurrent-gc-syncs=30",
		"--concurrent-namespace-syncs=20",
		"--concurrent-replicaset-syncs=10",
		"--concurrent-serviceaccount-token-syncs=10",
		"--configure-cloud-routes=false",
		"--contention-profiling=true",
		"--controller-start-interval=2m",
		"--controllers=foo,bar",
		"--enable-garbage-collector=false",
		"--experimental-cluster-signing-duration=10h",
		"--http2-max-streams-per-connection=47",
		"--kube-api-burst=100",
		"--kube-api-content-type=application/json",
		"--kube-api-qps=50.0",
		"--kubeconfig=/kubeconfig",
		"--leader-elect=false",
		"--leader-elect-lease-duration=30s",
		"--leader-elect-renew-deadline=15s",
		"--leader-elect-resource-lock=configmap",
		"--leader-elect-retry-period=5s",
		"--master=192.168.4.20",
		"--min-resync-period=8h",
		"--namespace-sync-period=10m",
		"--node-monitor-period=10s",
		"--port=10000",
		"--profiling=false",
		"--route-reconciliation-period=30s",
		"--service-account-private-key-file=/service-account-private-key",
		"--terminated-pod-gc-threshold=12000",
		"--use-service-account-credentials=true",
		"--cert-dir=/a/b/c",
		"--bind-address=192.168.4.21",
		"--secure-port=10001",
		"--concurrent-tenant-syncs=20",
		"--tenant-sync-period=10m",
	}
	fs.Parse(args)
	// Sort GCIgnoredResources because it's built from a map, which means the
	// insertion order is random.
	sort.Sort(sortedGCIgnoredResources(s.GarbageCollectorController.GCIgnoredResources))

	expected := &KubeControllerManagerOptions{
		Generic: &cmoptions.GenericControllerManagerConfigurationOptions{
			GenericControllerManagerConfiguration: &kubectrlmgrconfig.GenericControllerManagerConfiguration{
				Port:            10252,     // Note: InsecureServingOptions.ApplyTo will write the flag value back into the component config
				Address:         "0.0.0.0", // Note: InsecureServingOptions.ApplyTo will write the flag value back into the component config
				MinResyncPeriod: metav1.Duration{Duration: 8 * time.Hour},
				ClientConnection: componentbaseconfig.ClientConnectionConfiguration{
					ContentType: "application/json",
					QPS:         50.0,
					Burst:       100,
				},
				ControllerStartInterval: metav1.Duration{Duration: 2 * time.Minute},
				LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
					ResourceLock:  "configmap",
					LeaderElect:   false,
					LeaseDuration: metav1.Duration{Duration: 30 * time.Second},
					RenewDeadline: metav1.Duration{Duration: 15 * time.Second},
					RetryPeriod:   metav1.Duration{Duration: 5 * time.Second},
				},
				Controllers: []string{"foo", "bar"},
			},
			Debugging: &cmoptions.DebuggingOptions{
				DebuggingConfiguration: &componentbaseconfig.DebuggingConfiguration{
					EnableProfiling:           false,
					EnableContentionProfiling: true,
				},
			},
		},
		KubeCloudShared: &cmoptions.KubeCloudSharedOptions{
			KubeCloudSharedConfiguration: &kubectrlmgrconfig.KubeCloudSharedConfiguration{
				UseServiceAccountCredentials: true,
				RouteReconciliationPeriod:    metav1.Duration{Duration: 30 * time.Second},
				NodeMonitorPeriod:            metav1.Duration{Duration: 10 * time.Second},
				ClusterName:                  "k8s",
				ClusterCIDR:                  "1.2.3.4/24",
				AllocateNodeCIDRs:            true,
				CIDRAllocatorType:            "CloudAllocator",
				ConfigureCloudRoutes:         false,
			},
			CloudProvider: &cmoptions.CloudProviderOptions{
				CloudProviderConfiguration: &kubectrlmgrconfig.CloudProviderConfiguration{
					Name:            "gce",
					CloudConfigFile: "/cloud-config",
				},
			},
		},
		CSRSigningController: &CSRSigningControllerOptions{
			&csrsigningconfig.CSRSigningControllerConfiguration{
				ClusterSigningCertFile: "/cluster-signing-cert",
				ClusterSigningKeyFile:  "/cluster-signing-key",
				ClusterSigningDuration: metav1.Duration{Duration: 10 * time.Hour},
			},
		},
		DeprecatedFlags: &DeprecatedControllerOptions{
			&kubectrlmgrconfig.DeprecatedControllerConfiguration{
				DeletingPodsQPS:    0.1,
				RegisterRetryCount: 10,
			},
		},
		EndpointController: &EndpointControllerOptions{
			&endpointconfig.EndpointControllerConfiguration{
				ConcurrentEndpointSyncs: 10,
			},
		},
		GarbageCollectorController: &GarbageCollectorControllerOptions{
			&garbagecollectorconfig.GarbageCollectorControllerConfiguration{
				ConcurrentGCSyncs: 30,
				GCIgnoredResources: []garbagecollectorconfig.GroupResource{
					{Group: "", Resource: "events"},
				},
				EnableGarbageCollector: false,
			},
		},
		NamespaceController: &NamespaceControllerOptions{
			&namespaceconfig.NamespaceControllerConfiguration{
				NamespaceSyncPeriod:      metav1.Duration{Duration: 10 * time.Minute},
				ConcurrentNamespaceSyncs: 20,
			},
		},
		PodGCController: &PodGCControllerOptions{
			&podgcconfig.PodGCControllerConfiguration{
				TerminatedPodGCThreshold: 12000,
			},
		},
		ReplicaSetController: &ReplicaSetControllerOptions{
			&replicasetconfig.ReplicaSetControllerConfiguration{
				ConcurrentRSSyncs: 10,
			},
		},
		SAController: &SAControllerOptions{
			&serviceaccountconfig.SAControllerConfiguration{
				ServiceAccountKeyFile:  "/service-account-private-key",
				ConcurrentSATokenSyncs: 10,
			},
		},
		TenantController: &TenantControllerOptions{
			&tenantconfig.TenantControllerConfiguration{
				TenantSyncPeriod:      metav1.Duration{Duration: 10 * time.Minute},
				ConcurrentTenantSyncs: 20,
			},
		},
		SecureServing: (&apiserveroptions.SecureServingOptions{
			BindPort:    10001,
			BindAddress: net.ParseIP("192.168.4.21"),
			ServerCert: apiserveroptions.GeneratableKeyCert{
				CertDirectory: "/a/b/c",
				PairName:      "kube-controller-manager",
			},
			HTTP2MaxStreamsPerConnection: 47,
		}).WithLoopback(),
		InsecureServing: (&apiserveroptions.DeprecatedInsecureServingOptions{
			BindAddress: net.ParseIP("192.168.4.10"),
			BindPort:    int(10000),
			BindNetwork: "tcp",
		}).WithLoopback(),
		Authentication: &apiserveroptions.DelegatingAuthenticationOptions{
			CacheTTL:   10 * time.Second,
			ClientCert: apiserveroptions.ClientCertAuthenticationOptions{},
			RequestHeader: apiserveroptions.RequestHeaderAuthenticationOptions{
				UsernameHeaders:     []string{"x-remote-user"},
				GroupHeaders:        []string{"x-remote-group"},
				ExtraHeaderPrefixes: []string{"x-remote-extra-"},
			},
			RemoteKubeConfigFileOptional: true,
		},
		Authorization: &apiserveroptions.DelegatingAuthorizationOptions{
			AllowCacheTTL:                10 * time.Second,
			DenyCacheTTL:                 10 * time.Second,
			RemoteKubeConfigFileOptional: true,
			AlwaysAllowPaths:             []string{"/healthz"}, // note: this does not match /healthz/ or /healthz/*
		},
		Kubeconfig: "/kubeconfig",
		Master:     "192.168.4.20",
	}

	// Sort GCIgnoredResources because it's built from a map, which means the
	// insertion order is random.
	sort.Sort(sortedGCIgnoredResources(expected.GarbageCollectorController.GCIgnoredResources))

	if !reflect.DeepEqual(expected, s) {
		t.Errorf("Got different run options than expected.\nDifference detected on:\n%s", diff.ObjectReflectDiff(expected, s))
	}
}

type sortedGCIgnoredResources []garbagecollectorconfig.GroupResource

func (r sortedGCIgnoredResources) Len() int {
	return len(r)
}

func (r sortedGCIgnoredResources) Less(i, j int) bool {
	if r[i].Group < r[j].Group {
		return true
	} else if r[i].Group > r[j].Group {
		return false
	}
	return r[i].Resource < r[j].Resource
}

func (r sortedGCIgnoredResources) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}
