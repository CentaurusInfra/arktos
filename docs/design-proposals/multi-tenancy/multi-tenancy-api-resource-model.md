

# Multi-Tenancy API Resource Model in Arktos

Qian Chen, Xiaoning Ding


## Introduction

This document is a part of the multi-tenancy design documents in Arktos. It mainly describes how we organize API resources in API server in a multi-tenancy manner.

Please refer to other design documents in the "multi-tenancy" document directory if you want to learn about other parts in the multi-tenancy design.

## API Resource Model

### Tenant Spaces & System Space

API Server stores all API Resources and manages the access to these resources. Another layer of separation is needed if we want tenants to have their own API resources without worrying about name conflicts.

We achieve this by implementing a new concept "space". A space is pretty much a virtual cluster. It contains both namespace-scoped resources and cluster-scoped resources. 

When a tenant is initialized, by default a space with the name of the tenant is created. 

Below are some examples resources in a space for tenant "t1":

>- **/api/v1/tenants/t1/namespaces/default/pods/pod1**
>- **/api/v1/tenants/t1/persistentvolumes/pv1**

As we can see, spaces provide an extra layer of resource separation. Within his space, a tenant can freely create his own resources like namespaces or PVs. They don't need to worry about naming conflicts.

If an API resource is not under any tenant space, then it's in the **system space**. There is only one system space in the whole cluster, which by default is only accessible to system tenant users.

Example resources in the system space:

>- **/api/v1/tenants/system/namespaces/default/pods/pod1**
>- **/api/v1/tenants/system/persistentvolumes/pv1**

The below section describes what changes are required to implement tenant spaces, including endpoint resolution, storage, access control, etc.

### Tenant Object

A tenant is a new API resource type defined in system space. A sample yaml file is as follows.

```text
#sample-tenant.yaml 
apiVersion: v1
kind: Tenant
metadata:
  name: sample-tenant-1  
```

### Tenant Name in Object Meta

A new field , "tenant", is added to its object metadata. 

Following are the sample yaml files:

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

### Resource URL Endpoint Resolution

**Full path** is the path that uniquely identifies an API resource, whether it's in system space or tenant spaces. The **objectMata.selflink** field of an object is always its full path.

**Short path** is desgined for compatibility reasons. A short path does not have the "tenants/###/" part in the url. Instead, it's automatically set by API Server based on the access credentials associated with the request.

For example, when a user in tenant t1 issues an API request to the following API resource:
>- **/api/v1/namespaces/ns-1/pods/pod1**

It will be automatically resolved to the following full path, based on the tenant information in the associated access token:
>- **/api/v1/tenants/t1/namespaces/ns-1/pods/pod1**

Short path enables users to use their existing CLI tool (such as Kubectl) without changes.

The endpoint handlers in API Server are modified to extract the space info from the URL. If not found, the handlers will extract the tenant information from the identity info associated with the current API request. This happens transparently to API invokers.

For developing and testing purpose, an Arktos cluster can be configured to use a default tenant. Default tenant is also required if Arktos works with an IAM service which doesn't provide tenant concepts. The actual tenant name of the default tenant can be anything. It's determined by the API Server configuration parameter "**default-tenant**".

Combining the above logics, below is the URL resolution logics added in endpoint handlers:

```go

if (request.user == null || request.user.tenant == null) 
    &&  default-tenant != null {
   	    request.user.tenant = default-tenant
} 

if target space is specified in resource URL {
   request.tenant = target space
} else {
   if request.user.tenant != system {
    	request.tenant = request.user.tenant
   } 
}

```

URL resolution only fills in the target space information and it doesn't do access controls. Access control is done in authorizers.

### ETCD Key Paths

The key paths of the resources follow follow very simple rules:

1. All resources in system space still have the same etcd key.
* this design may be changed as it impedes the implmentation of tenant-name-based api server partition and etcd partition. *
2. Resources in tenant space will have tenant name before resource name or namespace name, depending whether it's a namespace-scoped resource.

Here are some examples:

  1. A non-namespace-scoped resource in system space will be 

      ```/registry/{resource-type-plural}/{resource}```

      Example: 
      
      ```/registry/apiregistration.k8s.io/apiservices/v1beta1.node.k8s.io```
   
  2. A namespace-scoped resource in system space will be
  
       ```/registry/{resource-type-plural}/{namespace}/{resource}```
        
     Example: 
     
     ```/registry/serviceaccounts/kube-system/job-controller```.
      
      
  3. A non-namespace-scoped resource in tenant space will be
  
       ```/registry/{resource-type-plural}/{tenant}/{resource}```
        
      Example: 
        
       ```/registry/namespaces/tenant-a/kube-public```
        
  4. A namespace-scoped resource in tenant space will be
  
       ```/registry/{resource-type-plural}/{tenant}/{namespace}/{resource}```
        
     Example: 
     
     ```/registry/serviceaccounts/tenant-a/kube-system/job-controller```.

### CRD Support

#### Principles and Outlines of Design

 The following Arktos multi-tenancy design principles will be observed in the design of CRD:
 
 1. **isolation**. Each regular tenant can independently create/delete/update their CRDs (as long as no collision with the system forced CRD, which will be described later) and custom resources based on the CRDs. There will be NO CRD G/V/K collision between two regular tenants. Actually, the CRDs and custom resources of one regular tenant are invisible to another regular tenant.  

 2. **Manageability**. A cluster operator (he should be a user under the system tenant) can introduce a new CRD, where he can choose to:
  * make it usable only to the system tenant.
  * or, make it usable only to a spefic set of to regular tenants and the system tenants, 
  * or, usable to all the tenants.
  * Additionally, the applicable range of a CRD can be changed by the cluster operator at any time in a simple and quick manner. That will be useful to CRD test and rolling update.  


  
  When a system CRD is usable to all regular tenants, this sharing can be:
  * **forced**. It overrides the local CRDs with the same name in non-system tenant spaces. Once a forced system CRD is deployed, a regular tenant's attempt to create a CRD with the same name will be rejected. 

    Cluster admins are suggested to use it only for those resource types mandatory to every tenant.  
  * **optional**. The regular tenant is allowed to override such system CRDs by defining a local CRD.

  *(What the managebility should look like is not finalized as we are still working on it. So far, the work on forced-sharing system CRD is checked in.)*
 
 Note: it does not contradict the principle of isolation. The isolation principle is about two regular tenants, while this principle is about the system tenant and regular tenants. 

 3. **Autonomy**. It means that the tenant can install his own CRD and deploy the resources and CRD operators within his own tenant's space, without the help from the cluster admins. Autonomy also means that the tenant admin has has control on what CRD will be used in his space when there is an collision between the tenant's CRD and that of the system tenant (surely, the system CRD is not forced). The tenant can choose MultiTenancyCRD policy to be :
  * NeverUseSystemCRDUnlessForced
  * SystemCRDFirst
  * LocalCRDFirst

  Besides, the tenant user can change the MultiTenancyCRD policy anytime. 

  The default policy will be LocalCRDFirst. Under the default policy, if a tenant has deployed CRD objects and CRD operators based on its own CRD, the introduction of an overlapping system CRD will not cause break.

 4. **Backward compatibility**. The legacy CRD definitions and CRD operators continue to work in Arktos. More specifically, it means:
    * An existing defintion of CRD (usually in a .yaml file) can be applied without any change.
    * An existing CRD operator image can work in Arktos without re-compiling or image rebuilding.
    * An existing CRD operator source code build and work in Arktos. So developers can create new operators or revise existing operators in Arktos without learning new APIs.

    Note that the backward compability is only about the CRD operators working in a given tenant's space. If we would like to an operator to work across multiple tenants, code changes are needed to collect info from different tenants and differentiate them.

Wneed to point out that, regular tenants have no access to certain resources, such as nodes and daemonsets. So a CRD definition or a CRD operator using those resources will no longer be supported in a regular tenant's space. Surely, they continue to work in the system tenant's space. 
 
 *(The design on how regular tenants use CRDs such as [rook](ttps://rook.io), which need access to cluster-scope resources like nodes, is still under investigation.)*

#### Detailed Design

 The key changes to realize the isolation principles are:
 * CustomResourceDefinition will be changed to a tenant-scoped resource. As a result, same CRD G/V/K can co-exist as long as they live under different tenants. 
 * APIResource struct ( which is the data structure to holds the info of resource types suspported in API server) will have a new field, Tenant. For non-CRD resources, this field is empty, which means the resources are applicable to all tenants. For CRD resources, it indicates the belonging tenant.
 * The CRD shown in the response to a resource discovery request should only show those usable to tenant in the request.

The key changes to realize managability and automony are:
* A new field, ApplicableRange, will be added to the CustomResourceDefinitionSpec. The field is ignored in a regular-tenant CRD. Yet for a system-tenant CRD, it indicates the scope of the regular tenants that this CRD is applicable to. This field is a list of annotations, which are used to finding matching tenants. (other designs may also work, as long as the field give info in finding matching tenants)

* A system tenant CRD with the following label is forced shared, which overrides all the local CRDs of regular tenants which has the same name.

  **arktos.futurewei.com/crd-sharing-policy: forced**
 
* the logic to find a matching CRD for a request in the API server will be changed as the following:

```
GetMatchingCRD(G/V/K, tenant) (found_CRD, error)
  if (found a system forced sharing CRD) {
    return (the system CRD), nil
  }

  switch (MultiTenancyCRDPolicy) {
    case NeverUseSystemCRDUnlessForced:
      if (found matching CRD in tenant's space ) {
          return (tenant's CRD), nil
      } 
      return nil, NotFoundError

    case SystemCRDFirst:
      if (found matching CRD in system tenant's space ) && (tenant is in the system CRD's ApplicableRange) {
          return (the system CRD), nil
      } 
      if (found matching CRD in tenant's space ) {
          return (tenant's CRD), nil
      } 

      return nil, NotFoundError

    case LocalCRDFirst:
      if (found matching CRD in tenant's space ) {
          return (tenant's CRD), nil
      } 
      if (found matching CRD in system tenant's space ) && (tenant is in the system CRD's ApplicableRange) {
          return (the system CRD), nil
      } 

      return nil, NotFoundError
    }
  }
  ```
 
 Note that the above search-CRD logic is in the APIServer side, so we don't need to worry about authorizing regular tenant to access some system resources.

 To maintain backward compatility, "short path" will also be implemented in CRD request resolution. More details about "short path" is given in the above section of "Resource URL Endpoint Resolution".

 Some other changes include:
 * In CRD validation, the ResourceScope will be checked. If a regular-tenant CRD has the resource scope set to "Cluster-Scope", it will be changed to "Tenant-scoped".
 
## Appendix: Design Discussions

### Comparison with Other resource models

We also consider the following approaches and decided to use the native multi-tenancy design presented in this doc. 

1.	[Virtual Cluster Based Multi-Tenancy](https://www.cncf.io/blog/2019/06/20/virtual-cluster-extending-namespace-based-multi-tenancy-with-a-cluster-view/). In this method, each tenant owns a dedicated kubernetes control plane, including API servers/controllers/etcd.

2.	We also consider the design which makes no changes to the api server url formats and add no new field in resources, but embeds the tenant info in the existing fields. For namespace-scope objects, the value of namespace field will be {tenant}--{namespace}. For tenant-scoped objects, the object name will be {tenant}--{resource-name}. 
        
   The benefit of this method is that the code change to apiserver/etcd/client-go is minimized. Yet it will encounter the following issues:

	* 	The cross reference of objects of different types under the same tenant will be inefficient. When a PVC needs to find the matching PV, or a pod needs to get the podSecurityPolicy, it needs to identify the scope of the resource, and extract the tenant info from the namespace field or name field. It could be an even bigger problem when dealing with dynamic objects, where the object type is not known.  
	*  The watching or list of resources under a tenant will be difficult. Kubernetes now support watch/list base on field boundary of the etcd keys. As tenant name is just part of another field, we need to change the behavior of K8s. 

	*  The internal namespace/object names, with tenant info embedded, are different from what the clients of api server needs. Therefore, extra translation is needed for incoming api urls and object bodies, as well as the outgoing object bodies and self links, etc.  

	We choose the native multi-tenancy design, as it provides the following advantages comparing the above methods:

	* All the tenants can share one control plane. It is lightweight comparing to the virtual cluster based multi-tenancy design where each tenant needs one control plane. It is also more efficient in resource utilization as the data-plane and control-plane resources can be shared across tenants. 

	* A simple and straightforward mechanism to provide strong multi-tenancy over K8s. The second option described above is less straightforward. 

	In short, this design pave the way to build a very scalable public cloud management platform. 

### System Tenant

Which option do we choose, for non-tenant resources: option 1-- /api/v1/tenants/system/clusterroles/cls1; option2: /api/v1/clusteroles/cls1

We decided to use option 2. 

If we use option 1, we need to do the following redefine the scope of all the existing cluster-scope resources to be tenant-scope and its tenant value can only “system”. 
It is huge code change and the resource model definition is less clear than what we have now. It needs to make change to 
1.	Api server url resolution
2.	Resource definition
3.	Client-go
4.	Anywhere it is necessary to identify the scope of a resource object ( a lot of such code in kubectl)

Actually We just need to make sure only system-tenant has access to the cluster-scope resources. Only code change to the user authorization is needed.

### Default Tenant

Originally we had a built-in special tenant named "system".  In the current design, it's just a pre-created space and nothing is special.

Here is how it works:

* When API Server is started with command-line parameter "default-tenant=xxx". It creates a new object /api/v1/tenants/xxx if it doesn't exist yet.
* The corresponding space is created by the tenant controller.
* In URL resolution part, API Server will automatically use "xxx" as tenant if request.user.tenant is not available.

### Other CRD Designs discussed

#### Copy-CRD-to-tenant scheme
We have considered the design to copy the system CRD to each tenant's space for simplicity in CRD searching logic. In this design, when the system tenant decides to publish a CRD, the system copies the CRD to spaces of applicable regular tenants. Yet I am moving away from it as it is less flexible and responsive than desired. Consider the following scenarios:
* When copying a CRD from system's space to tenant's space, the tenant's own CRD is overwritten. If the system CRD contains some tricky bugs and the tenant would like to revert to his own copy CRD after a period, he has to retrieve the CRD source file and apply it. If the original CRD yaml file is gone, he cannot even revert. While with current design, it can be done by just changing MultiTenancyCRDPolicy of the tenant.
* If the system tenant would like to recall a published CRD, it needs to remove the CRD from each affected tenant. Yet with the no-copy design, it involves only one deletion in the system space.

Besides, the copy-CRD-to-tenant design also has the following disadvantages comparing to design given earlier:
1. delay in deployment. It costs time and resources to copying.
2. It needs a new controller to watch for the CRD, which will copy the CRD to tenant spaces or remove CRD from tenant spaces. 

The advantage of copy-CRD-to-tenant scheme is simplicity, as all the CRDs that the tenant has access are in the tenant's own space and we don't need to worry about authorization. However, as explained above, the logic to search matching CRD is in the api server side, not the client side. Therefore, no concern about client authorization is necessary. Besides, per the pseudo-code given above, the complexity of the CRD matching is acceptable.

#### More Flexibility on CRD Sharing Between the System and Regular Tenants

