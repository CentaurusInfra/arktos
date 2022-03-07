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
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	arktoscheme "k8s.io/arktos-ext/pkg/generated/clientset/versioned/scheme"
	arktosinformer "k8s.io/arktos-ext/pkg/generated/informers/externalversions/arktosextensions/v1"
	arktosv1 "k8s.io/arktos-ext/pkg/generated/listers/arktosextensions/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/features"
)

const (
	mizarNetworkType = "mizar"
	mizarInternalIPStart = 20

	resource_group ="mizar.com"
	resource_version = "v1"
	resource_vpc = "vpcs"
	resource_subnet = "subnets"
)

var seed = rand.NewSource(time.Now().UnixNano())
var ran = rand.New(seed)

// Temporary solution for mizar VPC range cannot overlapping issue
// VPC range inclusive [vpcRangeStart, vpcRangeEnd] - cannot have overlapping across TPs
type vpcUsedCache struct {
	vpcRangeStart int
	vpcRangeEnd   int
	vpcUsedCache  map[int]bool
	vpcCacheLock  sync.RWMutex	// protects vpcUsedCache
}

// MizarArktosNetworkController delivers grpc message to Mizar to update VPC with arktos network name
type MizarArktosNetworkController struct {
	// Used to create CRDs - VPC or Subnet of tenant
	dynamicClient dynamic.Interface

	netClientset        *arktos.Clientset
	networkLister       arktosv1.NetworkLister
	networkListerSynced cache.InformerSynced
	syncHandler         func(eventKeyWithType KeyWithEventType) error
	queue               workqueue.RateLimitingInterface
	recorder            record.EventRecorder
	grpcHost            string
	grpcAdaptor         IGrpcAdaptor

	vpcCache            *vpcUsedCache
}

// NewMizarArktosNetworkController starts arktos network controller for mizar
func NewMizarArktosNetworkController(dynamicClient dynamic.Interface, netClientset *arktos.Clientset, kubeClientset *kubernetes.Clientset,
	networkInformer arktosinformer.NetworkInformer, grpcHost string, grpcAdaptor IGrpcAdaptor, vpcRangeStart int, vpcRangeEnd int) *MizarArktosNetworkController {
	utilruntime.Must(arktoscheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClientset.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	c := &MizarArktosNetworkController{
		dynamicClient:       dynamicClient,
		netClientset:        netClientset,
		networkLister:       networkInformer.Lister(),
		networkListerSynced: networkInformer.Informer().HasSynced,
		queue:               workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		recorder:            eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "mizar-arktos-network-controller"}),
		grpcHost:            grpcHost,
		grpcAdaptor:         grpcAdaptor,
	}

	networkInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.createNetwork,
	})

	c.networkLister = networkInformer.Lister()
	c.networkListerSynced = networkInformer.Informer().HasSynced
	c.syncHandler = c.syncNetwork

	if utilfeature.DefaultFeatureGate.Enabled(features.MizarVPCRangeOverlap) {
		klog.Infof("features MizarVPCRangeOverlap enabled")
		if !isValidVPCRange(vpcRangeStart, vpcRangeEnd) {
			klog.Fatalf("Invalid VPC range [%d, %d]", vpcRangeStart, vpcRangeEnd)
		} else {
			klog.Infof("VPC range [%d, %d]", vpcRangeStart, vpcRangeEnd)
		}
		c.vpcCache = &vpcUsedCache{
			vpcRangeStart: vpcRangeStart,
			vpcRangeEnd:   vpcRangeEnd,
			vpcUsedCache:  make(map[int]bool),
		}

		err := c.populateCache()
		if err != nil {
			klog.Fatalf("Unable to get used VPC range from registry. Error %v", err)
		}
	}

	return c
}

// Run update from mizar cluster
func (c *MizarArktosNetworkController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	defer klog.Info("shutting down mizar arktos network controller")

	klog.Info("Starting Mizar arktos network controller")
	if !cache.WaitForCacheSync(stopCh, c.networkListerSynced) {
		klog.Error("failed to wait for cache to sync")
		return
	}
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
}

func (c *MizarArktosNetworkController) populateCache() error {
	c.vpcCache.vpcCacheLock.Lock()
	defer c.vpcCache.vpcCacheLock.Unlock()

	resource := schema.GroupVersionResource{Group: resource_group, Version: resource_version, Resource: resource_vpc}
	vpcs, err := controller.ListUnstructuredObjects(resource, c.dynamicClient, metav1.TenantSystem, metav1.NamespaceDefault)
	if err != nil {
		klog.Fatalf("Error in getting mizar vpc objects: %v", err)
	}
	for _, vpcData := range vpcs.Items {
		vpc := &MizarVPC{}
		rawData, err := json.Marshal(vpcData)
		if err != nil {
			klog.Fatalf("Error in marshal mizar vpc object: %v", err)
		}
		err = json.Unmarshal(rawData, vpc)
		if err != nil {
			klog.Fatalf("Error in unmarshal mizar vpc object: %v", err)
		}
		ipPrefix := getVPCStart(vpc.Spec.IP)
		if ipPrefix >= c.vpcCache.vpcRangeStart && ipPrefix <= c.vpcCache.vpcRangeEnd {
			c.vpcCache.vpcUsedCache[ipPrefix] = true
			c.vpcCache.vpcRangeStart = ipPrefix + 1
		}
	}

	// Not allow mizar internal ip
	c.vpcCache.vpcUsedCache[mizarInternalIPStart] = true

	return nil
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

	switch event {
	case EventType_Create:
		err := c.processNetworkCreation(key)
		if err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("unimplemented for eventType %v", event))
	}
	return nil
}

func (c *MizarArktosNetworkController) processNetworkCreation(key string) error {
	tenant, name, err := cache.SplitMetaTenantKey(key)
	if err != nil {
		return err
	}

	network, err := c.networkLister.NetworksWithMultiTenancy(tenant).Get(name)
	if err != nil {
		return err
	}

	//Find out the paths of default template to create vpc and subnet
	vpc := network.Spec.VPCID
	//subnet := vpc + subnetSuffix
	subnet := fmt.Sprintf("%s%s", vpc, subnetSuffix)
	klog.V(4).Infof("Processing arktos network: %#v. vpc [%v], subnet [%v]. Network Type [%v]", network, vpc, subnet, network.Spec.Type)

	//skip update or create if type is not mizar or network status is ready
	if network.Spec.Type != mizarNetworkType {
		c.recorder.Eventf(network, corev1.EventTypeNormal, "NotRelevent", "Skip processing non mizar network")
		return nil
	}

	// Create default VPC and Subnet
	err, permErr := c.createVpcAndSubnet(vpc, subnet, c.dynamicClient)
	if permErr != nil {
		klog.Errorf("Create VPC and Subnet failed for tenant %v. Not retriable: (%v)", network.Tenant, permErr)
		return nil
	}
	if err != nil {
		klog.Errorf("Create VPC and Subnet failed for tenant %v. Error: (%v).", network.Tenant, err)
		return err
	}

	msg := &BuiltinsArktosMessage{
		Name: network.Name,
		Vpc:  network.Spec.VPCID,
	}

	response := c.grpcAdaptor.CreateArktosNetwork(c.grpcHost, msg)
	switch response.Code {
	case CodeType_OK:
		klog.Infof("Mizar handled arktos network %v successfully", key)
	case CodeType_TEMP_ERROR:
		klog.Warningf("Mizar hit temporary error for arktos network and vpc id update: %s", key)
		return errors.New("Arktos network and vpc id update failed on mizar side, will try again.....")
	case CodeType_PERM_ERROR:
		klog.Errorf("Mizar hit permanent error for Arktos network creation for Arktos network: %s", key)
		return errors.New("Arktos network and vpc id update failed permanently on mizar side")
	}

	c.recorder.Eventf(network, corev1.EventTypeNormal, "SucessfulCreate", "Created Mizar VPC %s and subnet %s for tenant %v", vpc, subnet, network.Tenant)

	klog.V(3).Infof("Created VPC (%s) and Subnet(%s) for tenant %s successfully", vpc, subnet, network.Tenant)

	return nil
}

// Return: first error is recoverable, 2nd error cannot be recovered
func (c *MizarArktosNetworkController) createVpcAndSubnet(vpc, subnet string, dynamicClient dynamic.Interface) (error, error) {
	// Create VPC object
	vpcIpStart, vpcSpec, err, permErr := c.generateVPCSpec(vpc)
	if err != nil || permErr != nil {
		return err, permErr
	}

	vpcData, err := json.Marshal(vpcSpec)
	if err != nil {
		klog.Errorf("Error marshalling VPC %s spec. Error: %v", vpc, err)
		return err, nil
	}
	err = controller.CreateUnstructuredObject(vpcData, dynamicClient)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		klog.Errorf("Error creating VPC %s. Error: %v", vpc, err)
		return err, nil
	}

	// Create Subnet object
	subnetSpec, err := generateSubnetSpec(vpc, subnet, vpcIpStart)
	if err != nil {
		klog.Errorf("Error getting Subnet %s spec. Error: %v", subnet, err)
		return err, nil
	}
	err = controller.CreateUnstructuredObject(subnetSpec, dynamicClient)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		klog.Errorf("Error creating Subnet %s for VPC %s. Error: %v", subnetSpec, vpc, err)
		return err, nil
	}

	return nil, nil
}

// Return: first error is recoverable, 2nd error cannot be recovered
func (c *MizarArktosNetworkController) generateVPCSpec(vpcName string) (int, *MizarVPC, error, error) {
	// TODO: this is a quick solution to randomize VPC start ip address. Due to variously reasons, Arktos
	//   needs randomize VPC start ip to prevent service ip collision for now.
	// This is a simplified version to avoid reserved internal address - however, it may collision with real external ip address.
	// Will log as an issue and solve in the future
	// randomize ip start segment:
	var ipStart int
	if utilfeature.DefaultFeatureGate.Enabled(features.MizarVPCRangeOverlap) {
		if c.vpcCache.vpcRangeStart > c.vpcCache.vpcRangeEnd {
			return 0, nil, nil, fmt.Errorf("Mizar VPC range exhausted. %#v", c.vpcCache.vpcUsedCache)
		}
		c.vpcCache.vpcCacheLock.Lock()
		defer c.vpcCache.vpcCacheLock.Unlock()

		ipStart = c.vpcCache.vpcRangeStart
		for {
			if ipStart > c.vpcCache.vpcRangeEnd {
				return 0, nil, nil, fmt.Errorf("Mizar VPC range exhausted. %#v", c.vpcCache.vpcUsedCache)
			}

			value, isOK := c.vpcCache.vpcUsedCache[ipStart]
			if isOK && value {
				ipStart++
			} else {
				c.vpcCache.vpcUsedCache[ipStart] = true
				c.vpcCache.vpcRangeStart = ipStart + 1
				break
			}
		}
	} else {
		ipStart = ran.Intn(89) + 11 // IpStart range [11, 99] - 20
		// Exclude mizarInternalIPStart as it is used by mizar internally
		if ipStart == mizarInternalIPStart {
			ipStart = mizarInternalIPStart + 1
		}
	}

	vpc := &MizarVPC{
		TypeMeta: TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", resource_group, resource_version),
			Kind:       "Vpc",
		},
		Metadata: ObjectMeta{
			Name:      vpcName,
			Namespace: metav1.NamespaceDefault,
			Tenant:    metav1.TenantSystem,
		},
		Spec: MizarVPCSpec{
			IP:      fmt.Sprintf("%d.0.0.0", ipStart),
			Prefix:  "8",
			Divider: 1,
			Status:  "Init",
		},
	}

	return ipStart, vpc, nil, nil
}

func generateSubnetSpec(vpcName, subnetName string, vpcIpStart int) ([]byte, error) {
	subnetIpSeg := ran.Intn(256) // 0-255
	subnet := &MizarSubnet{
		TypeMeta: TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", resource_group, resource_version),
			Kind:       "Subnet",
		},
		Metadata: ObjectMeta{
			Name:      subnetName,
			Namespace: metav1.NamespaceDefault,
			Tenant:    metav1.TenantSystem,
		},
		Spec: MizarSubnetSpec{
			IP:       fmt.Sprintf("%d.%d.0.0", vpcIpStart, subnetIpSeg),
			Prefix:   "16",
			Bouncers: 1,
			VPC:      vpcName,
			Status:   "Init",
		},
	}

	return json.Marshal(subnet)
}

// Currently only allows [11-99], mizar default VPC 20 will be excluded implicitly
func isValidVPCRange(rangeStart, rangeEnd int) bool {
	if rangeStart < 10 || rangeStart > 99 {
		return false
	}
	if rangeEnd < 10 || rangeEnd > 99 {
		return false
	}
	if rangeStart > rangeEnd {
		return false
	}
	return true
}

func getVPCStart(s string) int {
	ips := strings.Split(s, ".")
	i, _ := strconv.Atoi(ips[0])
	return i
}