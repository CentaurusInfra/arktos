# Multi-Tenancy Network

Xiaoning Ding, Hong Chen

## Introduction

This document is a part of the multi-tenancy design documents in Arktos. It describes network-related design changes to support multi-tenancy, including network model, backward-compatibility, DNS and service changes, etc.

Please refer to other design documents in the "multi-tenancy" document directory if you want to learn about other parts in the multi-tenancy design.


## Network Isolation

following resources are network related, which need to be isolated inside network boundary:

* Pod (with regarding to IP address & connectivity)
* Service (including DNS service, and kubernetes service)
* Endpoints / EndpointSlice
* Ingress / egress
* Network Policy

See below illustrative diagram for isolation across networks:
![network resource isolation disgram](images/network-resource-isolation.png)

Network isolation is achieved with a new API object: network:

* Every pod is associated with a certain network object. If not specified it will be associated to a default network object, which is created when a space is initialized. 
* A pod can only communicate with pods within the same network.
* Every network has its own DNS service. An associated DNS service is automatically created when a new network object is created. (**tbd: decide when to deploy the actual DNS server pods**)
* As a general rule, all network related resources are associated with the network object, and semantically residing within the network scope. 

Pods in a network __should__ be able to access API server nodes by their IP addresses - this is required for access to kubernetes service. Network providers need to satisfy this prerequisite.

(**TBD: is it possible to leverage the fact that pods are running on hosts which are already on the node network(e.g. pod being dual-homed with one nic dedicated for node network access)?**)

### Network Object
A network object contains its type, and type-specific configurations. It's a not namespace-scoped object, so that one network object can be shared by applications in multiple namespaces in a same space.

The "type" field is the only mandatory field in a network object. For now there are two types defined: flat and vpc.

A command-line parameter named "default-network-template-path" of tenant controller will decide which default network will be created for a new space.

For flat network env, each space should still have its own default network, as kube-dns service is isolated across tenants.

The content of default network template file should reflect the Network object in json format, with ```{{.}}``` the replacement of tenant name, like
```json
{
    "metadata": {
        "name": "default",
        "finalizers": ["arktos.futurewei.com/network"]
    },
    "spec": {
        "type": "vpc",
        "vpcID": "{{.}}-default-network"
    }
}
```

Below is the definition of a flat network:

```yaml
apiVersion: v1
kind: Network
metadata:
  name: default
spec:
  type: flat
```

And here is a sample of a VPC network:

```yaml
apiVersion: v1
kind: Network
metadata:
  name: vpc-1
spec:
  type: vpc
  vpcID: vpc-1a2b3c4d
```
When a pod is attached to the default network, nothing is needed in pod spec:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - name: nginx
    image: nginx
    ports:
      - containerPort: 443
```
When a pod is attached to a certain network, it needs to set its "network" field:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - name: nginx
    image: nginx
    ports:
      - containerPort: 443
  network: vpc-1
```


When a pod is attached to a certain network and it wants to specify subnet or IP, it needs to be specified in pod fields:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - name: nginx
    image: nginx
    ports:
      - containerPort: 443
  network: vpc-1
  nics:
    - subnet: subnet-1
      ip: 192.168.0.12
```

If these settings are set on a pod attached to a flat network, the settings will be ignored by the flat network controller and also the corresponding CNI plugins. 

(**TBD: for a flat network, can we automatically limit its communication scope to that network?**)

(**TBD: do we plan to support multiple network providers? How would it impact the type definition?**)

### Network Controller

A network controller watches its interesting network objects based on network type.

A flat network controller only initializes the DNS service and deployment for a new network, and also deletes them when a network object is removed. It doesn't do anything else.

A VPC network controller will call VPC service provider to allocate ports for newly created pods and deallocate ports when pods are deleted, in addition to the DNS service initialization and removal that's done in flat network controller. 

### DNS

Each network has its own DNS service, which only contains the DNS entries of pods in that that network, regardless of network type. 

The reasons that DNS service is per-network instead of per-space are:

* For VPC networks IPs could be overlapped. A pod with resolved IP 192.168.0.1 could be in different networks. Therefore it's useless for the DNS clients.
* VPC networks are isolated by nature. It will be difficult to have a shared DNS pod that's accessible to pods in various VPC networks.

(**TBD: is it possible to have _shared_ DNS able to serve multiple network?**)

### Service

#### Type definition

Addition of service spec is spec.network field. By default, it is the default network of the tenant. 

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
  network: my-network
  selector:
    app: MyApp
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9376
```

#### Naming Problem & Solution

Services, as one of the critical network related resource types, are withing the network boundary. There is no such *shared* services of tenant or whole cluster, as both the critical virtual IP address and the associated endpoints (typically leading to pods) are all of same network.

Putting service type in tenant level is out of primacy as it has following flaws:
1. service IP address has to be the same across all networks, which implies some range of service IP needs to be predefined as shared across networks of a tenant (tenant manageability burden);  
1. associating network-agnostic service to network-specific endpoints would introduce significant inconsistency;
1. kubectl query for service (e.g. ```kubectl describe service```) has no way to return meaningful content of endpoints.

The well-known services (default/kubernetes, kube-system/kube-dns) present some problem here. Service kubernetes has tenant/namespace(default) scope in current implementation, unable to have multiple kubernets services with each of them in one network due to name conflict. The same rational applies to kube-dns.
docs/design-proposals/multi-tenancy/images/network-resource-isolation.png
One solution is to making its name colliding space to tenant+namespace+network, which is also breaking and inconsistent with other types (e.g. introducing network as one more parameter to call kubectl get service).

Another solution is to change their name to schema that can uniquely differentiate by combining with network name; this is awkward and breaking in general. 

One plausible remedy is making stringent containment of network to namespaces - resources in one namespace belongs solely to one network; one network can accommodate multiple namespace. This cannot handle the default/kubernetes service problem, as it is of default namespace.

None of the two of above seems ideal. However, the latter, fiddling with the service name, if coupled with trick applicable only to kubernetes & kube-dns, seems pragmatic and practical. The idea is suffixing service name with network name (e.g. kubernetes_mynetwork), also answering query for kubernetes with that of kubernetes_mynetwork (by alias record or other form of dynamic technique), so that pods are able to look up them without using the network-suffixed names. This approach is not flexible and depending on dns ability; it works fine with limited number of well-known aliases, though. coreDNS, one of the most commonly used dns server binaries, has such dynamic aliasing ability.

One of the confusions is ```kubectl get service kubenetes``` would fail with no such resource error - this is breaking, but for good reason - to remind users of service resource being network specific in our system. However, pods are able to query DNS for kubernetes without caring about the new name schema as they are network related and able to locate the proper DNS service in the interesting network scope.

#### Service IP

Service IP address is allocated from the network-specific pool. 

These are two types of service IP pool:

##### Arktos managed pool
Service IP addresses are explicitly managed by Arktos for every network. Each network might have different range of service IPs.

Service IPAM is provided through 2 changes:

1. each network object specifies its service IP range;

    ```yaml
    apiVersion: v1
    kind: Network
    metadata:
      name: default
    spec:
      type: flat
      service:
      - cidr: 10.0.0.0/16
      - cidr: 192.168.0.0/24
    ```
   
1. on service creation, the current service IP address assignment mechanism is used - in scope of individual network instead of the whole cluster, though.

##### Network provider managed pool
Service IP addresses are implicitly managed by the capable network provider. Network object can have the service IP cidr (or not); at best for information - Arktos delegates IPAM to the external network provider, via network/service controller (detailed design TBD) interacting with the external network provider.

(**TBD: Can we support network provider to mutate service IP after it has been assigned? If so, how does the mutation propagate to Arktos?**)

Network object specifies service IPAM as *external*:

    apiVersion: v1
    kind: Network
    metadata:
      name: default
    spec:
      type: flat
      service:
        ipam: external

#### Semantic support

For flat typed networks, regular kube-proxy may be used to provide pod access to service.

For VPC-isolated networking, service IP support has to be provided w/o relying on iptable/IPVS rules on host netns. In other words, traditional kube-proxy can not used in VPC-isolated networking. A dedicated controller may be employed to establish & maintain service IP/pod IP mappings.
(**TBD: more details required**)

### Endpoints
Each endpoints object associates with a network, the same as its service.

For the services of _kubernetes_ and _kube-dns_ (in fact they are kubernetes-{network} and kube-dns-{network}), there are two special set of endpoints objects:

* kubernetes related EPs

They are in default namespace, named as kubernetes-{network\, for the kubernetes service of the network.

Query for default-ns scoped kubernetes-{network} shall get back the proper content based on the cluster kubernetes endpoints object. System does not duplicate such endpoints; instead it derives content based on the cluster kubernetes endpoints. This would incurs quite some code change to kube-apiserver.

Updates originated from regular tenants are disallowed.

Alternative is to duplicate in every network, when network is being provisioned. It is burdensome to keep all synced to the root one (which is maintained by api-server). 

* kube-dns related EPs

They are in kube-system namespace, named as ube-dns-{network\}, for the kube-dns service of the network.

kube-dns-{network} shall be managed by Endpoints controller just like a regular endpoints.

__EndpointSlices__, the new type introduced in k8s v1.17, is out of current scope.



### Ingress/egress/network policies
(**TBD: more details required**)

## Architectural Views

Components not decided yet at current phase are not included in these views.

### Data model

![Data model view](images/network-data-model.png)

### Module Decomposition

![Module Decomposition view](images/network-module-decomposition.png)

### Runtime component relationship

![C&C view](images/network-c-and-c.png)

### Key Scenario: tenant & DNS provisioning

![tenant & DNS provision](images/network-scenario-tenant-init.png)
