# Network Interface Hotplug

It is not rare that admin of virtual machine (vm for short) wants to dynamically add secondary network interfaces while the instance is still running; this hotplug ability is available from various hypersisors like qemu/kvm, vmware etc. Alkaid, our platform managing vm pods as the first class citizen as well as traditional container based ones, provides the similar feature of network interface hotplug. 

This doc describes how secondary nic is plugged into a running vm instance in Alkaid, from the perspective of end user (vm admin).

## Launching a vm pod
VM pod is defined, in k8s way, by pod spec file. For vm pod that might have nic hotplug, there is a requirement of pod spec - nic __MUST be specified explicitly defined, and nic name MUST be present__. 

Below  is an example of such vm pod spec, denoted as vm-nic-hotplug.yaml:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx-vm
spec:
  vpc: vpcdemo
  nics:
  - name: eth0
  virtualMachine:
    name: cirros-vm
    image: "download.cirros-cloud.net/0.3.5/cirros-0.3.5-x86_64-disk.img"
    imagePullPolicy: IfNotPresent
    resources:
      limits:
        cpu: "1"
        memory: 200Mi
```

Run following command; the vm pod is created and enters in Running state soon afterwards:
```bash
kubectl apply -f vm-nic-hotplug.yaml
``` 

## Specify the secondary nic to plugin
NIC is one of the types of resources a pod has; in line of k8s spirit, pod spec keeps the desired state of resources, and Alkaid system (notably kubelet) identifies the actual state of resources, attempts to reconcile the difference between the actual and desired states. In case of nic, kubelet identifies the difference of the nic spec and nic status collected from the underlying runtime system, issues necessary commands to the runtime requesting attaching nic to specific vm instance.

Update of pod spec is specified as patch spec, denoted as patch-nic-hoptlug.yaml
```yaml
spec:
  nics:
    - name: "eth1"
```

## Request nic hotplug
pacth spec is send via kubectl
```bash
kubectl patch pod nginx-vm -p "$(cat patch-nic-hotplug.yaml)"
```
After a short while, please observe the nic status which includes the added nic resource. For now, the state is unknown due to implementation limitation; it will be the proper state reflecting the actual hotplug result after the defect is addressed as planned.

## Use the plugged nic from inside vm pod
Logged in on the vm pod by
```bash
kubectl attach -t nginx-pod
```
Inside of vm instance, run following commands to verify that the secondary nic is ready
```bash
netstat -i
ip address
```

## Plug out the secondary nic
Following yaml file, denoted as patch-delete-eth1.yaml, identifies eth1 to be plug out:
```yaml
spec:
  nics:
    - $patch: delete
      name: "eth1"
```

Run following command to request eth1 plugout:
```bash
kubectl patch pod nginx-pod -p "$(cat patch-delete-eth1.yaml)"
```
