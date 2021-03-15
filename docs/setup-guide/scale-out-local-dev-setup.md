# Setting up local dev environment for scale out

## Scenarios

1. Two Tenant Partitions

1. Two Resource Partitions

1. HA proxy (not required if not using cloud KCM)

## Prerequsite

1. 4 dev box (tested on ubuntu 16.04), 2 for RP, 2 for TPs. Record ip as TP1_IP, TP2_IP, RP1_IP, RP2_IP

1. One dev box for HA proxy, can share with dev boxes used for TP or RP. Record ip as PROXY_IP

## Steps

### Setting up HA proxy
1. Install HA proxy 2.3.0

1. Set up environment variables (no changes have been made for RP2 nor tested)

```
export TENANT_PARTITION_IP=[TP1_IP],[TP2_IP]
export RESOURCE_PARTITION_IP=[RP1_IP]
```

1. Run ./hack/scalability/setup_haproxy.sh (depends on your HA proxy version and environment setup, you might need to comment out some code in the script)

### Setting up TPs
1. Make sure hack/arktos-up.sh can be run at the box

1. Set up environment variables

```
# optional, used for cloud KCM only but not tested
export SCALE_OUT_PROXY_IP=[PROXY_IP]
export SCALE_OUT_PROXY_PORT=8888

# required
export IS_RESOURCE_PARTITION=false
export RESOURCE_SERVER=[RP1_IP]<,[RP2_IP]>
```

1. Run ./hack/arktos-up-scale-out-poc.sh

1. Expected last line of output: "Tenant Partition Cluster is Running ..."

Note:

1. As certificates generating and sharing is confusing and time consuming in local test environment. We will use insecure mode for local test for now. Secured mode can be added back later when main goal is acchieved.

### Setting up RPs
1. Make sure hack/arktos-up.sh can be run at the box

1. Set up environment variables

```
export IS_RESOURCE_PARTITION=true
export TENANT_SERVER=[TP1_IP]<,[TP2_IP]>
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

1. After switched all kubeconfigs from proxy, system tenant appears in both TPs. This is not ideal. Trying to point KCM kubeconfig to HA proxy. 

1. Currently tested with 2TP/2RP.

1. Haven't made changes to HA proxy 2RP, kubectl get nodes only has nodes from first RP, which is expected.

1. Currently local RP started as node tained to be NoSchedule. Need to manually remove the taint so that pod can be scheduled.
```
kubectl --kubeconfig <kubeconfig points to RP api server> taint nodes <node_name> node.kubernetes.io/not-ready:NoSchedule-
``` 
