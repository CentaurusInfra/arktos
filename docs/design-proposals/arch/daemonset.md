## DaemonSet in Arktos

Due to the nature Arktos being a multiple-tenanted system, DaemonSet is only allowed in system tenant; only cluster admin is permitted to create/delete DaemonSet resources.

### DaemonSet in Scale-up Arktos
Scale-up Arktos has single master. With the admission controller blocking non-system tenant DaemonSet enabled by API Server, Arktos is able to ensure DaemonSet proper support.

### DaemonSet in Scale-out Arktos
[Scale-out Arktos](arktos_scale_out.md) may have multiple tenant partitions and resource partitions; each of them has master components including API Server, controller manager, etc. DaemonSet resource will be managed by tenant partition, as the controllers in tenant partition have the global view of Node resources.

In tenant partition, DaemonSet controller populates daemon Pods for the specific DaemonSet, using the global view of Nodes; its scheduler schedules each daemon Pod to the proper Node subsequently.

On the node, kubelet gets notification of daemon Pod's creation, then prepares environment for the Pod, finally starts the pod in the prepared environment. The environment preparation includes fetching the specified API resources (like secret, configmap) as defined in Pod spec and provisioning in form of files/data in node's filesystem. In order to ensure API resources are fecthed from the proper tenant partition, kubelet shall maintain the mapping of daemon Pod to tenant partition origin; whenever the needed resource of the daemon Pod is fetched, kubelet shall lookup the mapping and use that of the proper tenant partition.

In case of the tenant partition that manages a DaemonSet fails, other tenant partitions are available for cluster admin to manage DaemonSets; the DaemonSet previously managed on the failed TP may not be updated, system functionality being degraded. There is possibilities to recover the affected DaemonSet to functioning TP, but it is out of our current design interests.

#### Alternatives
We considered other options for DaemonSet in scale-out Arktos.

**DaemonSet in Resource Partition**

DaemonSet resource is managed combinely by all resource partitions; tenant partition is not involved. Each resource partition has its own copy of the DaemonSet resource; its DaemonSet controller and scheduler manage the DaemonSet in (local) scope of its Nodes.

This architecture has some defects:
* Lack of built-in mechanism to distribute and sync DaemonSet copies to all tenant partitions;
* Increase of inconsistency of tenanted API resource accesses (e.g. kubelet need to get Pods from RP besides TP's).

**DaemonSet in Hybrid**

This is a variant of the above-mentioned DS in RP: cluster admin manages DaemonSet at tenant partition; system implements a built-in distribution mechanism (DaemonSet distribution controller) to push DeamonSet change to all resource partitions.

This architecture mitigates the DS sync problem, but it brings more incompatibility of scale-out and scale-up.

### Reequired Changes
The minimum changes required to support DaemonSet of Arktos

#### API Server (admission Controller)
* The admission controller module to block non-system tenanted DaemonSet.
* API Servers (at least of tenant partitions) enables the admission controller when it is started

Below is the diagram of DaemonSet admission:
<p align="center"> <img src="images/daemonset_OPD/daemonset-support-DS-creating-DSadding.jpg"> </p>


#### Kubelet
Kubelet shall be able to identify the API resources (configmap, secret etc) from the same tenant partition as the daemon Pod to be started, in environment preparing process.

Below is the diagram of kubelet environment preparing process:
<p align="center"> <img src="images/daemonset_OPD/daemonset-support-DS-creating-podStarting-envPreparing.jpg"> </p>
