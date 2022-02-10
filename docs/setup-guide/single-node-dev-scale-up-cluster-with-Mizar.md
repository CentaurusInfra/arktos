# How to Setup a Dev Cluster of single node on AWS EC2 instance running Ubuntu 18.04, 20.04 x86 

0. Pre-requisite - setup local development environment with Mizar

0.1) Create AWS EC2 instance 
```
     - OS: Ubuntu 18.04 and Ubuntu 20.04
     - Instance Type: t2.2xlarge
     - Storage Size: 128GB or more

```

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
   sudo groupadd docker
   sudo usermode -aG docker $USER
   make clean
   ./hack/arktos-up.sh
```

OR

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

2' Follow the following two documents to test with Mizar team Phu - 2.   General pod connectivity: pods in same non-system tenant should be able to communicate;

```bash
   https://mizar.readthedocs.io/en/latest/user/getting_started/
```

```bash
   https://github.com/CentaurusInfra/mizar/blob/dev-next/docs/design/mp_arktos.md
```

2'.1) Creating a new VPC using yaml file with procedure at https://mizar.readthedocs.io/en/latest/user/getting_started/

``` bash
    cat ~/TMP/file_name_here.vpc-tenant-phu.yaml
```

```
apiVersion: mizar.com/v1
kind: Vpc
metadata:
  name: vpc-tenant-phu
spec:
  ip: "24.0.0.0"
  prefix: "16"
  dividers: 1
  status: "Init"
```

2'.2) Creating a new Subnet using yaml file with procedure at https://mizar.readthedocs.io/en/latest/user/getting_started/

```bash
    cat ~/TMP/file_name_here.subnets-phu.yaml

```

Note: The 9019248 below is CNI number of above VPC 'vpc-tenant-phu'.
```
apiVersion: mizar.com/v1
kind: Subnet
metadata:
  name: net-tenant-phu
spec:
  vni: "9019248"
  ip: "24.0.24.0"
  prefix: "24"
  bouncers: 1
  vpc: "vpc-tenant-phu"
  status: "Init"
```

2'.3) Creating a new tenant 'tenant-phu'

```bash
   ./cluster/kubectl.sh create tenant tenant-phu
   ./cluster/kubectl.sh get tenant
   ./cluster/kubectl.sh get network --tenant tenant-phu
```

2'.4) Creating a new Network using yaml file with procedure at https://github.com/CentaurusInfra/mizar/blob/dev-next/docs/design/mp_arktos.md

```bash
   cat ~/TMP/file_name_here.network-tenant-phu.yaml
```

```
apiVersion: arktos.futurewei.com/v1
kind: Network
metadata:
  name: network-tenant-phu
  tenant: tenant-phu
spec:
  type: mizar
  vpcID: vpc-tenant-phu
```

2'.5) Creating a new Network using yaml file with procedure at https://github.com/CentaurusInfra/mizar/blob/dev-next/docs/design/mp_arktos.md

```bash
   cat ~/TMP/file_name_here.pod-tenant-phu.yaml
```

```
   apiVersion: v1
kind: Pod
metadata:
  name: nginx-tenant-phu
  namespace: default
  tenant: tenant-phu
  labels:
    arktos.futurewei.com/network: network-tenant-phu
spec:
  containers:
  - name: nginx
    image: nginx
    ports:
      - containerPort: 443
```

2'.6) Check the pod of 'nginx-tenant-phu' and see it is in ContainerCreating

```bash
   cluster/kubectl.sh get pod --tenant system |grep nginx-tenant-phu
   cluster/kubectl.sh get pod -AT |grep nginx-tenant-phu
```

2'.7) Check the log of pod mizar-operator

```bash
   ./cluster/kubectl.sh get pods -AT |grep mizar-operator
   ./cluster/kubectl.sh log mizar-operator-6b78d7ffc4-9fdh9
```

```
<snip>
Scheduled 1 tasks of which:
* 1 ran successfully:
    - 1 BouncerProvisioned(param=<mizar.common.wf_param.HandlerParam object at 0x7f20c8235c10>)

This progress looks :) because there were no failed tasks or missing dependencies

===== Luigi Execution Summary =====

[2021-11-04 17:57:01,937] kopf.objects         [INFO    ] [default/net-tenant-phu-b-9fcc6f7c-f5a9-414d-8efc-7b73576e5e9d] Handler 'bouncer_opr_on_bouncer_provisioned' succeeded.
[2021-11-04 17:57:01,938] kopf.objects         [INFO    ] [default/net-tenant-phu-b-9fcc6f7c-f5a9-414d-8efc-7b73576e5e9d] Updating is processed: 1 succeeded; 0 failed.
```

2'.8) Check the log of pod mizar-daemon

```bash
   ./cluster/kubectl.sh get pods -AT |grep mizar-daemon
   ./cluster/kubectl.sh log mizar-daemon-5tnlb
```

```
<snip>
INFO:root:Consuming interfaces for pod: nginx-mytenant-22-default-mytenant Current Queue: []
INFO:root:Deleting interfaces for pod coredns-default-798fbcc5f4-qdkpl-kube-system-mytenant-1 with interfaces []
ERROR:root:Timeout, no new interface to consume! coredns-default-798fbcc5f4-qdkpl-kube-system-mytenant-1 []
INFO:root:Consumed
INFO:root:Deleting interfaces for pod coredns-network-tenant-phu-6776c9c4cb-c7l7c-kube-system-tenant-phu with interfaces []
```


3. Follow up the procedure of issue 1142 at https://github.com/CentaurusInfra/arktos/issues/1142 to test - 3.	General pod isolation: a pod in one tenant may not communicate with pods in other tenants;
```bash
   Stop test when step 2 failed to test
```

4. Build two node scale-up cluster to test - 4.	Worker node joining: new worker node should be able to join cluster, and basic pod connectivity should be provided.
```
   Please see the procedure at https://github.com/q131172019/arktos/blob/CarlXie_singleNodeArktosCluster/docs/setup-guide/multi-node-dev-scale-up-cluster-with-Mizar.md to create new worker node to join cluster
```

5. General pod connectivity: pods in system tenant should be able to communicate in another single-node cluster with Mizar

Note: if you do this test in same cluster as step 2, when you create new VPC for system tenant, ensure the ip in spec part below is not same as 20.0.0.0.

5.1) Creating a new VPC using yaml file with procedure at https://mizar.readthedocs.io/en/latest/user/getting_started/

``` bash
    cat ~/TMP/mizar-system-tenant/vpc.yaml
```

```
apiVersion: mizar.com/v1
kind: Vpc
metadata:
  name: vpc-ying
spec:
  ip: "21.0.0.0"
  prefix: "16"
  dividers: 1
  status: "Init"
```

5.2) Creating a new Subnet using yaml file with procedure at https://mizar.readthedocs.io/en/latest/user/getting_started/

```bash
    cat ~/TMP/mizar-system-tenant/subnet.yaml

```

Note: The 9131671 below is CNI number of above VPC 'vpc-ying'.
```
apiVersion: mizar.com/v1
kind: Subnet
metadata:
  name: net-ying
spec:
  vni: "9131671"
  ip: "21.0.0.0"
  prefix: "24"
  bouncers: 1
  vpc: "vpc-ying"
  status: "Init"
```

5.3) Creating a new Network using yaml file with procedure at https://github.com/CentaurusInfra/mizar/blob/dev-next/docs/design/mp_arktos.md

```bash
   cat ~/TMP/mizar-system-tenant/network.yaml
```

```
apiVersion: arktos.futurewei.com/v1
kind: Network
metadata:
  name: network-ying
spec:
  type: mizar
  vpcID: vpc-ying
```

5.4) Creating a new Netwrk using yaml file with procedure at https://github.com/CentaurusInfra/mizar/blob/dev-next/docs/design/mp_arktos.md

```bash
   cat ~/TMP/mizar-system-tenant/pod.yaml
```

```
cat ~/TMP/mizar-system-tenant/pod.yaml
apiVersion: v1
kind: Pod
metadata:
  name: ying-nginx
  labels:
    arktos.futurewei.com/network: network-ying
spec:
  containers:
  - name: nginx
    image: nginx
    ports:
      - containerPort: 443
```

5.5) Check the pod of 'ying-nginx' and see it is in ContainerCreating

```bash
   cluster/kubectl.sh get pod --tenant system |grep ying-nginx
   cluster/kubectl.sh get pod -AT |grep ying-nginx

```

