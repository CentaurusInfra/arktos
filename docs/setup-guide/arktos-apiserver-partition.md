
# Setting up Partitioned Arktos Environment

[![Go Report Card](https://goreportcard.com/badge/github.com/futurewei-cloud/arktos)](https://goreportcard.com/report/github.com/futurewei-cloud/arktos)
[![LICENSE](https://img.shields.io/badge/license-apache%202.0-green)](https://github.com/futurewei-cloud/arktos/blob/master/LICENSE)

## What this document is about

This document serves as a manual to set up data partitioned Arktos cluster across multiple machines. It has the following features:

1. Multiple ETCD clusters can be configured to serve in a single Arktos cluster. One system cluster, others are tenant data clusters.

1. Multiple API servers can be configured to serve in a single Arktos clusters. ETCD Data is partitioned across API servers.

1. Multiple controller instances (currently we suppport replicaset controller and deployment controller) can be running in parralled in different processes (named as workload controller manager). Each of them handles a portion of the requests to controller 

## How to set up partitioned Arktos environment

### Prerequsite

Currently we require setting up developer environment. See [dev environment setup](setup-dev-env.md)

### Plan

. Get hostname and ip for all hosts where you plan to launch additional api server. For example host-2 that has ip address 100.1.1.2
. Set environment variable on host-1

```
export APISERVERS_EXTRA="host-2:100.1.1.2"
```

### Start ETCD system cluster and 1st Api Server, plus Kube CM, Workload Controller Manager, Kube Scheduler on host-1

```
cd $GOPATH/src/arktos
./hack/arktos-up.sh
```
Note: Updated to automatically pick up new api servers on 4/24/2020.

### Start another apiserver on host-2

. Verify we can access etcd from the worker host
```
etcdctl --endpoints=http://<1st master server ip>:2379 get "" --prefix=true --keys-only
```

. Copy all files from 1st API Server /var/run/kubernetes to host where you planed to run api server

. Start Api server (in a different host)
```
# configure api server with data partition
export APISERVER_SERVICEGROUPID=2
export REUSE_CERTS=true
cd $GOPATH/src/arktos
./hack/arktos-apiserver-partition.sh start_apiserver <1st Api Server ip>
```

### Start ETCD tenant data cluster on host-3

. On host-3, install ETCD 3.4.4-arktos.1 from Artkos or from ETCD repo
```
# Install ETCD 3.4.4-arktos.1 from Artkos
cd $GOPATH/src/arktos
./hack/install-etcd.sh

# Install ETCD 3.4.4-arktos.1 from ETCD repo
# https://github.com/futurewei-cloud/etcd/releases/tag/v3.4.4-arktos.1
```

. Start ETCD tenant cluster manually
```
# Find start arguments from ETCD system cluster process. Change cluster-id to 1-63 (system cluster id is 0) 
# Example below:
# etcd --name ip-172-30-0-122 --cluster-id 1 --listen-peer-urls http://172.30.0.122:2380 --advertise-client-urls http://172.30.0.122:2379 --data-dir /tmp/tmp.2b2q8vW8N1.122 --listen-client-urls http://172.30.0.122:2379,http://127.0.0.1:2379 --initial-advertise-peer-urls http://172.30.0.122:2380 --initial-cluster-token etcd-cluster-1 --initial-cluster ip-172-30-0-122=http://172.30.0.122:2380 --initial-cluster-state new --debug 2> /tmp/etcd.log > /dev/null &
```

. Configure ETCD tenant cluster in existing Artkos cluster. Apply tenant cluster yml. Example as following:

```yml
apiVersion: v1
kind: StorageCluster 
storageClusterId: "1"
serviceAddress: "172.30.0.122:2379"
metadata:
  name: "c1"
```

Note: If tenant data cluster is not started, corresponding tenant data will be written into ETCD system cluster.

### Aggregated watch with kubectl
. Create KubeConfig for all api servers that needs to be connected together
```
$ cd $GOPATH/src/arktos
$ ./hack/create-kubeconfig.sh command /home/ubuntu/kubeconfig/kubeconfig-<apiserver#>.config
# Copy generated print out from the above command and run it in shell
```
. Copy both kubeconfig files into the hosts where you want to use kubectl to access api servers
. Watch all api servers listed in kubeconfig
```
kubectl get deployments --kubeconfig "<kubeconfigfilepath1 kubeconfigfilepath2 ... kubeconfigfilepathn>" --all-namespaces --all-tenants -w
```
Note: Kubectl currently does not support automatically detect api servers during watch as it is a rare case.

## Testing Senario

Step 1: Set data partition for api servers
```
kubectl apply -f datapartition.yaml
```
Sample data partition spec:
```
apiVersion: v1
kind: DataPartitionConfig
serviceGroupId: "1"
rangeStart: "A"
isRangeStartValid: false 
rangeEnd: "m"
isRangeEndValid: true
metadata:
  name: "partition-1"
---
apiVersion: v1
kind: DataPartitionConfig
serviceGroupId: "2"
rangeStart: "m"
isRangeStartValid: true 
rangeEnd: "z"
isRangeEndValid: false 
metadata:
  name: "partition-2"
```
Note: No matter what partitions have been set, we always load default tenant
```
kubectl get tenants -T default # this does not work
```

Step 2. Create tenants

Sample tenant spec 
```
apiVersion: v1
kind: Tenant
metadata:
  name: a
spec:
  storageClusterId: "1"
```
If separate ETCD cluster is used for tenant, the following command can be used to check ETCD data:
```
etcdctl --endpoints=http://<etcd host ip>:2379 get "" --prefix=true --keys-only
```

Step 3. Create namespaces under tenants

Sample namespace spec 
```
apiVersion: v1
kind: Namespace
metadata:
  name: namespace-a
  tenant: a 
```

Step 4. Create deployments under tenants

Sample deployment spec 
```
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: namespace-a
  tenant: a 
  labels:
    app: nginx
spec:
  replicas: 1 
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
```

Step 5. Watch deployments updates using kubectl 
```
kubectl get deployments --kubeconfig "<kubeconfigfilepath1 kubeconfigfilepath2 ... kubeconfigfilepathn>" --all-namespaces --all-tenants -w
```

