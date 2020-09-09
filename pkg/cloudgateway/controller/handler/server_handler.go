package handler

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset/versioned"
	listers "k8s.io/kubernetes/pkg/client/listers/cloudgateway/v1"
)

// EServerHandler is a server object handler
type EServerHandler struct {
	serverLister  listers.EServerLister
	siteLister    listers.ESiteLister
	gatewayClient clientset.Interface
}

func NewEServerHandler(serverLister listers.EServerLister, siteLister listers.ESiteLister,
	gatewayClient clientset.Interface) *EServerHandler {
	h := &EServerHandler{
		serverLister:  serverLister,
		siteLister:    siteLister,
		gatewayClient: gatewayClient,
	}

	return h
}

func (h *EServerHandler) ObjectCreated(tenant string, namespace string, obj interface{}) {
	server := obj.(*v1.EServer)
	klog.V(4).Info("ServerHandler.ObjectCreated: %v", server)

	// Server check
	serverSite, err := h.siteLister.ESitesWithMultiTenancy(namespace, tenant).Get(server.ESiteName)
	if err != nil {
		klog.Errorf("Get site from server error, server:%v", server)
		// TODO(nkwangjun): Record event for service create
		nerr := h.updateServerStatus(server, v1.ServerError, namespace, tenant)
		if nerr != nil {
			klog.Error("UpdateServer to error failed, %v, err:%v", server, nerr)
		}
		return
	}

	if server.VirtualPresenceIp == "" {
		// Request virtual presence
		vp, err := RequestVirtualPresence(*serverSite)
		if err != nil {
			klog.Errorf("Request virtual presence for server error:%v, serverSite:%v", err, serverSite)
			// TODO(nkwangjun): Record event
			nerr := h.updateServerStatus(server, v1.ServerError, namespace, tenant)
			if nerr != nil {
				klog.Error("UpdateServer to error failed, %v, err:%v", server, nerr)
			}
			return
		}

		// Update virtual presence for server
		err = h.updateServerVP(server, vp.VirtualPresenceIp, namespace, tenant)
		if err != nil {
			klog.Errorf("Update server vp error, server:%v", server)
			nerr := ReleaseVirtualPresence(*vp)
			if nerr != nil {
				klog.Fatalf("Release virtual presence ip error:%v, %v", nerr, vp)
			}
			return
		}
	}

	// TODO(nkwangjun): Sync transfer traffic, update status
}

func (h *EServerHandler) ObjectDeleted(tenant string, namespace string, obj interface{}) {
	server := obj.(*v1.EServer)
	klog.V(4).Info("EServerHandler.ObjectDeleted")

	// TODO(nkwangjun): Sync transfer traffic, update status

	// Release virtual presence
	if server.VirtualPresenceIp != "" {
		// Delete the virtual presence
		err := h.gatewayClient.CloudgatewayV1().VirtualPresencesWithMultiTenancy(namespace,
			tenant).Delete(server.VirtualPresenceIp, &metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("Delete virtual presence error, %s", server.VirtualPresenceIp)
		}
	}
}

func (h *EServerHandler) updateServerVP(server *v1.EServer, vpIp string, namespace string, tenant string) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// We can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	serverCopy := server.DeepCopy()
	serverCopy.VirtualPresenceIp = vpIp
	// We use Update func to update the virtual presence ip
	_, err := h.gatewayClient.CloudgatewayV1().EServersWithMultiTenancy(namespace, tenant).Update(serverCopy)
	return err
}

func (h *EServerHandler) updateServerStatus(server *v1.EServer, phase v1.ServerPhase, namespace string,
	tenant string) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// We can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	serverCopy := server.DeepCopy()
	serverCopy.Status = v1.ServerStatus{Phase: phase}
	_, err := h.gatewayClient.CloudgatewayV1().EServersWithMultiTenancy(namespace, tenant).Update(serverCopy)
	return err
}
