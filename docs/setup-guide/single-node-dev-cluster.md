# How to Setup a Dev Cluster of single node on AWS EC2 instance running Ubuntu 18.04, 16.04 and 20.04

0. Pre-requisite - setup local development environment
```
  https://github.com/CentaurusInfra/arktos/blob/master/docs/setup-guide/setup-dev-env.md
```

1. Run script to create a single arktos cluster

   Note: If your Dev Cluster of single node is on AWS EC2 instance running Ubuntu 20.04, please FIRST comment out the following three lines at https://github.com/CentaurusInfra/arktos/blob/master/pkg/kubelet/cm/container_manager_linux.go#L804,#L806 before running the command './hack/arktos-up.sh'.

        if cpu != memory {
		return "", fmt.Errorf("cpu and memory cgroup hierarchy not unified.  cpu: %s, memory: %s", cpu, memory)
	}

```bash
   $ ./hack/arktos-up.sh
```

   Note: If your Dev Cluster of single node is on AWS EC2 instance running Ubuntu 18.04, after you run the command './hack/arktos-up.sh', you may experience the following error. Just simply remove the symbolic link and re-run the command './hack/arktos-up.sh'.

         ln: failed to create symbolic link '/home/ubuntu/go/src/arktos/_output/bin': File exists 

```bash
   $ rm -i /home/ubuntu/go/src/arktos/_output/bin
   $ ./hack/arktos-up.sh
```


2. Open another terminal to use arktos cluster
```bash
  $ export KUBECONFIG=/var/run/kubernetes/admin.kubeconfig
Or
  $ export KUBECONFIG=/var/run/kubernetes/adminN(N=0,1,...).kubeconfig

  $ cluster/kubectl.sh

Alternatively, you can write to the default kubeconfig:

  $ export KUBERNETES_PROVIDER=local

  $ sudo cluster/kubectl.sh config set-cluster local --server=https://ip-172-31-3-192:6443 --certificate-authority=/var/run/kubernetes/server-ca.crt
  $ sudo cluster/kubectl.sh config set-credentials myself --client-key=/var/run/kubernetes/client-admin.key --client-certificate=/var/run/kubernetes/client-admin.crt
  $ sudo cluster/kubectl.sh config set-context local --cluster=local --user=myself
  $ sudo cluster/kubectl.sh config use-context local
  $ sudo cluster/kubectl.sh config get-contexts
  $ cluster/kubectl.sh
  $ cluster/kubectl.sh get nodes
  $ cluster/kubectl.sh get all --all-namespaces
```
