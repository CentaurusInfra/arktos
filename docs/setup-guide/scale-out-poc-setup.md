### Brief 

This introduces the tool to set up the the nginx proxy, tenant-partition cluster(s), and the resource-partition cluster(s).

### Environment variables setup
```bash
export TENANT_PARTITION_IP=[tenant-partition-ip]
export RESOURCE_PARTITION_IP=[resource-partition-ip]
export SCALE_OUT_PROXY_ENDPOINT=http://[nginx-proxy-ip]:8888
```

### nginx proxy setup
On the machine to run nginx proxy (it could be the tenant partition machine, the resource partition machine, or a third machine), run:

```bash
./hack/scale_out_poc/setup_nginx_proxy.sh
```

### On the TENANT partition:

Run 
```bash
export IS_RESOURCE_PARTITION=false 
./hack/arktos-up-scale-out-poc.sh
```

### On the RESOURCE partition:

Run 
```bash
export IS_RESOURCE_PARTITION=true 
./hack/arktos-up-scale-out-poc.sh
```

### Tests done
This is verified on my local setups with two AWS machines. 


## GCE ScaleOut Cluster Setup for Kubemark Perf Tests
A single resource partition and single tenant partition ScaleOut cluster for Kubemark perf testing can be deployed as follows:

### Deploy admin cluster
Build with make quick-relase and deploy admin cluster.
```bash
export MASTER_DISK_SIZE=200GB MASTER_ROOT_DISK_SIZE=200GB KUBE_GCE_ZONE=us-east2-b MASTER_SIZE=n1-highmem-32 NODE_SIZE=n1-highmem-16 NUM_NODES=2 NODE_DISK_SIZE=200GB GOPATH=$HOME/go KUBE_GCE_ENABLE_IP_ALIASES=true KUBE_GCE_PRIVATE_CLUSTER=true CREATE_CUSTOM_NETWORK=true KUBE_GCE_INSTANCE_PREFIX=k8s-scaleout KUBE_GCE_NETWORK=k8s-scaleout ENABLE_KCM_LEADER_ELECT=false SHARE_PARTITIONSERVER=false LOGROTATE_FILES_MAX_COUNT=10 LOGROTATE_MAX_SIZE=200M TEST_CLUSTER_LOG_LEVEL=--v=2 APISERVERS_EXTRA_NUM=0 WORKLOADCONTROLLER_EXTRA_NUM=0 ETCD_EXTRA_NUM=0 KUBEMARK_NUM_NODES=100

./cluster/kube-up.sh
```

### Deploy ScaleOut kubemark cluster
```bash
SCALEOUT_CLUSTER=true ./test/kubemark/start-kubemark.sh
```

### Cleanup of ScaleOut kubemark cluster
```bash
SCALEOUT_CLUSTER=true ./test/kubemark/stop-kubemark.sh

./cluster/kube-down.sh
```

