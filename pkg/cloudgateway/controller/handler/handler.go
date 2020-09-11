package handler

// Handler interface contains the methods that are required
type Handler interface{
	ObjectCreated(tenant string, namespace string, obj interface{})
	ObjectDeleted(tenant string, namespace string, obj interface{})
}