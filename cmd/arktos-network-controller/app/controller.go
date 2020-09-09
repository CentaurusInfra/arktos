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

package app

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	arktos "k8s.io/arktos-ext/pkg/generated/clientset/versioned"
	arktoscheme "k8s.io/arktos-ext/pkg/generated/clientset/versioned/scheme"
	arktosinformer "k8s.io/arktos-ext/pkg/generated/informers/externalversions/arktosextensions/v1"
	arktosv1 "k8s.io/arktos-ext/pkg/generated/listers/arktosextensions/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	dnsServiceDefaultName = "kube-dns"
	dnsBaseName           = "coredns"
	dnsRoleName           = "system:coredns"
	dnsRoleBindingName    = "system:coredns"
	flatNetworkType       = "flat"
	clusterAddonLabelKey  = "k8s-app"
)

// Controller represents the flat network controller
type Controller struct {
	domainName   string
	cacheSynced  cache.InformerSynced
	store        arktosv1.NetworkLister
	queue        workqueue.RateLimitingInterface
	netClientset *arktos.Clientset
	svcClientset *kubernetes.Clientset
	recorder     record.EventRecorder
}

// New creates the controller object
func New(domainName string, netClientset *arktos.Clientset, svcClientset *kubernetes.Clientset, informer arktosinformer.NetworkInformer) *Controller {
	utilruntime.Must(arktoscheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: svcClientset.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	return &Controller{
		domainName:   domainName,
		store:        informer.Lister(),
		queue:        workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		cacheSynced:  informer.Informer().HasSynced,
		netClientset: netClientset,
		svcClientset: svcClientset,
		recorder:     eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "flat-network-controller"}),
	}
}

// Run starts the control loop with workers processing the items
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("starting flat network controller")
	klog.V(5).Info("waiting for informer caches to sync")
	if !cache.WaitForCacheSync(stopCh, c.cacheSynced) {
		klog.Error("failed to wait for cache to sync")
		return
	}

	klog.V(5).Info("staring workers of flat network controller")
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.V(5).Infof("%d workers started", workers)
	<-stopCh
	klog.Info("shutting down flat network controller")
}

func (c *Controller) runWorker() {
	for {
		item, queueIsEmpty := c.queue.Get()
		if queueIsEmpty {
			break
		}

		c.process(item)
	}
}

// process will read a single work item off the work queue and attempt to process it
func (c *Controller) process(item interface{}) {
	defer c.queue.Done(item)

	key, ok := item.(string)
	if !ok {
		klog.Errorf("unexpected item in queue: %v", item)
		c.queue.Forget(item)
		return
	}

	tenant, name, err := parseKey(key)
	if err != nil {
		klog.Errorf("unexpected string in queue; discarding: %s", key)
		c.queue.Forget(item)
		return
	}

	net, err := c.store.NetworksWithMultiTenancy(tenant).Get(name)
	if err != nil {
		klog.Warningf("failed to retrieve network in local cache by tenant %s, name %s: %v", tenant, name, err)
		c.queue.Forget(item)
		return
	}

	klog.V(5).Infof("processing network %s/%s", net.Tenant, net.Name)

	if net.Spec.Type == flatNetworkType {
		err = manageFlatNetwork(net, c.netClientset, c.svcClientset, true, c.domainName)
	} else {
		err = manageNonFlatNetwork(net, c.netClientset, c.svcClientset, true, c.domainName)
	}

	if err != nil {
		c.queue.AddRateLimited(key)
		c.recorder.Eventf(net, corev1.EventTypeWarning, "FailedProvision", "failed to provision network %s/%s: %v", net.Tenant, net.Name, err)
		return
	}

	c.recorder.Eventf(net, corev1.EventTypeNormal, "SuccessfulProvision", "successfully provision network %s/%s", net.Tenant, net.Name)
	c.queue.Forget(item)
}

// manageNonFlatNetwork is the core logic to manage a non-flat typed network object
func manageNonFlatNetwork(net *v1.Network, netClient arktos.Interface, svcClient kubernetes.Interface, toDeployDNS bool, domainName string) error {
	if net.DeletionTimestamp != nil {
		// graceful termination; for now does nothing
		// todo: manage resource cleanup properly
		return nil
	}

	var svc *corev1.Service
	var err error
	if len(net.Status.DNSServiceIP) == 0 {
		if svc, err = createOrGetDNSService(net, svcClient); err != nil {
			return fmt.Errorf("failed to get or create per-network DNS service: %v", err)
		}
	}

	if toDeployDNS {
		if err = deployDNSForNetwork(net, svcClient, domainName); err != nil {
			return fmt.Errorf("failed to deploy per-network DNS: %v", err)
		}
	}

	if len(net.Status.DNSServiceIP) == 0 {
		// dns service IP might be empty if it is of external IPAM and the external provider has not assign one yet
		if len(svc.Spec.ClusterIP) == 0 && net.Status.Phase == v1.NetworkPending {
			// network status is already pending
			return nil
		}

		netReady := net.DeepCopy()
		if len(svc.Spec.ClusterIP) > 0 {
			// since dns service gets IP addr allocated, we need to update network object with the DNS service IP
			netReady.Status.DNSServiceIP = svc.Spec.ClusterIP
			netReady.Status.Message = "DNS service IP allocated"
			netReady.Status.Phase = v1.NetworkReady
		} else {
			netReady.Status.Phase = v1.NetworkPending
			netReady.Status.Message = "waiting for DNS service IP assigned"
		}
		_, err = netClient.ArktosV1().NetworksWithMultiTenancy(netReady.Tenant).UpdateStatus(netReady)
	} else {
		// got dns service IP assigned; make request to ready phase if applicable
		if len(net.Status.Phase) == 0 || net.Status.Phase == v1.NetworkPending {
			netReady := net.DeepCopy()
			netReady.Status.Phase = v1.NetworkReady
			netReady.Status.Message = "DNS service IP allocated"
			_, err = netClient.ArktosV1().NetworksWithMultiTenancy(netReady.Tenant).UpdateStatus(netReady)
		}
	}

	return err
}

// manageFlatNetwork is the core logic to manage a flat typed network object
func manageFlatNetwork(net *v1.Network, netClient arktos.Interface, svcClient kubernetes.Interface, toDeployDNS bool, domainName string) error {
	if net.DeletionTimestamp != nil {
		// graceful termination; for now does nothing
		// todo: manage resource cleanup properly
		return nil
	}

	if len(net.Status.DNSServiceIP) != 0 {
		klog.V(5).Infof("network %s/%s/%s already have DNS service IP %s; skipped", net.Tenant, net.Namespace, net.Name, net.Status.DNSServiceIP)
		return nil
	}

	svc, err := createOrGetDNSService(net, svcClient)
	if err != nil {
		return fmt.Errorf("failed to get or create per-network DNS service in tenant %s for network %s: %v", net.Tenant, net.Name, err)
	}

	if toDeployDNS {
		if err = deployDNSForNetwork(net, svcClient, domainName); err != nil {
			return fmt.Errorf("failed to deploy per-network DNS: %v", err)
		}
	}

	// since dns service is in place, flat type network is Ready now
	netReady := net.DeepCopy()
	netReady.Status.Phase = v1.NetworkReady
	netReady.Status.DNSServiceIP = svc.Spec.ClusterIP
	netReady.Status.Message = "DNS service ready; network ready"
	_, err = netClient.ArktosV1().NetworksWithMultiTenancy(netReady.Tenant).UpdateStatus(netReady)
	return err
}

func createOrGetDNSService(net *v1.Network, svcClient kubernetes.Interface) (*corev1.Service, error) {
	nsDNS := metav1.NamespaceSystem
	nameDNS := dnsServiceDefaultName + "-" + net.Name
	dns := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameDNS,
			Tenant:    net.Tenant,
			Namespace: nsDNS,
			Labels: map[string]string{
				v1.NetworkLabel:      net.Name,
				clusterAddonLabelKey: nameDNS,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "dns",
					Protocol:   "UDP",
					Port:       53,
					TargetPort: intstr.FromInt(53),
				},
				{
					Name:       "dns-tcp",
					Protocol:   "TCP",
					Port:       53,
					TargetPort: intstr.FromInt(53),
				},
				{
					Name:       "metrics",
					Protocol:   "TCP",
					Port:       9153,
					TargetPort: intstr.FromInt(9153),
				},
			},
			Selector: map[string]string{
				clusterAddonLabelKey: nameDNS,
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	svc, err := svcClient.CoreV1().ServicesWithMultiTenancy(nsDNS, net.Tenant).Create(&dns)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			// todo: consider differing temporary errors from permanent errors
			// todo: to fail the network provision in case of permanent errors
			return nil, err
		} else {
			svc, err = svcClient.CoreV1().ServicesWithMultiTenancy(nsDNS, net.Tenant).Get(nameDNS, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
		}
	}

	return svc, nil
}

// Add puts key of the network object in the work queue
func (c *Controller) Add(obj interface{}) {
	net, ok := obj.(*v1.Network)
	if !ok {
		klog.Fatalf("got non-network object %v", obj)
	}

	c.queue.Add(genKey(net))
}

func (c *Controller) Update(_, newObj interface{}) {
	newNetwork, ok := newObj.(*v1.Network)
	if !ok {
		klog.Fatalf("got non-network new object %v", newObj)
	}

	c.queue.Add(genKey(newNetwork))
}

func genKey(net *v1.Network) string {
	return fmt.Sprintf("%s/%s", net.Tenant, net.Name)
}

func parseKey(key string) (tenant, name string, err error) {
	segs := strings.Split(key, "/")
	if len(segs) != 2 {
		err = fmt.Errorf("invalid key format")
		return
	}

	tenant = segs[0]
	name = segs[1]
	return
}
