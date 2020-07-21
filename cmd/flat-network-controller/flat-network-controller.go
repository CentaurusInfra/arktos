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

// The external network controller is responsible for running controller loops for the flat network providers.
// Most of canonical CNI plugins can be used on so-called flat networks.

package main

import (
	"flag"
	"time"

	arktosext "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	"k8s.io/arktos-ext/pkg/generated/informers/externalversions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"k8s.io/kubernetes/cmd/flat-network-controller/app"
)

const defaultWorkers = 4

var (
	masterURL  string
	kubeconfig string
	domainName string
	workers    int
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	if workers <= 0 {
		workers = defaultWorkers
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
	controller := app.New(domainName, netClient, kubeClient, netInformer)
	netInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.Enqueue(obj)
		},
	})

	informerFactory.Start(stopCh)
	controller.Run(workers, stopCh)

	klog.Infof("flat network controller exited")
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.IntVar(&workers, "concurrent-workers", defaultWorkers, "The number of workers that are allowed to process concurrently.")
	flag.StringVar(&domainName, "cluster-domain", "cluster.local", "the cluster-internal domain name for Services.")
}
