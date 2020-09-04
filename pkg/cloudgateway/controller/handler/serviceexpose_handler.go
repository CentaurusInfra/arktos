package handler

import "k8s.io/klog"

// ServiceExposeHandler is a service expose object handler
type ServiceExposeHandler struct{}

func (t *ServiceExposeHandler) Init() error{
	klog.V(4).Info("ServiceExposeHandler.Init")
	return nil
}

func (t *ServiceExposeHandler) ObjectCreated(obj interface{}) {
	klog.V(4).Info("ServiceExposeHandler.ObjectCreated")
}

func (t *ServiceExposeHandler) ObjectUpdated(obj interface{}) {
	klog.V(4).Info("ServiceExposeHandler.ObjectUpdated")
}

func (t *ServiceExposeHandler) ObjectDeleted(obj interface{}) {
	klog.V(4).Info("ServiceExposeHandler.ObjectDeleted")
}
