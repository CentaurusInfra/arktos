package handler

import "k8s.io/klog"

// EPolicyHandler is a epolicy object handler
type EPolicyHandler struct{}


func (t *EPolicyHandler) ObjectCreated(tenant string, namespace string, obj interface{}) {
	klog.V(4).Info("EPolicyHandler.ObjectCreated")
}

func (t *EPolicyHandler) ObjectDeleted(tenant string, namespace string, obj interface{}) {
	klog.V(4).Info("EPolicyHandler.ObjectDeleted")
}
