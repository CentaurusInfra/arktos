---
title: Arktos Multi-Tenancy Based on Mizar
authors:
- "@sindica"
---

# Arktos Multi-Tenancy Based on Mizar

## Table of Contents

## Introduction

This document is a part of the multi-tenancy network design document that focus on how to integrated Mizar network solution into Arktos.

The generic network design is in [Multi-Tenancy Network](multi-tenancy-network.md)

## Goals

* Primary: Support multi-tenancy communication based on existing Mizar VPC implementation. Multi-tenancy communication 
includes connectivity amongs pods in the same tenant, as well as network isolation across different tenants.
* Primary: Based on generic multi-tenancy network design, modifying existing Mizar controllers in KCM to automatically 
attach tenant pod to VPC created for this tenant.
* Primary: Mizar and Arktos decouple, i.e. Mizar manage VPCs and subnets in VPC only, it should not be aware of the ownership
of different pods.
* Secondary: Support subnet defined in Mizar.


## Proposal

Inject Mizar adapter into Arktos, completely isolated from Mizar management plane and relatively isolated from Arktos network
management components.

### Related Mizar Specs

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

### Related Arktos Specs
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

### Arktos Workflow
#### Workflow for default VPC and subnet in tenant
1. Client creates Tenant
2. Tenant controller creates default network object for tenant
3. Mizar-arktos-network-controller creates Mizar VPC and subnet, updates corresponding arktos network object
4. Client creates pod for tenant
5. Mizar-pod-controller gets VPC and subnet from arktos network object and annotate pod

Note: In 2022-01-30 release cycle, only one VPC and one subnet will be supported in Mizar-Arktos integration. Hence only 
default VPC and default subnet will be supported.

#### Workflow for secondary VPC and subnet in tenant
1. Client creates secondary network object for tenant
2. Mizar-arktos-network-controller creates Mizar VPC and subnet, update corresponding arktos network object
3. Client creates pod for tenant, specifying which network object it will be referring to
4. Mizar-pod-controller gets VPC and subnet from arktos network object and annotate pod

Note: This is a stretch goal for 2022-01-30 release cycle.

#### Workflow for secondary subnet in VPC
1. Client creates secondary network object with existing VPC and different subnet
2. Mizar-arktos-network-controller creates Mizar subnet
3. Client creates pod for tenant, specifying which network object it will be referring to
4. Mizar-pod-controller gets VPC and subnet from arktos network object and annotate pod

Note: This is a non-goal in 2022-01-30 release cycle as subnets in same VPC do not isolate from others and behave the same as 
in a single subnet.


