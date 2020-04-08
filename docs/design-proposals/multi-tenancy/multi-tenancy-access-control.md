# Access Control in Arktos Multi-Tenancy

Peng Du, Xiaoning Ding

## Introduction

This document is a part of the multi-tenancy design documents. It describes access control related topics in multi-tenancy design, including authentication, authorization and admission control. 

Please refer to other design documents in the "multi-tenancy" document directory if you want to learn about other parts in the multi-tenancy design.

## Authentication

Same to Kubernetes, Arktos supports configurable multiple authenticators. Each authenticator implements a different way of authenticating users or service accounts. For an incoming API request, each authenticator will be tried in sequence until one of them succeed. 

The key changes to each authenticator is that it needs to retrieve the tenant information and put it into request context.

### 1.Certificate Authenticator

The existing X509 certificate authenticator reads "/CN" and "/O" from cert subject, and map them to user name and groups respectively. To make the authenticator tenancy-aware, "/O" is changed to map to tenant and "/OU" (Organization Unit) to map to groups instead. To maintain backward compatibility, two formats are to be used. If "tenant:" is at the beginning of "/O", then what's after the it will be treated as tenant. For example, with "/O=tenant:tenantA/OU=app1/OU=app2", tenant will be set as "tenantA", and groups will be ["app1", "app2"]. If "tenant:" is not found at the beginning of "/O", the string before the first ":" in /CN will be used as tenant. For example, with "/CN=tenantB:demo/O=app1/O=app2", tenant will be set to "tenantB".

Here is a sample certificate request with the new format specifying tenant explicitly in ***/O***:

```text
openssl req -new -key demo.pem -out demo-csr.pem -subj "/CN=userA/O=tenant:tenantA/OU=app1"
```

And here is a sample certificate request with the old format where tenant is take implicitly from ***/CN***:

```text
openssl req -new -key demo.pem -out demo-csr.pem -subj "/CN=tenant:tenantA/O=app1/O=app2"
```

If both of the above formats fail to produce a valid tenant name, the request will be rejected.

### 2.Token File Authenticator
With --token-auth-file=/path/to/tokenfile, token is read from tokenfile with the following format:

```text
[token],[user],[uid],[groups...],[other data]
```

Token file is prepared by system admin and supplied to cluster at start-up time. To support multi-tenancy, this format is changed to:

```text
[token],[user],[uid],[groups...],[other data],,[tenant]
```

The empty ",," at the end indicates a tenant name is to follow.  This new format provides backward compatibility. If no such field of arrangement exists, it will be identified as the old format and thus tenant is defaulted to "system".

### 3.Basic Auth File Authenticator
With--basic-auth-file=/path/to/somefile, token is read from tokenfile with the following format:

```text
[password],[user],[uid],[groups...],[other data]
```

Password file is prepared by system admin and supplied to cluster at start-up time. To support multi-tenancy, this format is changed to:

```text
[password],[user],[uid],[groups...],[other data],,[tenant]
```

The empty ",," at the end indicates a tenant name is to follow.  This new format provides backward compatibility. If no such field of arrangement exists, it will be identified as the old format and thus tenant is defaulted to "system".

### 4.Service Account Authenticator
Service account can use JSON Web Token (JWT) as  a bearer token to authenticate with apiserver.  Both service account and sercret will be changed to tenant-scope. The tenant name is to be added to the bearer token so that it can be retrieved and verified during authentication.

### 5.Webhook Token Authentication

Arktos delegates user management to external components such as OpenStack Keystone through webhook token authentication using [k8s-keystone-auth](https://github.com/kubernetes/cloud-provider-openstack/blob/master/docs/using-keystone-webhook-authenticator-and-authorizer.md). A token is issued by Keystone once a user successfully logs in at Keystone. This token is passed from kubectl to apiserver for authentication as a bearer token, and in turn it is passed by k8s-keystone-auth to the Keystone service for validation. Upon success, user info such as user's name, domain, project, roles is returned to Arktos. For arktos, Keystone domain is mapped to "tenant", while the combination of Keystone project and role maps to Arktos role, and Keystone role assignment which contains project, role and user is mapped to the Arktos rolebinding accordingly. Code change is required in apiserver to extract the domain returned from Keystone and set it to tenant in context.

### 6.OpenID Connect Tokens

[OpenID Connect](https://openid.net/connect/) is used for authentication with some OAuth2 providers such as Azure Active Directory, AWS, and Google, where tokens are obtained and passed to apiserver as JWT token for authentication. Changes are to be made to extract and set tenant in context.

## Authorization

Arktos supports different ways of authorization: RBAC, ABAC, Node and Webhook.

In general the authorization rule is:

* A system tenant user can access any space.
* A non-system tenant user can only access its own space.
* Optionally, a non-system tenant admin can create a policy object in its space to allow other tenants to access its space. The policy object will specify which tenants are allowed to access which resources.

### 1.RBAC Authorizer

The following new check are added into the RBAC implementation:

```go
if (user.tenant != "system") && (user.tenant != resource.tenant) {
   return DecisionDeny
}
```

Any cross-tenant attempt not issued by a system tenant user will be blocked here. If this check passes, it continues to the original RBAC logics:

```go
If any ClusterRoleBinding allows the request {
   return DecisionAllow
}

If the requested resource is a namespace-scoped resource {
   if any RoleBinding within the namespace allows the request {
      return DecisionAllow
   }
}

return DecisionDeny
```

With this change, all existing RBAC policies still work.  

### 2.ABAC Authorizer

The ABAC authorizer needs to be changed to read in an extra field, "tenant", from the ABAC policy file. And also compare the user.tenant with requestInfo.tenant in its authorization check.

Below is a sample ABAC policy file that says user Alice in tenant A can access resources tenants/tenant-a/* :

```go
{"apiVersion": "abac.authorization.kubernetes.io/v1beta1", "kind": "Policy", "spec": {"tenant": "tenant A", "user": "alice", "tenant": "tenant-a", "namespace": "*", "resource": "*", "apiGroup": "*"}}
```

If the tenant field is missing, it is the system tenant. If the space field is missing, it's a resource in system space.


### 3.Node Authorizer & Webhook Authorizer

Node authorizer is specially for authorizing requests from Kubelet. All these requests are made by users in group "system:node" with a username of "system:node:<nodeName>". And Webhook authorizer is a general callback mechanism to do authorization.

No code change is required for these two authorizers.


### 4.Cross-Tenant Access (P2)

There are two ways to enable cross-tenant access:

* cluster admins define cross-tenant access in ABAC policy files.
* Tenant Admins creates an authorization policy file in its own space. So far the content format can be same to the ABAC policy file except the "space" field will be ignore.

### 5.Enforce policies Cross-Tenant (P2)

The idea is to allow system tenants to publish policy objects to tenant spaces:

* Define a new annotation "multi-tenancy.k8s.io/published"
* If an object in system space is marked with this annotation, one controller will copy this object to all tenant spaces.
* After the objects are copied to the tenant spaces, tenant users are not allowed to modify or delete the object.
* If system tenant removes the object from system space, the object will be removed from all tenant spaces too.

(**TBD: more details needed**)

## Admission Control

Some new admission controllers are needed to handle the following checks:

* Tenant-level quota enforcement
* Some API resources are not allowed to be created in a tenant space. Now the list includes: nodes and DaemonSets. 


## Appendix: Design Discussions

### Node, NodeSelector and DaemonSet

Tenant users shouldn't see or manage node resources. Node resources are system resources. However nodes are referenced in tenant resources, such as "nodeName" field in a pod, node selector in deployment object, etc. 

The plan is:
1. Node objects will only be under system scope, as it is now. An admission controller will prevent nodes from being created in tenant scope. DaemonSet objects are also not allowed in tenant scope.
2. Node references such as "nodeName" in pods are still kept, for performance reason. Tenant users can only see the node name (suggest 
3. Node selectors in tenant resources are disabled or limit supported, enforced by tenant admission controller, depending on configuration.  By default it's disabled. 

### ClusterRoles

As we discussed earlier, it is better not to introduce a new resource type of TenantRole. We are going to use Role and ClusterRole only.

I am considering to downgrade the scope of ClusterRole from cluster-scope to tenant-scope. So that each tenant can define their own ClusterRoles without worrying name collision. (In this sense, we may need to rename ClusterRole to something like WideRangeRole or GeneralRole ). 

A concern we have that is how to differ the system-tenant rules which applies to all the tenants from the rules that apply to the system-tenant itself only. 

I found that there is a field of “ScopeType” in the type of Rule.I found this field was defined but not actively used in K8s. (I search ScopeType in k8s repo and found no appearance other than the Rule type definition).

So each regular user can define two type of roles, ClusterRole and Role, which operates on a tenant scope and a namespace scope. “ScopeType” field in the rule will simply be ignored.

And a system user also have two types of roles, ClusterRole and Role. Yet the field of “ScopeType” in the ClusterRole rule will be used. If ScopeType value is “Cluster”, the rule applies to all the resources under all the tenants. If the value is “tenanted” ( that will a new value we add), the rule is  about the resources under the system tenant only. 

Actually, I feel the “ScopeType” field will be very useful for a scalable platform like Arktos. For example, we can define a rule which applies to a specific set of tenants, so we can have different system administrators to take care different portion of tenants.

### Integration with Keystone 

The Keystone token saved by the end client, which will be sent over in a http request to Arktos API server, does not include the info of roles. The official doc is at https://docs.openstack.org/keystone/pike/admin/identity-tokens.html. Chapter 3 in book "Identity, Authentication, and Access Management in OpenStack" (downloadable at https://github.com/hocchudong/thuctap012017/blob/master/TamNT/Openstack/Keystone/ref/Identity%2C%20Authentication%2C%20and%20Access%20Management%20in%20OpenStack.PDF) gives the full story on the evolution of token format and why the info like info are no longer included in client token.

So Arktos needs to check with Keystone to get the role info when it receives the http request. 

Arktos needs to identify the scope of the request to be cluster/tenant/namespace scope, then Arktos sends out a system/domain/project scoped request. To reduce the burden on the Keystone, Arktos needs to cache the results.

### Resource Quota Isolation

In k8s, the computation resources (such as cpu and memory) are provisioned via [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/), which is an optional namespace-scope feature. Similar quota based resource provisioning mechanism will be implemented in Arktos. Yet the resource quota control in the tenant level is mandatory, in order to guarantee the performance and usage isolation among different tenants. 

In more details:

1. ResourceQuota control is mandatory in the tenant level. Namely, every tenant must have the ResourceQuota set. If the quota are not specified by the user when creating the tenant, the system will set them to the default values. 	The resource quota control in the namespace levle is still optional. A namespace can choose to specify ResourceQuota or not.
2. A pod request will be accepted only if both the tenant and the namespace resource quota are not violated. In the case that the resource quota of the tenant is less than the sums of the quota of the namespaces, the requests are handled in a first-come-first-serve basis.
3.	Quota control on more resource types will be supported. K8s 1.15 supports control on the resource of cpu and memory. As Arktos will support both VMs and containers, resource control over resources like network bandwidth or the number of VMs should also be considered. Yet research on what resources need to be done is outside the scope of this doc and it will be covered by a different proposal. 

### Usage and Performance Isolation

To prevent a tenant over-use the system resources, we also need to constrain/track/monitor how each tenant uses the system. 

Thus we need:

1. A tenant-basis rate limiting mechanism to 
2. A tenent-level usage metric/log data collection mechanism.

Note that the k8s API server does not have rate limiting in place as it is designed to work in a trustable environment. 
