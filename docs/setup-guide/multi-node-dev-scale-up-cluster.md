# How to Setup a Dev Cluster of multi-nodes with flannel in two modes

It may be desired to setup a dev cluster having one or more worker nodes in order to play with comprehensive features of Arktos. The simple and easy-to-use arktos-up.sh feels short in this case; this doc describes the minimum effort to run a cluster having 1 master node and extra worker nodes joining after the master is up.

This doc, as at the moment written, does not mandate which cni plugin to be used; it is up to the user. We have verified that Flannel works well with multi-tenancy Arktos cluster; the instructions laid out here are based on our experience with Flannel at GCP env or AWS env with Ubuntu 18.04 x86 image.

Assuming you have got the Arktos repo downloaded to your local disk and the current folder (i.e ~/go/src/arktos/) is at the root of the repo, and 

- Setup master node and worker nodes based on [set up developer environment](setup-dev-env.md) to install needed packages (docker, make, gcc, jq and golang).
- And master node has been tested successfully based on [set up single node cluster](single-node-dev-scale-up-cluster.md)
- SSH/SCP should work from worker nodes to master node in order to copy the worker secret files from the master node (in AWS, use private key file of keypair to access to master/worker nodes).
- Needed ports are opened between master node and worker nodes
  * Allow access to kube-api port 6443 on master node from work nodes (in AWS, add this rule into inbound rules of security group for master node)
  * Allow access to kubelet port 10251 and kube-proxy port 10255 on worker nodes from master node(in AWS, add this rule into inbound rules of security group for worker nodes)
- On all nodes using AWS EC2 instances, Stop Source / destination check
  * Instance -> Networking -> Change source / destination check -> click check box at Stop to stop Source / destination check
- On all nodes, after rebooting the permisson of others for file /var/run/docker.sock should be readable and writable.

```bash
   sudo chmod o+rw /var/run/docker.sock; sudo ls -alg /var/run/docker.sock
```

OR

```bash
   sudo groupadd docker
   sudo usermode -aG docker $USER
```


Method #1 - DaemonSet Mode

On master node:

1.0) Make sure the following directories are empty. If not, clean them up using sudo permisson
```bash
   sudo ls -alg /opt/cni/bin/; sudo rm -r /opt/cni/bin/*; sudo ls -alg /opt/cni/bin/
   sudo ls -alg /etc/cni/net.d/; sudo rm -r /etc/cni/net.d/bridge.conf;sudo rm -r /etc/cni/net.d/10-flannel.conflist sudo ls -alg /etc/cni/net.d/
```

1.1) bootstrap the cluster by starting the master node (no CNI plugin at first)
   Run the following commands when you are first time
```bash
   export ARKTOS_NO_CNI_PREINSTALLED=y
   make clean
   ./hack/arktos-up.sh
```

   After you first run './hack/arktos-up.sh' successfully, you can use './hack/arktos-up.sh -O' intead
```bash
   export ARKTOS_NO_CNI_PREINSTALLED=y
   ./hack/arktos-up.sh -O
```

   Note: arktos-up.sh should be stuck in "Waiting for node ready at api server" messages. Don't worry, the apiserver is already up at this point, just the master node status is not "Ready"  and pod/kube-dns in name space 'kube-system' is not in state of 'Running' as we have not installed the network plugin yet. 

1.2) Open another terminal to the master node to install CNI plugin of flannel and check the status of node
```bash
   ./cluster/kubectl.sh apply -f https://github.com/coreos/flannel/raw/master/Documentation/kube-flannel.yml
```

   After that, "arktos-up.sh" should get rid of "Waiting for node ready at api server" messages and be successfully started. You can check the status of all resources including pod/kube-dns and flannel network

```bash
   ./cluster/kubectl.sh get all --all-namespaces
   ./cluster/kubectl.sh get pods --all-namespaces
   ps -ef |grep falnel |grep -v grep
   sudo cat /etc/cni/net.d/10-flannel.conflist
   cat /tmp/flanneld.log
   cat /run/flannel/subnet.env

   ifconfig -a
   ip route
```
```bash
   ./cluster/kubectl.sh get nodes
```

   Note: Please reference the blog [Kubernetes: Flannel networking](https://blog.laputa.io/kubernetes-flannel-networking-6a1cb1f8ec7c) if you want to know how flannel network works in kubernetes.

   Note: CNI plugin of Calico is not supported because the resource 'EndpointSlices' is not supported by Arktos so far.


1.3) On worker node: ensure following worker secret files copied from the master node

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


On worker node: start the worker node to register into cluster

1.4) First make sure the following directories in the worker node are empty. If not, clean them up using sudopermission.
```bash
   sudo ls -alg /opt/cni/bin/; sudo rm -r /opt/cni/bin/*; sudo ls -alg /opt/cni/bin/
   sudo ls -alg /etc/cni/net.d/; sudo rm -r /etc/cni/net.d/bridge.conf;sudo rm -r /etc/cni/net.d/10-flannel.conflist sudo ls -alg /etc/cni/net.d/
```

1.5) Start the worker node and register into cluster
   Run the following commands when you are first time
```bash
   export ARKTOS_NO_CNI_PREINSTALLED=y
   export KUBELET_IP=`hostname -i`;echo $KUBELET_IP
   make clean
   ./hack/arktos-worker-up.sh
```
OR
   After you first run './hack/arktos-up.sh' successfully, you can use './hack/arktos-up.sh -O' intead
```bash
   export ARKTOS_NO_CNI_PREINSTALLED=y
   export KUBELET_IP=`hostname -i`;echo $KUBELET_IP
   ./hack/arktos-worker-up.sh -O
```

On the master node:

1.6) Check status of nodes

    You should see the work node is displayed and its status should be "Ready". But probably you will see the worker node is in "NotReady" state.

```bash
   ./cluster/kubectl.sh get nodes
```
```bash
   NAME               STATUS     ROLES    AGE     VERSION
   ip-172-31-22-85    Ready      <none>   13m     v0.9.0
   ip-172-31-29-128   NotReady   <none>   3m39s   v0.9.0
```

   Check the kubelet log on worker node:
```bash
   grep ip-172-31-29-128 /tmp/kubelet.worker.log |tail -2
```
```bash
   E1116 05:30:12.750708   17517 controller.go:129] failed to ensure node lease exists, will retry in 7s, error: leases.coordination.k8s.io "ip-172-31-29-128" is forbidden: User "system:node:ip-172-31-22-85" cannot get resource "leases" in API group "coordination.k8s.io" in the namespace "kube-node-lease": can only access node lease with the same name as the requesting node
   E1116 05:30:19.752760   17517 controller.go:129] failed to ensure node lease exists, will retry in 7s, error: leases.coordination.k8s.io "ip-172-31-29-128" is forbidden: User "system:node:ip-172-31-22-85" cannot get resource "leases" in API group "coordination.k8s.io" in the namespace "kube-node-lease": can only access node lease with the same name as the requesting node
```

1.7) Run the following command to create clusterrolebinding 'system-node-role-bound' to bind the group 'system:nodes' to clusterrole 'system:node' so that the master node has corresponding permission to get resource "leases" in API group "coordination.k8s.io" in the namespace "kube-node-lease"

```bash
   ./cluster/kubectl.sh create clusterrolebinding system-node-role-bound --clusterrole=system:node --group=system:nodes
   ./cluster/kubectl.sh get clusterrolebinding/system-node-role-bound -o yaml
```

```bash
   ./cluster/kubectl.sh get nodes
```

```bash
   NAME               STATUS   ROLES    AGE    VERSION
   ip-172-31-22-85    Ready    <none>   18m    v0.9.0
   ip-172-31-29-128   Ready    <none>   8m1s   v0.9.0
```

1.8) Check the status of flannel pod running on worker node and fix the issue

```bash
   ./cluster/kubectl.sh get pods --all-namespaces -o wide
```
```bash
   NAMESPACE     NAME                               HASHKEY               READY   STATUS    RESTARTS   AGE     IP              NODE               NOMINATED NODE   READINESS GATES
   kube-system   coredns-default-6c8b75ff85-ffxk8   2098388192664718022   1/1     Running   0          19m     10.244.0.65     ip-172-31-22-85    <none>           <none>
   kube-system   kube-dns-554c5866fc-m9jgx          2830239144273075720   3/3     Running   0          19m     10.244.0.66     ip-172-31-22-85    <none>           <none>
   kube-system   kube-flannel-ds-fwxln              5019221519058267550   0/1     Error     6          9m24s   172.31.29.128   ip-172-31-29-128   <none>           <none>
   kube-system   kube-flannel-ds-nj9lr              5649932688255010940   1/1     Running   0          14m     172.31.22.85    ip-172-31-22-85    <none>           <none>
   kube-system   virtlet-drnzl                      7477111360539274047   3/3     Running   0          13m     172.31.22.85    ip-172-31-22-85    <none>           <none>
   kube-system   virtlet-g5x6k                      6083367271442737085   3/3     Running   0          9m13s   172.31.29.128   ip-172-31-29-128   <none>           <none>
```

   Check the log of pod 'kube-flannel-ds-fwxln' on worker node:
```bash
   sudo cat /var/log/pods/system_kube-system_kube-flannel-ds-fwxln_697ad8fe-9f60-4641-bd07-8b65397ba7f8/kube-flannel/5.log
```

```bash
   2021-11-16T05:31:53.503829615Z stderr F I1116 05:31:53.503552       1 main.go:217] CLI flags config: {etcdEndpoints:http://127.0.0.1:4001,http://127.0.0.1:2379 etcdPrefix:/coreos.com/network etcdKeyfile: etcdCertfile: etcdCAFile: etcdUsername: etcdPassword: help:false version:false autoDetectIPv4:false autoDetectIPv6:false kubeSubnetMgr:true kubeApiUrl: kubeAnnotationPrefix:flannel.alpha.coreos.com kubeConfigFile: iface:[] ifaceRegex:[] ipMasq:true subnetFile:/run/flannel/subnet.env subnetDir: publicIP: publicIPv6: subnetLeaseRenewMargin:60 healthzIP:0.0.0.0 healthzPort:0 charonExecutablePath: charonViciUri: iptablesResyncSeconds:5 iptablesForwardRules:true netConfPath:/etc/kube-flannel/net-conf.json setNodeNetworkUnavailable:true}
   2021-11-16T05:31:53.50389087Z stderr F W1116 05:31:53.503668       1 client_config.go:608] Neither --kubeconfig nor --master was specified.  Using the inClusterConfig.  This might not work.
   2021-11-16T05:32:23.506033195Z stderr F E1116 05:32:23.505730       1 main.go:234] Failed to create SubnetManager: error retrieving pod spec for 'kube-system/kube-flannel-ds-fwxln': Get "https://10.0.0.1:443/api/v1/namespaces/kube-system/pods/kube-flannel-ds-fwxln": dial tcp 10.0.0.1:443: i/o timeout
```

1.9) On worker node, add one more iptable rule
As a temporary measure to accommodate Flannel pods to access api server through the kubernetes service IP (the notorious 10.0.0.1) at the absense of kube-proxy, we need to place a bit trick by running below command at the worker node, assuming the kupe-apiserver is listening at https://172.31.22.85:6443 (please substitute with the proper value of your own cluster)

```bash
   sudo iptables -t nat -A OUTPUT -p tcp -d 10.0.0.1 --dport 443 -j DNAT --to-destination 172.31.22.85:6443
```

1.10) On master node, delete old pod of flannel running on worker node
```bash
   cluster/kubectl.sh delete pod/kube-flannel-ds-fwxln -n kube-system
```
```bash
   cluster/kubectl.sh get pods --all-namespaces -o wide
```

```bash
   NAMESPACE     NAME                               HASHKEY               READY   STATUS    RESTARTS   AGE   IP              NODE               NOMINATED NODE   READINESS GATES
   kube-system   coredns-default-6c8b75ff85-ffxk8   2098388192664718022   1/1     Running   0          22m   10.244.0.65     ip-172-31-22-85    <none>           <none>
   kube-system   kube-dns-554c5866fc-m9jgx          2830239144273075720   3/3     Running   0          22m   10.244.0.66     ip-172-31-22-85    <none>           <none>
   kube-system   kube-flannel-ds-fg2fl              7051182752843277513   1/1     Running   0          3s    172.31.29.128   ip-172-31-29-128   <none>           <none>
   kube-system   kube-flannel-ds-nj9lr              5649932688255010940   1/1     Running   0          17m   172.31.22.85    ip-172-31-22-85    <none>           <none>
   kube-system   virtlet-drnzl                      7477111360539274047   3/3     Running   0          17m   172.31.22.85    ip-172-31-22-85    <none>           <none>
   kube-system   virtlet-g5x6k                      6083367271442737085   3/3     Running   0          12m   172.31.29.128   ip-172-31-29-128   <none>           <none>
```

   Check the log of pod - kube-flannel-ds-fg2fl running on worker node
```bash
   cluster/kubectl.sh logs pod/kube-flannel-ds-fg2fl -n kube-system
```
```bash
I1116 05:39:20.311817       1 main.go:217] CLI flags config: {etcdEndpoints:http://127.0.0.1:4001,http://127.0.0.1:2379 etcdPrefix:/coreos.com/network etcdKeyfile: etcdCertfile: etcdCAFile: etcdUsername: etcdPassword: help:false version:false autoDetectIPv4:false autoDetectIPv6:false kubeSubnetMgr:true kubeApiUrl: kubeAnnotationPrefix:flannel.alpha.coreos.com kubeConfigFile: iface:[] ifaceRegex:[] ipMasq:true subnetFile:/run/flannel/subnet.env subnetDir: publicIP: publicIPv6: subnetLeaseRenewMargin:60 healthzIP:0.0.0.0 healthzPort:0 charonExecutablePath: charonViciUri: iptablesResyncSeconds:5 iptablesForwardRules:true netConfPath:/etc/kube-flannel/net-conf.json setNodeNetworkUnavailable:true}
W1116 05:39:20.311943       1 client_config.go:608] Neither --kubeconfig nor --master was specified.  Using the inClusterConfig.  This might not work.
I1116 05:39:20.603991       1 kube.go:120] Waiting 10m0s for node controller to sync
I1116 05:39:20.604071       1 kube.go:378] Starting kube subnet manager
I1116 05:39:21.604353       1 kube.go:127] Node controller sync successful
I1116 05:39:21.604401       1 main.go:237] Created subnet manager: Kubernetes Subnet Manager - ip-172-31-29-128
I1116 05:39:21.604408       1 main.go:240] Installing signal handlers
I1116 05:39:21.605025       1 main.go:459] Found network config - Backend type: vxlan
I1116 05:39:21.605652       1 main.go:651] Determining IP address of default interface
I1116 05:39:21.606171       1 main.go:698] Using interface with name eth0 and address 172.31.29.128
I1116 05:39:21.606201       1 main.go:720] Defaulting external address to interface address (172.31.29.128)
I1116 05:39:21.606208       1 main.go:733] Defaulting external v6 address to interface address (<nil>)
I1116 05:39:21.606309       1 vxlan.go:137] VXLAN config: VNI=1 Port=0 GBP=false Learning=false DirectRouting=false
I1116 05:39:21.703010       1 kube.go:339] Setting NodeNetworkUnavailable
I1116 05:39:21.707423       1 main.go:408] Current network or subnet (10.244.0.0/16, 10.244.1.0/24) is not equal to previous one (0.0.0.0/0, 0.0.0.0/0), trying to recycle old iptables rules
I1116 05:39:21.919287       1 iptables.go:240] Deleting iptables rule: -s 0.0.0.0/0 -d 0.0.0.0/0 -j RETURN
I1116 05:39:22.001812       1 iptables.go:240] Deleting iptables rule: -s 0.0.0.0/0 ! -d 224.0.0.0/4 -j MASQUERADE --random-fully
I1116 05:39:22.003069       1 iptables.go:240] Deleting iptables rule: ! -s 0.0.0.0/0 -d 0.0.0.0/0 -j RETURN
I1116 05:39:22.004036       1 iptables.go:240] Deleting iptables rule: ! -s 0.0.0.0/0 -d 0.0.0.0/0 -j MASQUERADE --random-fully
I1116 05:39:22.005067       1 main.go:340] Setting up masking rules
I1116 05:39:22.005853       1 main.go:361] Changing default FORWARD chain policy to ACCEPT
I1116 05:39:22.005971       1 main.go:374] Wrote subnet file to /run/flannel/subnet.env
I1116 05:39:22.005992       1 main.go:378] Running backend.
I1116 05:39:22.006016       1 main.go:396] Waiting for all goroutines to exit
I1116 05:39:22.006045       1 vxlan_network.go:60] watching for new subnet leases
I1116 05:39:22.101236       1 iptables.go:216] Some iptables rules are missing; deleting and recreating rules
I1116 05:39:22.101270       1 iptables.go:240] Deleting iptables rule: -s 10.244.0.0/16 -j ACCEPT
I1116 05:39:22.101576       1 iptables.go:216] Some iptables rules are missing; deleting and recreating rules
I1116 05:39:22.101597       1 iptables.go:240] Deleting iptables rule: -s 10.244.0.0/16 -d 10.244.0.0/16 -j RETURN
I1116 05:39:22.102424       1 iptables.go:240] Deleting iptables rule: -d 10.244.0.0/16 -j ACCEPT
I1116 05:39:22.102637       1 iptables.go:240] Deleting iptables rule: -s 10.244.0.0/16 ! -d 224.0.0.0/4 -j MASQUERADE --random-fully
I1116 05:39:22.103307       1 iptables.go:228] Adding iptables rule: -s 10.244.0.0/16 -j ACCEPT
I1116 05:39:22.103616       1 iptables.go:240] Deleting iptables rule: ! -s 10.244.0.0/16 -d 10.244.1.0/24 -j RETURN
I1116 05:39:22.201201       1 iptables.go:228] Adding iptables rule: -d 10.244.0.0/16 -j ACCEPT
I1116 05:39:22.201243       1 iptables.go:240] Deleting iptables rule: ! -s 10.244.0.0/16 -d 10.244.0.0/16 -j MASQUERADE --random-fully
I1116 05:39:22.202553       1 iptables.go:228] Adding iptables rule: -s 10.244.0.0/16 -d 10.244.0.0/16 -j RETURN
I1116 05:39:22.204499       1 iptables.go:228] Adding iptables rule: -s 10.244.0.0/16 ! -d 224.0.0.0/4 -j MASQUERADE --random-fully
I1116 05:39:22.301498       1 iptables.go:228] Adding iptables rule: ! -s 10.244.0.0/16 -d 10.244.1.0/24 -j RETURN
I1116 05:39:22.303743       1 iptables.go:228] Adding iptables rule: ! -s 10.244.0.0/16 -d 10.244.0.0/16 -j MASQUERADE --random-fully
```

   Check the status of nodes:
```bash
   ./cluster/kubectl.sh get nodes
```

```bash
   NAME               STATUS   ROLES    AGE   VERSION
   ip-172-31-22-85    Ready    <none>   26m   v0.9.0
   ip-172-31-29-128   Ready    <none>   16m   v0.9.0
```

1.11) Start 2nd worker node to join cluster by following up the above steps 1.3) through step 1.10).

   on master node: Check the status of nodes and pods
```bash
   ./cluster/kubectl.sh get nodes
```

```bash
   NAME               STATUS   ROLES    AGE   VERSION
   ip-172-31-22-85    Ready    <none>   32m   v0.9.0
   ip-172-31-24-185   Ready    <none>   79s   v0.9.0
   ip-172-31-29-128   Ready    <none>   22m   v0.9.0
```

```bash
   ./cluster/kubectl.sh get pods --all-namespaces -o wide
```

```bash
   NAMESPACE     NAME                               HASHKEY               READY   STATUS    RESTARTS   AGE     IP              NODE               NOMINATED NODE   READINESS GATES
   kube-system   coredns-default-6c8b75ff85-ffxk8   2098388192664718022   1/1     Running   0          32m     10.244.0.65     ip-172-31-22-85    <none>           <none>
   kube-system   kube-dns-554c5866fc-m9jgx          2830239144273075720   3/3     Running   0          32m     10.244.0.66     ip-172-31-22-85    <none>           <none>
   kube-system   kube-flannel-ds-7h6nv              8115799907707745269   1/1     Running   0          100s    172.31.24.185   ip-172-31-24-185   <none>           <none>
   kube-system   kube-flannel-ds-fg2fl              7051182752843277513   1/1     Running   0          9m34s   172.31.29.128   ip-172-31-29-128   <none>           <none>
   kube-system   kube-flannel-ds-nj9lr              5649932688255010940   1/1     Running   0          27m     172.31.22.85    ip-172-31-22-85    <none>           <none>
   kube-system   virtlet-6drfl                      8480505743384093590   3/3     Running   0          89s     172.31.24.185   ip-172-31-24-185   <none>           <none>
   kube-system   virtlet-drnzl                      7477111360539274047   3/3     Running   0          26m     172.31.22.85    ip-172-31-22-85    <none>           <none>
   kube-system   virtlet-g5x6k                      6083367271442737085   3/3     Running   0          22m     172.31.29.128   ip-172-31-29-128   <none>           <none>
```

1.12) Test whether the ngnix application can be deployed successfully
```bash
   ./cluster/kubectl.sh run nginx --image=nginx --replicas=10
   ./cluster/kubectl.sh get pods -n default -o wide
```

```bash
   NAME                     HASHKEY               READY   STATUS    RESTARTS   AGE   IP            NODE               NOMINATED NODE   READINESS GATES
   nginx-5d79788459-2ssb9   4714815331945249933   1/1     Running   0          9s    10.244.1.24   ip-172-31-29-128   <none>           <none>
   nginx-5d79788459-5d9p7   3245382200846785472   1/1     Running   0          9s    10.244.0.67   ip-172-31-22-85    <none>           <none>
   nginx-5d79788459-8gfrf   4231171286710049709   1/1     Running   0          9s    10.244.1.23   ip-172-31-29-128   <none>           <none>
   nginx-5d79788459-bh2dl   2227188740354401744   1/1     Running   0          9s    10.244.1.25   ip-172-31-29-128   <none>           <none>
   nginx-5d79788459-g4v56   4357703955321943380   1/1     Running   0          9s    10.244.2.3    ip-172-31-24-185   <none>           <none>
   nginx-5d79788459-k74rq   4931046737888471496   1/1     Running   0          9s    10.244.2.5    ip-172-31-24-185   <none>           <none>
   nginx-5d79788459-n7vhx   7564874686242188556   1/1     Running   0          9s    10.244.1.27   ip-172-31-29-128   <none>           <none>
   nginx-5d79788459-r2wlr   3098101036923465124   1/1     Running   0          9s    10.244.2.2    ip-172-31-24-185   <none>           <none>
   nginx-5d79788459-svzgd   2241339277610995766   1/1     Running   0          9s    10.244.2.4    ip-172-31-24-185   <none>           <none>
   nginx-5d79788459-trlw9   1683625407392884361   1/1     Running   0          9s    10.244.1.26   ip-172-31-29-128   <none>           <none>
```

1.13) Test whether nginx pods on three nodes can talk each other using 'curl'
   If you like, you need test connectivities among 6 subnets on 3 nods
```bash
   10.244.1.24 ---> 10.244.0.x(67)
   10.244.1.24 ---> 10.244.2.x (2,3,4,5)
   10.244.0.67 ---> 10.244.1.x (23,24,25,26,27)
   10.244.0.67 --> 10.244.2.x (2,3,4,5)
   10.244.2.3 --> 10.244.0.x(67)
   10.244.2.3 --> 10.244.1.x (23,24,25,26,27)
```

For example:
```bash
   ./cluster/kubectl.sh exec -ti nginx-5d79788459-2ssb9 -- curl 10.244.0.67
```
```bash
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto;
font-family: Tahoma, Verdana, Arial, sans-serif; }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```

Method #2 - Process Mode (to support M TPs X N RPs)

```bash
   To be completed.
```
