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

package nodelifecycle

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"

	"k8s.io/client-go/informers"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type TenantPartitionManager struct {
	Client            clientset.Interface
	PodInformer       coreinformers.PodInformer
	PodGetter         corelisters.PodLister
	DaemonSetInformer appsv1informers.DaemonSetInformer
	DaemonSetStore    appsv1listers.DaemonSetLister
}

func GetInsecureClient(ipAddr string) (clientset.Interface, error) {
	cfg, err := GetInsecureConfig(ipAddr)
	if err != nil {
		return nil, fmt.Errorf("Error building Tenant Partition Config to %v: %s", ipAddr, err.Error())
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building kubernetes clientset to %v: %s", ipAddr, err.Error())
	}

	return client, nil
}

func GetInsecureConfig(tenantServerUrl string) (*rest.Config, error) {
	template := `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %v
  name: tenant-partition-cluster
contexts:
- context:
    cluster: tenant-partition-cluster
  name: node-controller-context
current-context: node-controller-context
`

	content := fmt.Sprintf(template, tenantServerUrl)

	tmpfile, err := ioutil.TempFile("", "kubeconfig")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpfile.Name())

	if err := ioutil.WriteFile(tmpfile.Name(), []byte(content), 0666); err != nil {
		return nil, err
	}

	return clientcmd.BuildConfigFromFlags("", tmpfile.Name())
}

func GetTenantPartitionClients(tenantServers []string) ([]clientset.Interface, error) {

	for _, tenantServer := range tenantServers {
		if err := validateUrl(tenantServer); err != nil {
			return nil, err
		}
	}

	clients := []clientset.Interface{}

	for _, tenantServer := range tenantServers {
		tenantServerUrl := strings.TrimSpace(tenantServer)
		client, err := GetInsecureClient(tenantServerUrl)
		if err != nil {
			return nil, fmt.Errorf("Error in getting client for tenant partition (%v) ： %v", tenantServerUrl, err)
		}

		clients = append(clients, client)
	}

	return clients, nil
}

func getTenantPartitionManagerFromClient(client clientset.Interface, stop <-chan struct{}) *TenantPartitionManager {
	tpInformer := informers.NewSharedInformerFactory(client, 0)
	go tpInformer.Core().V1().Pods().Informer().Run(stop)
	go tpInformer.Apps().V1().DaemonSets().Informer().Run(stop)
	tpAccessor := &TenantPartitionManager{
		Client:            client,
		PodInformer:       tpInformer.Core().V1().Pods(),
		PodGetter:         tpInformer.Core().V1().Pods().Lister(),
		DaemonSetInformer: tpInformer.Apps().V1().DaemonSets(),
		DaemonSetStore:    tpInformer.Apps().V1().DaemonSets().Lister(),
	}

	return tpAccessor
}

func GetTenantPartitionManagersFromKubeClients(clients []clientset.Interface, stop <-chan struct{}) ([]*TenantPartitionManager, error) {
	tpAccessors := []*TenantPartitionManager{}

	for _, client := range clients {
		tpAccessors = append(tpAccessors, getTenantPartitionManagerFromClient(client, stop))
	}

	return tpAccessors, nil
}

func GetTenantPartitionManagersFromServerNames(tenantServers []string, stop <-chan struct{}) ([]*TenantPartitionManager, error) {
	clients, err := GetTenantPartitionClients(tenantServers)
	if err != nil {
		return nil, err
	}

	return GetTenantPartitionManagersFromKubeClients(clients, stop)
}

func validateUrl(urlString string) error {
	_, err := url.ParseRequestURI(urlString)
	if err != nil {
		return err
	}

	u, err := url.Parse(urlString)
	if err != nil {
		return err
	}
	if u.Scheme == "" {
		return fmt.Errorf("Invalid url (%v): missing scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("Invalid url (%v): missing host")
	}

	host, _, _ := net.SplitHostPort(u.Host)
	if net.ParseIP(host) == nil {
		return fmt.Errorf("Invalid url (%v): invalid host IP")
	}

	return nil
}
