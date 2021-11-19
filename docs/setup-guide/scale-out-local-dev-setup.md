# Setting up local dev environment for scale out with flannel in two modes - process mode and daemonset mode

## Scenarios

1. Two Tenant Partitions

1. Two Resource Partitions

1. HA proxy (not required if not using cloud KCM)

## Prerequsite

1. 4 dev box (tested on ubuntu 18.04), 2 for RP, 2 for TPs. Record ip as TP1_IP, TP2_IP, RP1_IP, RP2_IP

1. One dev box for HA proxy, can share with dev boxes used for TP or RP. Record ip as PROXY_IP

## Steps

### Setting up HA proxy
1. Install HA proxy 2.3.0

1. Set up environment variables (no changes have been made for RP2 nor tested)

```
export TENANT_PARTITION_IP=[TP1_IP],[TP2_IP]
export RESOURCE_PARTITION_IP=[RP1_IP]
```

1. Run ./hack/scalability/setup_haproxy.sh (depends on your HA proxy version and environment setup, you might need to comment out some code in the script)

-- Method #1 - Install Flannel in process mode
### Setting up TPs
1. Make sure hack/arktos-up.sh can be run at the box

1. Set up environment variables

```
# optional, used for cloud KCM only but not tested
export SCALE_OUT_PROXY_IP=[PROXY_IP]
export SCALE_OUT_PROXY_PORT=8888
export TENANT_SERVER_NAME=tp-name (e.g. tp1)

# required
export IS_RESOURCE_PARTITION=false
export RESOURCE_SERVER=[RP1_IP]<,[RP2_IP]>
export TENANT_PARTITION_SERVICE_SUBNET=[service-ip-cidr]
```

an examplative allocation for 2 TPs could be

| tp1 | tp2 |
| --- | --- |
| 10.0.0.0/16 | 10.1.0.0/16 |

1. Run ./hack/arktos-up-scale-out-poc.sh

1. Expected last line of output: "Tenant Partition Cluster is Running ..."

Note:

1. As certificates generating and sharing is confusing and time consuming in local test environment. We will use insecure mode for local test for now. Secured mode can be added back later when main goal is acchieved.

### Setting up RPs
1. Make sure hack/arktos-up.sh can be run at the box

1. Set up environment variables

```
export IS_RESOURCE_PARTITION=true
export TENANT_SERVER=[TP1_IP]<,[TP2_IP]>
export RESOURCE_PARTITION_POD_CIDR=[pod-cidr]
```

an examplative allocation of pod cidr for 2 RPs could be

| rp1 | rp2 |
| --- | --- |
| 10.244.0.0/16 | 10.245.0.0/16 |

1. Run ./hack/arktos-up-scale-out-poc.sh

1. Expected last line of output: "Resource Partition Cluster is Running ..."

### Patching Network Routing Across RPs
Depending on your situation, you may need to change instruction properly - the bottom line is pods from one RP should be able to access pods of other RP.

Below is what we did in our test lab using AWS EC2 resources, where RP1/RP2 nodes are in same subnet.
On both RP nodes,
1. stop the source/dest check (on AWS console, using EC2 instance Action menu, choosing Networking | Change source-destination check);
2. manually add relevant routing entries of each node, so that each routing table is complete for all nodes of whole cluster across RPs, e.g. (assuming pod cidr of rp0 is 10.244.0.0/16, rp1 10.245.0.0/16)

on RP1,
```
sudo ip r add 10.245.0.0/24 via [RP2-IP]
```

on RP2
```
sudo ip r add 10.244.0.0/24 via [RP1-IP]
```

-- Method #2 - Install Flannel in daemonset mode (For 1 TP X 1+N RPs is under working)
Issues:
1. ./hack/arktos-up-scale-out-poc.sh should not install flannel in process mode when SCALE_OUT_TP_ENABLE_DAEMONSET=true
   Need bash code change

2. arktos-flannel.deamonset.yaml works for RP1 cluser master node after manual changes and nginx pods can be deployed on RP1 cluster master node
3. two worker nodes join RP1 cluster in "NotReady" state because flannel pods are Pending




### Patching Network Routing Across RPs
Depending on your situation, you may need to change instruction properly - the bottom line is pods from one RP should be able to access pods of other RP.

Below is what we did in our test lab using AWS EC2 resources, where RP1/RP2 nodes are in same subnet.
On both RP nodes,
1. stop the source/dest check (on AWS console, using EC2 instance Action menu, choosing Networking | Change source-destination check);
2. manually add relevant routing entries of each node, so that each routing table is complete for all nodes of whole cluster across RPs, e.g. (assuming pod cidr of rp0 is 10.244.0.0/16, rp1 10.245.0.0/16)

on RP1,
```
sudo ip r add 10.245.0.0/24 via [RP2-IP]
```

on RP2
```
sudo ip r add 10.244.0.0/24 via [RP1-IP]

### Setting up TPs
1. Make sure hack/arktos-up.sh can be run at the box

1. Set up environment variables

```
# optional, used for cloud KCM only but not tested
export SCALE_OUT_PROXY_IP=[PROXY_IP]
export SCALE_OUT_PROXY_PORT=8888
export TENANT_SERVER_NAME=tp-name (e.g. tp1)

# required
export ARKTOS_NO_CNI_PREINSTALLED=y
export SCALE_OUT_TP_ENABLE_DAEMONSET=true

export IS_RESOURCE_PARTITION=false
export RESOURCE_SERVER=[RP1_IP]<,[RP2_IP]>
export TENANT_PARTITION_SERVICE_SUBNET=[service-ip-cidr]
```

an examplative allocation for 2 TPs could be

| tp1 | tp2 |
| --- | --- |
| 10.0.0.0/16 | 10.1.0.0/16 |

1. Run ./hack/arktos-up-scale-out-poc.sh 

1. Expected last line of output: "Tenant Partition Cluster is Running ..."

Note:

1. As certificates generating and sharing is confusing and time consuming in local test environment. We will use insecure mode for local test for now. Secured mode can be added back later when main goal is acchieved.

### Setting up RPs
1. Make sure hack/arktos-up.sh can be run at the box

2. Set up environment variables

```
export ARKTOS_NO_CNI_PREINSTALLED=y
export SCALE_OUT_TP_ENABLE_DAEMONSET=true

export IS_RESOURCE_PARTITION=true
export TENANT_SERVER=[TP1_IP]<,[TP2_IP]>
export RESOURCE_PARTITION_POD_CIDR=[pod-cidr]
```

an examplative allocation of pod cidr for 2 RPs could be

| rp1 | rp2 |
| --- | --- |
| 10.244.0.0/16 | 10.245.0.0/16 |

3. Run ./hack/arktos-up-scale-out-poc.sh (changes: flannel is not installed in process mode if SCALE_OUT_TP_ENABLE_DAEMONSET=true)

   Expected last line of output: "Waiting for node ready" because flannel network plugin is not installed yet

4. On TP1 node, run ./cluster/kubectl.sh apply -f arktos-flannel.deamonset.yaml
```bash
   ./cluster/kubectl.sh apply -f arktos-flannel.deamonset.yaml
   ./cluster/kubectl.sh get ds
   ./cluster/kubectl.sh get pods --all-namespaces
```
5. On RP1 node, you will see expected last line of output: "Resource Partition Cluster is Running ..."
   and check node status
```bash
   ./cluster/kubectl.sh get nodes
```
```bash
   NAME               STATUS   ROLES    AGE     VERSION
   ip-172-31-13-237   Ready    <none>   3m53s   v0.9.0
```
6. Test whether the ngnix application can be deployed successfully
```bash
   ./cluster/kubectl.sh run nginx --image=nginx --replicas=2
   ./cluster/kubectl.sh get pods -n default -o wide
```
```bash
   NAMESPACE     NAME                                              HASHKEY               READY   STATUS    RESTARTS   AGE     IP              NODE               NOMINATED NODE   READINESS GATES
   default       nginx-5d79788459-dwcmx                            8933199855509516061   1/1     Running   0          13s     10.244.0.187    ip-172-31-13-237   <none>           <none>
   default       nginx-5d79788459-h8j5h                            5289368905140319516   1/1     Running   0          13s     10.244.0.186    ip-172-31-13-237   <none>           <none>
   kube-system   coredns-default-ip-172-31-5-56-67d8d65fb8-ntxmz   2333267612604322023   1/1     Running   0          11m     10.244.0.182    ip-172-31-13-237   <none>           <none>
   kube-system   kube-dns-554c5866fc-425fz                         1986526451047783165   3/3     Running   0          11m     10.244.0.181    ip-172-31-13-237   <none>           <none>
   kube-system   kube-flannel-ds-kq7bf                             2755923839726458021   1/1     Running   0          4m48s   172.31.13.237   ip-172-31-13-237   <none>           <none>
   kube-system   virtlet-xwt84                                     7194442764852737610   3/3     Running   0          4m43s   172.31.13.237   ip-172-31-13-237   <none>           <none>
```
   Delete nginx application:
```bash
   ./cluster/kubectl.sh delete deployment/nginx
   ./cluster/kubectl.sh get pods -n default -o wide
```

### Join worker node into RP1 cluster
1. Make sure hack/arktos-up.sh can be run at the box

2. Ensure following worker secret files copied from the master node
```bash
   mkdir -p /tmp/arktos
   scp -i "/home/ubuntu/AWS/keypair/CarlXieKeyPairAccessFromWin.pem" ubuntu@ip-172-31-13-237:/var/run/kubernetes/kubelet.kubeconfig /tmp/arktos/kubelet.kubeconfig
   scp -i "/home/ubuntu/AWS/keypair/CarlXieKeyPairAccessFromWin.pem" ubuntu@ip-172-31-13-237:/var/run/kubernetes/client-ca.crt /tmp/arktos/client-ca.crt
   ls -alg /tmp/arktos
```
```bash
   -rw-r--r-- 1 ubuntu 1310 Nov 19 00:11 client-ca.crt
   -rw-r--r-- 1 ubuntu  312 Nov 19 00:11 kubelet.kubeconfig
```

3. Clean up the directories - /etc/cni/net.d/ and /opt/cni/bin/ as well as the processes - kubelet and flannel
```bash
   sudo ls -alg /opt/cni/bin/; sudo rm -r /opt/cni/bin/*; sudo ls -alg /opt/cni/bin/

   sudo ls -alg /etc/cni/net.d/
   sudo rm -r /etc/cni/net.d/bridge.conf
   sudo rm -r /etc/cni/net.d/10-flannel.conflist
   sudo ls -alg /etc/cni/net.d/
```
```bash
   sudo kill -9 `ps -ef |grep kubelet |grep -v grep |awk '{print $2}'`
   sudo kill -9 `ps -ef |grep flannel |grep -v grep |awk '{print $2}'`
```

4. Set up environment variables
```bash
   export ARKTOS_NO_CNI_PREINSTALLED=y
   export SCALE_OUT_TP_ENABLE_DAEMONSET=true
   export KUBELET_IP=`hostname -i`; echo $KUBELET_IP
```

5. Start worker node to join RP1 cluster
```bash
   ./hack/arktos-worker-up.sh
```
Note: here we try to join two worker nodes into RP1 clister

6. On RP1 node, check node status
```bash
   ./cluster/kubectl.sh get nodes
```
```bash
   NAME               STATUS     ROLES    AGE   VERSION
   ip-172-31-13-237   Ready      <none>   96m   v0.9.0
   ip-172-31-26-244   NotReady   <none>   77m   v0.9.0
   ip-172-31-29-26    NotReady   <none>   85m   v0.9.0
```

7. On TP1 node, check the status of flannel pods running on worker nodes
```bash
   ./cluster/kubectl.sh get pods --all-namespaces -o wide
```
```bash
   NAMESPACE     NAME                                              HASHKEY               READY   STATUS    RESTARTS   AGE   IP              NODE               NOMINATED NODE   READINESS GATES
   kube-system   coredns-default-ip-172-31-5-56-67d8d65fb8-ntxmz   2333267612604322023   1/1     Running   0          97m   10.244.0.182    ip-172-31-13-237   <none>           <none>
   kube-system   kube-dns-554c5866fc-425fz                         1986526451047783165   3/3     Running   0          97m   10.244.0.181    ip-172-31-13-237   <none>           <none>
   kube-system   kube-flannel-ds-4bm9z                             8382328744370754654   0/1     Pending   0          73m   <none>          ip-172-31-26-244   <none>           <none>
   kube-system   kube-flannel-ds-kq7bf                             2755923839726458021   1/1     Running   0          90m   172.31.13.237   ip-172-31-13-237   <none>           <none>
   kube-system   kube-flannel-ds-mp2vr                             389084117387529988    0/1     Pending   0          81m   <none>          ip-172-31-29-26    <none>           <none>
   kube-system   virtlet-xwt84                                     7194442764852737610   3/3     Running   0          90m   172.31.13.237   ip-172-31-13-237   <none>           <none>
```
   Check the flannel pod bound to worker node#1 - ip-172-31-29-26 and this means scheduler works
```bash
   grep kube-flannel-ds-mp2vr /tmp/*.log
```
```bash
/tmp/etcd.log:2021-11-19 00:14:13.769884 D | etcdserver/api/v3rpc: start time = 2021-11-19 00:14:13.769439287 +0000 UTC m=+965.241791675, time spent = 410.69µs, remote = 172.31.5.56:44484, response type = /etcdserverpb.KV/Txn, request count = 1, request size = 4532, response count = 0, response size = 40, request content = compare:<target:MOD key:"/registry/pods/system/kube-system/kube-flannel-ds-mp2vr" mod_revision:0 > success:<request_put:<key:"/registry/pods/system/kube-system/kube-flannel-ds-mp2vr" value_size:4469 >> failure:<>
/tmp/etcd.log:2021-11-19 00:14:13.774461 D | etcdserver/api/v3rpc: start time = 2021-11-19 00:14:13.773581695 +0000 UTC m=+965.245934082, time spent = 838.191µs, remote = 172.31.5.56:44484, response type = /etcdserverpb.KV/Txn, request count = 1, request size = 4699, response count = 0, response size = 40, request content = compare:<target:MOD key:"/registry/pods/system/kube-system/kube-flannel-ds-mp2vr" mod_revision:581 > success:<request_put:<key:"/registry/pods/system/kube-system/kube-flannel-ds-mp2vr" value_size:4636 >> failure:<request_range:<key:"/registry/pods/system/kube-system/kube-flannel-ds-mp2vr" > >
/tmp/etcd.log:2021-11-19 00:14:13.778167 D | etcdserver/api/v3rpc: start time = 2021-11-19 00:14:13.777433639 +0000 UTC m=+965.249785996, time spent = 703.326µs, remote = 172.31.5.56:44600, response type = /etcdserverpb.KV/Txn, request count = 1, request size = 1098, response count = 0, response size = 40, request content = compare:<target:MOD key:"/registry/events/system/kube-system/kube-flannel-ds-mp2vr.16b8ca4306decbd0" mod_revision:0 > success:<request_put:<key:"/registry/events/system/kube-system/kube-flannel-ds-mp2vr.16b8ca4306decbd0" value_size:1006 lease:4007215437127664378 >> failure:<>
/tmp/kube-apiserver0.log:I1119 00:14:13.774921   21594 wrap.go:47] POST /api/v1/tenants/system/namespaces/kube-system/pods/kube-flannel-ds-mp2vr/binding: (2.17936ms) 201 [hyperkube/v0.9.0 (linux/amd64) kubernetes/$Format/scheduler 172.31.5.56:35922]
/tmp/kube-controller-manager.log:I1119 00:14:13.772080   22176 event.go:278] Event(v1.ObjectReference{Kind:"DaemonSet", Namespace:"kube-system", Name:"kube-flannel-ds", UID:"949ae64b-5a5e-4bbd-90e6-4afac60a9664", APIVersion:"apps/v1", ResourceVersion:"422", FieldPath:"", Tenant:"system"}): type: 'Normal' reason: 'SuccessfulCreate' Created pod: kube-flannel-ds-mp2vr
/tmp/kube-controller-manager.log:I1119 00:14:13.775441   22176 vm_controller.go:62] in vm controller, pod kube-flannel-ds-mp2vr is updated
/tmp/kube-scheduler.log:I1119 00:14:13.770969   22179 eventhandlers.go:183] add event for unscheduled pod system/kube-system/kube-flannel-ds-mp2vr
/tmp/kube-scheduler.log:I1119 00:14:13.771835   22179 scheduler.go:576] Attempting to schedule pod: system/kube-system/kube-flannel-ds-mp2vr
/tmp/kube-scheduler.log:I1119 00:14:13.772241   22179 default_binder.go:53] Attempting to bind system/kube-system/kube-flannel-ds-mp2vr to ip-172-31-29-26
/tmp/kube-scheduler.log:I1119 00:14:13.775202   22179 eventhandlers.go:215] delete event for unscheduled pod system/kube-system/kube-flannel-ds-mp2vr
/tmp/kube-scheduler.log:I1119 00:14:13.775217   22179 eventhandlers.go:239] add event for scheduled pod system/kube-system/kube-flannel-ds-mp2vr
/tmp/kube-scheduler.log:I1119 00:14:13.776080   22179 scheduler.go:741] pod system/kube-system/kube-flannel-ds-mp2vr is bound successfully on node "ip-172-31-29-26", 2 nodes evaluated, 1 nodes were found feasible.

```
   Check the flannel pod bound to worker node#1 - ip-172-31-26-244 and this means scheduler works
```bash
   grep kube-flannel-ds-4bm9z /tmp/*.log  
```
```bash
/tmp/etcd.log:2021-11-19 00:22:07.565517 D | etcdserver/api/v3rpc: start time = 2021-11-19 00:22:07.565207744 +0000 UTC m=+1439.037560188, time spent = 270.78µs, remote = 172.31.5.56:44484, response type = /etcdserverpb.KV/Txn, request count = 1, request size = 4534, response count = 0, response size = 40, request content = compare:<target:MOD key:"/registry/pods/system/kube-system/kube-flannel-ds-4bm9z" mod_revision:0 > success:<request_put:<key:"/registry/pods/system/kube-system/kube-flannel-ds-4bm9z" value_size:4471 >> failure:<>
/tmp/etcd.log:2021-11-19 00:22:07.569952 D | etcdserver/api/v3rpc: start time = 2021-11-19 00:22:07.569546404 +0000 UTC m=+1439.041898773, time spent = 364.907µs, remote = 172.31.5.56:44484, response type = /etcdserverpb.KV/Txn, request count = 1, request size = 4702, response count = 0, response size = 40, request content = compare:<target:MOD key:"/registry/pods/system/kube-system/kube-flannel-ds-4bm9z" mod_revision:637 > success:<request_put:<key:"/registry/pods/system/kube-system/kube-flannel-ds-4bm9z" value_size:4639 >> failure:<request_range:<key:"/registry/pods/system/kube-system/kube-flannel-ds-4bm9z" > >
/tmp/etcd.log:2021-11-19 00:22:07.575337 D | etcdserver/api/v3rpc: start time = 2021-11-19 00:22:07.573315982 +0000 UTC m=+1439.045668436, time spent = 1.978649ms, remote = 172.31.5.56:44600, response type = /etcdserverpb.KV/Txn, request count = 1, request size = 1099, response count = 0, response size = 40, request content = compare:<target:MOD key:"/registry/events/system/kube-system/kube-flannel-ds-4bm9z.16b8cab1573a4438" mod_revision:0 > success:<request_put:<key:"/registry/events/system/kube-system/kube-flannel-ds-4bm9z.16b8cab1573a4438" value_size:1007 lease:4007215437127665368 >> failure:<>
/tmp/kube-apiserver0.log:I1119 00:22:07.570424   21594 wrap.go:47] POST /api/v1/tenants/system/namespaces/kube-system/pods/kube-flannel-ds-4bm9z/binding: (1.625024ms) 201 [hyperkube/v0.9.0 (linux/amd64) kubernetes/$Format/scheduler 172.31.5.56:37682]
/tmp/kube-controller-manager.log:I1119 00:22:07.568051   22176 event.go:278] Event(v1.ObjectReference{Kind:"DaemonSet", Namespace:"kube-system", Name:"kube-flannel-ds", UID:"949ae64b-5a5e-4bbd-90e6-4afac60a9664", APIVersion:"apps/v1", ResourceVersion:"584", FieldPath:"", Tenant:"system"}): type: 'Normal' reason: 'SuccessfulCreate' Created pod: kube-flannel-ds-4bm9z
/tmp/kube-controller-manager.log:I1119 00:22:07.571175   22176 vm_controller.go:62] in vm controller, pod kube-flannel-ds-4bm9z is updated
/tmp/kube-scheduler.log:I1119 00:22:07.566906   22179 eventhandlers.go:183] add event for unscheduled pod system/kube-system/kube-flannel-ds-4bm9z
/tmp/kube-scheduler.log:I1119 00:22:07.567769   22179 scheduler.go:576] Attempting to schedule pod: system/kube-system/kube-flannel-ds-4bm9z
/tmp/kube-scheduler.log:I1119 00:22:07.568218   22179 default_binder.go:53] Attempting to bind system/kube-system/kube-flannel-ds-4bm9z to ip-172-31-26-244
/tmp/kube-scheduler.log:I1119 00:22:07.570654   22179 scheduler.go:741] pod system/kube-system/kube-flannel-ds-4bm9z is bound successfully on node "ip-172-31-26-244", 3 nodes evaluated, 1 nodes were found feasible.
/tmp/kube-scheduler.log:I1119 00:22:07.570993   22179 eventhandlers.go:215] delete event for unscheduled pod system/kube-system/kube-flannel-ds-4bm9z
/tmp/kube-scheduler.log:I1119 00:22:07.571023   22179 eventhandlers.go:239] add event for scheduled pod system/kube-system/kube-flannel-ds-4bm9z
```

$8. On worker node #1 - ip-172-31-29-26, check the log of kube-flannel-ds-mp2vr and directory /var/log/pods is blank 
```bash
   ls -al /var/log/pods
```
```bash
total 8
drwxr-xr-x  2 root root   4096 Nov 17 23:48 .
drwxrwxr-x 13 root syslog 4096 Nov 18 06:25 ..
```

9.  On worker node #1 - ip-172-31-26-244, check the log of kube-flannel-ds-4bm9z` and directory /var/log/pods is blank
```bash
   ls -al /var/log/pods
```
```bash
total 8
drwxr-xr-x  2 root root   4096 Nov 18 00:09 .
drwxrwxr-x 13 root syslog 4096 Nov 18 06:25 ..
```


### Test Cluster
1. Use kubectl with kubeconfig. For example:

```
kubectl --kubeconfig /var/run/kubernetes/scheduler.kubeconfig get nodes
```

1. Create pod for system tenant. For example:
```
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
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

1. Check pod is running

```
kubectl --kubeconfig /var/run/kubernetes/scheduler.kubeconfig get pods
```

1. Get ETCD pods in each TP
```
etcdctl get "" --prefix=true --keys-only | grep pods
```

### Note
1. Current change break arktos-up.sh. To verify it works on the host, please use arktos-up.sh on master branch

1. If there is no code changes, can use "./hack/arktos-up-scale-out-poc.sh -O" to save compile time

1. After switched all kubeconfigs from proxy, system tenant appears in both TPs. This is not ideal. Trying to point KCM kubeconfig to HA proxy. 

1. Currently tested with 2TP/2RP.

1. Haven't made changes to HA proxy 2RP, kubectl get nodes only has nodes from first RP, which is expected.

1. Currently local RP started as node tained to be NoSchedule. Need to manually remove the taint so that pod can be scheduled.
```
kubectl --kubeconfig <kubeconfig points to RP api server> taint nodes <node_name> node.kubernetes.io/not-ready:NoSchedule-
``` 
