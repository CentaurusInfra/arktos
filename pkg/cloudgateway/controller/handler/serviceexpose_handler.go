package handler

import (
	"fmt"
	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset/versioned"
	listers "k8s.io/kubernetes/pkg/client/listers/cloudgateway/v1"
)

// ServiceExposeHandler is a service expose object handler
type ServiceExposeHandler struct {
	serviceLister listers.EServiceLister
	siteLister    listers.ESiteLister
	policyLister  listers.EPolicyLister
	serverLister  listers.EServerLister
	gatewayLister listers.EGatewayLister
	gatewayClient clientset.Interface
}

// ServiceExposeObj
type ServiceExposeObj struct {
	serviceExpose   v1.ServiceExpose
	service         v1.EService
	allowedSites    []v1.ESite
	allowedPolicies []PolicyObj
}

// PolicyObj
type PolicyObj struct {
	policy         v1.EPolicy
	allowedServers []v1.EServer
}

// NewServiceExposeHandler creates a new ServiceExposeHandler
func NewServiceExposeHandler(sl listers.EServiceLister, siteLister listers.ESiteLister,
	policyLister listers.EPolicyLister, serverLister listers.EServerLister,
	gatewayLister listers.EGatewayLister) *ServiceExposeHandler {
	se := &ServiceExposeHandler{
		serviceLister: sl,
		siteLister:    siteLister,
		policyLister:  policyLister,
		serverLister:  serverLister,
		gatewayLister: gatewayLister,
	}

	return se
}

func (h *ServiceExposeHandler) Init() error {
	klog.V(4).Info("ServiceExposeHandler.Init")
	return nil
}

// Handle the service expose request
// 1. Generate traffic flows from the request
//    traffic flows contains the site and flows basic info
// 2. Send the traffic flows control message to the dataflow model if the associated site is
//    in the cloud, or send it to the associated site from the hub communication tunnel
// 3. The dataflow model can be implemented use the driver/adapter mode, if use transform mechanism data flow
//    by openvswitch, it can work with tap/tun device to do the data flow. In this case, hub communication tunnel
//    must support the Binary Message transfer
func (h *ServiceExposeHandler) ObjectCreated(tenant string, namespace string, obj interface{}) {
	se := obj.(*v1.ServiceExpose)
	klog.V(4).Info("ServiceExposeHandler.ObjectCreated: %v", se)

	// Transform the service expose to obj
	seObj, err := h.getServiceExposeObj(tenant, namespace, se)
	if err != nil {
		// Update service expose to wrong status and return
		// TODO(nkwangjun): update status here
		return
	}

	// Send traffic flow required info
	// 1. Send to the service site
	// 2. Send to the server site
	// 3. Record event every step
	// 4. Update status of the service expose
	// 5. If associated gateway not be assigned, wait until the message send successful
	h.syncServiceExpose(seObj)
}

func (h *ServiceExposeHandler) syncServiceExpose(seObj *ServiceExposeObj) {

}

func (h *ServiceExposeHandler) getServiceExposeObj(tenant string, namespace string, expose *v1.ServiceExpose) (
	*ServiceExposeObj, error) {
	seObj := &ServiceExposeObj{
		serviceExpose: *expose,
	}

	// Check the service by expose
	serviceName := expose.EServiceName
	service, err := h.serviceLister.EServicesWithMultiTenancy(namespace, tenant).Get(serviceName)
	if err != nil {
		klog.Errorf("Get service in service expose error, serviceexpose:%v, service name:%s", expose, serviceName)
		return nil, err
	}

	// If VirtualPresenceIp is not assigned, return error
	if service.VirtualPresenceIp == "" {
		klog.Errorf("Serviceexpose error, service not ready, there is no virtual presence ip, service:%v",
			service)
		return nil, err
	}

	seObj.service = *service

	// Check exposed site
	siteNames := expose.ESites
	allowedSites := []v1.ESite{}
	seObj.allowedSites = allowedSites
	for _, siteName := range siteNames {
		site, err := h.siteLister.ESitesWithMultiTenancy(namespace, tenant).Get(siteName)
		if err != nil {
			klog.Errorf("Get site in service expose to error, serviceexpose:%v, site name:%s", expose, siteName)
			return nil, err
		}

		allowedSites = append(allowedSites, *site)
	}

	policyNameList := expose.EPolicys
	allowedPolicies := []PolicyObj{}
	seObj.allowedPolicies = allowedPolicies
	for _, policyName := range policyNameList {
		policy, err := h.policyLister.EPoliciesWithMultiTenancy(namespace, tenant).Get(policyName)
		if err != nil {
			klog.Errorf("Get policy in service expose to error, serviceexpose:%v, policy name:%s", expose,
				policyName)
			return nil, err
		}

		policyObj := &PolicyObj{
			policy:         *policy,
			allowedServers: []v1.EServer{},
		}

		// check policy server is valid or not
		for _, serverName := range policy.AllowedServers {
			server, err := h.serverLister.EServersWithMultiTenancy(namespace, tenant).Get(serverName)
			if err != nil {
				klog.Errorf("Get server in service expose policy allowed to error, serviceexpose:%v, "+
					"policy name:%s, server name:%s", expose, policyName, serverName)
				return nil, err
			}

			// If VirtualPresenceIp is not assigned, return error
			if server.VirtualPresenceIp == "" {
				klog.Errorf("Serviceexpose error, service not ready, there is no virtual presence ip, server:%v",
					server)
				return nil, fmt.Errorf("Server virtual presence ip is invalid, %v", server)
			}

			policyObj.allowedServers = append(policyObj.allowedServers, *server)
		}

		allowedPolicies = append(allowedPolicies, *policyObj)
	}

	return seObj, nil
}

func (h *ServiceExposeHandler) updateServiceVP(service *v1.EService, vpIp string, namespace string, tenant string) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// We can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	serviceCopy := service.DeepCopy()
	serviceCopy.VirtualPresenceIp = vpIp
	// We use Update func to update the virtual presence ip
	_, err := h.gatewayClient.CloudgatewayV1().EServicesWithMultiTenancy(namespace, tenant).Update(serviceCopy)
	return err
}

func (h *ServiceExposeHandler) ObjectUpdated(tenant string, namespace string, obj interface{}) {
	klog.V(4).Info("ServiceExposeHandler.ObjectUpdated")
}

func (h *ServiceExposeHandler) ObjectDeleted(tenant string, namespace string, obj interface{}) {
	klog.V(4).Info("ServiceExposeHandler.ObjectDeleted")
}

