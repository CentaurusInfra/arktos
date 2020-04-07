
# Setting up Partitioned Arktos Environment

[![Go Report Card](https://goreportcard.com/badge/github.com/futurewei-cloud/arktos)](https://goreportcard.com/report/github.com/futurewei-cloud/arktos)
[![LICENSE](https://img.shields.io/badge/license-apache%202.0-green)](https://github.com/futurewei-cloud/arktos/blob/master/LICENSE)

## What this document is about

This document serves as a manual to set up data partitioned Arktos cluster across multiple machines. It has the following features:

1. Multiple API servers can be configured to serve in a single Kubernetes clusters. ETCD Data is partitioned across API servers.

1. Multiple controller instances (currently we suppport replicaset controller and deployment controller) can be running in parralled in different processes (named as workload controller manager). Each of them handles a portion of the requests to controller 


## How to set up partitioned Arktos environment

### Prerequsite

Currently we require setting up developer environment. See [dev environment setup](setup-dev-env.md)

### Start ETCD and 1st Api Server, plus Kube CM, Workload Controller Manager, Kube Scheduler

```
$ cd $GOPATH/src/arktos
$ ./hack/arktos-up.sh
```
Note: this will soon be changed.

### Start apiserver on a worker host

. Verify we can access etcd from the worker host
```
$ curl http://<1st master server ip>:2379/v2/keys
# expected return
# {"action":"get","node":{"dir":true}}
```

. Start 2nd Api server (in a different host)
```
# configure api server with data partition
$ export APISERVER_SERVICEGROUPID=2
$ ./hack/arktos-apiserver-partition.sh start_apiserver <ETCD server ip>
```

### Get kubeconfig from master servers to client host
On 1st and 2nd api server hosts, run the following command and get kubeconfig file that connects to the api server

. Create a folder for kubeconfig
```
$ mkdir -p /home/ubuntu/kubeconfig
```
. Get kubeconfig files used by Kube Controller Manager, Workload Controller Manager, and Kubectl
```
$ cd $GOPATH/src/arktos
$ ./hack/create-kubeconfig.sh command /home/ubuntu/kubeconfig/kubeconfig-<apiserver#>.config
# Copy generated print out from the above command and run it in shell
```
. Get kubeconfig files used by Scheduler 
```
$ ./hack/create-kubeconfig.sh command /var/run/kubernetes/scheduler.kubeconfig /home/ubuntu/kubeconfig/scheduler-<apiserver#>.kubeconfig
# Copy generated print out from the above command and run it in shell
```
. Get kubeconfig files used by Kubelet
```
$ ./hack/create-kubeconfig.sh command /var/run/kubernetes/kubelet.kubeconfig  /home/ubuntu/kubeconfig/kubelet-<apiserver#>.kubeconfig
# Copy generated print out from the above command and run it in shell
```
. Get kubeconfig files used by KubeProxy
```
$ ./hack/create-kubeconfig.sh command /var/run/kubernetes/kube-proxy.kubeconfig /home/ubuntu/kubeconfig/kube-proxy-<apiserver#>.kubeconfig
# Copy generated print out from the above command and run it in shell
```

. Copy both kubeconfig files into the hosts where you want to access api servers. Expected kubeconfig files as listed below:
```
-rw-r--r--  1 root   root    6078 Apr  3 21:46 kubeconfig-1.config
-rw-r--r--  1 root   root   12146 Apr  3 22:01 kubeconfig-2.config
-rw-r--r--  1 root   root    6106 Apr  3 21:51 kubelet-1.kubeconfig
-rw-r--r--  1 root   root    6097 Apr  3 22:01 kubelet-2.kubeconfig
-rw-r--r--  1 root   root    6082 Apr  3 21:52 kube-proxy-1.kubeconfig
-rw-r--r--  1 root   root    6081 Apr  3 22:01 kube-proxy-2.kubeconfig
-rw-r--r--  1 root   root    6054 Apr  3 21:50 scheduler-1.kubeconfig
-rw-r--r--  1 root   root    6053 Apr  3 22:01 scheduler-2.kubeconfig
```

### Run workload controller managers with multiple apiserver configs
Kill existing workload controller manager first. Make sure delete controller instance from etcd, otherwise, it takes 5 minutes to discover the instance is dead.
```
$ ./hack/arktos-apiserver-partition.sh start_workload_controller_manager /home/ubuntu/kubeconfig/kubeconfig-1.config /home/ubuntu/kubeconfig/kubeconfig-2.config
```

### Run kube controller managers with multiple apiserver configs
```
$ ./hack/arktos-apiserver-partition.sh start_kube_controller_manager /home/ubuntu/kubeconfig/kubeconfig-1.config /home/ubuntu/kubeconfig/kubeconfig-2.config
```

### Run the kube scheduler with multiple apiserver configs
```
$ ./hack/arktos-apiserver-partition.sh start_kube_scheduler /home/ubuntu/kubeconfig/scheduler-1.config /home/ubuntu/kubeconfig/scheduler-2.config
```

### Run the kubelet with multiple apiserver configs
```
$ ./hack/arktos-apiserver-partition.sh start_kubelet /home/ubuntu/kubeconfig/kubelet-1.config /home/ubuntu/kubeconfig/kubelet-2.config
```

### Run the kube proxy with multiple apiserver configs
```
$ ./hack/arktos-apiserver-partition.sh start_kube_proxy /home/ubuntu/kubeconfig/kube-proxy-1.config /home/ubuntu/kubeconfig/kube-proxy-2.config
```

### Skip building steps
Copy existing binaries to a folder, for example, /home/ubuntu/output/
Run the following commands to skip building
```
$ export BINARY_DIR=/home/ubuntu/output/
$ bash ./hack/arktos-apiserver-partition.sh start_XXX ...
```
Note: make sure to add the last "/".


## Testing Senario

Step 1: Set data partition for api servers
```
kubectl apply -f datapartition-s1.yaml
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

