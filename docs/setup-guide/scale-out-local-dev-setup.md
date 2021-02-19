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
export SCALE_OUT_PROXY_ENDPOINT=http://[PROXY_IP]:8888
export IS_RESOURCE_PARTITION=false
```

1. Run ./hack/arktos-up-scale-out-poc.sh

1. Expected last line of output: "Tenant Partition Cluster is Running ..."

### Setting up RP
1. Make sure hack/arktos-up.sh can be run at the box

1. Set up environment variables

```
export SCALE_OUT_PROXY_ENDPOINT=http://[PROXY_IP]:8888
export IS_RESOURCE_PARTITION=true
export TENANT_SERVERS=http://[TP1_IP]:8080,http://[TP2_IP]:8080
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

