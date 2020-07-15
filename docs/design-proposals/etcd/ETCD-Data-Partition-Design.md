---
title: ETCD Data Partition Design for Arktos
authors:
  - "@sindica"
---

# ETCD Data Partition Design for Arktos

## Table of Contents

## Motivation

Community version of Kubernetes use a single ETCD cluster to store objects. 
One of the goals of Arktos is to greatly increase the size of storage by 
serving single Arktos cluster from multiple ETCD clusters.

With current community design, each ETCD cluster has its own revision number,
starting from 1 and increases by 1 for each sequential write. Simply putting
multiple ETCD clusters together will have revision number collision and
tracking issue. 

This document will address how to solving the etcd revision number issue in
Arktos. In addition, the strategy of classifying objects into different ETCD
clusters will also be discussed.

### Goals

* Primary: Define a mechanism that the data from multiple ETCD clusters can be
put together with relatively sortable revision numbers.
* Primary: To prevent from mishandling, there shall be no collision of revision 
numbers.
* Primary: Support existing senarios such as get, write, list, list from revision
number, watch from revision number, etc.
* Primary: Automate new ETCD cluster discovery 
* Secondary: Allow object to be located directly from its key.
* Secondary: Efficiency of API server and clients and complexity of code changes.

### Non-Goals

* Sending events to a separated ETCD cluster is not covered in this design doc
* Scaling out of API server/controller framework is not discussed in this doc
* Reallocate objects from one ETCD cluster to another is not discussed in this doc
* Automatically rebalance ETCD data across multiple clusters is not discussed in this doc 

## Proposal

### ETCD Revision Generation

* We updated the existing ETCD revision number generation algorithm from "starting from
1 and always increases by 1" to include the factors of timestamp, cluster id, and write event.
 - Event number: bit 0-12
 - Cluster id: bit 13-18
 - Timestamp in millisecond: bit 19-62
It will allow maximum 8K events per millisecond in a single cluster, up to 64 clusters, till 
approximately January 1st, 2500.

### ETCD Objects Partition

* There will be a default ETCD cluster called system partition. It has all the data that is not
created/owned by [tenants](../multi-tenancy/multi-tenancy-overview.md). Hence, it will have system
objects such as nodes, as well as tenant objects as they are created and managed by system admin.
* There will have multiple ETCD clusters called tenant partitions. They have all the data that is
created/owned by tenants, including namespaces for a tenant, pods created in one of the namespaces
of the tenants, etc. An ETCD tenant cluster can have objects from multiple tenants. Objects for a single
tenant can only located in one tenant cluster.

#### API Changes

* We added a top level Kubernetes object named '**StorageCluster**' with path '**/api/v1/storageclusters**'.
In addition, we added a field named '**StorageClusterId**' into '**[Tenant](../multi-tenancy/multi-tenancy-api-resource-model.md)**' object.
  - This allows admin to view and allocate tenants to etcd clusters manually
  - This allows api server to discover storage clusters and decide where to create/locate objects

```yaml
apiVersion: v1
kind: StorageCluster 
storageClusterId: "1"
serviceAddress: "172.30.0.122:2379"
metadata:
  name: "cluster-1"
```

```yaml
apiVersion: v1
kind: Tenant
spec:
  storageClusterId: "1"
metadata:
  name: aa 
```

```text
$ curl http://127.0.0.1:8080/api/v1/storageclusters
{
  "kind": "StorageClusterList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/storageclusters",
    "resourceVersion": "491"
  },
  "items": [
    {
      "metadata": {
        "name": "cluster-1",
        "selfLink": "/api/v1/storageclusters/cluster-1",
        "uid": "984734c2-0051-44fd-a773-7fb0f039dd39",
        "hashKey": 4581467420285698949,
        "resourceVersion": "426",
        "creationTimestamp": "2020-05-26T21:16:03Z"
      },
      "storageClusterId": "1",
      "serviceAddress": "172.30.0.122:2379"
    }
  ]
}
```

```text
$ curl http://127.0.0.1:8080/api/v1/tenants/aa
{
  "kind": "Tenant",
  "apiVersion": "v1",
  "metadata": {
    "name": "aa",
    "selfLink": "/api/v1/tenants/aa",
    "uid": "150adc36-885f-46c3-9db4-c6f9876fcf01",
    "hashKey": 2305167293877862255,
    "resourceVersion": "833715489820647426",
    "creationTimestamp": "2020-05-22T22:21:41Z"
  },
  "spec": {
    "finalizers": [
      "kubernetes"
    ],
    "storageClusterId": "1"
  },
  "status": {
    "phase": "Active"
  }
}
```

#### API Structs

Using storage cluster, Api servers can automatically connect to ETCD cluster(s)
that hosting specific tenant data.
  
Here are the StorageCluster data struct introduced into Arktos:

```text
type StorageCluster struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// A string that specifies the storage object identity
	StorageClusterId string

	// A string that specifies the backend storage server address
	ServiceAddress string
}
``` 

Adjustment in TenantSpec struct:

```text
type TenantSpec struct {
	Finalizers []FinalizerName

    // StorageClusterId specifies the storage location of objects belong to this tenant
	StorageClusterId string
}
```

#### Affected Components

APIServer and registry:
* Created StorageCluster struct, also created informer
* Added field StorageClusterId into TenantSpec struct
* Added automated watch new ETCD cluster from api servers 

Controller:
* Added check StorageClusterId when tenant was created/updated into Tenant controller 

Kubectl:
* Print out StorageClusterId for tenant spec

## Risks and Mitigations

1. Wether 8k events per millisecond is enough for a single ETCD cluster? We will compare new
revision # generated from current timestamp and old revision #, if old revision + 1 is already
larger than new revision #, the new revision number will be discarded. Warning will be logged.
1. Whether 64 ETCD clusters are enough? This is current design limit. We choose to stick to it in
initial implementation.

## Graduation Criteria

1. Performance test
1. Automatic environment setup

## Revision History
- 2020-05-18 - initial design document - update ETCD revision number generation algorithm
- 2020-05-21 - ETCD objects partition design



 
