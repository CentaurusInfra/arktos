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
