---
title: API Server Data Partition Design for Arktos
authors:
  - "@sindica"
---

# API Server Data Partition Design for Arktos

## Table of Contents

## Motivation

Community version of Kubernetes can take up to 5K nodes per cluster, each 
node has 30 pods without significant performance issue. One of the goals 
of Arktos is to greatly increase the number of nodes that can be 
managed by a single cluster.

With current community design, each api server gets all data from 
backend storage (ETCD for now), api server becomes one of the bottlenecks
 in Arktos scaling up.

This document will address how to partition data across api server instances.

### Goals

* Primary: Define a mechanism that data from backend storage can be effectively
partitioned across multiple api server instances
* Primary: Allows clients to get all data continuously without service interruption
* Primary: Allows users to manage data partitions without service interruption
* Secondary: Single partition cluster performance shall aligned with community version
* Secondary: System will automatically adjust data partition based on data volume and 
traffic pattern

### Non-Goals

* Data partition of backend storage is not covered in this design doc
* Scaling out of controller framework is not discussed in this doc
* Automatically adjust data partition configuration based on data and traffic pattern
will not be addressed in this design doc 

## Proposal

### API Changes

* We added a top level Kubernetes object named '**datapartitionconfigs**' with path
'**/api/v1/datapartitionconfigs**'.
 - This allows each api server to get the data partition setting assigned to it
 - This allows admin to view and adjust data partition across api servers manually
 - This allows managing component to list and adjust data partition across api servers
based on data volume and traffic automatically

```yaml
apiVersion: v1
kind: DataPartitionConfig
serviceGroupId: "1"
rangeStart: "A"
isRangeStartValid: false 
rangeEnd: "m"
isRangeEndValid: true
metadata:
  name: "partition-1"
``` 

```text
$ curl http://127.0.0.1:8080/api/v1/datapartitionconfigs
{
  "kind": "DataPartitionConfigList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/datapartitionconfigs",
    "resourceVersion": "487"
  },
  "items": [
    {
      "metadata": {
        "name": "partition-1",
        "selfLink": "/api/v1/datapartitionconfigs/partition-1",
        "uid": "c8cd663c-5d0f-44e1-aeef-80aa0ebeb56a",
        "hashKey": 214864085683041732,
        "resourceVersion": "397",
        "creationTimestamp": "2020-04-06T22:17:13Z"
      },
      "rangeStart": "A",
      "rangeEnd": "m",
      "isRangeEndValid": true,
      "serviceGroupId": "1"
    }
  ]
}
```

* We introduced a field '**ServiceGroupId**' into '**EndpointSubset**'.
  - This allows each api server started with a service group id find the data partition
setting associated with the service group
  - This allows managing component to list and adjust number of api servers in each group
based on traffic pattern

```text
$ kubectl describe endpoints kubernetes
Name:         kubernetes
Namespace:    default
Tenant:       default
Labels:       <none>
Annotations:  <none>
Subsets:
  Service Group Id:   1
  Addresses:          172.30.0.148
  NotReadyAddresses:  <none>
  Ports:
    Name   Port  Protocol
    ----   ----  --------
    https  6443  TCP

Events:  <none>
```

### API Structs

Using data partition identified by service group, Arktos cluster can have multiple api 
server instances in the same service group, achieving the purpose of HA. It also allows 
dynamic data partitioning mechanism.

Here are the DataPartitionConfig data struct introduced into Arktos:
```text
type DataPartitionConfig struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Range start is inclusive
	RangeStart string

	// Whether this is an open end start
	IsRangeStartValid bool

	// Range end is exclusive
	RangeEnd string

	// Whether this is an open end end
	IsRangeEndValid bool

	// Which service group is using this data configuration
	ServiceGroupId string
}
```

Adjustment in EndpointSubset struct:
```text
type EndpointSubset struct {
	Addresses         []EndpointAddress
	NotReadyAddresses []EndpointAddress
	Ports             []EndpointPort

	// The service group id of the api server cluster
	// +optional
	ServiceGroupId string
}
```

### Affected Components

APISerer and registry:
* Created DataPartitionConfig struct, also created informer
* Added field ServiceGroupId into EndpointSubset struct; organized api server into service 
group via EndpointSubset struct
* Added filter by range into api server to restrict the data watching from backend storage 
* Reset watch channel to backend storage when current service group data partition is updated

Scheduler/Controller/Kubelet/KubeProxy/:
* Allows connecting to multiple api servers simultaneously
* Supported aggregated watching in informers so that data partition configuration in api server
is transparent to clients
* Support automatically connecting/disconnecting from api server when service group is created/updated/deleted (TODO) 

Kubectl:
* Allows connecting to multiple api servers simultaneously
* Support aggregated watching

Other components:

## Risks and Mitigations

1. Api server clustering mechanism is redefined. We will need to educate framework admin for
the new definition of clustering configuration in kubeconfig.

## Graduation Criteria

1. Performance test
1. Automatic environment setup

## Implementation History

- 2020-1-22 - initial design document
- 2020-2-12 - api server watch etcd data by range (jshao) 
- 2020-2-24 - support aggregated watch in Scheduler/Controller/Kubelet/KubeProxy
- 2020-3-19 - reset api server watch channel based on data partition updates
- 2020-4-03 - support aggregated watch in kubectl
- 2020-4-03 - script and manual ready for test environment set up (jshao)

