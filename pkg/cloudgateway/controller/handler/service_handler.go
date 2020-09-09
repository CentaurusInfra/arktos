package handler

import (
	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset/versioned"
	listers "k8s.io/kubernetes/pkg/client/listers/cloudgateway/v1"
)

// EServiceHandler is a service object handler
type EServiceHandler struct {
	serviceLister listers.EServiceLister
	siteLister    listers.ESiteLister
	gatewayClient clientset.Interface
}

func NewEServiceHandler(serviceLister listers.EServiceLister, siteLister listers.ESiteLister,
	gatewayClient clientset.Interface) *EServiceHandler {
	h := &EServiceHandler{
		serviceLister: serviceLister,
		siteLister:    siteLister,
		gatewayClient: gatewayClient,
	}

	return h
}

func (h *EServiceHandler) Init() error {
	klog.V(4).Info("EServiceHandler.Init")
	return nil
}

func (h *EServiceHandler) ObjectCreated(tenant string, namespace string, obj interface{}) {
	service := obj.(*v1.EService)
	klog.V(4).Info("ServiceHandler.ObjectCreated: %v", service)

	// Service check
	serviceSite, err := h.siteLister.ESitesWithMultiTenancy(namespace, tenant).Get(service.ESiteName)
	if err != nil {
		klog.Errorf("Get site from service error, service:%v", service)
		// TODO(nkwangjun): Record event for service create
		return
	}

	// Request virtual presence
	vp, err := RequestVirtualPresence(*serviceSite)
	if err != nil {
		klog.Errorf("Request virtual presence for service error:%v, service:%v", err, serviceSite)
		// TODO(nkwangjun): Record event
		return
	}

	// Update virtual presence for service
	err = h.updateServiceVP(service, vp.VirtualPresenceIp, namespace, tenant)
	if err != nil {
		klog.Errorf("Update service vp error, service:%v", service)
		nerr := ReleaseVirtualPresence(*vp)
		if nerr != nil {
			klog.Fatalf("Release virtual presence ip error:%v, %v", nerr, vp)
		}
		return
	}

	// TODO(nkwangjun): Sync transfer traffic, update status
}

func (h *EServiceHandler) ObjectUpdated(tenant string, namespace string, obj interface{}) {
	klog.V(4).Info("EServiceHandler.ObjectUpdated")
}

func (h *EServiceHandler) ObjectDeleted(tenant string, namespace string, obj interface{}) {
	klog.V(4).Info("EServiceHandler.ObjectDeleted")
}

func (h *EServiceHandler) updateServiceVP(service *v1.EService, vpIp string, namespace string, tenant string) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// We can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	serviceCopy := service.DeepCopy()
	serviceCopy.VirtualPresenceIp = vpIp
	// We use Update func to update the virtual presence ip
	_, err := h.gatewayClient.CloudgatewayV1().EServicesWithMultiTenancy(namespace, tenant).Update(serviceCopy)
	return err
}
