

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

As we can see, spaces provides an extra layer of resource separation. Under a certain space, a tenant can freely create their own resources like namespaces or PVs. They don't need to worry about naming conflicts.

If an API resource is not under any tenant space, then it's in the **system space**. There is only one system space in the whole cluster, which by default is only accessible to system tenant users.

Example resources in the system space:

>- **/api/v1/namespaces/default/pods/pod1**
>- **/api/v1/persistentvolumes/pv1**

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

**Full path** is the path that uniquely identifies an API resource, whether it's in system world or tenant worlds. The **objectMata.selflink** field of an object is always its full path.

**Short path** is for compatibility reasons and only for resources in tenant worlds. A short path does not have the world part in the url. Instead, it's automatically set by API Server based on the access credentials associated with the request.

For example, when a user in tenant t1 issues an API request to the following API resource:
>- **/api/v1/namespaces/ns-1/pods/pod1**

It will be automatically resolved to the following full path, based on the tenant information in the associated access token:
>- **/api/v1/tenants/t1/namespaces/ns-1/pods/pod1**

Short path enables users to use their existing CLI tool (such as Kubectl) without changes.

The endpoint handlers in API Server are changed to parse the world part from the URL, and also extract the tenant information from the identify associated with current API request if there is no.  This happens transparently to API invokers.

For developing and testing purpose, an Arktos cluster can be configured to use a default tenant. Default tenant is also required if Arktos works with an IAM service which doesn't provide tenant concepts. The actual tenant name of the default tenant can be anything. It's determined by the API Server configuration parameter "**default-tenant**".

Combining the above logics, below is the URL resolution logics added in endpoint handlers:

```go

if (request.user == null || request.user.tenant == null) 
    &&  default-tenant != null {
   	    request.user.tenant = default-tenant
} 

if target world is specified in resource URL {
   request.tenant = target world
} else {
   if request.user.tenant != system {
    	request.tenant = request.user.tenant
   } 
}

```

URL resolution only fills in the target world information and it doesn't do access controls. Access control is done in authorizers.

### ETCD Key Paths

The key paths of the resources follow follow very simple rules:

1. All resources in system space still have the same etcd key.
2. Resources in tenant space will have tenant name before resource name or namespace name, depending whether it's a namespace-scoped resource.

Here are some examples:

  1. A non-namespace-scoped resource in system world will be 

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

 Each tenant can install their CRD independently, without worrying about resource name collisions. The API group/vision/kind (G/V/K) defined by the CRD object is only accessible to that tenant.
 
 For CRDs installed in system world, by default it's only accessible for tenant users. But if it's marked with "multi-tenancy.k8s.io/published" annotation, it will be accessible to all tenants.
 
 If there is a collision between CRDs in system world and tenant world, the latter one takes precedence if it's accessed by a tenant.
 
 (**TBD: Requires more implementation details**)

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

Originally we had a built-in special tenant named "default".  In the current design, it's just a pre-created world and nothing is special.

Here is how it works:

* When API Server is started with command-line parameter "default-tenant=xxx". It creates a new object /api/v1/tenants/xxx if it doesn't exist yet.
* The corresponding world is created by the tenant controller.
* In URL resolution part, API Server will automatically use "xxx" as tenant if request.user.tenant is not available.



