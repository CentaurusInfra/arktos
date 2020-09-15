# Arktos to Enforce the Multi-tenant Network Feature

This document captures the steps applied to an Arktos cluster lab enabling the unique multi-tenant network feature. The machines in this lab used are AWS EC2 t2-large (2 CPUs, 8GB mem), Ubuntu 18.04 LTS.

The steps might change with the progress of development.

If you would like to try with Flannel cni plugin, please ensure read [multi-node setup guide](multi-node-dev-cluster.md).

1. Prepare lab machines. Particularly, build arktos-network-controller (as it is not part of arktos-up.sh yet), and disable the local DNS cache:
```bash
make all WHAT=cmd/arktos-network-controller
sudo rm -f /etc/resolv.conf
sudo ln -s /run/systemd/resolve/resolv.conf /etc/resolv.conf
```

2. Enable network related feature and change cni installation setting by
```bash
export FEATURE_GATES="AllAlpha=false,MandatoryArktosNetwork=true"
export ARKTOS_NO_CNI_PREINSTALLED=y
// todo: the default-network-template-file related env set here
```

3. Prepare the default network template file; its content is specific to the cni/network provider you are going to use. For instance, below works for flat typed network (cni plugin is flannel):
```json
{
  "metadata": {
    "name": "default",
    "finalizers": ["arktos.futurewei.com/network"]
  },
  "spec": {
    "type": "flat"
  }
}
```

4. Start Arktos cluster
```bash
./hack/arktos-up.sh
```
After the cluster is up, there will be the first network, "default", in system tenant. Its state is empty at this moment.
```bash
./cluster/kubectl.sh get net
NAME      TYPE   VPC   PHASE   DNS
default   flat
```

5. Start the arktos-network-controller
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

6. Install CNI plugin; for flannel, we ran
```bash
./cluster/kubectl.sh apply -f https://github.com/coreos/flannel/raw/master/Documentation/kube-flannel.yml
```

If you want to, you are able to add more worker nodes to the cluster, by following [multi-node setup guide](multi-node-dev-cluster.md).

From now on, you should be able to play with multi-tenant and the network features.
