package httpserver

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudhub/utils"
)

const (
	NamespaceGateway        string = "cloudgateway"
	CaSecretName            string = "casecret"
	CaDataName              string = "cadata"
	CaKeyDataName           string = "cakeydata"
	CloudGatewaySecretName  string = "cloudgatewaysecret"
	CloudGatewayCertName    string = "cloudgatewaydata"
	CloudGatewayKeyDataName string = "cloudgatewaykeydata"
)

func GetSecret(secretName string, ns string) (*v1.Secret, error) {
	client, err := utils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create KubeClient, error: %v", err)
	}
	return client.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
}

func CreateSecret(secret *v1.Secret, ns string) error {
	client, err := utils.KubeClient()
	if err != nil {
		return fmt.Errorf("failed to create KubeClient, error: %s", err)
	}
	if err := CreateNamespaceIfNeed(client, ns); err != nil {
		return fmt.Errorf("failed to create Namespace for CloudGateway, err: %v", err)
	}
	if _, err := client.CoreV1().Secrets(ns).Create(secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			client.CoreV1().Secrets(ns).Update(secret)
		} else {
			return fmt.Errorf("failed to create secret %s in namespace %s, err: %v", secret.Name, ns, err)
		}
	}
	return nil
}

func CreateCaSecret(certDER, key []byte) error {
	caSecret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      CaSecretName,
			Namespace: NamespaceGateway,
		},
		Data: map[string][]byte{
			CaDataName:    certDER,
			CaKeyDataName: key,
		},
		StringData: map[string]string{},
		Type:       "Opaque",
	}
	return CreateSecret(caSecret, NamespaceGateway)
}

func CreateCloudGatewaySecret(certDER, key []byte) error {
	cloudGatewayCert := &v1.Secret{
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      CloudGatewaySecretName,
			Namespace: NamespaceGateway,
		},
		Data: map[string][]byte{
			CloudGatewayCertName:    certDER,
			CloudGatewayKeyDataName: key,
		},
		StringData: map[string]string{},
		Type:       "Opaque",
	}
	return CreateSecret(cloudGatewayCert, NamespaceGateway)
}

func CreateNamespaceIfNeed(client *kubernetes.Clientset, ns string) error {
	if _, err := client.CoreV1().Namespaces().Get(ns, metav1.GetOptions{}); err == nil {
		return nil
	}
	newNamespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	_, err := client.CoreV1().Namespaces().Create(newNamespace)
	if err != nil && apierrors.IsAlreadyExists(err) {
		err = nil
	}
	return err
}
