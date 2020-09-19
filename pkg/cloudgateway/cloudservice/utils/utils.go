package utils

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudhub/utils"
)

func GenerateResource(serviceName string) (string, error) {
	gatewayClient, err := utils.GatewayClient()
	if err != nil {
		klog.Errorf("failed to create GatewayClient, error: %v", err)
		return "", err
	}
	service, err := gatewayClient.CloudgatewayV1().EServicesWithMultiTenancy("default", "system").Get(serviceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get service name, error: %v", err)
		return "", err
	}
	resource := fmt.Sprintf("site/%s/%s:%d", service.ESiteName, serviceName, service.Port)
	return resource, nil
}

func GetServiceURL(resource string) (string, error) {
	res := strings.Split(resource, "/")
	host := strings.Split(res[2], ":")
	newResource, err := GenerateResource(host[0])
	if err != nil {
		klog.Errorf("target service cannot reach: %v", err)
		return "", fmt.Errorf("target service cannot reach: %v", err)
	}
	r := strings.Split(newResource, "/")
	return r[2], nil
}
