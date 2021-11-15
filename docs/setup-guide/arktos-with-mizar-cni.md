# Arktos deployment with Mizar CNI 

This document is intended for new users to install the Arktos platform with Mizar as the underlying network technology.

Prepare lab machine, the preferred OS is **Ubuntu 18.04**. If you are using AWS, the recommended instance size is ```t2.2xlarge``` and the storage size is ```128GB``` or more.

For On-premise setup, the preferred OS is **Ubuntu 18.04**. The recommended instance size is ```8 CPU and 32GB RAM``` and the storage size is ```128GB``` or more.

The steps might change with the progress of development.

1. Check the kernel version:

```bash
uname -a
```

Update the kernel if the kernel version is below `5.6.0-rc2`

```bash
wget https://raw.githubusercontent.com/CentaurusInfra/mizar/dev-next/kernelupdate.sh
sudo bash kernelupdate.sh
```

2. Clone the Arktos repository and install the required dependencies:

```bash
git clone https://github.com/CentaurusInfra/arktos.git ~/go/src/k8s.io/arktos 
sudo bash $HOME/go/src/k8s.io/arktos/hack/setup-dev-node.sh
echo export PATH=$PATH:/usr/local/go/bin\ >> ~/.profile
echo cd \$HOME/go/src/k8s.io/arktos >> ~/.profile
source ~/.profile

```
  
3. Start Arktos cluster
```bash
CNIPLUGIN=mizar ./hack/arktos-up.sh
```

Then wait till you see:

```text
To start using your cluster, you can open up another terminal/tab and run:

  export KUBECONFIG=/var/run/kubernetes/admin.kubeconfig
Or
  export KUBECONFIG=/var/run/kubernetes/adminN(N=0,1,...).kubeconfig

  cluster/kubectl.sh

Alternatively, you can write to the default kubeconfig:

  export KUBERNETES_PROVIDER=local

  cluster/kubectl.sh config set-cluster local --server=https://ip-172-31-16-157:6443 --certificate-authority=/var/run/kubernetes/server-ca.crt
  cluster/kubectl.sh config set-credentials myself --client-key=/var/run/kubernetes/client-admin.key --client-certificate=/var/run/kubernetes/client-admin.crt
  cluster/kubectl.sh config set-context local --cluster=local --user=myself
  cluster/kubectl.sh config use-context local
  cluster/kubectl.sh
```

4. Leave the "arktos-up.sh" terminal and open another terminal to the master node. Verify mizar pods i.e. mizar-operator and mizar-daemon pods are in running state, for that run:

```bash
./cluster/kubectl.sh get pods
```
You should see the following output
```text
NAME                              HASHKEY               READY   STATUS    RESTARTS   AGE
mizar-daemon-qvf8h                3609709351651248785   1/1     Running   0          8m
mizar-operator-67df55cbd4-fbbtz   2504797451733876877   1/1     Running   0          8m
```

Now you can use the Arktos cluster with Mizar CNI.
