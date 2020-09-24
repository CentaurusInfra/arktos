# How to Setup a Dev Cluster of multi-nodes

It may be desired to setup a dev cluster having one or more worker nodes in order to play with comprehensive features of Arktos. The simple and easy-to-use arktos-up.sh feels short in this case; this doc describes the minimum effort to run a cluster having 1 master node and extra worker nodes joining after the master is up.

This doc, as at the moment written, does not mandate which cni plugin to be used; it is up to the user. We have verified that Flannel works well with multi-tenancy Arktos cluster; the instructions laid out here are based on our experience with Flannel at GCP env.

Assuming you have got the Arktos repo downloaded to your local disk and the current folder is at the root of the repo,

0. Make sure the following directories are empty. If not, clean them up.
```
/opt/cni/bin/
/etc/cni/net.d/
```

1. bootstrap the cluster by starting the master node (no CNI plgin at first)
```bash
export ARKTOS_NO_CNI_PREINSTALLED=y
./hack/arktos-up.sh
```

Note: arktos-up.sh should be stuck in "Waiting for node ready at api server" messages. Don't worry, the apiserver is already up at this point, just the master node status is not "Ready" as we have not installed the network plugin yet. 

2. Open another terminal to the master node to install CNI plugin
```bash
./cluster/kubectl.sh apply -f https://github.com/coreos/flannel/raw/master/Documentation/kube-flannel.yml
```

After that, "arktos-up.sh" should get rid of "Waiting for node ready at api server" messages and be successfully started.

3. In the lab machine to be added as a worker node, ensure following worker secret files copied from the master node:
If the worker node is a GCP instance  
```bash
mkdir -p /tmp/arktos
gcloud compute scp <master-node-instance>:/var/run/kubernetes/kubelet.kubeconfig /tmp/arktos/
gcloud compute scp <master-node-instance>:/var/run/kubernetes/client-ca.crt /tmp/arktos/
```

If the worker node is an AWS EC2, you can download files /var/run/kubernetes/kubelet.kubeconfig and /var/run/kubernetes/client-ca.crt from the master node, and then upload to /tmp/arktos/ in the worker node.

At the moment this doc is written, kube-proxy does not support multi-tenancy yet, and it won't be deployed in the cluster. After kube-proxy issue has been fixed (we will allocate resource to cope with it soon), probably we will also need to copy over kube-proxy related secret and artifacts.

As a temporary measure to accommodate Flannel pods to access api server through the kubernetes service IP (the notorious 10.0.0.1) at the absense of kube-proxy, we need to place a bit trick by running below command __at the worker node__, assuming the kupe-apiserver is listening at https://10.138.0.19:6443 (please substitute with the proper value of your own cluster):
```bash
sudo iptables -t nat -A OUTPUT -p tcp -d 10.0.0.1 --dport 443 -j DNAT --to-destination 10.138.0.19:6443
```

NOTE: you need to re-run the above command once you restart the machine.

Please be advised that this is a temporary quirk only; after we have the full service support by proper kube-proxy or other means, we don't need it any more.

4. start the worker node and register into cluster

First Make sure the following directories in the worker node are empty. If not, clean them up.
```
/opt/cni/bin/
/etc/cni/net.d/
```

Then at worker node, run following commands:
```bash
export ARKTOS_NO_CNI_PREINSTALLED=y
export KUBELET_IP=<worker-ip>
./hack/arktos-worker-up.sh
```

After the script returns, go to master node terminal and run command "[arktos_repo]/cluster/kubectl.sh get nodes", you should see the work node is displayed and its status should be "Ready".

5. label worker node as vm runtime capable (optional)

If you would like to allow this work node to run VM-based pods, please run below command at the master console:
```bash
./cluster/kubectl.sh label node <worker-node-name> extraRuntime=virtlet
```
The work-node-name is the new worker just added; its name can be found by ```./cluster/kubectl.sh get node```.

You should be able to notice that node in READY state after a while; you can run container pods now:
```bash
./cluster/kubectl.sh run nginx --image=nginx --replicas=2
./cluster/kubectl.sh get pod -o wide
```

To run a small cirros VM pod, you can use below yaml content:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: cirros-vm
spec:
  virtualMachine:
    name: vm
    keyPairName: "foo"
    image: download.cirros-cloud.net/0.3.5/cirros-0.3.5-x86_64-disk.img
    imagePullPolicy: IfNotPresent
```

# How to set up multiple partitioned apiservers

It may be desired to setup a cluster having 2 or 3 apiservers in order to play with the scalability features of Arktos. It is simple and easy to implement it using arktos-up.sh, arktos-apiserver-partition.sh, and install-etcd.sh. The  doc describes the minimum effort to run a cluster having 2 apiservers (1 apiserver first, plus a apiserver who joins later).

Instructions are in [API Server Partition](arktos-apiserver-partition.md)
