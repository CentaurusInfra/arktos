# How to Setup a Dev Cluster of multi-nodes

It may be desired to setup a dev cluster having one or more worker nodes in order to play with comprehensive features of Arktos. The simple and easy-to-use arktos-up.sh feels short in this case; this doc describes the minimum effort to run a cluster having 1 master node and extra worker nodes joining after the master is up.

This doc, as at the moment written, does not mandate which cni plugin to be used; it is up to the user. We have verified that Flannel works well with multi-tenancy Arktos cluster; the instructions laid out here are based on our experience with Flannel at GCP env or AWS env with Ubuntu 18.04 x86 image.

Assuming you have got the Arktos repo downloaded to your local disk and the current folder (i.e ~/go/src/arktos/) is at the root of the repo, and 

- Setup master node and workwer nodes based on [set up developer environment](setup-dev-env.md) to install needed packages (docker, make, gcc, jq and golang).
- SSH/SCP should work from worker nodes to master node in order to copy the worker secret files from the master node (in AWS, use private key file of keypair to access to master/worker nodes).
- Needed ports are opened between master node and worker nodes
  * Allow access to kube-api port 6443 on master node from work nodes (in AWS, add this rule into inbound rules of security group for master node)
  * Allow access to kubelet port 10251 and kube-proxy port 10255 on worker nodes from master node(in AWS, add this rule into inbound rules of security group for worker nodes)
- On master node, the permisson of others for file /var/run/docker.sock should be readable and writable.
  Here is output of running command 'ls -al'

```bash
srw-rw-rw- 1 root docker 0 Aug  3 22:15 /var/run/docker.sock
```
  
  Normally if the machine is rebooted, the permission of this file is changed to default permission below.

```bash
srw-rw---- 1 root docker 0 Aug  9 23:18 /var/run/docker.sock
```
  
  Please run the command to add the permission for 'others' using sudo
```bash
sudo chmod o+rw /var/run/docker.sock
ls -al
```



0. Make sure the following directories are empty. If not, clean them up using sudo permisson
```
sudo rm -rf /opt/cni/bin/*
sudo rm -rf /etc/cni/net.d/*
```

1. bootstrap the cluster by starting the master node (no CNI plugin at first)
```bash
export ARKTOS_NO_CNI_PREINSTALLED=y
make clean
./hack/arktos-up.sh
```

Note: arktos-up.sh should be stuck in "Waiting for node ready at api server" messages. Don't worry, the apiserver is already up at this point, just the master node status is not "Ready"  and pod/kube-dns in name space 'kube-system' is not in state of 'Running' as we have not installed the network plugin yet. 

2. Open another terminal to the master node to install CNI plugin of flannel
```bash
./cluster/kubectl.sh apply -f https://github.com/coreos/flannel/raw/master/Documentation/kube-flannel.yml
```

After that, "arktos-up.sh" should get rid of "Waiting for node ready at api server" messages and be successfully started. You can check the status of all resources including pod/kube-dns and flannel network

```bash
export KUBECONFIG=/var/run/kubernetes/admin.kubeconfig
./cluster/kubectl.sh get all --all-namespaces
ifconfig -a
ip route
sudo ls -alg /etc/cni/net.d/10-flannel.conflist
```

Note: Please reference the blog [Kubernetes: Flannel networking](https://blog.laputa.io/kubernetes-flannel-networking-6a1cb1f8ec7c) if you want to know how flannel network works in kubernetes.

Note: CNI plugin of Calico is not supported because the resource 'EndpointSlices' is not supported by Arktos so far.

3. In the lab machine to be added as a worker node, ensure following worker secret files copied from the master node:
If the worker node is a GCP instance  
```bash
gcloud beta compute ssh --zone "us-west2-a" "<worker node machine name>"  --project "<gce project name>"
```

If the worker node is an AWS EC2, you can download files /var/run/kubernetes/kubelet.kubeconfig and /var/run/kubernetes/client-ca.crt from the master node, and then upload to /tmp/arktos/ in the worker node.

```bash
mkdir -p /tmp/arktos
scp -i "<private key of keypair of master node>" ubuntu@<master-node-instance>:/var/run/kubernetes/kubelet.kubeconfig /tmp/arktos/kubelet.kubeconfig
scp -i "<private key of keypair of master node>" ubuntu@<master-node-instance>:/var/run/kubernetes/client-ca.crt /tmp/arktos/client-ca.crt
```

4. start the worker node and register into cluster

First make sure the following directories in the worker node are empty. If not, clean them up using sudopermission.
```
sudo rm -rf /opt/cni/bin/*
sudo rm -rf /etc/cni/net.d/*
```

Then at worker node, run following commands:
```bash
export ARKTOS_NO_CNI_PREINSTALLED=y

hostname -i
export KUBELET_IP=<worker-ip>

OR
export KUBELET_IP=`hostname -i`

echo $KUBELET_IP
make clean
./hack/arktos-worker-up.sh
```

After the script returns, go to master node terminal and run command "./cluster/kubectl.sh get nodes", you should see the work node is displayed and its status should be "Ready".

But when you run command "./cluster/kubectl.sh get all --all-namespaces", you will see new pod of flannel 'pod/kube-flannel-ds-xxxxx' for worker node is not in Running state. If you check the log of this pod 'pod/kube-flannel-ds-xxxxx', you will see the following error.

```bash
I0803 22:50:37.646013       1 main.go:520] Determining IP address of default interface
I0803 22:50:37.646394       1 main.go:533] Using interface with name eth0 and address 172.31.2.184
I0803 22:50:37.646415       1 main.go:550] Defaulting external address to interface address (172.31.2.184)
W0803 22:50:37.646432       1 client_config.go:608] Neither --kubeconfig nor --master was specified.  Using the inClusterConfig.  This might not work.
E0803 22:51:07.648172       1 main.go:251] Failed to create SubnetManager: error retrieving pod spec for 'kube-system/kube-flannel-ds-vgftf': Get "https://10.0.0.1:443/api/v1/namespaces/kube-system/pods/kube-flannel-ds-xxxxx": dial tcp 10.0.0.1:443: i/o timeout
```

5. On worker node, add one more rule of Linux iptables to fix the above issue

At the moment this doc is written, kube-proxy does not support multi-tenancy yet, and it won't be deployed in the cluster. After kube-proxy issue has been fixed (we will allocate resource to cope with it soon), probably we will also need to copy over kube-proxy related secret and artifacts.

As a temporary measure to accommodate Flannel pods to access api server through the kubernetes service IP (the notorious 10.0.0.1) at the absense of kube-proxy, we need to place a bit trick by running below command __at the worker node__, assuming the kupe-apiserver is listening at https://10.138.0.19:6443 (please substitute with the proper value of your own cluster):

```bash
sudo iptables -t nat -A OUTPUT -p tcp -d 10.0.0.1 --dport 443 -j DNAT --to-destination 10.138.0.19:6443
sudo iptables -t nat -L (for verification)
```

NOTE: you need to re-run the above command once you restart the machine.

Please be advised that this is a temporary quick only; after we have the full service support by proper kube-proxy or other means, we don't need it any more.

Then you should see pod 'pod/kube-flannel-ds-xxxxx' is in Running state after you run command "./cluster/kubectl.sh get all --all-namespaces". 

6. Test whether the ngnix application can be deployed successfully

NOTE: You need first run the following command to create clusterrolebinding 'system-node-role-bound' to bind the group 'system:nodes' to clusterrole 'system:node' so that the master node has corresponding permission to get secret for every namespace and transfer the secret to worker node during pod creation.

```bash
./cluster/kubectl.sh create clusterrolebinding system-node-role-bound --clusterrole=system:node --group=system:nodes
./cluster/kubectl.sh get clusterrolebinding/system-node-role-bound -o yaml
```

Then you can run container pods for nginx now and see all pods should be in Running state.
```bash
./cluster/kubectl.sh run nginx --image=nginx --replicas=2
./cluster/kubectl.sh get all -n default
```
