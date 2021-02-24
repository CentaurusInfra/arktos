# Setting up local dev environment for scale out

## Scenarios

1. Two Tenant Partitions

1. Single Resource Partition (2/19/2021)

1. HA proxy

## Prerequsite

1. 3 dev box (tested on ubuntu 16.04), 1 for RP, 2 for TPs. Record ip as TP1_IP, TP2_IP, RP_IP

1. One dev box for HA proxy, can share with dev boxes used for TP or RP. Record ip as PROXY_IP

## Steps

### Setting up HA proxy
1. Install HA proxy 2.3.0

1. Set up environment variables

```
export TENANT_PARTITION_IP=[TP1_IP],[TP2_IP]`
export RESOURCE_PARTITION_IP=[RP_IP]
```

1. Run ./hack/scalability/setup_haproxy.sh (depends on your HA proxy version and environment setup, you might need to comment out some code in the script)

### Setting up TPs
1. Make sure hack/arktos-up.sh can be run at the box

1. Set up environment variables

```
export SCALE_OUT_PROXY_IP=[PROXY_IP]
export SCALE_OUT_PROXY_PORT=8888
export IS_RESOURCE_PARTITION=false
export RESOURCE_SERVER=[RP_IP]
export REUSE_CERTS=true
```

1. Run ./hack/arktos-up-scale-out-poc.sh

1. Expected last line of output: "Tenant Partition Cluster is Running ..."

Note:

1. As we start to picking up secure mode in scale out, api server certificates will be shared across all partitions in 
development environment. The first TP that started needs to generate api server certificates and be copied over to other
TP/RP before they start.

1. To generate first set of certs, run `REUSE_CERTS=false; ./hack/arktos-up-scale-out-poc.sh`

1. After the first TP started, copy all files in /var/run/kubernetes to other TP/RP hosts. To avoid recopy the certificate
files, don't use `REUSE_CERTS=false`


### Setting up RP
1. Make sure hack/arktos-up.sh can be run at the box

1. Set up environment variables

```
export SCALE_OUT_PROXY_IP=[PROXY_IP]
export SCALE_OUT_PROXY_PORT=8888
export IS_RESOURCE_PARTITION=true
export TENANT_SERVERS=http://[TP1_IP]:8080,http://[TP2_IP]:8080
export REUSE_CERTS=true
```

1. Run ./hack/arktos-up-scale-out-poc.sh

1. Expected last line of output: "Resource Partition Cluster is Running ..."

### Test Cluster

1. Use kubectl with kubeconfig. For example:

```
kubectl --kubeconfig /var/run/kubernetes/scheduler.kubeconfig get nodes
```

1. Create pod for system tenant. For example:
```
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
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

1. Check pod is running

```
kubectl --kubeconfig /var/run/kubernetes/scheduler.kubeconfig get pods
```

1. Get ETCD pods in each TP
```
etcdctl get "" --prefix=true --keys-only | grep pods
```

### Note
1. Current change break arktos-up.sh. To verify it works on the host, please use arktos-up.sh on master branch

1. If there is no code changes, can use "./hack/arktos-up-scale-out-poc.sh -O" to save compile time

1. Currently tested with 2TP/1RP. Pods can be scheduled for both TPs.

