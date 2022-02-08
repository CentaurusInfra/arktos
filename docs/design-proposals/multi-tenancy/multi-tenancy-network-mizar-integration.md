---
title: Arktos Multi-Tenancy Network Based on Mizar
authors:
- "@sindica"
---

# Arktos Multi-Tenancy Network Based on Mizar

## Table of Content
1. [Introduction](#intro)
2. [Goals](#goal)
3. [Proposal](#proposal)<br>
   3.1. [Related Mizar Specs](#mizar-spec)<br>
   3.2. [Related Arktos Specs](#arktos-spec)<br>
   3.3. [Arktos Pod Creation Workflow](#pod-creation-wf)<br>
   &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;3.3.1 [Workflow for default VPC and subnet in tenant](#pod-creation-default-wf)<br>
   &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;3.3.2 [Workflow for secondary VPC and subnet in tenant](#pod-creation-second-vpc-wf)<br>
   &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;3.3.3 [Workflow for secondary subnet in VPC](#pod-creation-second-subnet-wf)<br>

## 1. Introduction <a name="intro"></a>

This document is a part of the multi-tenancy network design document that focuses on how to integrated [Mizar network solution](https://github.com/CentaurusInfra/mizar) into Arktos.

The generic network design is in [Multi-Tenancy Network](multi-tenancy-network.md).

## 2. Goals <a name="goal"></a>

* Primary: support multi-tenancy communication based on existing Mizar network implementation. Multi-tenancy communication 
includes connectivity amongs pods in the same tenant, as well as network isolation across different tenants.
* Primary: upon tenant creation, automatically create Mizar VPC and subnet for future communication within this tenant.
* Primary: upon tenant pod creation, automatically attach pod to Mizar VPC/subnet created for this tenant.
* Primary: Mizar and Arktos decouple, i.e. Mizar manage VPCs and subnets in VPC only, it should not be aware of the ownership
of different pods and other objects.
* Secondary: support multiple subnets in same VPC for a single tenant. (Post 2022-01-30)
* Secondary: support multiple VPC/subnets in same tenant. (Post 2022-01-30)

## 3. Proposal <a name="proposal"></a>

Inject Mizar adapter into Arktos, completely isolated from Mizar management plane and relatively isolated from Arktos network
management components.

### 3.1. Related Mizar Specs <a name="mizar-spec"></a>

* VPC Spec
```
apiVersion: "mizar.com/v1"
kind: Vpc
metadata:
  name: <vpc name>
spec:
  ip: "<ip address>"
  prefix: "<ip address prefix length>"
  dividers: <number of dividers in vpc>
  status: "<vpc provisioning status>"
```

* Subnet Spec
```
apiVersion: mizar.com/v1
kind: Subnet
metadata:
  name: <subnet name>
spec:
  ip: "<ip address in vpc>"
  prefix: "<ip address prefix length>"
  bouncers: <number of bouncers in subnet>
  vpc: "<vpc name>"
  status: "<subnet provisioning status>"
```

* Pod Spec
```
apiVersion: v1
kind: Pod
metadata:
  name: <pod name>
  annotations:
    mizar.com/vpc: "<vpc name>"
    mizar.com/subnet: "<subnet name>"
```

### 3.2 Related Arktos Specs <a name="arktos-spec"></a>
* Network Spec
```
apiVersion: arktos.futurewei.com/v1
kind: Network
metadata:
  name: <VPC name defined in arktos>
  tenant: <tenant name>
spec:
  type: vpc
  vpcID: <VPC name in Mizar>
```

* Pod Spec
```
piVersion: v1
kind: Pod
metadata:
  name: <pod name> 
  tenant: <tenant name>
  labels:
    arktos.futurewei.com/network: <VPC name defined in arktos> 
```

### 3.3 Arktos Pod Creation Workflow <a name="pod-creation-wf"></a>
#### 3.3.1 Workflow for default VPC and subnet in tenant <a name="pod-creation-default-wf"></a>
1. Client creates Tenant
2. Tenant controller creates default network object for tenant
3. Mizar-arktos-network-controller (existing controller) **creates Mizar VPC and subnet, updates corresponding arktos network object (TODO)**
   * Target VPC Spec
```
# Sample VPC Spec

apiVersion: "mizar.com/v1"
kind: Vpc
metadata:
  name: vpc-tenant-a
spec:
  ip: "10.0.0.0"
  prefix: "8"
  dividers: 5
  status: "Init"

# Proposed VPC template
{
   "apiVersion": "mizar.com/v1",
   "kind": "Vpc",
   "metadata": {
      "name": "{$vpc_name}"
   },
   "spec": {
      "ip": "{$ip_address}",
      "prefix": "{$ip_prefix_length}",
      "dividers": {$num_of_dividers},
      "status": "Init"
   }
}
```

   * Target Subnet Spec
```
# Sample Subnet Spec
apiVersion: mizar.com/v1
kind: Subnet
metadata:
  name: subnet-tenant-a
spec:
  ip: "10.0.0.0"
  prefix: "16"
  bouncers: 3
  vpc: "vpc-tenant-a"
  status: "Init"
  
# Proposed Subnet template
{
   "apiVersion": "mizar.com/v1",
   "kind": "Subnet",
   "metadata": {
      "name": "{$subnet_name}"
   },
   "spec": {
      "ip": "{$ip_address}",
      "prefix": "$ip_prefix_length",
      "bouncers": {$num_of_bouncers},
      "vpc": "{$vpc_name}",
      status: "Init"
   }
}
```


4. Client creates pod for tenant
5. Mizar-pod-controller (existing controller) **gets VPC and subnet from arktos network object and annotate pod (TODO)**

Note:
1. In 2022-01-30 release cycle, only one VPC and one subnet will be supported in Mizar-Arktos integration. Hence only 
default VPC and default subnet will be supported.
2. Highlighted text described what needs to be done in 2022-01-30 release cycle. Others are existing logic.
3. It is better simplify pod spec and not add multiple labels/annotations into pod spec for same network configuration. However, in order to 
minimize changes in Mizar, Arktos will check whether it is possible to not annotate pod with Mizar VPC/Subnet. If it is not possible, Arktos
will annotate pod with Mizar VPC/Subnet in relase cycle 2022-01-30.

#### 3.3.2 Workflow for secondary VPC and subnet in tenant <a name="pod-creation-second-vpc-wf"></a>
1. **Client creates secondary network object for tenant**
2. Mizar-arktos-network-controller creates Mizar VPC and subnet, update corresponding arktos network object
3. Client creates pod for tenant, **specifying which network object it will be referring to**
4. Mizar-pod-controller gets VPC and subnet from arktos network object and annotate pod

**Note**: This is not planned in 2022-01-30 release cycle.

#### 3.3.3 Workflow for secondary subnet in VPC <a name="pod-creation-second-subnet-wf"></a>
1. **Client creates secondary network object with existing VPC and different subnet**
2. Mizar-arktos-network-controller **creates Mizar subnet**
3. Client creates pod for tenant, specifying which network object it will be referring to
4. Mizar-pod-controller gets VPC and subnet from arktos network object and annotate pod

**Note**: This is a non-goal in 2022-01-30 release cycle as subnets in same VPC do not isolate from others and behave the same as 
in a single subnet.


