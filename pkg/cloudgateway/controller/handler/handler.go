package handler

// handler interface contains the methods that are required
type Handler interface{
	Init() error
	ObjectCreated(obj interface{})
	ObjectDeleted(obj interface{})
	ObjectUpdated(obj interface{})
}
