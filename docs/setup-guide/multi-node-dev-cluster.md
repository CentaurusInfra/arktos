# How to Setup a Dev Cluster of multi-nodes

It may be desired to setup a dev cluster having one or more worker nodes in order to play with comprehensive features of Arktos. The simple and easy-to-use arktos-up.sh feels short in this case; this doc describes the minimum effort to run a cluster having 1 master node and extra worker nodes joining after the master is up.

1. Start the worker node with default network solution, bridge, and register into arktos scale up cluster:

* Create folder /var/run/kubernetes or start and stop ./hack/arktos-up.sh so it will create the folder automatically.
* Copy /var/run/kubernetes/client-ca.crt file from arktos master. Or if you started arktos-up.sh in step 1, it will be created automatically.
* The following command will add worker into existing arktos cluster that is started with bridge network:

```bash
API_HOST=<master ip> ./hack/arktos-worker-up.sh
```

* The following command will add worker into existing arktos cluster that is started with Mizar CNIPlugin:
```bash
CNIPLUGIN=mizar API_HOST=<master ip> ./hack/arktos-worker-up.sh
```

After the script returns, go to master node terminal and run command "[arktos_repo]/cluster/kubectl.sh get nodes", you should see the work node is displayed and its status should be "Ready".

* label worker node as vm runtime capable (optional)

If you would like to allow this work node to run VM-based pods, please run below command at the master console:
```bash
./cluster/kubectl.sh label node <worker-node-name> extraRuntime=virtlet
```
The work-node-name is the new worker just added; its name can be found by ```./cluster/kubectl.sh get node```.

You should be able to notice that node in READY state after a while; you can run container pods now:
```bash
./cluster/kubectl.sh run nginx --image=nginx --replicas=2
./cluster/kubectl.sh get pod -o wide
```

To run a small cirros VM pod, you can use below yaml content:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: cirros-vm
spec:
  virtualMachine:
    name: vm
    keyPairName: "foo"
    image: download.cirros-cloud.net/0.3.5/cirros-0.3.5-x86_64-disk.img
    imagePullPolicy: IfNotPresent
```
