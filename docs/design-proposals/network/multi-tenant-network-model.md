kubernetes network model for multi-tenancy
==========================================

Canonical kubernetes network model is so-called flat-fully-connected, supplemented by network policy, external service mesh. With the introduction of multi-tenancy, it is possible and desirable that kubernetes cluster provides richer network resource isolation across tenants.


# Terminology
## VPC 
VPC, virtual private cloud, denotes a connected network, particularly without NAT involved in network communication. VPC is the unit of IP address allocation and connectivity. Inside the VPC, IP address does not overlap each other; end points with IP address assigned should be able to connect to each other as long as they are in same VPC without NAT, given proper devices like routers are employed within the VPC.

## VPC Compatibility
VPC A is said compatible to another VPC B means 3 things:
1. IP addresses allocated in VPC A won't conflict with those of VPC B;
2. Pods of VPC B are able to connect to IP addresses (hence services and pods) of VPC A directly;
3. Service IP of VPC A, when accessed by pods of VPC B, may lead to connection to VPC B's pod.

VPC A's pods may be unable to access VPC B's IP address directly, in general cases, as A may be compatible to multiple VPCs, which can conflict with each other with IP address spaces.

## Tenant
Tenant is the unit of users that requests and utilizes resources. For network resources, it is very desirable they are isolated across tenants. Tenants may share the same global VPC, or have their own VPC or multiple VPCs.


# Multi-tenant Network
Though we have other options, we choose following constraint right now for the network model:
1. each tenant has one VPC or more VPCs for pods of tenant;
2. each tenant has single service VPC, which is "compatible" to its pod tenant VPCs;
3. infra VPC is "compatible" to tenant VPCs;

When all tenant VPCs are "compatible" as well, it can be collapsed to the vanilla Kubernetes network actually.

# Network Resources
We only concern about IP address & connectivity here.

## Node
Nodes of Kubernetes cluster get their IP address from external network provider. These nodes can be viewed as the infra VPC. Infra VPC could be "compatible" to tenant VPC, which means IP address spaces not conflicts, and tenant pods can communicate directly to infra pods (not necessary the other way around should exist multi-tenant), in other words, infra & tenant VPC merge to one single VPC - this is what the vanilla Kubernetes cluster has. If we would like to isolate infra VPC from tenant ones, we need to provide connectivity between tenant pods and infra pods through services.

## Pod
Apparently pod gets its IP address inside the VPC scope. 
Tenant pod should be able to "connect" the tenant service directly; which leads to one tenant pod. 
Rarely pod in a tenant needs to access infra service like Kubernetes service; in that case, pod looks up the infra service from DNS service (an infra service itself), gets service ip address (which is in scope of infra VPC as it is for an infra service), and go access to the target infra pod.

## Service
### Tenant Service
Tenant service gets its IP address from tenant service VPC scope. Since it is "compatible" to tenant VPCs, tenant pods, on whatever tenant pod VPC, are able to access it by the service IP address. Depending on the service implementation, access to service will ultimately lead to a certain tenant pod - could be on same VPC as the client pod, or pod on another VPC of same tenant - which is implementation specific.

### Infra Service
Infra service gets IP address from the shared infra VPC. Since infra pods are on the same infra VPC, this service/pod mapping is a bit easier.
There are two special infra services:
1. Kubernetes Service
Service wrapping the Kubernetes API server. It facilitates pods accessing API servers like custom controller pods. 
2. DNS Service
DNS name server service that each pod needs to use to look up named service and get its IP address. DNS pods are running on infra VPC as infra pods, accessing Kubernetes API server through Kubernetes service. 
DNS pods are infra pods by default; if a tenant really wants to, it can provide tenant pods to override the infra ones, inside its own tenant. 

## Network Policy
Each tenant should have its own set of network policy objects, applying to its tenant scope only.

Ingress 
Ingress makes a tenant application exposed to outside of cluster via a proxy. Ingress object is backed by NodePort typed service object.


# Hard-isolation vs soft-isolation
The level of isolation depends on two things in the above depicted network model:
1. tenant pod VPC isolation
if VPCs of a tenant is isolated from VPCs of other tenants, IP address and network connectivity is said to be hard-isolated.
2. DNS pod
If name lookup response is tenant specific, and DNS pods are tenant pods storing tenant scoped names only, it is hard-isolated DNS.

With these two hard-isolated aspects, cluster is categorized as hard-isolation across tenants; otherwise, it is soft-isolated.
