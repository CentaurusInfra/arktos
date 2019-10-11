

# Multi-Tenancy in Alkaid

Qian Chen, Xiaoning Ding


## Motivation

Kubernetes provides a solution for automating deployment, scaling, and operations of application containers across clusters of hosts. However, it does not have support multi-tenancy. There is no explicit “tenant” concept in the system, and there is no resource isolation, usage isolation or performance isolation for different tenants.

Alkaid is evolved from Kubernetes but designed for public cloud. For a cluster management system supporting public cloud workloads, multi-tenancy is a fundamental requirement. This design document proposes works need to be done to support multi-tenancy for Alkaid.


## Hard Multi-Tenancy Model

There are several different multi-tenancy models. Kubernetes community has [some documents](https://docs.google.com/document/d/15w1_fesSUZHv-vwjiYa9vN_uyc--PySRoLKTuDhimjc/edit#heading=h.3dawx97e3hz6) to compare these models. In general, they can be categorized as “soft” models or “hard” models. In some scenarios like private cloud or within an organization, there is a certain level of trust between different tenants. In that case soft model may be enough.

In the scenario of a public cloud there isn’t any trust among tenants. And cloud vendors cannot trust tenants as well. For these reasons, we adopt the hard multi-tenancy model in this design.

Our hard multi-tenancy model is one where:

* Tenants do not trust each other.
* Tenants cannot see each other.
* Tenants don't share data or interact with each other. 
* Tenants do not impact each other in terms of performance and usage.
* A Tenant owns its resources in an exclusive and isolated way.

## Resources Model

### Tenant Object

A tenant is defined as a group of users/identities who have the exclusive access to its own resources in parallel to other tenants within an Alkaid system. A tenant can have one or multiple namespaces. Two namespaces can have the same name as long as they are under different tenants.

A tenant is an API resource type. A sample yaml file is as follows.

```text
#sample-tenant.yaml 
apiVersion: v1
kind: Tenant
metadata:
  name: sample-tenant-1  
```

### Resource Hierarchy

A resource in Alkaid belongs to one and only one of the following three scopes:
  1.	Cluster scope
  2.	Tenant scope
  3.	Namespace scope

The list of resources under each scope is given in the [Appendix](#Resource-Types-in-Different-Scopes).

A Tenant-scope resource can only be owned by one tenant. For instance, a namespace must be under one specific tenant. Similarly, a namespace-scope resource belongs to one specific namespace (surely, also under the tenant who owns this namespace). 

It needs to be pointed out that two tenants may share cluster-scope resources. For example, two tenants may have VMs in the same node(s) (node is a cluster-scope resource). But each tenant has no idea that other tenants are using the same nodes. A tenant should not even have any knowledge whether there are any other tenant(s) in the cluster at all.

### Tenant Name in Object Meta

For namespace-scope and namespace-scope resource types, a new field , "tenant", will be added to its object metadata. 

Following are the sample yaml files for a namespace ( a tenant-scope resource) and a pod ( a namespace-scope resource).

```text
#sample-namespace.yaml 
apiVersion: v1
kind: Namespace
metadata:
  name: sample-namespace-1 
  tenant: sample-tenant-1 
```

```text
apiVersion: v1
kind: Pod
metadata:
  name: memory-demo
  namespace: sample-namespace-1
  tenant: sample-tenant-1 
spec:
  containers:
  # the following parts are ignored
```

### API URLs
All Alkaid API URLs start with “/apis/{api-group-name}/{version}”, just like k8s. The API URLs for the resources of different scopes are as follows:
  1. Cluster-scoped resources can be addressed by “/apis/{api-group-name}/{version}/*”. 
  2. Tenant-scoped resources are addressed by paths like “/apis/{api-group-name}/{version}/tenants/{tenant}/*”. 
  3. Namespace-scoped resources are addressed by paths like “/apis/{api-group-name}/{version}/tenants/{tenant}/namespaces/{namespace}/*”.
 
  All the resources scoped by a namespace will be deleted when the namespace is deleted. All the resources under a tenant and under all the namespaces of the tenant will be deleted when the tenant is deleted. 

### ETCD Key Paths
The key paths of the resources in backend ETCD will also vary according to their scopes:
  1. The path of cluster-scope resources will be 

      ```/registry/{resource-type-plural}/{resource}```

      Example: ```/registry/apiregistration.k8s.io/apiservices/v1beta1.node.k8s.io```
  2.	The path of tenant-scoped resources will be
  
        ```/registry/{resource-type-plural}/{tenant}/{resource}```
        
        Example: ```/registry/namespaces/system/kube-public```
  3.	The path of namespace-scoped resources will be:
  
        ```/registry/{resource-type-plural}/{tenant}/{namespace}/{resource}```
        
        Example: ```/registry/serviceaccounts/system/kube-system/job-controller```.

### Special Tenants and Namespaces

Two special tenants will be created automaticallyif they don’t exist.

  1. *System*. It is the tenant for the Alkaid system itself.

  2. *Default*. If the tenant of a resource type is not defined, it will be assigned to the default tenant.

Naturally, end users are not allowed to use “system” or “default” as the name of their tenants. Also, these two tenants cannot be deleted.

When a tenant is created, two namespaces will be created under the tenant:

  1. *Default*. The default namespace for objects with no other namespaces.

  2. *System*. The namespace for objects created for system operations.


## Layered Access Control

Role-based access control (RBAC) model will remain in Alkaid, yet with more layers than in K8s. 
In RBAC, a role contains rules that represent a set of permissions. Different scopes have different type of roles. It can be:
1.	A Role within a namespace. A Role is used to grant access to resources within a single namespace. 
2.	A TenantRole within a tenant scope. A TenantRole can be used to grant the same permission as a Role. But it can also be used to grant access to 
  
    a.	Tenant-scope resources 
    
    b.	Namespaced resources across multiple namespaces

3.	A ClusterRole for the Alkaid system-wide scope. A ClusterRole can grant permissions to

    a.	Resources within a namespace or a tenant

    b.	Resources across multiple namespaces within a tenant or across multiple tenants

    c.	Non-resource endpoints (like /healthz)

A role binding grants the permissions defined in a role to a user or a set of users. A RoleBinding, TenantRoleBinding and SystemRoleBinding will serve the aforementioned three role types of different scopes resepectively.

## Resource Quota Isolation

In k8s, the computation resources (such as cpu and memory) are provisioned via [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/), which is an optional namespace-scope feature. Similar quota based resource provisioning mechanism will be implemented in Alkaid. Yet the resource quota control in the tenant level is mandatory, in order to guarantee the performance and usage isolation among different tenants. 

In more details:
1. ResourceQuota control is mandatory in the tenant level. Namely, every tenant must have the ResourceQuota set. If the quota are not specified by the user when creating the tenant, the system will set them to the default values. 	The resource quota control in the namespace levle is still optional. A namespace can choose to specify ResourceQuota or not.
2. A pod request will be accepted only if both the tenant and the namespace resource quota are not violated. In the case that the resource quota of the tenant is less than the sums of the quota of the namespaces, the requests are handled in a first-come-first-serve basis.
3.	Quota control on more resource types will be supported. K8s 1.15 supports control on the resource of cpu and memory. As Alkaid will support both VMs and containers, resource control over resources like network bandwidth or the number of VMs should also be considered. Yet research on what resources need to be done is outside the scope of this doc and it will be covered by a different proposal. 

## Usage and Performance Isolation

To prevent a tenant over-use the system resources, we also need to constrain/track/monitor how each tenant uses the system. Thus we need:
1. A tenant-basis rate limiting mechanism to 
2. A tenent-level usage metric/log data collection mechanism.

Note that the k8s API server does not have rate limiting in place as it is designed to work in a trustable environment. 

## Comparison With Other Approaches ##

We also consider the following approaches and decided to use the native multi-tenancy design presented in this doc. 
1.	[Virtual Cluster Based Multi-Tenancy](https://www.cncf.io/blog/2019/06/20/virtual-cluster-extending-namespace-based-multi-tenancy-with-a-cluster-view/), which is proposed by Alibaba. In this method, each tenant owns a dedicated kubernetes control plane, including API servers/controllers/etcd.
2.	We also consider the design which makes no changes to the api server url formats and add no new field in resources, but embeds the tenant info in the existing fields. For namespace-scope objects, the value of namespace field will be {tenant}--{namespace}. For tenant-scoped objects, the object name will be {tenant}--{resource-name}. 
        
        The benefit of this method is that the code change to apiserver/etcd/client-go is minimized. Yet it will encounter the following issues:

        a.	The cross reference of objects of different types under the same tenant will be inefficient. When a PVC needs to find the matching PV, or a pod needs to get the podSecurityPolicy, it needs to identify the scope of the resource, and extract the tenant info from the namespace field or name field. It could be an even bigger problem when dealing with dynamic objects, where the object type is not known. 

        b.	The watching or list of resources under a tenant will be difficult. Kubernetes now support watch/list base on field boundary of the etcd keys. As tenant name is just part of another field, we need to change the behavior of K8s to access etcd for this. 

We choose the native multi-tenancy design, as it provides the following advantages comparing the above methods:
1. All the tenants can share one control plane. It is lightweight comparing to the virtual cluster based multi-tenancy design where each tenant needs one control plane. It is also more efficient in resource utilization as the data-plane and control-plane resources can be shared across tenants. 
2. A simple and straightforward mechanism to provide strong multi-tenancy over K8s. The second option desribed above is less straightforward. 

In short, this design pave the way to build a very scalable public cloud management platform. 


## Code Changes

A complete multi-tenancy support requires a lot of work. Starting from the basic, we plan to divide the work into several milestones.  
### Milestone I: Resource Model ###

This milestone can be further split several phases. 

**Phase I: API Server Changes**
  
    1.	Define the new resource type of Tenant and expose this resource in APIServer.
    2.	Add the Tenant info to the ObjectMeta of resources.
    3.  Define the scope of each resource as given in [Appendix](#Resource-Types-in-Different-Scopes). A new boolean property, TenantScoped, is added to the definition of each resource, in addition to the exisintg property of NamespaceScoped.  The value of TenantScoped is true for namespace-scoped and tenant-scoped resources, and false for cluster-scoped resources. So the scope of a resource can be deduced from the values of its TenantScoped and NamespaceScoped property.
    4.	Update the key path of the resources in etcd to include the tenant info, as given in [ETCD Key Paths](#ETCD-Key-Paths). Note that there are no change to the values of the K-V pairs in etcd. 
    5.	Install new multi-tenancy aware APIs.
    6.  Generate self-links with tenant info included.

**Phase II: Client-Go Changes**

    Client-go is the clientset to access Kubernetes API. This phase will Update client-go code generators to support multi-tenancy.  The new generators should add the field of tenant in the resource objects, send out multi-tenancy-aware API requests to API server, and make use of the tenant info in the response correctly. The changes will be done to the following generator:
      * client-gen
      * lister-gen
      * informer-gen 

**Phase III: Changes in Other Related Components**  

    1.	Multi-tenancy aware kubectl. The kubectl will be
      *	Able to create/delete/update tenants.
      *	Support operation targeted at a specific tenant or across multiple tenants
    2.	Update other control plane components the control plane code for make the control plane work with multiple tenants. These componentes include but not limited to:
      * controller
      * scheduler
      * kubelet
    3. Add a new controller to monitor the lifecycle of tenants. 


### MileStone 2:  Access Control###

    1.	IAM integration.
    2.	Add the new types of TenantRole/TenantRoleBinding. 
    3.	Update the ClusterRole/ClusterRoleBinding.
    4.	Update the PolicyRule to make it work with the new scope model. 

### Phase 3: Performance and Usage Isolation ###

    1.	Tenant-level Resource quota support.
    2.  Tenant-level rate limiting
    3.  Tenant-level usage metrics/statistics


## Appendix

### Resource Types in Different Scopes

Following are the list of resources belonging to each scope. Those highlighted with asterisks are new additions or revised comparing to [k8s API 1.15.](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/)

#### Cluster-scope resources
  1.	*Tenant 
  2.	ComponentStatus
  3.	Node
  4.	MutatingWebhookConfiguration
  5.	ValidatingWebhookConfiguration
  6.	APIService
  7.	ClusterRole. 
  8.	ClusterRoleBinding
  9.  StorageClass
  10. CSIDriver
  11. CSINode
  12. RuntimeClass

#### Tenant-scope resources:
  1.	Namespace
  2.	PersistentVolume
  3.	CustomResourceDefinition
  4.	TokenReview
  5.	CertificateSigningRequest
  6.	PodSecurityPolicy
  7.	PriorityClass
  8.	VolumeAttachment
  9.	SelfSubjectAccessReview
  10.	SelfSubjectRulesReview
  11.	SubjectAccessReview
  12. AuditSink
  13.	*TenantRoleBinding. It is similar to the existing ClusterRoleBinding/RoleBining in K8S, but working on a tenant level.
  14.	*TenantRole. It is similar to the existing ClusterRole/Role in K8S, but working on a tenant level.
  15.	*TenantResourceQuota. It is similar to the existing namespace-scoped ResourceQuota in K8s, yet it is tenant-scoped.

#### Namespace-Scoped resources:

The following namespace-scoped resources in k8s will continue to exist in Alkaid with similar functionalities, except that their objectMeta include the name of tenant now. 
  1.	PersistentVolumeClaim
  2.	Pod
  3.	PodTemplate
  4.	ResourceQuota
  5.	Secret
  6.	Service
  7.	ServiceAccount
  8.	ControllerRevision
  9.	DaemonSet
  10.	Deployment
  11.	ReplicaSet
  12.	StatefulSet
  13.	LocalSubjectAccessReview
  14.	HorizontalPodAutoscaler
  15.	CronJob
  16.	Job
  17.	Lease
  18.	Event
  19.	Ingress
  20.	NetworkPolicy
  21.	PodDisruptionBudget
  22.	RoleBinding
  23.	Role
  24. ConfigMap
  25. Endpoint
  26. LimitRange
  27. PodPreset
  28. ReplicationController

