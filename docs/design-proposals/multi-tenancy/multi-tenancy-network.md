# Multi-Tenancy Network

Xiaoning Ding, Hong Chen

## Introduction

This document is a part of the multi-tenancy design documents in Arktos. It describes network-related design changes to support multi-tenancy, including network model, backward-compatibility, DNS and service changes, etc.

Please refer to other design documents in the "multi-tenancy" document directory if you want to learn about other parts in the multi-tenancy design.

## Network Isolation

Network isolation is achieved with a new API object: network:

* Every pod is associated with a certain network object. If not specified it will be associated to a default network object, which is created when a space is initialized. 
* A pod can only communicate with pods within the same network.
* Every network has its own DNS service. An associated DNS service is automatically created when a new network object is created. (**tbd: decide when to deploy the actual DNS server pods**)


### Network Object

A network object contains its type, and type-specific configurations. It's a not namespace-scoped object, so that one network object can be shared by applications in multiple namespaces in a same space.

The "type" field is the only mandatory field in a network object. For now there are two types defined: flat and vpc.

A command-line parameter named "default-network-template-path" of tenant controller will decide which default network will be created for a new space.

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
  vpc-id: vpc-1a2b3c4d
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


### Network Controller

A network controller watches its interesting network objects based on network type.

A flat network controller only initializes the DNS service and deployment for a new network, and also deletes them when a network object is removed. It doesn't do anything else.

A VPC network controller will call VPC service provider to allocate ports for newly created pods and deallocate ports when pods are deleted, in addition to the DNS service initialization and removal that's done in flat network controller. 

### DNS

Each network has its own DNS service, which only contains the DNS entries of pods in that that network, regardless of network type. 

The reasons that DNS service is per-network instead of per-space are:

* For VPC networks IPs could be overlapped. A pod with resolved IP 192.168.0.1 could be in different networks. Therefore it's useless for the DNS clients.
* VPC networks are isolated by nature. It will be difficult to have a shared DNS pod that's accessible to pods in various VPC networks.

### Service
For VPC-isolated networking, service IP support has to be provided w/o relying on iptable/IPVS rules on host netns. In other words, traditional kube-proxy can not used in VPC-isolated networking.

(**TBD: more details required**)

### Ingress/egress/network policies
(**TBD: more details required**)

