# Scheduler, Node Agent and Controllers in Arktos Multi-Tenancy

Xiaoning Ding, Qian Chen, Peng Du

## Introduction

This document is a part of the multi-tenancy design documents in Arktos. It describes most of the non-API-Server changes, including changes in scheduler, node agent and controllers.

Please refer to other design documents in the "multi-tenancy" document directory if you want to learn about other parts in the multi-tenancy design.

## Scheduler

Scheduler and controllers work in a similar way. They all list/watch some API objects and then make update to API certain objects. This section discusses some common changes that are required in schedulers and controllers to make them tenancy-aware. And also a new tenant controller.

### General Tenancy-aware changes

Scheduler and controllers need to use right client-go APIs to list/watch all objects, or update an object in a certain space. Also tenant-space comparison is required in their implementation.

Take the scheduler as an example:

* When list/watch objects like pods or PVs, it should list/watch all objects in system space and tenant spaces using credentials of system user. This is achieved by calling client-go API with TenantAll:

```go
Client.CoreV1().PodsWithMultiTenancy(pod.Namespace, metav1.TenantAll).List()
```

* When correlating a pod to other resources, space comparison needs to be added:

```go
(... existing comparisons ) && (pod.Tenant == rs.Tenant)
```
 
* When update a certain pod, it should use space-specific calls: 

```go
Client.CoreV1().PodsWithMultiTenancy(pod.Namespace, pod.Tenant).UpdateStatus(pod)
```

### P2: Tenancy-Fairness

Tenancy-faireness is required in schedulers and most controllers. It ensures one tenant won't cause unresponsiveness of other tenants. For example, scheduler shouldn't only schedule pods from one tenant in a certain period. This would make other tenants feel that scheduling is stopped in their space.

The implementation varies depending on the logics of different controllers.

## Controllers

### New: Tenant Controller

A new controller, tenant controller, is introduced to reconcile the changes related to API resource /api/v1/tenants/{tenantName}.

When a new tenant object is created, tenant controller creates the corresponding space and the following default resources in the new space:

   * /api/v1/tenants/{tenantName}/namespaces/default
   * /api/v1/tenants/{tenantName}/namespaces/kube-system
   * /api/v1/tenants/{tenantName}/namespaces/kube-public
   * /api/v1/tenants/{tenantName}/networks/default
 
The next-level default resources, such as such as the default service account for a namespace and default DNS service for a network, will be created by the corresponding controllers.

When a tenant object is deleted, tenant controller will delete all resources in the corresponding space. This is aligned with the behavior of namespace deletion. 

In some cases there are requirements of data backup during tenant deletion. It's not included in current design as the requirement isn't clear. This can also be done by an external tool if needed before deleting tenant objects.

## Node Agents
### Runtime Isolation

Pods running on a same node can belong to different tenants. For containers they share the host OS kernel. A strong-isolation container runtime is needed.

For now we are using Kata as the container runtime. In future we want to evolve some lightweight strong-isolation runtimes, such as Firecracker-style runtimes or gVisor-style runtimes.

### Storage Isolation

PV/PVC already provides isolation. It's orthogonal to multi-tenancy.
space