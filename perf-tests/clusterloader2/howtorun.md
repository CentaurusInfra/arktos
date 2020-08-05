Process under GCP project. Please remember perf-tests running on kubemark master, so all logs should be checked under /var/log/ on kubemark-master

## Pre-rerequisites: GCP config
1. run "gcloud version" to ensure your Google Cloud SDK is updated (suggested Google Cloud SDK 298.0.0 and up), Please refer to https://cloud.google.com/sdk/docs/downloads-apt-get or https://cloud.google.com/sdk/docs/downloads-versioned-archives to upgrade your google cloud SDK
2. ensure your docker login and access has been configured, if not, please run "gcloud auth configure-docker" to config

## Pre-rerequisites: build prepare
```
git clone [arktos git link]
cd arktos
make clean
make quick-release
```

## Start kubemark cluster
1. kube-up to start admin cluster, start-kubemark to start kubemark cluster
```
$ export MASTER_ROOT_DISK_SIZE=100GB MASTER_DISK_SIZE=200GB KUBE_GCE_ZONE=us-west2-b MASTER_SIZE=n1-highmem-32 NODE_SIZE=n1-highmem-8 NUM_NODES=8 NODE_DISK_SIZE=200GB KUBE_GCE_NETWORK=kubemark-500 GOPATH=$HOME/go KUBE_GCE_ENABLE_IP_ALIASES=true KUBE_GCE_PRIVATE_CLUSTER=true CREATE_CUSTOM_NETWORK=true KUBE_GCE_INSTANCE_PREFIX=kubemark-500 ENABLE_KCM_LEADER_ELECT=false
$ ./cluster/kube-up.sh 
$ ./test/kubemark/start-kubemark.sh
```
2. run the line below to change hollow-node “replicas: 500”  if you want to run 500 nodes cluster. Default value is 10 but minimal is 100.
```
$ ./cluster/kubectl.sh scale replicationcontroller hollow-node -n kubemark --replicas=500	
```

3. ensure all hollow-nodes are ready
```
$ kubectl --kubeconfig=$HOME/go/src/k8s.io/arktos/test/kubemark/resources/kubeconfig.kubemark get nodes | wc -l
502
```

4. start perf-tests
```
$ cd ./perf-tests/clusterloader2/
$ export PROMETHEUS_SCRAPE_ETCD=true ENABLE_PROMETHEUS_SERVER=true GOPATH=$HOME/go
$ nohup ./run-e2e.sh --nodes=500 --provider=kubemark --kubeconfig=$HOME/go/src/k8s.io/arktos/test/kubemark/resources/kubeconfig.kubemark --report-dir=$REPORTDIR --testconfig=testing/load/config.yaml --testconfig=testing/density/config.yaml --testoverrides=./testing/experiments/enable_prometheus_api_responsiveness.yaml --testoverrides=./testing/experiments/use_simple_latency_query.yaml
```


5. after all run finished, shutdown cluster
```
$ export MASTER_ROOT_DISK_SIZE=100GB MASTER_DISK_SIZE=200GB KUBE_GCE_ZONE=us-west2-b MASTER_SIZE=n1-highmem-32 NODE_SIZE=n1-highmem-8 NUM_NODES=8 NODE_DISK_SIZE=200GB KUBE_GCE_NETWORK=kubemark-500 GOPATH=$HOME/go KUBE_GCE_ENABLE_IP_ALIASES=true KUBE_GCE_PRIVATE_CLUSTER=true CREATE_CUSTOM_NETWORK=true KUBE_GCE_INSTANCE_PREFIX=kubemark-500 ENABLE_KCM_LEADER_ELECT=false
$ ./test/kubemark/stop-kubemark.sh 
$ ./cluster/kube-down.sh
```

## PartitionServer Config
Arktos support multi data partition server including apiserver, workload-controller-managere, ETCD. if you want to start multi data partition server, Please add the env below
### APISERVERS_EXTRA_NUM
set extra apiserver number, default equals 0.  
if you have extra apiserver, please ensure "--api-server-addresses=https://${extra apiserver external IP}" is added when you run perf-tests. Split by ; for multi servers.
### WORKLOADCONTROLLER_EXTRA_NUM
set extra workload controller member number, , default equals 0
### ETCD_EXTRA_NUM
set extra ETCD cluster number, default equals 0
### SHARE_PARTITIONSERVER
switch to share extra server or create single vms for each server, default equals false, it will create single vms for each server.
