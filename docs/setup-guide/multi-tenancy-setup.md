# Multi-Tenancy Setup Guide

This doc describes how to setup multi-tenancy on a testing cluster.

## Tenant Bootstrap
Tenant can be created using the following command:

```bash
kubectl create tenant $[tenant name]
```

When a new tenant is created, a cluster role and rolebinding are automatically created for this tenant and its user "admin". This admin cluster role allows full access to resources in this tenant's space.

In order to access the cluster, a certificate can be used to authenticate and authorize as user admin for this tenant.   

## Set Up Tenant For Testing

[Two scripts](../../hack/setup-multi-tenancy) are provided to quickly set up tenant on an Arktos cluster. There are two steps involved:

### 1. Create A Tenant

```bash
./create_tenant.sh mycompany
setting storage cluster to system
tenant/mycompany created
```

And now that tenant is created, we can verify the initial admin cluster role and rolebinding are created automatically:
```bash
kubectl get clusterrole,clusterrolebinding --tenant=mycompany --all-namespaces

NAME                                               AGE
clusterrole.rbac.authorization.k8s.io/admin-role   2m10s

NAME                                                              AGE
clusterrolebinding.rbac.authorization.k8s.io/admin-role-binding   2m9s
```

Next we set up user access using the other script to assume this role.

### 2. Set Up User Context

```
./setup_client.sh mycompany admin 127.0.0.1
setting up context mycompany-admin-context for mycompany/admin in group mycompany-group at host 127.0.0.1
generating a client key mycompany.key...
creating a sign request mycompany.csr...
creating an tenant certificate mycompany-admin.crt and get it signed by CA
Setting up tenant context...
cleaned up mycompany.csr
***************************
Context has been setup for tenant mycompany/admin in /var/run/kubernetes/admin.kubeconfig.

Use 'kubectl use-context mycompany-admin-context' to set it as the default context

Here's an example on how to access cluster:

+ kubectl --context=mycompany-admin-context get pods
No resources found.
```

This script produces certificates with the given tenant name, user name and cluster ip, and then creates a kubectl context in */var/run/kubernetes/admin.kubeconfig*. The 3rd parameter is optional so if none provided, localhost will be used.

Now the cluster can be accessed by tenant "mycompany" on kubectl by specifying "--context=mycompany-admin-context" as show in the example. 

Here is another example 
```
kubectl --context=mycompany-admin-context create -f ../../test/yaml/vanilla.yaml
pod/k8s-vanilla-pod created
kubectl --context=mycompany-admin-context get pods --all-namespaces
NAMESPACE   NAME              HASHKEY               READY   STATUS    RESTARTS   AGE
default     k8s-vanilla-pod   7997974693521935890   1/1     Running   0          4s
``` 

Multiple contexts can be created by calling this script with different parameters.

To prevent having to specify context in every kubectl call, 'use-context' can be used to set the current default context. See the script output.

### The Initial Cluster Role and Rolebinding 
The initial cluster role and binding gives user admin the "root" access in that specific tenant. By assuming this role, tenant owner can then create their own role and binding to set up their own RBAC rules. 