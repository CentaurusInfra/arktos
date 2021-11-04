# How to Setup a Dev Cluster of multir-node on AWS EC2 instance running Ubuntu 18.04, 20.04 x86 

0. Pre-requisite - setup local development environment with Mizar

0.1) Create AWS EC2 instance 
     - OS: Ubuntu 18.04 and Ubuntu 20.04
     - Instance Type: t2.2xlarge
     - Storage Size: 128HB or more

0.2) Follow up the step 1 in following procedure to upgrade kernel to 5.6.0-rc2

```
   https://github.com/Click2Cloud-Centaurus/arktos/blob/default-cni-mizar/docs/setup-guide/arktos-with-mizar-cni.md
```

```bash
   uname -a
   wget https://raw.githubusercontent.com/CentaurusInfra/mizar/dev-next/kernelupdate.sh
   sudo bash kernelupdate.sh
   uname -a
```  

0.3) Follow up the step 2 in following procedure to clone the Arktos repository and install the required dependencies
```
   https://github.com/Click2Cloud-Centaurus/arktos/blob/default-cni-mizar/docs/setup-guide/arktos-with-mizar-cni.md
```

```bash
   git clone https://github.com/CentaurusInfra/arktos.git ~/go/src/k8s.io/arktos 
   sudo bash $HOME/go/src/k8s.io/arktos/hack/setup-dev-node.sh
   echo export PATH=$PATH:/usr/local/go/bin\ >> ~/.profile
   echo cd \$HOME/go/src/k8s.io/arktos >> ~/.profile
   source ~/.profile
```

0.4) Start Arktos cluster in default mode (without mizar)
```bash
   sudo chmod o+rw /var/run/docker.sock; ls -al /var/run/docker.sock
   make clean
   ./hack/arktos-up.sh
```
   In another window:
```bash
   ./hack/arktos-up.sh get all --all-namespaces
   ./hack/arktos-up.sh get all --all-namespaces -AT
   ./cluster/kubectl.sh run nginx --image=nginx --replicas=2
   ./cluster/kubectl.sh get pods -o wide
   ./cluster/kubectl.sh exec -ti nginx-5d79788459-gz74w -n default -- /bin/bash
   # curl 10.88.0.5  (You will see "Welcome to nginx!" title)
   # curl 10.88.0.4  (You will see "Welcome to nginx!" title) 
   # curl 10.88.0.3  (You will see error "# curl 10.88.0.4  (You will see "Welcome to nginx!" title)")
```



0.5) Apply PR1114 to give "Support for Mizar CNI in arktos-up"
     PR1114: https://github.com/CentaurusInfra/arktos/pull/1114

```bash
   cd ~/go/src/k8s.io/arktos
   git checkout master
   git fetch origin pull/1114/head:pr1114
   git checkout -b CarlXie_20211101-Mizar
   git rebase pr1114
   git log
   git show c4a0ff73ced143fc954c6e34670dd10780f1eb5a
```

0.6) Start Arktos cluster with Mizar
     Note: You will see the warning "Waiting for node ready at api server" for 5 minutes more because it takes time for Mizar to compile codes and Arktos cluster will be up

```bash
   sudo chmod o+rw /var/run/docker.sock; ls -al /var/run/docker.sock  (if reboot machine)
   sudo rm -rf /opt/cni/bin/*; sudo ls -alg /opt/cni/bin/  (keep clean)
   sudo rm -rf /etc/cni/net.d/*; sudo ls -alg /etc/cni/net.d/  (keep clean)
   make clean
   CNIPLUGIN=mizar ./hack/arktos-up.sh
```

0.7) Follow up the following procedure to verify whether Mizar CRDs (vpcs,subnets,droplets,bouncers,dividers and endpoints) are all in "Provisioned" states. If not, reboot machine and go back to above step 0.6) until all Mizar CRDs are in "Provisioned" states

```
     https://github.com/CentaurusInfra/mizar/wiki/Mizar-Cluster-Health-Criteria
```

```bash
   ./cluster/kubectl.sh get crds
```
```
    NAME                            AGE
    bouncers.mizar.com              10m
    dividers.mizar.com              10m
    droplets.mizar.com              10m
    endpoints.mizar.com             10m
    networks.arktos.futurewei.com   10m
    subnets.mizar.com               10m
    vpcs.mizar.com                  10m
```
```bash
   ./cluster/kubectl.sh get crds
   ./cluster/kubectl.sh get vpcs
   ./cluster/kubectl.sh get subnets
   ./cluster/kubectl.sh get droplets
   ./cluster/kubectl.sh get bouncers
   ./cluster/kubectl.sh get dividers
```
```bash
   ./cluster/kubectl.sh get endpoints.mizar.com
   ./cluster/kubectl.sh get networks
```

0.8) Verify whether the pods of Mizar and Mizar Operator are all in "Running" states
```bash
   ./cluster/kubectl.sh get pods -o wide
   ./cluster/kubectl.sh get pods -o wide -AT
```

1. Create two network pods and test each other using ping command to test - 1. Basic pod connectivity: pods in “system” tenant should be able to communicate with each other 
```
     https://github.com/CentaurusInfra/mizar/wiki/Mizar-Cluster-Health-Criteria
```

netpod-single-node.yaml
```
apiVersion: v1
kind: Pod
metadata:
  name: netpod1
  labels:
    app: netpod
spec:
  restartPolicy: OnFailure
  terminationGracePeriodSeconds: 10
  containers:
  - name: netctr
    image: mizarnet/testpod
    ports:
    - containerPort: 9001
      protocol: TCP
    - containerPort: 5001
      protocol: UDP
    - containerPort: 7000
      protocol: TCP
---
apiVersion: v1
kind: Pod
metadata:
  name: netpod2
  labels:
    app: netpod
spec:
  restartPolicy: OnFailure
  terminationGracePeriodSeconds: 10
  containers:
  - name: netctr
    image: mizarnet/testpod
    ports:
    - containerPort: 9001
      protocol: TCP
    - containerPort: 5001
      protocol: UDP
    - containerPort: 7000
      protocol: TCP

```
```bash
   vi ~/TMP/netpod-single-node.yaml (copy from netpod-single-node.yaml)
   cat ~/TMP/netpod-single-node.yaml
   ./cluster/kubectl.sh apply -f ~/TMP/netpod-single-node.yaml
   ./cluster/kubectl.sh get pods -o wide
   ./cluster/kubectl.sh exec -ti netpod1 -- ping -c2 20.0.0.18 (IP of pod netpod2)
   ./cluster/kubectl.sh exec -ti netpod2 -- ping -c2 20.0.0.26 (IP of pod netpod1)
   ./cluster/kubectl.sh exec -ti netpod1 -n default -- /bin/bash
   # curl 20.0.0.18:7000
   netpod2
   # curl 20.0.0.26:7000
   netpod1
   # exit
```


2. Follow up the procedure of issue 1142 at https://github.com/CentaurusInfra/arktos/issues/1142 to test - 2.	General pod connectivity: pods in same non-system tenant should be able to communicate;

2.1) Create new tenant 'mytenant'

```bash
   ./cluster/kubectl.sh create tenant mytenant
```
```
   ./cluster/kubectl.sh get network -T
   ./cluster/kubectl.sh get service --all-namespaces --tenant mytenant -o wide
   ./cluster/kubectl.sh get pod --all-namespaces --tenant mytenant -o wide
   ./cluster/kubectl.sh get deployment --all-namespaces  --tenant mytenant -o wide
   ./cluster/kubectl.sh get endpoints --all-namespaces --tenant mytenant -o wide

```

2.2) Create the Pod yaml file by adding the tenant 'mytenant' and apply this Pod yaml file
```bash
   cat ~/TMP/netpod-arktos-team-single-node.yaml
```
```
apiVersion: v1
kind: Pod
metadata:
  name: my-netpod1
  labels:
    app: my-netpod
  tenant: mytenant
spec:
  restartPolicy: OnFailure
  terminationGracePeriodSeconds: 10
  containers:
  - name: netctr
    image: mizarnet/testpod
    ports:
    - containerPort: 9001
      protocol: TCP
    - containerPort: 5001
      protocol: UDP
    - containerPort: 7000
      protocol: TCP
---
apiVersion: v1
kind: Pod
metadata:
  name: my-netpod2
  labels:
    app: my-netpod
  tenant: mytenant
spec:
  restartPolicy: OnFailure
  terminationGracePeriodSeconds: 10
  containers:
  - name: netctr
    image: mizarnet/testpod
    ports:
    - containerPort: 9001
      protocol: TCP
    - containerPort: 5001
      protocol: UDP
    - containerPort: 7000
      protocol: TCP
```
```bash
   ./cluster/kubectl.sh apply -f ~/TMP/netpod-arktos-team-single-node.yaml
```

2.3) Check whether new pods are in Running state

```bash
   ./cluster/kubectl.sh get pods -AT
```
```
TENANT     NAMESPACE     NAME                               HASHKEY               READY   STATUS              RESTARTS   AGE
mytenant   default       my-netpod1                         5743084656396193665   0/1     ContainerCreating   0          48s
mytenant   default       my-netpod2                         1296240596527502600   0/1     ContainerCreating   0          48s
mytenant   kube-system   coredns-default-798fbcc5f4-qsrdd   7084949829346585805   0/1     ContainerCreating   0          10m
system     default       mizar-daemon-wfsdx                 3626414391297043142   1/1     Running             0          126m
system     default       mizar-operator-6b78d7ffc4-kv6n9    8147745129439049838   1/1     Running             0          126m
system     default       netpod1                            6285438668331818604   1/1     Running             0          17m
system     default       netpod2                            996610216049115966    1/1     Running             0          17m
system     kube-system   coredns-default-798fbcc5f4-gs92h   4660664115769723653   1/1     Running             0          126m
system     kube-system   kube-dns-554c5866fc-4hx2m          6383051253301223595   3/3     Running             0          126m
system     kube-system   virtlet-6bc92                      1075534439738712156   3/3     Running             0          121m
```

```bash
   ./cluster/kubectl.sh get pods --tenant mytenant
```
```
my-netpod1   5743084656396193665   0/1     ContainerCreating   0          76s
my-netpod2   1296240596527502600   0/1     ContainerCreating   0          76s
```

```bash
   cat /tmp/kubelet.log | grep my-netpod1 |tail -9
```
```
I1101 20:48:06.397537   11072 volume_manager.go:358] Waiting for volumes to attach and mount for pod "my-netpod1_default_mytenant(82f46ddd-2b12-48bd-b3c3-5aaf12265fdb)"
I1101 20:48:06.397690   11072 volume_manager.go:391] All volumes are attached and mounted for pod "my-netpod1_default_mytenant(82f46ddd-2b12-48bd-b3c3-5aaf12265fdb)"
I1101 20:48:06.397708   11072 kuberuntime_manager.go:529] No sandbox for pod "my-netpod1_default_mytenant(82f46ddd-2b12-48bd-b3c3-5aaf12265fdb)" can be found. Need to start a new one
I1101 20:48:06.397723   11072 kuberuntime_manager.go:948] computePodActions got {KillPod:true CreateSandbox:true SandboxID: Attempt:0 NextInitContainerToStart:nil ContainersToStart:[0] ContainersToKill:map[] ContainersToUpdate:map[] ContainersToRestart:[] Hotplugs:{NICsToAttach:[] NICsToDetach:[]}} for pod "my-netpod1_default_mytenant(82f46ddd-2b12-48bd-b3c3-5aaf12265fdb)"
I1101 20:48:06.398626   11072 event.go:278] Event(v1.ObjectReference{Kind:"Pod", Namespace:"default", Name:"my-netpod1", UID:"82f46ddd-2b12-48bd-b3c3-5aaf12265fdb", APIVersion:"v1", ResourceVersion:"2948", FieldPath:"", Tenant:"mytenant"}): type: 'Warning' reason: 'GettingClusterDNS' pod: "my-netpod1_default_mytenant(82f46ddd-2b12-48bd-b3c3-5aaf12265fdb)". For verification - ClusterDNS IP : "10.0.0.94"
E1101 20:48:09.424185   11072 kuberuntime_sandbox.go:86] CreatePodSandbox for pod "my-netpod1_default_mytenant(82f46ddd-2b12-48bd-b3c3-5aaf12265fdb)" failed: rpc error: code = Unknown desc = failed to setup network for sandbox "7292679bedba779c3e93d22be35ed24bb71d6fdc9decd35810a23424b6548423": rpc error: code = DeadlineExceeded desc = Deadline Exceeded
E1101 20:48:09.424241   11072 kuberuntime_manager.go:1024] createPodSandbox for pod "my-netpod1_default_mytenant(82f46ddd-2b12-48bd-b3c3-5aaf12265fdb)" failed: rpc error: code = Unknown desc = failed to setup network for sandbox "7292679bedba779c3e93d22be35ed24bb71d6fdc9decd35810a23424b6548423": rpc error: code = DeadlineExceeded desc = Deadline Exceeded
E1101 20:48:09.424310   11072 pod_workers.go:196] Error syncing pod 82f46ddd-2b12-48bd-b3c3-5aaf12265fdb ("my-netpod1_default_mytenant(82f46ddd-2b12-48bd-b3c3-5aaf12265fdb)"), skipping: failed to "CreatePodSandbox" for "my-netpod1_default_mytenant(82f46ddd-2b12-48bd-b3c3-5aaf12265fdb)" with CreatePodSandboxError: "CreatePodSandbox for pod \"my-netpod1_default_mytenant(82f46ddd-2b12-48bd-b3c3-5aaf12265fdb)\" failed: rpc error: code = Unknown desc = failed to setup network for sandbox \"7292679bedba779c3e93d22be35ed24bb71d6fdc9decd35810a23424b6548423\": rpc error: code = DeadlineExceeded desc = Deadline Exceeded"
I1101 20:48:09.424478   11072 event.go:278] Event(v1.ObjectReference{Kind:"Pod", Namespace:"default", Name:"my-netpod1", UID:"82f46ddd-2b12-48bd-b3c3-5aaf12265fdb", APIVersion:"v1", ResourceVersion:"2948", FieldPath:"", Tenant:"mytenant"}): type: 'Warning' reason: 'FailedCreatePodSandBox' Failed create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "7292679bedba779c3e93d22be35ed24bb71d6fdc9decd35810a23424b6548423": rpc error: code = DeadlineExceeded desc = Deadline Exceeded
```


3. Follow up the procedure of issue 1142 at https://github.com/CentaurusInfra/arktos/issues/1142 to test - 3.	General pod isolation: a pod in one tenant may not communicate with pods in other tenants;

```bash
   Stop test when step 2 failed to test
```

4. Build two node scale-up cluster to test - 4.	Worker node joining: new worker node should be able to join cluster, and basic pod connectivity should be provided.

4.1) Suppose you have a simple node cluster with Mizar and verify successfully using the proecedure at https://github.com/CentaurusInfra/mizar/wiki/Mizar-Cluster-Health-Criteria

4.2) Create a new worker node by following up the step 1, 2 at the procedure at https://github.com/Click2Cloud-Centaurus/arktos/blob/default-cni-mizar/docs/setup-guide/arktos-with-mizar-cni.md

4.3) Add security group rules on both nodes to ensure all traffic can go through each other

4.4) Follow up the step 3, 4 at the procedure at https://github.com/q131172019/arktos/blob/CarlXie_singleNodeArktosCluster/docs/setup-guide/multi-node-dev-scale-up-cluster.md to let work node join cluster 


4.5) Check the status of two nodes
```bash
./cluster/kubectl.sh get nodes
```
```
NAME               STATUS   ROLES    AGE     VERSION
ip-172-31-28-132   Ready    <none>   59m     v0.9.0
ip-172-31-28-9     Ready    <none>   5m37s   v0.9.0
```

4.6)  Deploy Nginx application
```bash
./cluster/kubectl.sh run nginx --image=nginx --replicas=10
```
4.7) Check the status of Nginx pods
```bash
./cluster/kubectl.sh get pods -o wide
```
```
NAME                              HASHKEY               READY   STATUS              RESTARTS   AGE     IP              NODE               NOMINATED NODE   READINESS GATES
mizar-daemon-jm68c                8489468700754586427   1/1     Running             1          6m22s   172.31.28.9     ip-172-31-28-9     <none>           <none>
mizar-daemon-qth7w                9137939745832437098   1/1     Running             0          59m     172.31.28.132   ip-172-31-28-132   <none>           <none>
mizar-operator-6b78d7ffc4-fm4pp   3559156546840379852   1/1     Running             0          59m     172.31.28.132   ip-172-31-28-132   <none>           <none>
netpod1                           6169807998527740827   1/1     Running             0          56m     20.0.0.41       ip-172-31-28-132   <none>           <none>
netpod2                           8273861328765425857   1/1     Running             0          56m     20.0.0.18       ip-172-31-28-132   <none>           <none>
nginx-5d79788459-48l6q            1654048144042850706   0/1     ContainerCreating   0          10s     <none>          ip-172-31-28-9     <none>           <none>
nginx-5d79788459-5g7w5            7238259642648495854   1/1     Running             0          10s     20.0.0.37       ip-172-31-28-132   <none>           <none>
nginx-5d79788459-5lrp9            2544086530344874677   0/1     ContainerCreating   0          10s     <none>          ip-172-31-28-9     <none>           <none>
nginx-5d79788459-bvqmt            8343757557393359410   1/1     Running             0          10s     20.0.0.45       ip-172-31-28-132   <none>           <none>
nginx-5d79788459-fqhhx            3882393408941073858   0/1     ContainerCreating   0          10s     <none>          ip-172-31-28-9     <none>           <none>
nginx-5d79788459-l46z5            5947515209238197563   0/1     ContainerCreating   0          10s     <none>          ip-172-31-28-9     <none>           <none>
nginx-5d79788459-l9m77            4466493257680056631   0/1     ContainerCreating   0          10s     <none>          ip-172-31-28-9     <none>           <none>
nginx-5d79788459-n6v7x            6802894149274701769   0/1     ContainerCreating   0          10s     <none>          ip-172-31-28-9     <none>           <none>
nginx-5d79788459-spvfs            1493173479578877424   0/1     ContainerCreating   0          10s     <none>          ip-172-31-28-9     <none>           <none>
nginx-5d79788459-zf92h            7575485586534491466   0/1     ContainerCreating   0          10s     <none>          ip-172-31-28-9     <none>           <none>
ubuntu@ip-172-31-28-132:~/go/src/k8s.io/arktos$
```
4.8) Check the error of containerd
```bash
journalctl -u containerd
```
```
Nov 01 22:31:19 ip-172-31-28-9 containerd[2324]: time="2021-11-01T22:31:19.318504821Z" level=info msg="No cni config template is specified, wait for other system components to drop the config."
Nov 01 22:31:19 ip-172-31-28-9 containerd[2324]: time="2021-11-01T22:31:19.419328912Z" level=info msg="No cni config template is specified, wait for other system components to drop the config."
Nov 01 22:31:19 ip-172-31-28-9 containerd[2324]: time="2021-11-01T22:31:19.520129201Z" level=info msg="No cni config template is specified, wait for other system components to drop the config."
Nov 01 22:31:19 ip-172-31-28-9 containerd[2324]: time="2021-11-01T22:31:19.721247645Z" level=info msg="No cni config template is specified, wait for other system components to drop the config."
Nov 01 22:31:19 ip-172-31-28-9 containerd[2324]: time="2021-11-01T22:31:19.821904690Z" level=info msg="No cni config template is specified, wait for other system components to drop the config."
Nov 01 22:31:19 ip-172-31-28-9 containerd[2324]: time="2021-11-01T22:31:19.922523863Z" level=info msg="No cni config template is specified, wait for other system components to drop the config."
Nov 01 22:31:20 ip-172-31-28-9 containerd[2324]: time="2021-11-01T22:31:20.023186444Z" level=info msg="No cni config template is specified, wait for other system components to drop the config."
Nov 01 22:31:20 ip-172-31-28-9 containerd[2324]: time="2021-11-01T22:31:20.123857308Z" level=info msg="No cni config template is specified, wait for other system components to drop the config."
```

4.9) Check the error in kubelet log at /tmp/kubelet.log
```bash
grep nginx-5d79788459-48l6q /tmp/kubelet.worker.log |tail -3
```
```
E1103 23:01:50.925691    3384 kuberuntime_manager.go:1024] createPodSandbox for pod "nginx-5d79788459-48l6q_default_system(9fef41f2-b9a0-4203-a26c-a3f28a5df19f)" failed: rpc error: code = Unknown desc = failed to setup network for sandbox "b81277168b31e4d5bae52855bff3a1e7e97fa35130db727dfbf8b3c8c833bd1b": rpc error: code = DeadlineExceeded desc = Deadline Exceeded
E1103 23:01:50.925771    3384 pod_workers.go:196] Error syncing pod 9fef41f2-b9a0-4203-a26c-a3f28a5df19f ("nginx-5d79788459-48l6q_default_system(9fef41f2-b9a0-4203-a26c-a3f28a5df19f)"), skipping: failed to "CreatePodSandbox" for "nginx-5d79788459-48l6q_default_system(9fef41f2-b9a0-4203-a26c-a3f28a5df19f)" with CreatePodSandboxError: "CreatePodSandbox for pod \"nginx-5d79788459-48l6q_default_system(9fef41f2-b9a0-4203-a26c-a3f28a5df19f)\" failed: rpc error: code = Unknown desc = failed to setup network for sandbox \"b81277168b31e4d5bae52855bff3a1e7e97fa35130db727dfbf8b3c8c833bd1b\": rpc error: code = DeadlineExceeded desc = Deadline Exceeded"
I1103 23:01:50.925803    3384 event.go:278] Event(v1.ObjectReference{Kind:"Pod", Namespace:"default", Name:"nginx-5d79788459-48l6q", UID:"9fef41f2-b9a0-4203-a26c-a3f28a5df19f", APIVersion:"v1", ResourceVersion:"1927", FieldPath:"", Tenant:"system"}): type: 'Warning' reason: 'FailedCreatePodSandBox' Failed create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "b81277168b31e4d5bae52855bff3a1e7e97fa35130db727dfbf8b3c8c833bd1b": rpc error: code = DeadlineExceeded desc = Deadline Exceeded
```
