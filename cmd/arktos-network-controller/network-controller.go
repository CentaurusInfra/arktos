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

// This controller implementation is based on design doc docs/design-proposals/multi-tenancy/multi-tenancy-network.md

package main

import (
	"flag"
	"net"
	"time"

	arktosext "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	"k8s.io/arktos-ext/pkg/generated/informers/externalversions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"k8s.io/kubernetes/cmd/arktos-network-controller/app"
)

const (
	defaultWorkers           = 4
	defaultKubeAPIServerPort = 6443
)

var (
	masterURL         string
	kubeconfig        string
	domainName        string
	workers           int
	kubeAPIServerIP   string
	kubeAPIServerPort int
	resourceSuffix    string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	if workers <= 0 {
		workers = defaultWorkers
	}

	if len(kubeAPIServerIP) == 0 {
		klog.Fatalf("--kube-apiserver-ip arg must be specified in this version.")
	}

	if net.ParseIP(kubeAPIServerIP) == nil {
		klog.Fatalf("--kube-apiserver-ip must be the valid ip address of kube-apiserver.")
	}

	defer klog.Flush()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("error getting client config: %s", err.Error())
	}

	netClient, err := arktosext.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("error building Arktos extension client: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("error building Kubernetes client: %s", err.Error())
	}

	informerFactory := externalversions.NewSharedInformerFactory(netClient, 10*time.Minute)
	stopCh := make(chan struct{})
	defer close(stopCh)

	netInformer := informerFactory.Arktos().V1().Networks()
	controller := app.New(resourceSuffix, domainName, kubeAPIServerIP, kubeAPIServerPort, netClient, kubeClient, netInformer)
	netInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.Add(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.Update(oldObj, newObj)
		},
	})

	informerFactory.Start(stopCh)
	controller.Run(workers, stopCh)

	klog.Infof("arktos network controller exited")
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.IntVar(&workers, "concurrent-workers", defaultWorkers, "The number of workers that are allowed to process concurrently.")
	flag.StringVar(&domainName, "cluster-domain", "cluster.local", "the cluster-internal domain name for Services.")
	flag.StringVar(&kubeAPIServerIP, "kube-apiserver-ip", "", "the ip address kube-apiserver is listening at.")
	flag.StringVar(&resourceSuffix, "resource-name-salt", "", "the salt suffix literal to append to sa/configmap names")
	flag.IntVar(&kubeAPIServerPort, "kube-apiserver-port", defaultKubeAPIServerPort, "the port number kube-apiserver is listening on.")
}
