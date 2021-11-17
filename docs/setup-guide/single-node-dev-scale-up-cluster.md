# How to Setup a Dev Cluster of single node on AWS EC2 instance running Ubuntu 20.04, 18.04, 16.04 x86 

0. Pre-requisite - setup local development environment based on [set up developer environment](setup-dev-env.md)

1. Run script to create a single arktos cluster

```bash
   ./hack/arktos-up.sh
```

   Note: If your Dev Cluster of single node is on AWS EC2 instance running Ubuntu 18.04, after you run the command './hack/arktos-up.sh', you may experience the following error. Just simply remove the symbolic link and re-run the command './hack/arktos-up.sh'.

         ln: failed to create symbolic link '/home/ubuntu/go/src/arktos/_output/bin': File exists 

```bash
   make clean
   ./hack/arktos-up.sh
```

2. Open another terminal to use arktos cluster
```bash
  ./cluster/kubectl.sh
  ./cluster/kubectl.sh get nodes
  ./cluster/kubectl.sh get all --all-namespaces
```
```bash
Alternatively, you can write to the default kubeconfig:

  export KUBERNETES_PROVIDER=local

  ./cluster/kubectl.sh config set-cluster local --server=https://<hostname>:6443 --certificate-authority=/var/run/kubernetes/server-ca.crt
  ./cluster/kubectl.sh config set-credentials myself --client-key=/var/run/kubernetes/client-admin.key --client-certificate=/var/run/kubernetes/client-admin.crt
  ./cluster/kubectl.sh config set-context local --cluster=local --user=myself
  ./cluster/kubectl.sh config use-context local
  ./cluster/kubectl.sh config get-contexts
  ./cluster/kubectl.sh
  ./cluster/kubectl.sh get nodes
  ./cluster/kubectl.sh get all --all-namespaces
```

3. Test whether the ngnix application can be deployed successfully
```bash
   ./cluster/kubectl.sh run nginx --image=nginx --replicas=2
```
```bash
   ./cluster/kubectl.sh get pod -n default -o wide
```
```bash
   ./cluster/kubectl.sh exec -ti <1st pod> -- curl <IP of 2nd nginx pod>
```
```bash
   ./cluster/kubectl.sh exec -ti <2nd pod> -- curl <IP of 1st nginx pod>
```
