package handler

import v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"

// Handler interface contains the methods that are required
type Handler interface{
	Init() error
	ObjectCreated(tenant string, namespace string, obj interface{})
	ObjectDeleted(tenant string, namespace string, obj interface{})
	ObjectUpdated(tenant string, namespace string, obj interface{})
}

// Generate a no used virtual presence ip from site
func RequestVirtualPresence(site v1.ESite) (*v1.VirtualPresence, error) {
	// Request a virtual presence ip which is not used
	return nil, nil
}

func ReleaseVirtualPresence(vp v1.VirtualPresence) error {
	// Release virtual presence
	return nil
}