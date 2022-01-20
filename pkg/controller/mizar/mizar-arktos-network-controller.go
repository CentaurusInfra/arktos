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

package mizar

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	arktoscheme "k8s.io/arktos-ext/pkg/generated/clientset/versioned/scheme"
	arktosinformer "k8s.io/arktos-ext/pkg/generated/informers/externalversions/arktosextensions/v1"
	arktosv1 "k8s.io/arktos-ext/pkg/generated/listers/arktosextensions/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	mizarNetworkType = "mizar"

	arktosName         = "arktos"
	homeSubPath        = "/hack/runtime/"
	vpcTemplateJson    = "/default_mizar_network_vpc_template.json"
	subnetTemplateJson = "/default_mizar_network_subnet_template.json"
)

var vpcDefaultTemplatePath, subnetDefaultTemplatePath = getTemplateFilePathName()

// MizarArktosNetworkController delivers grpc message to Mizar to update VPC with arktos network name
type MizarArktosNetworkController struct {
	// Used to create CRDs - VPC or Subnet of tenant
	dynamicClient dynamic.Interface

	// Used to create mapping to find out GVR via GVK before creating CRDs - VPC or Subnet
	discoveryClient discovery.DiscoveryInterface

	netClientset    *arktos.Clientset
	netLister       arktosv1.NetworkLister
	netListerSynced cache.InformerSynced
	syncHandler     func(eventKeyWithType KeyWithEventType) error
	queue           workqueue.RateLimitingInterface
	recorder        record.EventRecorder
	grpcHost        string
	grpcAdaptor     IGrpcAdaptor
}

// NewMizarArktosNetworkController starts arktos network controller for mizar
func NewMizarArktosNetworkController(dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface, netClientset *arktos.Clientset, kubeClientset *kubernetes.Clientset, networkInformer arktosinformer.NetworkInformer, grpcHost string, grpcAdaptor IGrpcAdaptor) *MizarArktosNetworkController {
	utilruntime.Must(arktoscheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClientset.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	c := &MizarArktosNetworkController{
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		netClientset:    netClientset,
		netLister:       networkInformer.Lister(),
		netListerSynced: networkInformer.Informer().HasSynced,
		queue:           workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		recorder:        eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "mizar-arktos-network-controller"}),
		grpcHost:        grpcHost,
		grpcAdaptor:     grpcAdaptor,
	}

	networkInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.createNetwork,
	})

	c.netLister = networkInformer.Lister()
	c.netListerSynced = networkInformer.Informer().HasSynced
	c.syncHandler = c.syncNetwork

	return c
}

// Run update from mizar cluster
func (c *MizarArktosNetworkController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Info("Starting Mizar arktos network controller")
	klog.V(5).Info("waiting for informer caches to sync")
	if !cache.WaitForCacheSync(stopCh, c.netListerSynced) {
		klog.Error("failed to wait for cache to sync")
		return
	}
	klog.V(5).Info("staring workers of network controller")
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	klog.V(5).Infof("%d workers started", workers)
	<-stopCh
	klog.Info("shutting down mizar arktos network controller")
}

func (c *MizarArktosNetworkController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *MizarArktosNetworkController) processNextWorkItem() bool {
	workItem, quit := c.queue.Get()

	if quit {
		return false
	}

	eventKeyWithType := workItem.(KeyWithEventType)
	key := eventKeyWithType.Key
	defer c.queue.Done(workItem)

	err := c.syncHandler(eventKeyWithType)
	if err == nil {
		c.queue.Forget(workItem)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("Handle arktos network of key %q failed with %v", key, err))
	c.queue.AddRateLimited(eventKeyWithType)

	return true
}

func (c *MizarArktosNetworkController) createNetwork(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", obj, err))
		return
	}
	c.queue.Add(KeyWithEventType{Key: key, EventType: EventType_Create})
}

func (c *MizarArktosNetworkController) syncNetwork(eventKeyWithType KeyWithEventType) error {
	key := eventKeyWithType.Key
	event := eventKeyWithType.EventType

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing service %q (%v)", key, time.Since(startTime))
	}()

	tenant, name, err := cache.SplitMetaTenantKey(key)
	if err != nil {
		return err
	}

	net, err := c.netLister.NetworksWithMultiTenancy(tenant).Get(name)
	if err != nil {
		return err
	}

	klog.Infof("Mizar-Arktos-Network-controller - get network: %#v.", net)

	switch event {
	case EventType_Create:
		err = c.processNetworkCreation(net, eventKeyWithType)
	default:
		panic(fmt.Sprintf("unimplemented for eventType %v", event))
	}
	if err != nil {
		return err
	}
	return nil
}

func (c *MizarArktosNetworkController) processNetworkCreation(network *v1.Network, eventKeyWithType KeyWithEventType) error {
	//Find out the paths of default template to create vpc and subnet
	vpc := network.Spec.VPCID
	//subnet := vpc + subnetSuffix
	subnet := fmt.Sprintf("%s%s", vpc, subnetSuffix)

	//skip update or create if type is not mizar or network status is ready
	key := eventKeyWithType.Key
	if network.Spec.Type != mizarNetworkType || network.Status.Phase == v1.NetworkReady {
		c.recorder.Eventf(network, corev1.EventTypeNormal, "processNetworkCreation", "Type is not mizar, nothing to be done in mizar cluster: %v.", network)

		// Create default VPC and Subnet after system tenant's arktos network is created successfully
		if network.Spec.Type == mizarNetworkType && network.Status.Phase == v1.NetworkReady {
			klog.V(4).Infof("For system tenant: start to create VPC(%s) and Subnet(%s)", vpc, subnet)
			if vpcDefaultTemplatePath == "" || subnetDefaultTemplatePath == "" {
				klog.Errorf("VPC default template path or Subnet default template path is blank")
				return errors.New("Ensure you are in Arktos home directory to start Arktos ......")
			}

			err := createVpcAndSubnetCRD(network.Tenant, vpc, subnet, vpcDefaultTemplatePath, subnetDefaultTemplatePath, c.discoveryClient, c.dynamicClient)
			if err != nil {
				klog.Errorf("For system tenant (%s): create actual VPC object or Subnet object in error (%v).", err)
				return err
			}
			klog.V(4).Infof("For system tenant: complete to create VPC(%s) and Subnet(%s) successfully", vpc, subnet)
		}

		return nil
	}

	msg := &BuiltinsArktosMessage{
		Name: network.Name,
		Vpc:  network.Spec.VPCID,
	}

	response := c.grpcAdaptor.CreateArktosNetwork(c.grpcHost, msg)

	code := response.Code
	context := response.Message

	switch code {
	case CodeType_OK:
		klog.Infof("Mizar handled arktos network and vpc id update successfully: %s", key)
	case CodeType_TEMP_ERROR:
		klog.Warningf("Mizar hit temporary error for arktos network and vpc id update: %s", key)
		c.queue.AddRateLimited(eventKeyWithType)
		return errors.New("Arktos network and vpc id update failed on mizar side, will try again.....")
	case CodeType_PERM_ERROR:
		klog.Errorf("Mizar hit permanent error for Arktos network creation for Arktos network: %s", key)
		return errors.New("Arktos network and vpc id update failed permanently on mizar side")
	}

	c.recorder.Eventf(network, corev1.EventTypeNormal, "processNetworkCreation", "successfully created network from mizar cluster: %v.", context)

	// Create default VPC and Subnet after non-system tenant's arktos network is created successfully
	klog.V(4).Infof("For non-system tenant (%s): start to create VPC(%s) and Subnet(%s)", network.Tenant, vpc, subnet)

	if vpcDefaultTemplatePath == "" || subnetDefaultTemplatePath == "" {
		klog.Errorf("VPC default template path or Subnet default template path is blank")
		return errors.New("Ensure you are in Arktos home directory to start Arktos ......")
	}

	err := createVpcAndSubnetCRD(network.Tenant, vpc, subnet, vpcDefaultTemplatePath, subnetDefaultTemplatePath, c.discoveryClient, c.dynamicClient)
	if err != nil {
		klog.Errorf("For non-system tenant (%s): create actual VPC object or Subnet object in error (%v).", err)
		return err
	}
	klog.V(4).Infof("For non-system tenant (%s): complete to create VPC(%s) and Subnet(%s) successfully", network.Tenant, vpc, subnet)

	return nil
}

func createVpcAndSubnetCRD(tenant, vpc, subnet, vpcDefaultTemplatePath, subnetDefaultTemplatePath string, discoveryClient discovery.DiscoveryInterface, dynamicClient dynamic.Interface) error {
	// Create VPC CRD
	err := createVpcOrSubnetCRD(tenant, vpc, vpcDefaultTemplatePath, discoveryClient, dynamicClient)
	if err != nil {
		klog.Errorf("Create actual VPC object (%s) in error (%v).", vpc, err)
		return err
	}

	// Create Subnet CRD
	err = createVpcOrSubnetCRD(tenant, subnet, subnetDefaultTemplatePath, discoveryClient, dynamicClient)
	if err != nil {
		klog.Errorf("Create actual Subnet object (%s) in error (%v).", subnet, err)
		return err
	}

	return nil
}

func createVpcOrSubnetCRD(tenant, vpcOrSubnetName, defaultTemplatePath string, discoveryClient discovery.DiscoveryInterface, dynamicClient dynamic.Interface) error {
	// Get json file in bytes format
	manifestData, err := GetCRDVpcOrSubnetSpec(defaultTemplatePath, vpcOrSubnetName, tenant)
	if err != nil {
		klog.Errorf("At (%s) get JSON spec for CRD (%s) in error: (%v)", defaultTemplatePath, vpcOrSubnetName, err)
		return err
	}

	// Create VPC or Subnet object
	err = createUnstructuredObject([]byte(manifestData), vpcOrSubnetName, discoveryClient, dynamicClient)
	if err != nil {
		klog.Errorf("Create CRD object (%s) in error: (%v)", vpcOrSubnetName, err)
		return err
	}

	return nil
}

func createUnstructuredObject(data []byte, vpcOrSubnetName string, discoveryClient discovery.DiscoveryInterface, dynamicClient dynamic.Interface) error {
	unstructuredObj := &unstructured.Unstructured{}
	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	// Get GVK(Group Version Kind)
	_, gvk, err := decUnstructured.Decode(data, nil, unstructuredObj)
	if err != nil {
		klog.Errorf("Getting GVK in error (%v).", err)
		return err
	}

	// Get mapping from GVK for GVR (Group Version Resource) used by dynamic client resource
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)

	if err != nil {
		klog.Errorf("Get mapping between GVK and GVR in error (%v).", err)
		return err
	}

	// Create dynamic client resource
	var dynamicClientResource dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		// namespaced resources should specify the namespace under tenant
		if unstructuredObj.GetNamespace() == "" {
			unstructuredObj.SetNamespace("default")
		}
		namespace := unstructuredObj.GetNamespace()
		dynamicClientResource = dynamicClient.Resource(mapping.Resource).NamespaceWithMultiTenancy(namespace, "system")

	} else {
		// for cluster-wide resources
		dynamicClientResource = dynamicClient.Resource(mapping.Resource)
	}

	// Create CRD resource - vpc or subnet
	actualObject, err := dynamicClientResource.Create(unstructuredObj, metav1.CreateOptions{})

	if err == nil {
		klog.V(4).Infof("Get actual object's name : (%s)", actualObject.GetName())
		klog.V(4).Infof("Get actual object's GVK : (%v)", actualObject.GroupVersionKind())
		klog.V(4).Infof("Get actual object's objectKind : (%v)", actualObject.GetObjectKind())
	} else {
		klog.Errorf("Create actual object's name: (%s) in error (%v).", unstructuredObj.GetName(), err)
	}

	return err
}

func GetCRDVpcOrSubnetSpec(defaultTemplatePath, vpcOrSubnetName, tenant string) ([]byte, error) {
	// For updating the data in default vpc/subnet template
	var availableData = map[string]string{
		"Tenant": tenant,
	}

	// Read template file
	jsonTmpl, err := readTemplateFile(defaultTemplatePath)
	if err != nil {
		klog.Errorf("Read default vpc/subnet template in error: (%v)", err)
		return nil, err
	}

	// Create Template with template file
	t, err := template.New(vpcOrSubnetName).Parse(jsonTmpl)
	if err != nil {
		klog.Errorf("Create Template with template file in error: (%v)", err)
		return nil, err
	}

	// Create json file in bytes format
	var bytesJson bytes.Buffer
	if err = t.Execute(&bytesJson, availableData); err != nil {
		klog.Errorf("Update default vpc/subnet template in error: (%v)", err)
		return nil, err
	}

	return bytesJson.Bytes(), nil
}

func readTemplateFile(path string) (string, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		klog.Errorf("Read Template File (%s) in error :(%v)", path, err)
		return "", err
	}

	return string(bytes), nil
}

// Initialized once for vpcDefaultTemplatePath and subnetDefaultTemplatePath
func getTemplateFilePathName() (string, string) {
	currentDir, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Get current directory (%s) in error (%v).", currentDir, err))
	}

	if !strings.HasSuffix(currentDir, arktosName) {
		klog.Errorf("Current directory (%s) is not in Arktos Home directory with error (%v).", currentDir, err)
		return "", ""
	}

	vpcDefaultTemplatePath := filepath.Join(currentDir, homeSubPath, vpcTemplateJson)
	subnetDefaultTemplatePath := filepath.Join(currentDir, homeSubPath, subnetTemplateJson)

	return vpcDefaultTemplatePath, subnetDefaultTemplatePath
}
