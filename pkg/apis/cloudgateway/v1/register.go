package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/apis/cloudgateway"
)

var SchemeGroupVersion = schema.GroupVersion{
	Group:		cloudgateway.GroupName,
	Version:	cloudgateway.Version,
}

var (
	SchemeBuilder	= runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme		= SchemeBuilder.AddToScheme
)

func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		SchemeGroupVersion,
		&ESite{},
		&ESiteList{},
		&EGateway{},
		&EGatewayList{},
		&VirtualPresence{},
		&VirtualPresenceList{},
		&EService{},
		&EServiceList{},
		&EServer{},
		&EServerList{},
		&EPolicy{},
		&EPolicyList{},
		&ServiceExpose{},
		&ServiceExposeList{},
		)

	// register the type in the scheme
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

