# How to Setup a Dev Cluster of multi-node on AWS EC2 instance running Ubuntu 18.04, 20.04 x86 

0. Pre-requisite - setup local development environment with Mizar
0.1) Create AWS EC2 instance 
     - OS: Ubuntu 18.04 and Ubuntu 20.04
     - Instance Type: t2.2xlarge
     - Storage Size: 128HB or more

```
   https://github.com/Click2Cloud-Centaurus/arktos/blob/default-cni-mizar/docs/setup-guide/arktos-with-mizar-cni.md
```

0.2) Follow up the step 1 in above procedure to upgrade kernel to 5.6.0-rc2
```bash
   uname -a
   wget https://raw.githubusercontent.com/CentaurusInfra/mizar/dev-next/kernelupdate.sh
   sudo bash kernelupdate.sh
   uname -a
```  

0.3) Follow up the step 2 in above procedure to clone the Arktos repository and install the required dependencies
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

0.7) Verify whether Mizar CRDs are all in "Provisioned" states
     https://github.com/CentaurusInfra/mizar/wiki/Mizar-Cluster-Health-Criteria

     Note: if Mizar CRDs - vpcs,subnets,droplets,bouncers,dividers and endpoints are not in "Provisioned" states, please reboot machines, then go back to step 0.6).
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
     https://github.com/CentaurusInfra/mizar/wiki/Mizar-Cluster-Health-Criteria
```bash
   vi ~/TMP/netpod-single-node.yaml (copy from netpod-single-node.yaml)
   cat ~/TMP/netpod-single-node.yaml
   ./cluster/kubectl.sh apply -f ~/TMP/netpod-single-node.yaml
   ./cluster/kubectl.sh get pods -o wide
   ./cluster/kubectl.sh exec -ti netpod1 -- ping -c2 20.0.0.18
   ./cluster/kubectl.sh exec -ti netpod2 -- ping -c2 20.0.0.26
   ./cluster/kubectl.sh exec -ti netpod1 -n default -- /bin/bash
   # curl 20.0.0.18:7000
   netpod2
   # curl 20.0.0.26:7000
   netpod1
   # exit
```


2. Follow up the procedure of issue 1142 at https://github.com/CentaurusInfra/arktos/issues/1142 to test - 2.	General pod connectivity: pods in same non-system tenant should be able to communicate;
```bash
```


3. Follow up the procedure of issue 1142 at https://github.com/CentaurusInfra/arktos/issues/1142 to test - 3.	General pod isolation: a pod in one tenant may not communicate with pods in other tenants;
```bash
```

4. Build two node scale-up cluster to test - 4.	Worker node joining: new worker node should be able to join cluster, and basic pod connectivity should be provided.
