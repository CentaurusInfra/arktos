package handler

import "k8s.io/klog"

// EPolicyHandler is a epolicy object handler
type EPolicyHandler struct{}

func (t *EPolicyHandler) Init() error{
	klog.V(4).Info("EPolicyHandler.Init")
	return nil
}

func (t *EPolicyHandler) ObjectCreated(obj interface{}) {
	klog.V(4).Info("EPolicyHandler.ObjectCreated")
}

func (t *EPolicyHandler) ObjectUpdated(obj interface{}) {
	klog.V(4).Info("EPolicyHandler.ObjectUpdated")
}

func (t *EPolicyHandler) ObjectDeleted(obj interface{}) {
	klog.V(4).Info("EPolicyHandler.ObjectDeleted")
}
