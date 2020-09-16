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

If mizar cni plugin is to be used, please replace containerd following the instruction of [multi-tansnt aware containerd](https://github.com/futurewei-cloud/containerd/releases/tag/tenant-cni-args).

2. Enable network related feature and change cni installation setting by (assuming flat typed networks here)
```bash
export FEATURE_GATES="AllAlpha=false,MandatoryArktosNetwork=true"
export ARKTOS_NO_CNI_PREINSTALLED=y
export ARKTOS_NETWORK_TEMPLATE=default
```

3. Start Arktos cluster
```bash
./hack/arktos-up.sh
```
After the cluster is started, there will be the first network, "default", in system tenant. Its state is empty at this moment.
```bash
./cluster/kubectl.sh get net
NAME      TYPE   VPC   PHASE   DNS
default   flat
```

4. Start the arktos-network-controller
```bash
./_output/local/bin/linux/amd64/arktos-network-controller --kubeconfig /home/ubuntu/.kube/config
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
Now, the default network of system tenant should be Ready
```bash
./cluster/kubectl.sh get net
NAME      TYPE   VPC   PHASE   DNS
default   flat         Ready   10.0.0.207
```

5. Install CNI plugin; below is for flannel
```bash
./cluster/kubectl.sh apply -f https://github.com/coreos/flannel/raw/master/Documentation/kube-flannel.yml
```

If you want to, you are able to add more worker nodes to the cluster, by following [multi-node setup guide](multi-node-dev-cluster.md).

From now on, you should be able to play with multi-tenant and the network features.
