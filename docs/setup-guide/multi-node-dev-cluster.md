# How to Setup a small Dev Cluster of multi-nodes

It may be desired to setup a dev cluster having 2 or 3 nodes in order to play with comprehansive features of Arktos. The simple and easy-to-use arktos-up.sh feels short in this case; this doc describes the minimum effort to run a cluster havinbg 2 nodes (1 master, plus a worker who joins later).

This doc, as at the moment written, does not mandate which cni plugin to be used. It is up to user, though we recommend to use calico, or our own alktron.

1. start the master node 
 the master node, run follwoing commands to start cluster, which has the master only.
```bash
./hack/arktos-up.sh
```

2. copy needed files to worker node

following files are required to copy over from master to worker node:
_output/local/bin/linux/amd64/hyperkube
/var/run/kubernetes/kubelet.kubeconfig
/var/run/kubernetes/kube-proxy.kubeconfig
/var/run/kubernetes/client-ca.crt
/tmp/kube-proxy.yaml


3. start worker node
at worker node, run follwoing commands
```bash
export KUBELET_IP=<worker-ip>
export KUBELET_KUBECONFIG=<path-to-kubelet.kubeconfig-copied-from-master>
./arktos-worker-up.sh
```

If kube-proxy is desired to run on worker, first make sure kube-proxy.yaml points to proper kube-proxy.kubeconfig, then run (note: for alktron as cni plugin, you don't need to run kube-proxy at all)
```bash
sudo ./hyperkube kube-proxy --v=3 --config=<path-to-kube-proxy.yaml> --master=https://<master-node-name>:6443
```

4. label worker node as vm runtime capable (optioanl)
at master node, run
```bash
./cluster/kubectl.sh label node <worker-node> extraRuntime=virtlet
```

After both nodes are in READY state, you should be able to run container or vm based pods on them, like below
```bash
./cluster/kubectl.sh run nginx --image=nginx --replicas=2
./cluster/kubectl.sh get pod -o wide
```

# How to set up multiple partitioned apiservers

It may be desired to setup a cluster having 2 or 3 apiservers in order to play with the scalability features of Arktos. It is simple and easy to implement it using arktos-up.sh, arktos-apiserver-partition.sh, and install-etcd.sh. The  doc describes the minimum effort to run a cluster havinbg 2 apiservers (1 apiserver first, plus a apiserver who joins later).

1. start the master node 
 On the master node, run the follwoing command to start cluster, which has the master only.
```bash
./hack/arktos-up.sh parition_begin partition_end
```
2. start the second apiserver 
  On another host, run the following command to join the existing etcd cluster in step 1 as a member
```bash
./hack/install-etcd.sh add hostname http://hostip:2379
./hack/arktos-apiserver-partition.sh start_apiserver parition_begin partition_end
```
3.  push kubeconfig on the host as secret
 ```bash
./hack/arktos-apiserver-partition.sh save_kubeconfig
```
4.  extract kubeconfig on the master
 ```bash
./hack/arktos-apiserver-partition.sh extract_kubeconfig path_to_kubeconfig
```
5.  restart workload controller manager 
 ```bash
./hack/arktos-apiserver-partition.sh start_workload_controller_manager path_to_kubeconfig_files
```
