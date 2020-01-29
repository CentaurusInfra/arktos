DNS Service in Multi-Tenancy Cluster
====================================

## Background
DNS is one of the critical add-ons for Kubernetes cluster. Pods rely on DNS service to look up services (including headless services). Canonical Kubernetes cluster has well-understood DNS service. In this doc, we will focus on the impact of DNS introduced by multi-tenancy. See [DNS for Services and Pods](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/) for more information.

The connectivity from regular pod to DNS pod via DNS service IP is out of this doc's scope. The assumption we have here is a pod gets DNS name lookup through DNS service IP.

This design proposal is a multi-tenant DNS solution for Kubernetes cluster. DNS records are stored in DNS pod regardless of their tenants - it is not so-called hard-isolation. DNS query from one pod can be of arbitrary tenant by default - no isolation is provided; however, it is possible to somehow (not 100%) limit DNS queries for the specified tenant only, presenting sort of soft-isolation.

## DNS Service
Kubernetes cluster sets up a DNS service, which is backed by a few DNS pods. The DNS service is of ClusterIP type, having a well-known Virtual IP address, e.g. 10.0.0.10. By default, pods sends DNS query requests to that VIP to get back VIP of service of interests.

Service Objects in API server should be extended to support multi-tenancy. In other words, service type is of tenant scope, it has tenant property in its metadata.

Its full name, in DNS term, should be something like "kube-dns.kube-system.<system-tenant-name>.svc.cluster.local" (system-tenant-name to be decided yet).

## DNS Pod
Cluster starts a deployment which specifies number of instances of DNS pods. The choices of DNS container image include kubedns, CoreDNS etc. The default DNS image used by Kubernetes v1.11+ is CoreDNS.

With introduction of multi-tenants, type A record of service foo inside of namespace bar of tenant baz, in DNS application should the fully qualified name foo.bar.baz.svc.cluster.local, given cluster base domain is cluster.local. DNS binary needs to be able to consume tenant as part of FQDN.

## Pod DNS configuration
There are a few options of DNS configuration provided by Kubernetes. No matter what option pod uses, its DNS configuration should have following properties, for the example of pod in namespace bar of tenant baz:
```text
search bar.baz.svc.cluster.local baz.svc.cluster.local svc.cluster.local cluster.local 
options ndots:6
```
Kubelet is responsible to inject such configuration to pod when pod is started.

Should we want to limit queries inside its tenant only, the search property could be
```text
search bar.baz.svc.cluster.local baz.svc.cluster.local
```
This however can not prevent pod from querying services of other tenants if FQDN is used directly.
## Process of Pod to resolve service IP
When a pod of namespace bar in tenant baz needs to lookup service foo, 
1. it uses /etc/resolv.conf to identity the DNS service IP, and derives the fully qualified domain name based (FQDN) on the search property - the first FQDN is foo.baz.bar.svc.cluster.local;
2. pod sends DNS query request to the DNS service IP, to look up foo.bar.baz.svc.cluster.local; 
3. request is eventually received by a DNS pod (probably redirect by kube-proxy or other means);
4. DNS pod locates foo.bar.baz.svc.cluster.local and finds its type A VIP address (or a set of IP addresses for headless service), responds with DNS answer;
5. the pod receives the DNS answer, and gets the type A address(es).

pod in another namespace in same tenant is able to get foo resolved by query of "foo.bar", at the second DNS attempt. Likewise, pod in other tenant does so by query of "foo.bar.baz" at the third DNS attempt.

## Affected Components
### DNS binary & image
[CoreDNS](https://github.com/coredns/coredns.git) is chosen as the first DNS binary to support multi-tenancy, due to its adoption in community.
We may need to fork CoreDNS repo.
<br/>Code change is around consuming tenant property from service definition to produce proper FQDN.
<br/>Other related work includes image building and publishing (assuming we have proper image registry account).
### DNS pod/service
DNS pod/service yaml file needs minor change reflecting the new binary image.
### Kubelet
Code change is mainly on Kubelet, to add tenant in DNS config search property.
### Admission Control
As tenant wil be part of FQDN, its naming convention has to abide by [RFC1123Label](https://tools.ietf.org/html/rfc1123); this needs to be enforced by admission control when tenant is being created.

## Assumptions & External Dependencies
1. DNS pods are able to access the API server by Kubernetes service VIP (as specified by env KUBERNETES_SERVICE_HOST) at certain tcp port (env KUBERNETES_SERVICE_PORT);
2. Service type is already tenant-scoped, having metadata.tenant property;
3. There is a system (infra) tenant; kubernetes & kube-dns services belong to that tenant;
4. There is a available image registry to upload the extended DNS image for cluster to pull (for small dev env it is fine without it).
