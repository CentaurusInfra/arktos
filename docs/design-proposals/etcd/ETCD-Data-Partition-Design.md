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
multiple ETCD clusters work together will have revision number collision and
tracking issue. 

This document will address how to solving the etcd revision number issue in
Arktos.   

### Goals

* Primary: Define a mechanism that the data from multiple ETCD clusters can be
put together with relatively sortable revision numbers.
* Primary: To prevent from mishandling, there shall be no collision of revision 
numbers.
* Primary: Support existing senarios such as get, write, list, list from revision
number, watch from revision number, etc.
* Secondary: Allow object to be located directly from its key.
* Secondary: Efficiency of API server and clients and complexity of code changes.

### Non-Goals

* Sending events to a separated ETCD cluster is not covered in this design doc
* Scaling out of API server/controller framework is not discussed in this doc
* Reallocate objects from one ETCD cluster to another is not discussed in this doc 

## Proposal

### ETCD Revision Generation

* We updated the existing ETCD revision number generation algorithm from "starting from
1 and always increases by 1" to include the factors of timestamp, cluster id, and write event.
 - Event number: bit 0-12
 - Cluster id: bit 13-18
 - Timestamp in millisecond: bit 19-62
It will allow maximum 8K events per millisecond in a single cluster, up to 64 clusters, till 
approximately January 1st, 2500.

## Risks and Mitigations

1. Wether 8k events per millisecond is enough for a single ETCD cluster? We will compare new
revision # generated from current timestamp and old revision #, if old revision + 1 is already
larger than new revision #, the new revision number will be discarded. Warning will be logged.
1. Whether 64 ETCD cluster is enough? This is current design limit. We choose to stick to it in
initial implementation.

## Graduation Criteria

## Implementation History
- 2020-05-18 - initial design document

 
