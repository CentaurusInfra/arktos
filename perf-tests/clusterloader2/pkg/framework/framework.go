/*
Copyright 2018 The Kubernetes Authors.

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

package framework

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	"k8s.io/kubernetes/perf-tests/clusterloader2/pkg/config"
	"k8s.io/kubernetes/perf-tests/clusterloader2/pkg/errors"
	"k8s.io/kubernetes/perf-tests/clusterloader2/pkg/framework/client"
	"k8s.io/kubernetes/perf-tests/clusterloader2/pkg/util"

	// ensure auth plugins are loaded
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var tenantID = regexp.MustCompile(`^test-[a-z0-9]+-[0-9]+$`)

var namespaceID = regexp.MustCompile(`^test-[a-z0-9]+-[0-9]+$`)

// Framework allows for interacting with Kubernetes cluster via
// official Kubernetes client.
type Framework struct {
	automanagedTenantPrefix string
	automanagedTenantCount  int

	automanagedNamespacePrefix string
	automanagedNamespaceCount  int
	clientSets                 *MultiClientSet
	dynamicClients             *MultiDynamicClient
	clusterConfig              *config.ClusterConfig
}

// NewFramework creates new framework based on given clusterConfig.
func NewFramework(clusterConfig *config.ClusterConfig, clientsNumber int) (*Framework, error) {
	return newFramework(clusterConfig, clientsNumber, clusterConfig.KubeConfigPath)
}

// NewRootFramework creates framework for the root cluster.
// For clusters other than kubemark there is no difference between NewRootFramework and NewFramework.
func NewRootFramework(clusterConfig *config.ClusterConfig, clientsNumber int) (*Framework, error) {
	kubeConfigPath := clusterConfig.KubeConfigPath
	if clusterConfig.Provider == "kubemark" {
		kubeConfigPath = clusterConfig.KubemarkRootKubeConfigPath
	}
	return newFramework(clusterConfig, clientsNumber, kubeConfigPath)
}

func newFramework(clusterConfig *config.ClusterConfig, clientsNumber int, kubeConfigPath string) (*Framework, error) {
	var err error
	f := Framework{
		automanagedNamespaceCount: 0,
		clusterConfig:             clusterConfig,
	}
	if f.clientSets, err = NewMultiClientSet(kubeConfigPath, clientsNumber); err != nil {
		return nil, fmt.Errorf("multi client set creation error: %v", err)
	}
	if f.dynamicClients, err = NewMultiDynamicClient(kubeConfigPath, clientsNumber); err != nil {
		return nil, fmt.Errorf("multi dynamic client creation error: %v", err)
	}
	return &f, nil
}

// GetAutomanagedTenantPrefix returns automanaged tenant prefix.
func (f *Framework) GetAutomanagedTenantPrefix() string {
	return f.automanagedTenantPrefix
}

// SetAutomanagedTenantPrefix sets automanaged tenant prefix.
func (f *Framework) SetAutomanagedTenantPrefix(tenantName string) {
	f.automanagedTenantPrefix = tenantName
}

// GetAutomanagedNamespacePrefix returns automanaged namespace prefix.
func (f *Framework) GetAutomanagedNamespacePrefix() string {
	return f.automanagedNamespacePrefix
}

// SetAutomanagedNamespacePrefix sets automanaged namespace prefix.
func (f *Framework) SetAutomanagedNamespacePrefix(nsName string) {
	f.automanagedNamespacePrefix = nsName
}

// GetClientSets returns clientSet clients.
func (f *Framework) GetClientSets() *MultiClientSet {
	return f.clientSets
}

// GetDynamicClients returns dynamic clients.
func (f *Framework) GetDynamicClients() *MultiDynamicClient {
	return f.dynamicClients
}

// GetClusterConfig returns cluster config.
func (f *Framework) GetClusterConfig() *config.ClusterConfig {
	return f.clusterConfig
}

// CreateAutomanagedNamespaces creates automanged namespaces.
func (f *Framework) CreateAutomanagedNamespaces(tenant string, namespaceCount int) error {
	if f.automanagedNamespaceCount != 0 {
		//return fmt.Errorf("automanaged namespaces already created")
	}
	startpos := 0
	endpos := 0
	if f.clusterConfig.Apiserverextranum == 0 {
		for i := 1; i <= namespaceCount; i++ {
			name := fmt.Sprintf("%s-%v", util.RandomDNS1123String(6, startpos, endpos), f.automanagedNamespacePrefix)
			if err := client.CreateNamespace(f.clientSets.GetClient(), tenant, name); err != nil {
				return err
			}
			f.automanagedNamespaceCount++
		}
	} else {
		namespaceinterval := 0
		apiservernum := f.clusterConfig.Apiserverextranum + 1
		if namespaceCount%apiservernum > 0 {
			namespaceinterval = namespaceCount/apiservernum + 1
		} else {
			namespaceinterval = namespaceCount / apiservernum
		}
		for server := 1; server <= apiservernum; server++ {
			endpos = startpos + (26 / apiservernum)
			for i := 1; i <= namespaceinterval; i++ {
				name := fmt.Sprintf("%s-%v", util.RandomDNS1123String(6, startpos, endpos), f.automanagedNamespacePrefix)
				if err := client.CreateNamespace(f.clientSets.GetClient(), tenant, name); err != nil {
					return err
				}
				f.automanagedNamespaceCount++
			}
			if namespaceCount-f.automanagedNamespaceCount < namespaceinterval {
				namespaceinterval = namespaceCount - f.automanagedNamespaceCount
			}
			startpos = endpos

		}
		f.automanagedNamespaceCount++
	}

	return nil
}

// ListAutomanagedNamespaces returns all existing automanged namespace names.
func (f *Framework) ListAutomanagedNamespaces(tenant string) ([]string, []string, error) {
	var automanagedNamespacesList, staleNamespaces []string
	namespacesList, err := client.ListNamespaces(f.clientSets.GetClient(), tenant)
	if err != nil {
		return automanagedNamespacesList, staleNamespaces, err
	}
	for _, namespace := range namespacesList {
		matched, err := f.isAutomanagedNamespace(namespace.Name)
		if err != nil {
			return automanagedNamespacesList, staleNamespaces, err
		}
		if matched {
			automanagedNamespacesList = append(automanagedNamespacesList, namespace.Name)
		} else {
			// check further whether the namespace is a automanaged namespace created in previous test execution.
			// this could happen when the execution is aborted abornamlly, and the resource is not able to be
			// clean up.
			matched := f.isStaleAutomanagedNamespace(namespace.Name)
			if matched {
				staleNamespaces = append(staleNamespaces, namespace.Name)
			}
		}
	}
	return automanagedNamespacesList, staleNamespaces, nil
}

func (f *Framework) deleteNamespace(tenant string, namespace string) error {
	clientSet := f.clientSets.GetClient()
	if err := client.DeleteNamespace(clientSet, tenant, namespace); err != nil {
		return err
	}
	if err := client.WaitForDeleteNamespace(clientSet, tenant, namespace); err != nil {
		return err
	}
	return nil
}

// DeleteAutomanagedNamespaces deletes all automanged namespaces.
func (f *Framework) DeleteAutomanagedNamespaces(tenant string) *errors.ErrorList {
	var wg wait.Group
	errList := errors.NewErrorList()
	automanagedNamespacesList, staleNamespaces, err := f.ListAutomanagedNamespaces(tenant)
	if err != nil {
		errList.Append(err)
		return errList
	}
	if len(automanagedNamespacesList) > 0 {
		for namespaceIndex := range automanagedNamespacesList {
			nsName := automanagedNamespacesList[namespaceIndex]
			wg.Start(func() {
				if err := f.deleteNamespace(tenant, nsName); err != nil {
					errList.Append(err)
					return
				}
			})
		}

	}
	if len(staleNamespaces) > 0 {
		for namespaceIndex := range staleNamespaces {
			nsName := staleNamespaces[namespaceIndex]
			wg.Start(func() {
				if err := f.deleteNamespace(tenant, nsName); err != nil {
					errList.Append(err)
					return
				}
			})
		}

	}
	wg.Wait()
	f.automanagedNamespaceCount = 0
	return errList
}

// DeleteNamespaces deletes the list of namespaces.
func (f *Framework) DeleteNamespaces(tenant string, namespaces []string) *errors.ErrorList {
	var wg wait.Group
	errList := errors.NewErrorList()
	for _, namespace := range namespaces {
		namespace := namespace
		wg.Start(func() {
			if err := f.deleteNamespace(tenant, namespace); err != nil {
				errList.Append(err)
				return
			}
		})
	}
	wg.Wait()
	return errList
}

// CreateObject creates object base on given object description.
func (f *Framework) CreateObject(tenant string, namespace string, name string, obj *unstructured.Unstructured, options ...*client.ApiCallOptions) error {
	return client.CreateObject(f.dynamicClients.GetClient(), tenant, namespace, name, obj, options...)
}

// PatchObject updates object (using patch) with given name using given object description.
func (f *Framework) PatchObject(tenant string, namespace string, name string, obj *unstructured.Unstructured, options ...*client.ApiCallOptions) error {
	return client.PatchObject(f.dynamicClients.GetClient(), tenant, namespace, name, obj)
}

// DeleteObject deletes object with given name and group-version-kind.
func (f *Framework) DeleteObject(gvk schema.GroupVersionKind, tenant string, namespace string, name string, options ...*client.ApiCallOptions) error {
	return client.DeleteObject(f.dynamicClients.GetClient(), gvk, tenant, namespace, name)
}

// GetObject retrieves object with given name and group-version-kind.
func (f *Framework) GetObject(gvk schema.GroupVersionKind, tenant string, namespace string, name string, options ...*client.ApiCallOptions) (*unstructured.Unstructured, error) {
	return client.GetObject(f.dynamicClients.GetClient(), gvk, tenant, namespace, name)
}

// ApplyTemplatedManifests finds and applies all manifest template files matching the provided
// manifestGlob pattern. It substitutes the template placeholders using the templateMapping map.
func (f *Framework) ApplyTemplatedManifests(manifestGlob string, templateMapping map[string]interface{}, options ...*client.ApiCallOptions) error {
	// TODO(mm4tt): Consider using the out-of-the-box "kubectl create -f".
	manifestGlob = os.ExpandEnv(manifestGlob)
	templateProvider := config.NewTemplateProvider(filepath.Dir(manifestGlob))
	manifests, err := filepath.Glob(manifestGlob)
	if err != nil {
		return err
	}
	for _, manifest := range manifests {
		klog.Infof("Applying %s\n", manifest)
		obj, err := templateProvider.TemplateToObject(filepath.Base(manifest), templateMapping)
		if err != nil {
			if err == config.ErrorEmptyFile {
				klog.Warningf("Skipping empty manifest %s", manifest)
				continue
			}
			return err
		}
		objList := []unstructured.Unstructured{*obj}
		if obj.IsList() {
			list, err := obj.ToList()
			if err != nil {
				return err
			}
			objList = list.Items
		}
		for _, item := range objList {
			if err := f.CreateObject(item.GetTenant(), item.GetNamespace(), item.GetName(), &item, options...); err != nil {
				return fmt.Errorf("error while applying (%s): %v", manifest, err)
			}
		}

	}
	return nil
}

func (f *Framework) isAutomanagedNamespace(name string) (bool, error) {
	return regexp.MatchString("[a-zA-Z0-9]{6,}-"+f.automanagedNamespacePrefix, name)
}

func (f *Framework) isStaleAutomanagedNamespace(name string) bool {
	return namespaceID.MatchString(name)
}

func (f *Framework) CreateAutomanagedTenants(tenantCount int) error {
	if f.automanagedTenantCount != 0 {
		//return fmt.Errorf("automanaged tenants already created")
	}

	startpos := 0
	endpos := 0
	if f.clusterConfig.Apiserverextranum == 0 {
		for i := 1; i <= tenantCount; i++ {
			name := fmt.Sprintf("%s-%v", util.RandomDNS1123String(6, startpos, endpos), f.automanagedTenantPrefix)
			if err := client.CreateTenant(f.clientSets.GetClient(), name); err != nil {
				return err
			}
			f.automanagedNamespaceCount++
		}
	} else {
		tenantinterval := 0
		apiservernum := f.clusterConfig.Apiserverextranum + 1
		if tenantCount%apiservernum > 0 {
			tenantinterval = tenantCount/apiservernum + 1
		} else {
			tenantinterval = tenantCount / apiservernum
		}
		for server := 1; server <= apiservernum; server++ {
			endpos = startpos + (26 / apiservernum)
			for i := 1; i <= tenantinterval; i++ {
				name := fmt.Sprintf("%s-%v", util.RandomDNS1123String(6, startpos, endpos), f.automanagedTenantPrefix)
				if err := client.CreateTenant(f.clientSets.GetClient(), name); err != nil {
					return err
				}
				f.automanagedTenantCount++
			}
			if tenantCount-f.automanagedNamespaceCount < tenantinterval {
				tenantinterval = tenantCount - f.automanagedNamespaceCount
			}
			startpos = endpos

		}
	}

	return nil
}

// ListAutomanagedTenants returns all existing automanged tenant names.
func (f *Framework) ListAutomanagedTenants() ([]string, []string, error) {
	var automanagedTenantList, staleTenants []string
	tenantsList, err := client.ListTenants(f.clientSets.GetClient())
	if err != nil {
		return automanagedTenantList, staleTenants, err
	}
	for _, tenant := range tenantsList {
		matched, err := f.isAutomanagedTenant(tenant.Name)
		if err != nil {
			return automanagedTenantList, staleTenants, err
		}
		if matched {
			automanagedTenantList = append(automanagedTenantList, tenant.Name)
		} else {
			// check further whether the tenant is a automanaged namespace created in previous test execution.
			// this could happen when the execution is aborted abornamlly, and the resource is not able to be
			// clean up.
			matched := f.isStaleAutomanagedTenant(tenant.Name)
			if matched {
				staleTenants = append(staleTenants, tenant.Name)
			}
		}
	}
	return automanagedTenantList, staleTenants, nil
}

func (f *Framework) deleteTenant(tenant string) error {
	clientSet := f.clientSets.GetClient()
	if err := client.DeleteTenant(clientSet, tenant); err != nil {
		return err
	}
	if err := client.WaitForDeleteTenant(clientSet, tenant); err != nil {
		return err
	}
	return nil
}

// DeleteAutomanagedTenants deletes all automanged tenants.
func (f *Framework) DeleteAutomanagedTenants() *errors.ErrorList {
	var wg wait.Group
	errList := errors.NewErrorList()
	automanagedTenantsList, staleTenants, err := f.ListAutomanagedTenants()
	if err != nil {
		errList.Append(err)
		return errList
	}
	if len(automanagedTenantsList) > 0 {
		for tenantIndex := range automanagedTenantsList {
			tenantName := automanagedTenantsList[tenantIndex]
			wg.Start(func() {
				if err := f.deleteTenant(tenantName); err != nil {
					errList.Append(err)
					return
				}
			})
		}

	}
	if len(staleTenants) > 0 {
		for tenantIndex := range staleTenants {
			tenantName := staleTenants[tenantIndex]
			wg.Start(func() {
				if err := f.deleteTenant(tenantName); err != nil {
					errList.Append(err)
					return
				}
			})
		}

	}
	wg.Wait()
	f.automanagedTenantCount = 0
	return errList
}

// DeleteTenants deletes the list of tenants.
func (f *Framework) DeleteTenants(tenants []string) *errors.ErrorList {
	var wg wait.Group
	errList := errors.NewErrorList()
	for _, tenant := range tenants {
		tenant := tenant
		wg.Start(func() {
			if err := f.deleteTenant(tenant); err != nil {
				errList.Append(err)
				return
			}
		})
	}
	wg.Wait()
	return errList
}

func (f *Framework) isAutomanagedTenant(name string) (bool, error) {
	return regexp.MatchString("[a-zA-Z0-9]{6,}-"+f.automanagedTenantPrefix, name)
}

func (f *Framework) isStaleAutomanagedTenant(name string) bool {
	return tenantID.MatchString(name)
}
