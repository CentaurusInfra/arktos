# Setting up local dev environment for scale out

## Table of Content
1. [Scenarios](scale-out-local-dev-setup.md#scenarios)
2. [Prerequsite](scale-out-local-dev-setup.md#prereq)
3. [Steps to set up Arktos scaleout cluster](scale-out-local-dev-setup.md#steps)<br>
    3.1. [Setting up TPs](scale-out-local-dev-setup.md#steps-setup-tps)<br>
    3.2. [Setting up RPs](scale-out-local-dev-setup.md#steps-setup-rps)<br>
    3.3. [Add worker(s)](scale-out-local-dev-setup.md#add-worker)<br>
4. [Use Mizar Network plugin](scale-out-local-dev-setup.md#setup-mizar)
5. [Test Cluster](scale-out-local-dev-setup.md#test-cluster)

## Scenarios <a name="scenarios"></a>

1. Two Tenant Partitions
2. Two Resource Partitions
3. Use CNIPlugin bridge or Mizar 

## Prerequsite <a name="prereq"></a>

1. 4 dev box (Ubuntu 18.04 or 20.04), 2 for RP, 2 for TPs. Record ip as TP1_IP, TP2_IP, RP1_IP, RP2_IP

1. Follow the [instruction](setup-dev-env.md) on to set up developer environments in each dev box 

## Steps to set up Arktos scaleout cluster <a name="steps"></a>

### Setting up TPs <a name="steps-setup-tps"></a>
1. Set up environment variables

```
# required
export IS_RESOURCE_PARTITION=false
export RESOURCE_SERVER=[RP1_IP]<,[RP2_IP]>
export TENANT_PARTITION_SERVICE_SUBNET=[service-ip-cidr]
```

an examplative allocation for 2 TPs could be

| tp1 | tp2 |
| --- | --- |
| 10.0.0.0/16 | 10.1.0.0/16 |

1. Run ./hack/arktos-up-scale-out-poc.sh

1. Expected last line of output: "Tenant Partition Cluster is Running ..."

Note:

1. As certificates generating and sharing is confusing and time consuming in local test environment. We will use insecure mode for local test for now. Secured mode can be added back later when main goal is acchieved.

### Setting up RPs <a name="steps-setup-rps"></a>

1. Set up environment variables

```
export IS_RESOURCE_PARTITION=true
export TENANT_SERVER=[TP1_IP]<,[TP2_IP]>
export RESOURCE_PARTITION_POD_CIDR=[pod-cidr]
```

an examplative allocation of pod cidr for 2 RPs could be

| rp1 | rp2 |
| --- | --- |
| 10.244.0.0/16 | 10.245.0.0/16 |

2. Run ./hack/arktos-up-scale-out-poc.sh

3. Expected last line of output: "Resource Partition Cluster is Running ..."

### Add workers <a name="add-worker"></a>

Workers can be added into existing arktos scale out resource partition.
* Add worker into arktos scale out cluster started with default network solution, bridge:
  1. On worker node, create folder /var/run/kubernetes or start ./hack/arktos-up.sh so it will create the folder automatically.
  2. Copy /var/run/kubernetes/client-ca.crt file from arktos master. Or if you started arktos-up.sh in step 2, it will be created automatically.
  3. Start worker with the following command, it will be automatically registered as into the cluster:

```bash
IS_SCALE_OUT=true API_HOST=<resource partition master ip> API_TENANT_SERVER=<tenant partition ips separated by comma> ./hack/arktos-worker-up.sh
```

## Use Mizar Network plugin <a name="setup-mizar"></a>
The above instruction shows how to set up arktos scaleout cluster with default network solution, bridge, in local dev environment. This section
shows how to start arktos scaleout cluster with [Mizar](https://github.com/CentaurusInfra/mizar), an advanced network solution that supports Arktos 
tenant isolation. Mizar was introduced into Arktos since release 1.0.

There are some additional environment variables that need to be setup in order to start arktos scaleout cluster with Mizar.
1. Set up environment variables in Tenant Partition(s)
```
# required for tenant partition
export IS_RESOURCE_PARTITION=false
export RESOURCE_SERVER=[RP1_IP]<,[RP2_IP]>
export TENANT_PARTITION_SERVICE_SUBNET=[service-ip-cidr]

# Mizar specific
export VPC_RANGE_START=<integration 11 to 99>
export VPC_RANGE_END=<integration 11 to 99>
SCALE_OUT_TP_ENABLE_DAEMONSET=true

# non-primary tenant partion only
IS_SECONDARY_TP=true
```

an example start up command will be
```
# 1st TP:
VPC_RANGE_START=11 VPC_RANGE_END=50 CNIPLUGIN=mizar SCALE_OUT_TP_ENABLE_DAEMONSET=true IS_RESOURCE_PARTITION=false RESOURCE_SERVER=172.30.0.41,172.30.0.60 TENANT_PARTITION_SERVICE_SUBNET=10.0.0.0/16 ./hack/arktos-up-scale-out-poc.sh

# 2nd TP:
VPC_RANGE_START=51 VPC_RANGE_END=99 IS_SECONDARY_TP=true CNIPLUGIN=mizar SCALE_OUT_TP_ENABLE_DAEMONSET=true IS_RESOURCE_PARTITION=false RESOURCE_SERVER=172.30.0.41,172.30.0.60 TENANT_PARTITION_SERVICE_SUBNET=10.0.0.0/16 ./hack/arktos-up-scale-out-poc.sh
```

2. Set up environment variables in Resource Partition(s)
```
# required for resource partition
export IS_RESOURCE_PARTITION=true
export TENANT_SERVER=[TP1_IP]<,[TP2_IP]>
export RESOURCE_PARTITION_POD_CIDR=[pod-cidr]

# Mizar specific
CNIPLUGIN=mizar 
```

An example start up command will be
```
CNIPLUGIN=mizar IS_RESOURCE_PARTITION=true TENANT_SERVER=172.30.0.14,172.30.0.156 RESOURCE_PARTITION_POD_CIDR=10.244.0.0/16 ./hack/arktos-up-scale-out-poc.sh
```

3. Add worker into arktos scale out cluster started with default network solution, bridge:
    1. On worker node, create folder /var/run/kubernetes or start ./hack/arktos-up.sh so it will create the folder automatically.
    2. Copy /var/run/kubernetes/client-ca.crt file from arktos master. Or if you started arktos-up.sh in step 2, it will be created automatically.
    3. Start worker with the following command, it will be automatically registered as into the cluster:
```bash
CNIPLUGIN=mizar IS_SCALE_OUT=true API_HOST=<resource partition master ip> API_TENANT_SERVER=<tenant partition ips separated by comma> ./hack/arktos-worker-up.sh
```

## Test Cluster <a name="test-cluster"></a>

1. Use kubectl with kubeconfig. For example:

```
kubectl --kubeconfig /var/run/kubernetes/admin.kubeconfig get pods
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
1. If there is no code changes, can use "./hack/arktos-up-scale-out-poc.sh -O" to save compile time.

1. Node information can only be retrieved in specific resource partition master, others resources such as pods/deployments/services need to be retrieved from associated tenant partition. 

1. In local test environment, insecured connections are used to avoid copy certificates cross hosts.

1. Currently local RP started as node tained to be NoSchedule. Need to manually remove the taint so that pod can be scheduled.
```
kubectl --kubeconfig <kubeconfig points to RP api server> taint nodes <node_name> node.kubernetes.io/not-ready:NoSchedule-
``` 

1. Currently Mizar-Arktos integation does not support multiple VPCs sharing same IP range, even across different tenant partition. Therefore, VPC_RANGE_START and VPC_RANGE_END
is introduced to ensure the VPC created for each tenant won't have overlapped IP range.
