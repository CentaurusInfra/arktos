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
   3.3. [Arktos VPC/Subnet Creation Workflow](#vpc-creation-wf)<br>
   &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;3.3.1 [Workflow for default VPC and subnet in tenant](#vpc-creation-default-wf)<br>
   &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;3.3.2 [Workflow for secondary VPC and subnet in tenant](#vpc-creation-second-vpc-wf)<br>
   &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;3.3.3 [Workflow for secondary subnet in VPC](#vpc-creation-second-subnet-wf)<br>

## 1. Introduction <a name="intro"></a>

This document is a part of the multi-tenancy network design document that focuses on how to integrated [Mizar network solution](https://github.com/CentaurusInfra/mizar) into Arktos.

The generic network design is in [Multi-Tenancy Network](multi-tenancy-network.md).

## 2. Goals <a name="goal"></a>

* Primary: support multi-tenancy communication based on existing Mizar network implementation. Multi-tenancy communication 
includes connectivity amongs pods in the same tenant, as well as network isolation across different tenants.
* Primary: upon tenant creation, automatically create Mizar VPC and subnet for future communication within this tenant.
* Primary: upon tenant pod creation, automatically attach pod to Mizar VPC/subnet created for this tenant.
* Primary: upon tenant service creation, automatically attach service to Mizar VPC/subnet created for this tenant.
* Secondary: support multiple subnets in same VPC. (Post 2022-01-30)
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

* Service Spec
```
apiVersion: v1
kind: Service
metadata:
  name: <service name>
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
apiVersion: v1
kind: Pod
metadata:
  name: <pod name> 
  tenant: <tenant name>
  labels:
    arktos.futurewei.com/network: <VPC name defined in arktos> 
```

* Service Spec
```
apiVersion: v1
kind: Service
metadata:
  name: <service name>
  tenant: <tenant name>
spec:
  clusterIP: <service cluster ip>
```

### 3.3 Arktos VPC/Subnet Creation Workflow <a name="vpc-creation-wf"></a>
#### 3.3.1 Workflow for default VPC and subnet in tenant <a name="vpc-creation-default-wf"></a>
1. Client creates Tenant
2. Tenant controller creates default network object for tenant
3. Mizar-arktos-network-controller (existing controller) creates Mizar VPC and subnet, updates corresponding arktos network object
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
```


4. Client creates pod for tenant
5. Mizar-pod-controller gets VPC and subnet from arktos network object and annotate pod
6. Client creates service for tenant
7. Mizar-service-controller gets VPC and subnet from arktos network object and annotate service 

Note:
1. In 2022-01-30 release cycle, only one VPC and one subnet will be supported in Mizar-Arktos integration. Hence only 
default VPC and default subnet will be supported.
2. Ideally, service ip should be within the VPC ip range assigned to the service. However, this logic is not implemented in Mizar and there might need significant changes in arktos to support it. This also caused API server cannot be accessed from non system pods. [Issue 1378]((https://github.com/CentaurusInfra/arktos/issues/1378)) is recorded and will be resolved after 130 release. 

#### 3.3.2 Workflow for secondary VPC and subnet in tenant <a name="vpc-creation-second-vpc-wf"></a>
1. **Client creates secondary network object for tenant**
2. Mizar-arktos-network-controller creates Mizar VPC and subnet, update corresponding arktos network object
3. Client creates pod for tenant, **specifying which network object it will be referring to**
4. Mizar-pod-controller gets VPC and subnet from arktos network object and annotate pod
5. Client creates service for tenant, **specifying which network object it will be referring to**
6. Mizar-service-controller gets VPC and subnet from arktos network object and annotate service

**Note**: This is not planned in 2022-01-30 release cycle.

#### 3.3.3 Workflow for secondary subnet in VPC <a name="vpc-creation-second-subnet-wf"></a>
1. **Client creates secondary network object with existing VPC and different subnet**
2. Mizar-arktos-network-controller **creates Mizar subnet**
3. Client creates pod for tenant, specifying which network object it will be referring to
4. Mizar-pod-controller gets VPC and subnet from arktos network object and annotate pod
5. Client creates service for tenant, **specifying which network object it will be referring to**
6. Mizar-service-controller gets VPC and subnet from arktos network object and annotate service

**Note**: This is a non-goal in 2022-01-30 release cycle as subnets in same VPC do not isolate from others and behave the same as 
in a single subnet.

