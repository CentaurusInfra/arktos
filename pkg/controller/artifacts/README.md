Network CRD & related controllers
=================================

## Network CRD

Network CRD is the custom API type to provide hard network isolation. Please see [multi-tenancy network design proposal](../../../docs/design-proposals/multi-tenancy/multi-tenancy-network.md). 

The CRD yaml file, crd-network.yaml, __MUST__ be applied __BEFORE__ any controller runs, in cluster provision process.

In test environment, simply running below command after kube-apiserver has been started:

```bash
kubectl apply -f crd-network.yaml
```

In real production environment, it should have RBAC related role definitions (__TODO: be provided later__).

## Controllers

In Arktos, each tenant shall have its own network objects separated from other tenants. Many components, including  tenant controller, network controller, service/endpoints controller, will work on network objects.

In typical Arktos system, most controllers run as part of kube-controller-manager (or workload-controller-manager), except for network provider specific ones (like network controller) - they usually run as standalone deployments.
