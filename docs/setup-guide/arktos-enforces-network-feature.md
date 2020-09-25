# Arktos to Enforce the Multi-tenant Network Feature

This document captures the steps applied to an Arktos cluster lab enabling the unique multi-tenant network feature. The machines in this lab used are AWS EC2 t2-large (2 CPUs, 8GB mem), Ubuntu 18.04 LTS.

The steps might change with the progress of development.

If you would like to try with Flannel cni plugin, please ensure to read [multi-node setup guide](multi-node-dev-cluster.md).

1. Prepare lab machines. Particularly, build arktos-network-controller (as it is not part of arktos-up.sh yet); disable local DNS cache (applicable to 127.0.0.53 name server; as it would cause coreDNS to crash with DNS loopback lookup):
```bash
make all WHAT=cmd/arktos-network-controller
sudo rm -f /etc/resolv.conf
sudo ln -s /run/systemd/resolve/resolv.conf /etc/resolv.conf
```
Also, please ensure the hostname and its ip address in /etc/hosts. For instance, if the hostname is ip-172-31-41-177, ip address is 172.31.41.177:
```text
127.0.0.1 localhost
172.31.41.177 ip-172-31-41-177
```
If this machine will be used as the master of a multi-node cluster, please set adequate permissive security groups. Fro AWS VM in this lab, we allowed inbound rule of ALL-Traffic 0.0.0.0/0.

If mizar cni plugin is to be used, please replace containerd following the instruction of [multi-tenant aware containerd](https://github.com/futurewei-cloud/containerd/releases/tag/tenant-cni-args).

2. Clean up some left-over network plugin and binaries.
Delete all the files under the following directories:
```
/opt/cni/bin/
/etc/cni/net.d/
```
You can skip this step if it is a newly-provisioned lab machine, yet it is recommeneded to check before moving on. 


3. Enable network related feature and change cni installation setting by
```bash
export FEATURE_GATES="AllAlpha=false,MandatoryArktosNetwork=true"
export ARKTOS_NO_CNI_PREINSTALLED=y
```

4. Set the default network template based on the the default network type you desire:
If you want the default network to be of "flat" type:
```bash
export ARKTOS_NETWORK_TEMPLATE=flat
```

Or, If you want the default network to be of "mizar" type:
```bash
export ARKTOS_NETWORK_TEMPLATE=mizar
```

Or, set the env var to a file path of your desire network object template file.

4. Start Arktos cluster
```bash
./hack/arktos-up.sh
```

Note: arktos-up.sh should be stuck in "Waiting for node ready at api server" messages. Don't worry, the apiserver is already up at this point, just the master node status is not "Ready" as we have not installed the network plugin yet. 

5. Leave the "arktos-up.sh" terminal and opend a another terminal to the master node. Run the following command to confirm that the first network, "default", in system tenant, has already been created. Its state is empty at this moment.
```bash
./cluster/kubectl.sh get net
NAME      TYPE   VPC   PHASE   DNS
default   flat
```

The output will be as follows if you set ARKTOS_NETWORK_TEMPLATE=mizar.
```bash
NAME      TYPE    VPC                      PHASE   DNS
default   mizar   system-default-network    
```

6. Start the arktos-network-controller

The current version has a limitation which requires cluster admin to specify the IP address of kube-apiserver that this controller connects to. We are working on the improvement that gets rid of this inconvenience. At this monent, please provide the ip address of the master node. Below is assuming 172.31.41.177:
```bash
./_output/local/bin/linux/amd64/arktos-network-controller --kubeconfig=~/.kube/config --kube-apiserver-ip=172.31.41.177
```
The config file has below content
```yaml
apiVersion: v1
clusters:
- cluster:
    server: http://127.0.0.1:8080
  name: local
contexts:
- context:
    cluster: local
    user: ""
  name: local-ctx
current-context: local-ctx
kind: Config
preferences: {}
users: []
```
Now, the default network of system tenant should be Ready.
```bash
./cluster/kubectl.sh get net
NAME      TYPE   VPC   PHASE   DNS
default   flat         Ready   10.0.0.207
```

The output will be slightly different if you set ARKTOS_NETWORK_TEMPLATE=mizar.

7. Leave the arktos-network-controller termainal running and open a new terminal to install CNI plugin; below is for flannel
```bash
./cluster/kubectl.sh apply -f https://github.com/coreos/flannel/raw/master/Documentation/kube-flannel.yml
```

After that, you should see the "arktos-up.sh" terminal no longer displaying "Waiting for node ready at api server" message and the arktos cluster should be successfully up. 

If you want to, you are able to add more worker nodes to the cluster, by following [multi-node setup guide](multi-node-dev-cluster.md).
In AWS env, please make sure the adequate security group is set properly. In our temporary lab, we allowed inbound rule of ALL-Traffic 0.0.0.0/0.

From now on, you should be able to play with multi-tenant and the network features.
