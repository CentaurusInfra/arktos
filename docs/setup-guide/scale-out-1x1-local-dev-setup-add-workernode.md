# How to setup a Scale-Out 1 TP X 1 RP (multi-node join) Dev Cluster with Flannel in process mode

## Scenarios
1. One Tenant Partition

2. One Resource Partition (Master node, worker nodes)

## Prerequsite
1. 4 dev box (tested on ubuntu 18.04), 3 for RP1 cluster, 1 for TP1. Record ip as TP1_IP, RP1_IP, RP1_WORKER1_IP, RP1_WORKER2_IP

2. Assuming you have got the Arktos repo downloaded to your local disk and the current folder (i.e ~/go/src/arktos/) is at the root of the repo, and 

- Setup all nodes based on [set up developer environment](https://github.com/q131172019/arktos/blob/CarlXie_singleNodeArktosCluster/docs/setup-guide/setup-dev-env.md) to install needed packages (docker, make, gcc, jq and golang).
- And all nodes has been tested successfully based on [set up single node cluster](https://github.com/q131172019/arktos/blob/CarlXie_singleNodeArktosCluster/docs/setup-guide/single-node-dev-scale-up-cluster.md)
- SSH/SCP should work from worker nodes to master node in order to copy the worker secret files from the master node (in AWS, use private key file of keypair to access to master/worker nodes).
- Needed ports are opened between master node and worker nodes
- On all nodes using AWS EC2 instances, stop source / destination check
  * Instance -> Networking -> Change source / destination check -> click check box at Stop to stop Source / destination check
- On all nodes, after rebooting the permisson of others for file /var/run/docker.sock should be readable and writable.

```bash
   sudo chmod o+rw /var/run/docker.sock; sudo ls -alg /var/run/docker.sock
``` 

## Steps
1. Understand the steps of [set up scale-out 1 X 1 environment](https://github.com/CentaurusInfra/arktos/blob/master/docs/setup-guide/scale-out-local-dev-setup.md) and run the following scripts to automatically start TP1 and RP1
1.1)  On TP1: 
```bash
   ./hack/scale-out-1x1-rp1-multi-nodes/scale-out-TP1-node.sh <RP1_IP>
```

1.2) On RP1: 
```bash
   ./hack/scale-out-1x1-rp1-multi-nodes/scale-out-RP1-node.sh <TP1_IP>
```

2. On worker nodes to join into RP1 cluster, run the script to automatically join into RP1 cluster
```bash
   ./hack/scale-out-1x1-rp1-multi-nodes/scale-out-RP1-worker-node-join.sh <RP1_IP>
```

3.  On RP1 node, check the status of node and check the flannel log on each node of RP1 cluster
```bash
   ./cluster/kubectl.sh get nodes
```
```bash
   cat /tmp/flanneld.log
```

4. Test whether the ngnix application can be deployed successfully
```bash
   ./cluster/kubectl.sh run nginx --image=nginx --replicas=10
   ./cluster/kubectl.sh get pod -n default -o wide
```
```bash
   ./cluster/kubectl.sh exec -ti <1st pod> -- curl <IP of another nginx pods>
```
```bash
   ./cluster/kubectl.sh exec -ti <2st pod> -- curl <IP of another nginx pods>
```
```bash
   ./cluster/kubectl.sh exec -ti <3rd pod> -- curl <IP of another nginx pods>
```

```bash
   ./cluster/kubectl.sh delete deployment/nginx
```

5. Please follow up the steps to do [end-to-end verification of service in scale-out cluster](https://github.com/CentaurusInfra/arktos/issues/1143)


# How to setup a Scale-Up Multi-node Dev Cluster with flannel in process mode

## Scenarios
1. One Master Node

2. Three Worker Node (worker-node-1, worker-node-2, worker-node-3)

## Prerequsite
1. 4 dev box (tested on ubuntu 18.04), 3 for worker nodes, 1 for master node. Record ip as MASTER_IP, WORKER1_IP, WORKER2_IP, WORKER3_IP

2. Assuming you have got the Arktos repo downloaded to your local disk and the current folder (i.e ~/go/src/arktos/) is at the root of the repo, and

- Setup all nodes based on [set up developer environment](https://github.com/q131172019/arktos/blob/CarlXie_singleNodeArktosCluster/docs/setup-guide/setup-dev-env.md) to install needed packages (docker, make, gcc, jq and golang).
- And all nodes has been tested successfully based on [set up single node cluster](https://github.com/q131172019/arktos/blob/CarlXie_singleNodeArktosCluster/docs/setup-guide/single-node-dev-scale-up-cluster.md)
- SSH/SCP should work from worker nodes to master node in order to copy the worker secret files from the master node (in AWS, use private key file of keypair to access to master/worker nodes).
- Needed ports are opened between master node and worker nodes
- On all nodes using AWS EC2 instances, stop source / destination check
  * Instance -> Networking -> Change source / destination check -> click check box at Stop to stop Source / destination check
- On all nodes, after rebooting the permisson of others for file /var/run/docker.sock should be readable and writable.

```bash
   sudo chmod o+rw /var/run/docker.sock; sudo ls -alg /var/run/docker.sock
```

## Steps
1.  On master node: run the following script to start single node scale-up cluster,
```bash
   ./hack/scale-up-multi-nodes/scale-up-master-node.sh
```
2.  On worker nodes: run the following script to join into scale-up cluster
```bash
   ./hack/scale-up-multi-nodes/scale-up-worker-node-join.sh <MASTER_NODE_IP>
```

3.  On master node, check the status of nodes and check the flannel log /tmp/flanneld.log on each node
```bash
   ./cluster/kubectl.sh get nodes
```
```bash
   cat /tmp/flanneld.log
```

4. Test whether the ngnix application can be deployed successfully
```bash
   ./cluster/kubectl.sh run nginx --image=nginx --replicas=10
   ./cluster/kubectl.sh get pod -n default -o wide
```
```bash
   ./cluster/kubectl.sh exec -ti <1st pod> -- curl <IP of  another nginx pods>
```
```bash
   ./cluster/kubectl.sh exec -ti <2st pod> -- curl <IP of  another nginx pods>
```
```bash
   ./cluster/kubectl.sh exec -ti <3rd pod> -- curl <IP of another nginx pods>
```

```bash
   ./cluster/kubectl.sh delete deployment/nginx
```

5. Please follow up the steps to do [end-to-end verification of service in scale-up cluster](https://github.com/CentaurusInfra/arktos/issues/1142)

