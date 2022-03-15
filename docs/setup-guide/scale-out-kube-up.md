# Setting up Arktos scale out environment in Google Cloud

This document gives brief introduction on how to set up Arktos scale out environment in Google Cloud.

Step 1. Request a VM in Google Cloud Platform, recommended configuration: ubuntu 18.04+, 8 cpu or above, disk size 200GB or up.

Step 2. Run "gcloud version" to ensure the Google Gloud DSK is updated (recommended Google Cloud SDK version is 298.0.0 and up). Please refer to https://cloud.google.com/sdk/docs/downloads-apt-get or https://cloud.google.com/sdk/docs/downloads-versioned-archives to upgrade your google cloud SDK. 

Step 3. Follow [dev environment setup instruction](setup-dev-env.md) to set up developer environment on this newly created GCE instance.

Step 4. Build prepare:

```bash
cd ~/go/src/arktos
make clean
make quick-release
```

Step 5. Set Arktos cluster configuration. The following are the typical environment variables and values used in Arktos 2TP 2RP scale out cluster with Mizar CNI Plugin:
```bash
export NUM_NODES=2                # number of nodes for each RP
export SCALEOUT_TP_COUNT=2        # number of tenant partitions to set up
export SCALEOUT_RP_COUNT=2        # number of resource partitions to set up
export RUN_PREFIX=<fill-in-your-cluster-name>       # cluster name

export MASTER_DISK_SIZE=200GB       # disk size for master machine to run API server, scheduler, controller manager, etc.
export MASTER_ROOT_DISK_SIZE=200GB 
export KUBE_GCE_ZONE=us-central1-b  # the location of the new arktos cluster
export MASTER_SIZE=n1-highmem-32    # machine size for masters
export NODE_SIZE=n1-highmem-16      # machine size for workers
export NODE_DISK_SIZE=200GB         # disk size for workers
export GOPATH=$HOME/go 
export KUBE_GCE_ENABLE_IP_ALIASES=true 
export KUBE_GCE_PRIVATE_CLUSTER=true 
export CREATE_CUSTOM_NETWORK=true KUBE_GCE_INSTANCE_PREFIX=${RUN_PREFIX} KUBE_GCE_NETWORK=${RUN_PREFIX} ENABLE_KCM_LEADER_ELECT=false ENABLE_SCHEDULER_LEADER_ELECT=false SHARE_PARTITIONSERVER=false LOGROTATE_FILES_MAX_COUNT=50 LOGROTATE_MAX_SIZE=200M KUBE_ENABLE_APISERVER_INSECURE_PORT=true KUBE_ENABLE_PROMETHEUS_DEBUG=true KUBE_ENABLE_PPROF_DEBUG=true
export SCALEOUT_CLUSTER=true 

export NETWORK_PROVIDER=mizar
```

Step 6. Start Arktos cluster:
```bash
./cluster/kube-up.sh
```

Step 7. Find kubeconfig and use kubectl to operate on the newly created Arktos cluster:
```bash
./cluster/kubectl.sh --kubeconfig cluster/kubeconfig.tp-1  get pods
```

Step 8. Tear down Arktos cluster:
```bash
./cluster/kube-down.sh
```

Note: Connection to GCP can be terminated unexpectedly, before tearing down the cluster, make sure your environment variables are set to be the same when you start the cluster.


