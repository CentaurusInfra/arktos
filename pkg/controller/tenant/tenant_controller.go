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

package tenant

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/json"
	arktosinformer "k8s.io/arktos-ext/pkg/generated/informers/externalversions/arktosextensions/v1"
	"k8s.io/client-go/metadata"
	"text/template"
	"time"

	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	arktosv1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	arktoslister "k8s.io/arktos-ext/pkg/generated/listers/arktosextensions/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	rbacinformers "k8s.io/client-go/informers/rbac/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/tenant/deletion"
	"k8s.io/kubernetes/pkg/util/metrics"

	"k8s.io/klog"
)

var tenantDefaultNamespaces = [...]string{core.NamespaceDefault, core.NamespaceSystem, core.NamespacePublic}

const (
	initialClusterRoleUser        = "admin"
	InitialClusterRoleName        = "admin-role"
	InitialClusterRoleBindingName = "admin-role-binding"
)

// TenantController is responsible for performing actions dependent upon a tenant phase
type TenantController struct {
	// namespaceLister that can list namespaces from a shared cache
	namespaceLister corelisters.NamespaceLister
	// clusterRoleLister that can list clusterRoles from a shared cache
	clusterRoleLister rbaclisters.ClusterRoleLister
	// clusterRoleBindingLister that can list clusterRoleBindings from a shared cache
	clusterRoleBindingLister rbaclisters.ClusterRoleBindingLister
	// tenantLister that can list tenants from a shared cache
	tenantLister corelisters.TenantLister
	// returns true when the tenant cache is ready
	tenantListerSynced cache.InformerSynced
	// tenants that have been queued up for processing by workers
	queue workqueue.RateLimitingInterface
	// kubeclient for api calls
	kubeClient clientset.Interface
	// sync handler for injection
	syncHandler func(key string) error

	// list network definitions in arktos
	networkLister       arktoslister.NetworkLister
	networkListerSynced cache.InformerSynced

	// client for network CR api calls
	networkClient arktos.Interface
	// default network spec template file path
	defaultNetworkTemplatePath string
	// templateGetter for injection
	templateGetter func(path string) (string, error)

	tenantedResourcesDeleter deletion.TenantedResourcesDeleterInterface
}

// NewTenantController creates a new iinstance of tenantcontroller
func NewTenantController(kubeClient clientset.Interface,
	tenantInformer coreinformers.TenantInformer,
	namespaceInformer coreinformers.NamespaceInformer,
	clusterRoleInformer rbacinformers.ClusterRoleInformer,
	clusterRoleBindingInformer rbacinformers.ClusterRoleBindingInformer,
	resyncPeriod time.Duration, // split this controller into tenant creation and deletion controllers if resyncPeriod causes performance degradation
	networkClient arktos.Interface,
	networkInformer arktosinformer.NetworkInformer,
	defaultNetworkTemplatePath string,
	metadataClient metadata.Interface,
	discoverResourcesFn func() ([]*metav1.APIResourceList, error),
	finalizerToken v1.FinalizerName) *TenantController {

	// create the controller so we can inject the enqueue function
	tenantController := &TenantController{
		kubeClient:                 kubeClient,
		networkClient:              networkClient,
		networkLister:              networkInformer.Lister(),
		networkListerSynced:        networkInformer.Informer().HasSynced,
		queue:                      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "tenant"),
		defaultNetworkTemplatePath: defaultNetworkTemplatePath,
		tenantedResourcesDeleter:   deletion.NewTenantedResourcesDeleter(kubeClient, metadataClient, discoverResourcesFn, finalizerToken),
	}

	if kubeClient != nil && kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		metrics.RegisterMetricAndTrackRateLimiterUsage("tenant_controller", kubeClient.CoreV1().RESTClient().GetRateLimiter())
	}

	// configure the tenant informer event handlers
	tenantInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				tenantController.enqueue(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				tenantController.enqueue(newObj)
			},
		},
		resyncPeriod,
	)
	tenantController.tenantLister = tenantInformer.Lister()
	tenantController.tenantListerSynced = tenantInformer.Informer().HasSynced
	tenantController.syncHandler = tenantController.syncTenant
	tenantController.templateGetter = readTemplate
	tenantController.namespaceLister = namespaceInformer.Lister()
	tenantController.clusterRoleLister = clusterRoleInformer.Lister()
	tenantController.clusterRoleBindingLister = clusterRoleBindingInformer.Lister()
	return tenantController
}

// Run starts the controller with the specified number of workers.
func (tc *TenantController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer tc.queue.ShutDown()

	klog.Infof("Starting tenant controller.")
	tc.createSystemTenantIfNotExist()
	defer klog.Infof("Shutting down tenant controller")

	if !controller.WaitForCacheSync("tenant", stopCh, tc.tenantListerSynced) {
		return
	}

	klog.V(5).Info("Starting workers of tenant controller")
	for i := 0; i < workers; i++ {
		go wait.Until(tc.worker, time.Second, stopCh)
	}
	<-stopCh
}

// worker processes the queue of tenant objects.
func (tc *TenantController) worker() {
	workFunc := func() bool {
		key, quit := tc.queue.Get()
		if quit {
			return true
		}
		defer tc.queue.Done(key)

		err := tc.processQueue(key.(string))
		if err == nil {
			// no error, forget this entry and return
			tc.queue.Forget(key)
			return false
		} else {
			// rather than wait for a full resync, re-add the tenant to the queue to be processed
			tc.queue.AddRateLimited(key)
			utilruntime.HandleError(err)
		}
		return false
	}

	for {
		quit := workFunc()

		if quit {
			return
		}
	}
}

// enqueue adds an object to the controller work queue
func (tc *TenantController) enqueue(obj interface{}) {
	klog.Infof("Starting tenant enque.")

	tenant, ok := obj.(*v1.Tenant)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("Not a tenant object: %v", obj))
		return
	}

	tc.queue.Add(tenant.Name)
}

// processQueue looks for a tenant with the specified name and synchronizes it
func (tc *TenantController) processQueue(tenantName string) (err error) {
	klog.Infof("Starting processsing queue for tenant: %v", tenantName)

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing tenant %q (%v)", tenantName, time.Since(startTime))
	}()

	_, err = tc.tenantLister.Get(tenantName)
	if errors.IsNotFound(err) {
		klog.Infof("tenant has been deleted %v", tenantName)
		return nil
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to retrieve tenant %v from store: %v", tenantName, err))
		return err
	}

	return tc.syncHandler(tenantName)
}

// syncTenant creates the default resources for a new tenant,
// and deletes the resources under the tenant and the tenant itself if the deletion timestamp is set
func (tc *TenantController) syncTenant(tenantName string) (err error) {
	klog.Infof("Starting syncing tenant: %v", tenantName)

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing tenant %q (%v)", tenantName, time.Since(startTime))
	}()

	// no error as its caller, processQueue, has checked.
	tenant, _ := tc.tenantLister.Get(tenantName)
	if tenant.DeletionTimestamp != nil && !tenant.DeletionTimestamp.IsZero() {
		//handling the deletion of a tenant
		return tc.tenantedResourcesDeleter.Delete(tenantName)
	}

	// handling the addition of a tenant
	if err, done := tc.syncNamespaces(tenantName); !done {
		return err
	}

	if err, done := tc.syncInitialRoleAndBinding(tenantName); !done {
		return err
	}

	if err, done := tc.syncDefaultNetworkObject(tenantName); !done {
		return err
	}

	return nil
}

func (tc *TenantController) syncDefaultNetworkObject(tenantName string) (error, bool) {
	failures := []error{}

	// create default network object, if applicable
	tenant, _ := tc.tenantLister.Get(tenantName) // no error as its caller, processQueue, has checked.
	if tenant.Status.Phase == v1.TenantTerminating {
		klog.Infof("Tenant %q is terminating; skipped the creation of default network", tenantName)
	} else if len(tc.defaultNetworkTemplatePath) == 0 {
		klog.Infof("No default network template path; skipped the creation of default network in tenant %q", tenantName)
	} else {
		defaultNetwork := arktosv1.Network{}
		if err := tc.constructDefaultNetwork(tenantName, &defaultNetwork); err != nil {
			failures = append(failures, err)
		} else {
			switch _, err = tc.networkLister.NetworksWithMultiTenancy(tenantName).Get(defaultNetwork.Name); {
			case err == nil:
				klog.V(4).Infof("Default network %s already exists for tenant %s. skipped creating it", defaultNetwork.Name, tenantName)
			case errors.IsNotFound(err):
				if _, err = tc.networkClient.ArktosV1().NetworksWithMultiTenancy(tenantName).Create(&defaultNetwork); err != nil && !errors.IsAlreadyExists(err) {
					failures = append(failures, err)
				}
			case err != nil:
				failures = append(failures, err)
			}
		}
	}
	return flattenedError(failures, tenantName)
}

func (tc *TenantController) syncNamespaces(tenant string) (error, bool) {
	failures := []error{}
	for _, nsName := range tenantDefaultNamespaces {
		switch _, err := tc.namespaceLister.NamespacesWithMultiTenancy(tenant).Get(nsName); {
		case err == nil:
			klog.V(4).Infof("namespace %s already exists. skipped creating it", nsName)
			continue
		case errors.IsNotFound(err):
		case err != nil:
			return err, false
		}

		ns := v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Tenant: tenant, Name: nsName},
		}
		if _, err := tc.kubeClient.CoreV1().NamespacesWithMultiTenancy(tenant).Create(&ns); err != nil && !errors.IsAlreadyExists(err) {
			failures = append(failures, err)
		}
	}
	return flattenedError(failures, tenant)
}

func (tc *TenantController) syncInitialRoleAndBinding(tenant string) (error, bool) {

	var failures []error

	shouldSkip := false
	switch _, err := tc.clusterRoleLister.ClusterRolesWithMultiTenancy(tenant).Get(InitialClusterRoleName); {
	case err == nil:
		klog.V(4).Infof("cluster role %s already exists. skipped creating it", InitialClusterRoleName)
		shouldSkip = true
	case errors.IsNotFound(err):
	case err != nil:
		failures = append(failures, err)
	}

	if !shouldSkip {
		// tenant admin acts as cluster admin for the tenant
		role := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   InitialClusterRoleName,
				Tenant: tenant,
			},
			Rules: initialClusterRoleRules(),
		}

		if _, err := tc.kubeClient.RbacV1().ClusterRolesWithMultiTenancy(tenant).Create(role); err != nil && !errors.IsAlreadyExists(err) {
			failures = append(failures, err)
		}
	}

	shouldSkip = false
	switch _, err := tc.clusterRoleBindingLister.ClusterRoleBindingsWithMultiTenancy(tenant).Get(InitialClusterRoleBindingName); {
	case err == nil:
		klog.V(4).Infof("cluster role binding %s already exists. skipped creating it", InitialClusterRoleBindingName)
		shouldSkip = true
	case errors.IsNotFound(err):
	case err != nil:
		failures = append(failures, err)
	}

	if !shouldSkip {
		binding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   InitialClusterRoleBindingName,
				Tenant: tenant,
			},
			Subjects: []rbacv1.Subject{
				{Kind: rbacv1.UserKind, Name: initialClusterRoleUser},
			},
			RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: InitialClusterRoleName},
		}

		if _, err := tc.kubeClient.RbacV1().ClusterRoleBindingsWithMultiTenancy(tenant).Create(binding); err != nil && !errors.IsAlreadyExists(err) {
			failures = append(failures, err)
		}
	}

	return flattenedError(failures, tenant)
}

func initialClusterRoleRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			Verbs:     []string{"*"},
			APIGroups: []string{"*"},
			Resources: []string{"*"},
		},
		{
			Verbs:           []string{"*"},
			NonResourceURLs: []string{"*"},
		},
	}
}
func (tc *TenantController) constructDefaultNetwork(tenant string, net *arktosv1.Network) error {
	// todo: validate content of template file
	tmpl, err := tc.templateGetter(tc.defaultNetworkTemplatePath)
	if err != nil {
		return err
	}
	t, err := template.New("default").Parse(tmpl)
	if err != nil {
		return err
	}

	var bytesJson bytes.Buffer
	if err = t.Execute(&bytesJson, tenant); err != nil {
		return err
	}

	if err = json.Unmarshal(bytesJson.Bytes(), net); err != nil {
		return err
	}

	// always override with the right tenant
	net.ObjectMeta.Tenant = tenant
	return nil
}

func readTemplate(path string) (string, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func flattenedError(failures []error, tenant string) (error, bool) {
	if len(failures) != 0 {
		ret := utilerrors.Flatten(utilerrors.NewAggregate(failures))
		klog.Errorf("Errors happened in tenant initialization of %v: %v", tenant, ret)
		return ret, false
	}
	return nil, true
}

// not returning any error as the system should continue run without an explicit system tenant
func (tc *TenantController) createSystemTenantIfNotExist() {
	_, err := tc.tenantLister.Get(metav1.TenantSystem)

	if apierrors.IsNotFound(err) {
		klog.Infof("Creating the syste tenant...")
		tc.kubeClient.CoreV1().Tenants().Create(&v1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: metav1.TenantSystem,
			},
			Spec: v1.TenantSpec{
				StorageClusterId: "0",
			},
		})
	}
}
