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

package main

import (
	"errors"
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client metric registration
	"k8s.io/kubernetes/pkg/features"
	cadvisortest "k8s.io/kubernetes/pkg/kubelet/cadvisor/testing"
	"k8s.io/kubernetes/pkg/kubelet/cm"
	"k8s.io/kubernetes/pkg/kubelet/dockershim"
	"k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
	"k8s.io/kubernetes/pkg/kubelet/kubeclientmanager"
	"k8s.io/kubernetes/pkg/kubemark"
	"k8s.io/kubernetes/pkg/master/ports"
	fakeiptables "k8s.io/kubernetes/pkg/util/iptables/testing"
	fakesysctl "k8s.io/kubernetes/pkg/util/sysctl/testing"
	_ "k8s.io/kubernetes/pkg/version/prometheus" // for version metric registration
	"k8s.io/kubernetes/pkg/version/verflag"
	fakeexec "k8s.io/utils/exec/testing"
)

type hollowNodeConfig struct {
	KubeconfigPath       string
	KubeletPort          int
	KubeletReadOnlyPort  int
	Morph                string
	NodeName             string
	ServerPort           int
	ContentType          string
	UseRealProxier       bool
	ProxierSyncPeriod    time.Duration
	ProxierMinSyncPeriod time.Duration
	NodeLabels           map[string]string
	// protocal://ip:port format, e.g.:
	// http://172.31.8.177.8080
	TenantServers  []string
	ResourceServer string
}

const (
	maxPods     = 110
	podsPerCore = 0
)

// TODO(#45650): Refactor hollow-node into hollow-kubelet and hollow-proxy
// and make the config driven.
var knownMorphs = sets.NewString("kubelet", "proxy")

func (c *hollowNodeConfig) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ResourceServer, "resource-server", c.ResourceServer, "url to the resource partition master.")
	fs.StringSliceVar(&c.TenantServers, "tenant-servers", c.TenantServers, "Comma separated string representing tenant api-server URLs.")
	fs.StringVar(&c.KubeconfigPath, "kubeconfig", "/kubeconfig/kubeconfig", "Path to kubeconfig file.")
	fs.IntVar(&c.KubeletPort, "kubelet-port", ports.KubeletPort, "Port on which HollowKubelet should be listening.")
	fs.IntVar(&c.KubeletReadOnlyPort, "kubelet-read-only-port", ports.KubeletReadOnlyPort, "Read-only port on which Kubelet is listening.")
	fs.StringVar(&c.NodeName, "name", "fake-node", "Name of this Hollow Node.")
	fs.IntVar(&c.ServerPort, "api-server-port", 443, "Port on which API server is listening.")
	fs.StringVar(&c.Morph, "morph", "", fmt.Sprintf("Specifies into which Hollow component this binary should morph. Allowed values: %v", knownMorphs.List()))
	fs.StringVar(&c.ContentType, "kube-api-content-type", "application/vnd.kubernetes.protobuf", "ContentType of requests sent to apiserver.")
	fs.BoolVar(&c.UseRealProxier, "use-real-proxier", true, "Set to true if you want to use real proxier inside hollow-proxy.")
	fs.DurationVar(&c.ProxierSyncPeriod, "proxier-sync-period", 30*time.Second, "Period that proxy rules are refreshed in hollow-proxy.")
	fs.DurationVar(&c.ProxierMinSyncPeriod, "proxier-min-sync-period", 0, "Minimum period that proxy rules are refreshed in hollow-proxy.")
	bindableNodeLabels := cliflag.ConfigurationMap(c.NodeLabels)
	fs.Var(&bindableNodeLabels, "node-labels", "Additional node labels")
}

func (c *hollowNodeConfig) createClientConfigFromFile() (*restclient.Config, error) {
	clientConfigs, err := clientcmd.LoadFromFile(c.KubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("error while loading kubeconfig from file %v: %v", c.KubeconfigPath, err)
	}
	configs, err := clientcmd.NewDefaultClientConfig(*clientConfigs, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error while creating kubeconfig: %v", err)
	}
	for _, config := range configs.GetAllConfigs() {
		config.ContentType = c.ContentType
		config.QPS = 10
		config.Burst = 20
	}
	return configs, nil
}

func (c *hollowNodeConfig) createHollowKubeletOptions() *kubemark.HollowKubletOptions {
	return &kubemark.HollowKubletOptions{
		NodeName:            c.NodeName,
		KubeletPort:         c.KubeletPort,
		KubeletReadOnlyPort: c.KubeletReadOnlyPort,
		MaxPods:             maxPods,
		PodsPerCore:         podsPerCore,
		NodeLabels:          c.NodeLabels,
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	command := newHollowNodeCommand()

	// TODO: once we switch everything over to Cobra commands, we can go back to calling
	// cliflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
	// normalize func and add the go flag set by hand.
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	// cliflag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// newControllerManagerCommand creates a *cobra.Command object with default parameters
func newHollowNodeCommand() *cobra.Command {
	s := &hollowNodeConfig{
		NodeLabels: make(map[string]string),
	}

	cmd := &cobra.Command{
		Use:  "kubemark",
		Long: "kubemark",
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()
			run(s)
		},
	}
	s.addFlags(cmd.Flags())

	return cmd
}

func run(config *hollowNodeConfig) {
	if !knownMorphs.Has(config.Morph) {
		klog.Fatalf("Unknown morph: %v. Allowed values: %v", config.Morph, knownMorphs.List())
	}

	// create clients to communicate with API server.
	clientConfigs, err := config.createClientConfigFromFile()
	if err != nil {
		klog.Fatalf("Failed to create a ClientConfig: %v. Exiting.", err)
	}

	if len(config.TenantServers) == 0 {
		klog.V(3).Infof("TenantServers is not set. Default to single tenant partition and clientConfig setting")
		config.TenantServers = make([]string, 1)
		config.TenantServers[0] = clientConfigs.GetConfig().Host
	}
	if config.ResourceServer == "" {
		klog.V(3).Infof("Resource is not set. Default to clientConfig setting")
		config.ResourceServer = clientConfigs.GetConfig().Host
	}

	// initialize the kubeclient manager
	kubeclientmanager.NewKubeClientManager()

	numberTenantPartitions := len(config.TenantServers)
	clients := make([]clientset.Interface, numberTenantPartitions)
	for i := 0; i < len(clients); i++ {
		tenantCfg := restclient.CopyConfigs(clientConfigs)
		for _, cfg := range tenantCfg.GetAllConfigs() {
			cfg.Host = config.TenantServers[i]
		}
		clientFromConfig, err := clientset.NewForConfig(tenantCfg)
		if err != nil {
			klog.Fatalf("Failed to create a ClientSet: %v. Exiting.", err)
		}
		clients[i] = clientFromConfig
	}

	if config.Morph == "kubelet" {
		f, c := kubemark.GetHollowKubeletConfig(config.createHollowKubeletOptions())

		heartbeatClientConfigs := restclient.CopyConfigs(clientConfigs)
		for _, heartbeatClientConfig := range heartbeatClientConfigs.GetAllConfigs() {
			heartbeatClientConfig.Timeout = c.NodeStatusUpdateFrequency.Duration
			// if the NodeLease feature is enabled, the timeout is the minimum of the lease duration and status update frequency
			if utilfeature.DefaultFeatureGate.Enabled(features.NodeLease) {
				leaseTimeout := time.Duration(c.NodeLeaseDurationSeconds) * time.Second
				if heartbeatClientConfig.Timeout > leaseTimeout {
					heartbeatClientConfig.Timeout = leaseTimeout
				}
			}
			heartbeatClientConfig.QPS = float32(-1)
			if heartbeatClientConfig.Host != config.ResourceServer {
				heartbeatClientConfig.Host = config.ResourceServer
			}
		}
		heartbeatClient, err := clientset.NewForConfig(heartbeatClientConfigs)
		if err != nil {
			klog.Fatalf("Failed to create a ClientSet: %v. Exiting.", err)
		}

		cadvisorInterface := &cadvisortest.Fake{
			NodeName: config.NodeName,
		}
		containerManager := cm.NewStubContainerManager()

		fakeDockerClientConfig := &dockershim.ClientConfig{
			DockerEndpoint:    libdocker.FakeDockerEndpoint,
			EnableSleep:       true,
			WithTraceDisabled: true,
		}

		arktosExtClientConfig := *clientConfigs
		for _, cfg := range arktosExtClientConfig.GetAllConfigs() {
			cfg.ContentType = "application/json"
			cfg.AcceptContentTypes = "application/json"
		}
		arktosExtClient, err := arktos.NewForConfig(&arktosExtClientConfig)
		if err != nil {
			klog.Fatalf("Failed to create an arktos ClientSet: %v. Exiting.", err)
		}

		hollowKubelet := kubemark.NewHollowKubelet(
			f, c,
			clients,
			arktosExtClient,
			heartbeatClient,
			cadvisorInterface,
			fakeDockerClientConfig,
			containerManager,
		)
		hollowKubelet.Run()
	}

	if config.Morph == "proxy" {
		client, err := clientset.NewForConfig(clientConfigs)
		if err != nil {
			klog.Fatalf("Failed to create API Server client: %v", err)
		}
		iptInterface := fakeiptables.NewFake()
		sysctl := fakesysctl.NewFake()
		execer := &fakeexec.FakeExec{
			LookPathFunc: func(_ string) (string, error) { return "", errors.New("fake execer") },
		}
		eventBroadcaster := record.NewBroadcaster()
		recorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "kube-proxy", Host: config.NodeName})

		hollowProxy, err := kubemark.NewHollowProxyOrDie(
			config.NodeName,
			client,
			client.CoreV1(),
			iptInterface,
			sysctl,
			execer,
			eventBroadcaster,
			recorder,
			config.UseRealProxier,
			config.ProxierSyncPeriod,
			config.ProxierMinSyncPeriod,
		)
		if err != nil {
			klog.Fatalf("Failed to create hollowProxy instance: %v", err)
		}
		hollowProxy.Run()
	}
}
