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

// The TTLController sets ttl annotations on nodes, based on cluster size.
// The annotations are consumed by Kubelets as suggestions for how long
// it can cache objects (e.g. secrets or config maps) before refetching
// from apiserver again.
//
// TODO: This is a temporary workaround for the Kubelet not being able to
// send "watch secrets attached to pods from my node" request. Once
// sending such request will be possible, we will modify Kubelet to
// use it and get rid of this controller completely.

package network

import (
	"fmt"
	"os"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/portsbinding"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"

	"k8s.io/klog"
)

type NetworkController struct {
	kubeClient clientset.Interface

	// A store of pods, populated by the shared informer passed to NetworkController
	podLister corelisters.PodLister
	// podListerSynced returns true if the pod store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	podListerSynced cache.InformerSynced

	// To allow injection for testing.
	syncHandler func(rsKey string) error

	// Nodes that need to be synced.
	queue workqueue.RateLimitingInterface
}

func NewNetworkController(podInformer coreinformers.PodInformer, kubeClient clientset.Interface) *NetworkController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	nc := &NetworkController{
		kubeClient:      kubeClient,
		podLister:       podInformer.Lister(),
		podListerSynced: podInformer.Informer().HasSynced,
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "network"),
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nc.createPort,
		UpdateFunc: nc.updatePort,
		DeleteFunc: nc.deletePort,
	})
	nc.podLister = podInformer.Lister()
	nc.podListerSynced = podInformer.Informer().HasSynced

	nc.syncHandler = nc.syncPod

	return nc
}

// Run begins watching and syncing.
func (nc *NetworkController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer nc.queue.ShutDown()

	klog.Infof("Starting Network controller")
	defer klog.Infof("Shutting down %v controller", "network")

	if !controller.WaitForCacheSync("network", stopCh, nc.podListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(nc.worker, time.Second, stopCh)
	}

	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (nc *NetworkController) worker() {
	for nc.processNextWorkItem() {
	}
}

func (nc *NetworkController) processNextWorkItem() bool {
	key, quit := nc.queue.Get()

	if quit {
		return false
	}
	defer nc.queue.Done(key)

	err := nc.syncHandler(key.(string))
	if err == nil {
		nc.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("Sync %q failed with %v", key, err))
	nc.queue.AddRateLimited(key)

	return true
}

func (nc *NetworkController) createPort(obj interface{}) {
	// When a pod is created, Scheduler seems always faster than network controller.
	// So wait for scheduler's update to create port.
}

// When a pod is updated.
func (nc *NetworkController) updatePort(old, cur interface{}) {
	new := cur.(*v1.Pod)
	prev := old.(*v1.Pod)
	needCreate := false
	needUpdate := false

	client := GetOpenstackClient()
	pod := new.DeepCopy()
	for i, nic := range pod.Spec.Nics {
		if nic.PortId == "" {
			// create port : pod.Spec.Nics[index].PortId
			pod.Spec.Nics[i].PortId = CreatePort(client, pod.Spec.VPC, nic.SubnetName, pod.Spec.NodeName)
			needCreate = true
		} else if new.Spec.NodeName != prev.Spec.NodeName {
			needUpdate = true
			// update port binding host only
			UpdatePort(client, pod.Spec.Nics[i].PortId, new.Spec.NodeName)
		}
	}

	if needUpdate || needCreate {
		_, err := nc.kubeClient.CoreV1().PodsWithMultiTenancy(pod.Namespace, pod.Tenant).Update(pod)
		if err != nil {
			klog.Errorf("Network-controller update error (%v).", err)
		}
	}
}

// When a pod is deleted, delete ports.
func (nc *NetworkController) deletePort(obj interface{}) {
	pod := obj.(*v1.Pod)
	client := GetOpenstackClient()

	for index, nic := range pod.Spec.Nics {
		if nic.PortId != "" {
			// delete port : pod.Spec.Nics[index].PortId
			DeletePort(client, pod.Spec.Nics[index].PortId)
		}
	}
}

func (nc *NetworkController) syncPod(key string) error {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing network %q (%v)", key, time.Since(startTime))
	}()

	tenant, namespace, _, err := cache.SplitMetaTenantNamespaceKey(key)
	if err != nil {
		return err
	}

	allPods, err := nc.podLister.PodsWithMultiTenancy(namespace, tenant).List(labels.Everything())
	if err != nil {
		return err
	}
	klog.Infof("Network-controller - list all pod %#v.", allPods)

	return err
}

func GetOpenstackClient() *gophercloud.ProviderClient {
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: os.Getenv("IdentityEndpoint"), //"http://192.168.113.135/identity/v3",
		Username:         os.Getenv("Username"),         //"admin",
		Password:         os.Getenv("Password"),         //openstack password
		DomainName:       os.Getenv("DomainName"),       //"default",
		TenantName:       os.Getenv("TenantName"),       //"demo",
	}

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		klog.Errorf("Network controller AuthenticatedClient : (%v)", err)
		return nil
	}

	return provider
}

func CreatePort(provider *gophercloud.ProviderClient, vpc string, subnetname string, nodeID string) string {
	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
		Region: os.Getenv("Region")}) //"RegionOne"
	if err != nil {
		klog.Error(err)
	}

	networkid, err := networks.IDFromName(client, vpc)
	if err != nil {
		klog.Error(err)
		return ""
	}
	subnetid, err := subnets.IDFromName(client, subnetname)
	if err != nil {
		klog.Error(err)
		return ""
	}
	createOpts := ports.CreateOpts{
		NetworkID: networkid,
		FixedIPs: []ports.IP{
			{SubnetID: subnetid},
		},
	}

	bindingCreateOpts := portsbinding.CreateOptsExt{
		CreateOptsBuilder: createOpts,
		HostID:            nodeID,
	}

	port, err := ports.Create(client, bindingCreateOpts).Extract()
	if err != nil {
		klog.Error(err)
		return ""
	}

	return port.ID
}

func DeletePort(provider *gophercloud.ProviderClient, portID string) {
	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
		Region: os.Getenv("Region")}) //"RegionOne"
	if err != nil {
		klog.Error(err)
	}

	err = ports.Delete(client, portID).ExtractErr()
	if err != nil {
		klog.Error(err)
	}
}

func UpdatePort(provider *gophercloud.ProviderClient, portID string, hostID string) {
	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
		Region: os.Getenv("Region")}) //"RegionOne"
	if err != nil {
		klog.Error(err)
	}

	portUpdateOpts := ports.UpdateOpts{}

	updateOpts := portsbinding.UpdateOptsExt{
		UpdateOptsBuilder: portUpdateOpts,
		HostID:            &hostID,
	}

	_, err = ports.Update(client, portID, updateOpts).Extract()
	if err != nil {
		klog.Error(err)
	}
}
