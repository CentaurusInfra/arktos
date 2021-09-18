/*
Copyright 2020 Authors of Arktos.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"bytes"
	"fmt"
	"html/template"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	v1 "k8s.io/arktos-ext/pkg/apis/arktosextensions/v1"
	"k8s.io/client-go/kubernetes"
)

func deployDNSForNetwork(net *v1.Network, client kubernetes.Interface, saltSuffix, domainName, kubeAPIServerIP, kubeAPIServerPort string) error {
	if err := ensureToCreateServiceAccount(net, client, saltSuffix); err != nil {
		return err
	}

	if err := ensureToCreateClusterRole(net, client); err != nil {
		return err
	}

	if err := ensureToCreateClusterRoleBindging(net, client, saltSuffix); err != nil {
		return err
	}

	if err := ensureToCreateConfigMap(net, client, domainName, saltSuffix); err != nil {
		return err
	}

	if err := ensureToCreateDeployment(net, client, kubeAPIServerIP, kubeAPIServerPort, saltSuffix); err != nil {
		return err
	}

	return nil
}

func getSuffixedName(base, suffix string) string {
	if len(suffix) == 0 {
		return base
	}
	return base + "-" + suffix
}

func ensureToCreateServiceAccount(net *v1.Network, client kubernetes.Interface, saltSuffix string) error {
	saName := getSuffixedName(dnsBaseName, saltSuffix)
	if _, err := client.CoreV1().ServiceAccountsWithMultiTenancy(metav1.NamespaceSystem, net.Name).Get(saName, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("unexpected error to get service account %s/%s/%s: %v", net.Tenant, metav1.NamespaceSystem, dnsBaseName, err)
		}

		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saName,
				Tenant:    net.Tenant,
				Namespace: metav1.NamespaceSystem,
			},
		}
		if _, err := client.CoreV1().ServiceAccountsWithMultiTenancy(metav1.NamespaceSystem, net.Tenant).Create(sa); err != nil {
			if !errors.IsAlreadyExists(err) {
				return fmt.Errorf("falied to create service account %s/%s/%s: %v", net.Tenant, metav1.NamespaceSystem, dnsBaseName, err)
			}
		}
	}

	return nil
}

func ensureToCreateClusterRole(net *v1.Network, client kubernetes.Interface) error {
	if _, err := client.RbacV1().ClusterRolesWithMultiTenancy(net.Tenant).Get(dnsRoleName, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("unexpected error to get cluster role %s/%s: %v", net.Tenant, dnsRoleName, err)
		}

		role := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   dnsRoleName,
				Tenant: net.Tenant,
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"list", "watch"},
					APIGroups: []string{""},
					Resources: []string{"endpoints", "services", "pods", "namespaces"},
				},
			},
			AggregationRule: nil,
		}
		if _, err := client.RbacV1().ClusterRolesWithMultiTenancy(net.Tenant).Create(role); err != nil {
			if !errors.IsAlreadyExists(err) {
				return fmt.Errorf("falied to create cluster role %s/%s: %v", net.Tenant, dnsRoleName, err)
			}
		}
	}

	return nil
}

func ensureToCreateClusterRoleBindging(net *v1.Network, client kubernetes.Interface, saltSuffix string) error {
	saName := getSuffixedName(dnsBaseName, saltSuffix)
	crbName := getSuffixedName(dnsRoleBindingName, saltSuffix)
	if _, err := client.RbacV1().ClusterRoleBindingsWithMultiTenancy(net.Tenant).Get(dnsRoleBindingName, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("unexpected error to get cluster role binding %s/%s: %v", net.Tenant, dnsRoleBindingName, err)
		}

		rolebinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   crbName,
				Tenant: net.Tenant,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      saName,
					Namespace: metav1.NamespaceSystem,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     dnsRoleName,
			},
		}
		if _, err := client.RbacV1().ClusterRoleBindingsWithMultiTenancy(net.Tenant).Create(rolebinding); err != nil {
			if !errors.IsAlreadyExists(err) {
				return fmt.Errorf("falied to create cluster role binding %s/%s: %v", net.Tenant, dnsRoleBindingName, err)
			}
		}
	}

	return nil
}

func ensureToCreateConfigMap(net *v1.Network, client kubernetes.Interface, domainName, saltSuffix string) error {
	name := dnsBaseName + "-" + net.Name
	name = getSuffixedName(name, saltSuffix)
	if _, err := client.CoreV1().ConfigMapsWithMultiTenancy(metav1.NamespaceSystem, net.Tenant).Get(name, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("unexpected error to get configmap %s/%s/%s: %v", net.Tenant, metav1.NamespaceSystem, name, err)
		}

		corefile, err := generateCorefile(net.Name, domainName)
		if err != nil {
			return fmt.Errorf("falied to create configmap %s/%s/%s: %v", net.Tenant, metav1.NamespaceSystem, name, err)
		}

		configmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Tenant:    net.Tenant,
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string]string{
				"Corefile": corefile,
			},
		}
		if _, err := client.CoreV1().ConfigMapsWithMultiTenancy(metav1.NamespaceSystem, net.Tenant).Create(configmap); err != nil {
			if !errors.IsAlreadyExists(err) {
				return fmt.Errorf("falied to create configmap %s/%s/%s: %v", net.Tenant, metav1.NamespaceSystem, name, err)
			}
		}
	}

	return nil
}

func ensureToCreateDeployment(net *v1.Network, client kubernetes.Interface, kubeAPIServerIP, kubeAPIServerPort, saltSuffix string) error {
	name := getSuffixedName(dnsBaseName+"-"+net.Name, saltSuffix)
	label := dnsServiceDefaultName + "-" + net.Name
	saName := getSuffixedName(dnsBaseName, saltSuffix)
	configmap := getSuffixedName(dnsBaseName+"-"+net.Name, saltSuffix)
	readOnlyRootFilesystem := true

	if _, err := client.AppsV1().DeploymentsWithMultiTenancy(metav1.NamespaceSystem, net.Tenant).Get(name, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("unexpected error to get deployment %s/%s/%s: %v", net.Tenant, metav1.NamespaceSystem, name, err)
		}

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Tenant:    net.Tenant,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					clusterAddonLabelKey: label,
					v1.NetworkLabel:      net.Name,
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						clusterAddonLabelKey: label,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							clusterAddonLabelKey: label,
							v1.NetworkLabel:      net.Name,
						},
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  "system-cluster-critical",
						ServiceAccountName: saName,
						Tolerations: []corev1.Toleration{
							{
								Key:      "CriticalAddonsOnly",
								Operator: "Exists",
							},
						},
						NodeSelector: map[string]string{
							"kubernetes.io/os": "linux",
						},
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
									{
										Weight: 100,
										PodAffinityTerm: corev1.PodAffinityTerm{
											LabelSelector: &metav1.LabelSelector{
												MatchExpressions: []metav1.LabelSelectorRequirement{
													{
														Key:      clusterAddonLabelKey,
														Operator: "In",
														Values:   []string{label},
													},
												},
											},
											TopologyKey: "kubernetes.io/hostname",
										},
									},
								},
							},
						},
						DNSPolicy: "Default",
						Volumes: []corev1.Volume{
							{
								Name: "config-volume",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configmap,
										},
										Items: []corev1.KeyToPath{
											{
												Key:  "Corefile",
												Path: "Corefile",
											},
										},
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "coredns",
								Image:           "coredns/coredns:1.7.0",
								ImagePullPolicy: "IfNotPresent",
								Args: []string{
									"-conf",
									"/etc/coredns/Corefile",
								},
								Env: []corev1.EnvVar{
									{Name: "KUBERNETES_SERVICE_HOST", Value: kubeAPIServerIP},
									{Name: "KUBERNETES_SERVICE_PORT", Value: kubeAPIServerPort},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "config-volume",
										ReadOnly:  true,
										MountPath: "/etc/coredns",
									},
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "dns",
										ContainerPort: 53,
										Protocol:      "UDP",
									},
									{
										Name:          "dns-tcp",
										ContainerPort: 53,
										Protocol:      "TCP",
									},
									{
										Name:          "metrics",
										ContainerPort: 9153,
										Protocol:      "TCP",
									},
								},
								SecurityContext: &corev1.SecurityContext{
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{
											"NET_BIND_SERVICE",
										},
									},
									ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
								},
								LivenessProbe: &corev1.Probe{
									Handler: corev1.Handler{
										HTTPGet: &corev1.HTTPGetAction{
											Path:   "/health",
											Port:   intstr.IntOrString{IntVal: 8080},
											Scheme: "HTTP",
										},
									},
									InitialDelaySeconds: 60,
									TimeoutSeconds:      5,
									SuccessThreshold:    1,
									FailureThreshold:    5,
								},
								ReadinessProbe: &corev1.Probe{
									Handler: corev1.Handler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/ready",
											Port: intstr.IntOrString{
												IntVal: 8181,
											},
											Scheme: "HTTP",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		if _, err := client.AppsV1().DeploymentsWithMultiTenancy(metav1.NamespaceSystem, net.Tenant).Create(deployment); err != nil {
			if !errors.IsAlreadyExists(err) {
				return fmt.Errorf("falied to create deployment %s/%s/%s: %v", net.Tenant, metav1.NamespaceSystem, name, err)
			}
		}
	}

	return nil
}

func generateCorefile(network, domainName string) (string, error) {
	const corefileTeml = `
    .:53 {
        errors
        health
        ready
        rewrite name kube-dns.kube-system.svc.{{ .DNSName }}. kube-dns-{{ .Network }}.kube-system.svc.{{ .DNSName }}.
        rewrite name kubernetes.default.svc.{{ .DNSName }}. kubernetes-{{ .Network }}.default.svc.{{ .DNSName }}.
        kubernetes {{ .DNSName }} in-addr.arpa ip6.arpa {
          pods insecure
          fallthrough in-addr.arpa ip6.arpa
          ttl 30
        }
        prometheus :9153
        forward . /etc/resolv.conf
        cache 30
        loop
        reload
        loadbalance
    }
`

	tmpl := template.Must(template.New("corefile").Parse(corefileTeml))
	var output bytes.Buffer
	object := struct {
		Network string
		DNSName string
	}{
		Network: network,
		DNSName: domainName,
	}
	if err := tmpl.Execute(&output, &object); err != nil {
		return "", fmt.Errorf("unexpected error to generate corefile: %v", err)
	}

	return output.String(), nil
}
