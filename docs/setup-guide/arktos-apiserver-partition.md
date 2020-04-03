
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
. On 1st and 2nd api server hosts, run the following command and get kubeconfig file that connects to the api server
```
$ cd $GOPATH/src/arktos
$ ./hack/create-kubeconfig.sh command <path to where you want to generate kubeconfig file>
$ ./hack/create-kubeconfig.sh command <path to where you want to copy kubeconfig file from> <path to where you want to generate kubeconfig file>
# Copy generated print out from the above command and run it in shell
```
If the path to where you want to copy kubeconfig file from is not specified, it will generate the command based on the kubeconfig file /var/run/kubernetes/admin.config. In the directory /var/run/kubernetes/, there are several kubeconfig files, such as controller.kubeconfig, scheduler.kubeconfig,... If we want to get kubeconfig for kube scheduler to run in client hosts, we might run 
```
 ./hack/create-kubeconfig.sh command /var/run/kubernetes/scheduler.kubeconfig  <path to where you want to generate kubeconfig file>
```

. Copy both kubeconfig files into the hosts where you want to access api servers

### Run workload controller managers with multiple apiserver configs
```
$ ./hack/arktos-apiserver-partition.sh start_workload_controller_manager <kubeconfig_filepath1> <kubeconfig_filepath2> ... <kubeconfig_filepathn>
```

### Run kube controller managers with multiple apiserver configs
```
$ ./hack/arktos-apiserver-partition.sh start_kube_controller_manager kubeconfig_target_filepath1 kubeconfig_target_filepath2 kubeconfig_target_filepath3, ...
```

### Run the kube scheduler with multiple apiserver configs
```
$ ./hack/arktos-apiserver-partition.sh start_kube_scheduler kubeconfig_target_filepath1 kubeconfig_target_filepath2 kubeconfig_target_filepath3, ...
```

### Run the kubelet with multiple apiserver configs
```
$ ./hack/arktos-apiserver-partition.sh start_kubelet kubeconfig_target_filepath1 kubeconfig_target_filepath2 kubeconfig_target_filepath3, ...
```

### Run the kube proxy with multiple apiserver configs
```
$ ./hack/arktos-apiserver-partition.sh start_kube_proxy kubeconfig_target_filepath1 kubeconfig_target_filepath2 kubeconfig_target_filepath3, ...
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
startTenant: "A"
isStartTenantValid: false 
endTenant: "m"
isEndTenantValid: true
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

