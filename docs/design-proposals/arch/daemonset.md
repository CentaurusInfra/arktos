## DaemonSet in Arktos

Due to the nature Arktos being a multiple-tenanted system, DaemonSet is only allowed in system tenant; only cluster admin is permitted to create/delete/update DaemonSet resources.

Multiple-TP also causes similar issue to Kube-proxy; changes to kube-proxy is covered in this design.

### What is not covered yet
At this stage of Arktos, PVC/PV for storage volume is not fully supported - out of scope of this DaemonSet design.

Recovering DaemonSets from failing TP is not in current design scope.

### DaemonSet in Scale-up Arktos
Scale-up Arktos has single master. With the admission controller blocking non-system tenant DaemonSet, Arktos is able to ensure DaemonSet proper support.

### DaemonSet in Scale-out Arktos
[Scale-out Arktos](arktos_scale_out.md) may have multiple tenant partitions and resource partitions; each of them has master components including API Server, controller manager, etc. DaemonSet resource will be managed by tenant partition, as the controllers in tenant partition have the global view of Node resources.

Usually DaemonSet requires other API resources to provision its daemon Pods with specific environmental data. For instance, configmap resource is used to initialize as app config file, secret as app sensitive data. These supporting resources shall be at the same Tenant Partion as the DaemonSet.

Different Daemonsets (of system tenant only) can reside on different Tenant Partitions. At one TP, DaemonSet should be uniquely named; however, a DaemonSet of TP could have the same name as one managed by aother TP - though this practice is not recommended.

At the specific tenant partition, DaemonSet controller populates daemon Pods for the specific DaemonSet, using the global view of Nodes; its scheduler schedules each daemon Pod to the proper Node subsequently.

On the node, kubelet gets notification of daemon Pod's creation, then prepares environment for the Pod (specifically including identifying some services and their IP addresses), finally starts the pod in the prepared environment. The environment preparation includes fetching the specified API resources (like secret, configmap, service account token) as defined in Pod spec and provisioning in form of files/data in node's filesystem. In order to ensure API resources are fetched from the proper tenant partition, kubelet shall maintain the mapping of daemon Pod to tenant partition origin; whenever the needed resource of the daemon Pod is fetched, kubelet shall lookup the mapping and use that of the proper tenant partition.

Though it is out of scope of DaemonSet design, the TP-origin of regular Pod warrants a brief discussion here. For system tenanted Pods - be daemon Pod or not - it shall keep track their origins when kubelet receives them, as there may be multiple TPs able to service system tenanted resources; for regular non-system tenant Pods, it is fine to derive its TP origin based on tenant name. The general design for system tenanted resource, which is beyond the scope of current DaemonSet design, will study this topic in more depth.

Each node may typically run daemon Pods of all DaemonSet from all the TP's. These daemon Pods should not conflict with each other to compete for same system resources, e.g. no two are trying to set up network as CNI plugin; whether their DaemonSet names are same or not is immaterial.

In case of the tenant partition that manages a DaemonSet fails, other tenant partitions, if applicable, are available for cluster admin to manage DaemonSets; the DaemonSet previously managed on the failed TP may not be updated, system functionality being degraded. There is possibilities to recover the affected DaemonSet to functioning TP, but it is out of our current design interests.

Arranging Daemonset and its supporting resources within the unit of TP as depicted above does not put unnecessary constraint of how system tenant being managed across TPs. Whether Arktos scale-out chooses to have multiple TPs with each being able to manage its own system tenant resources, or to have a single dedicated Partition for system tenant resources likely enhanced by some sort of HA, this design stands - as long as DaemonSet and its supporting API resources are managed by the same TP.

Providing a centralized DaemonSet management tool on top of all and every TP is convenience gain for cluster admin; however it is beyond the core design. The rudimentary utility using kubectl config context is able to mitigate before a better solution is available.

### Alternatives
#### Where DaemonSet in
We considered other options for DaemonSet in scale-out Arktos.

**DaemonSet in Resource Partition**

DaemonSet resource is managed combinely by all resource partitions; tenant partition is not involved. Each resource partition has its own copy of the DaemonSet resource; its DaemonSet controller and scheduler manage the DaemonSet in (local) scope of its Nodes.

This architecture has some defects:
* Lack of built-in mechanism to distribute and sync DaemonSet copies to all tenant partitions;
* Increased operation complexity: multiple RP's each runs master controllers;
* Decreased scheduler accuracy: scheduler of RP schedules daemon Pods which cannot have the resource allocation reflected in the scheduler of TP, which leads to inaccuracy of the deceived resources.
* Overlap of TP/RP: Pod/configmap/secret may be managed by TP or RP - depending on whether they are DaemonSet related or not, which tarnishes the clear cut of TP/RP.

**DaemonSet in Hybrid**

This is a variant of the above-mentioned DS in RP: cluster admin manages DaemonSet at tenant partition; system implements a built-in distribution mechanism (DaemonSet distribution controller) to push DeamonSet change to all resource partitions.

This architecture mitigates the DS sync problem, but it brings more incompatibility of scale-out and scale-up, as scale-up has no such sync in the first place.

#### Scope of TP Origin Tracking
Regarding the scope of changes to keep track of TP origin, there are options. It applies to both resources in kubelet and resources in kube-proxy.

**System-tenant Resources Only**

Only resources of system tenant need to keep track of TP origin; user tenant has not change.

Its pro is smaller memory footprint; smaller impact on existing system design.

**All tenants**

All reources used in kubelet, regardless of tenat they are in, keep track of their TP origins.

Its pro is consistency, no assumption that user tenant being on one single TP.

We'll implement by keeping track for all tenants.

### Required Changes
The minimum changes required to support DaemonSet of Arktos

#### API Server (admission Controller)
* The admission controller module to block non-system tenanted DaemonSet.
* API Servers (at least of tenant partitions) enables the admission controller when it is started

Below is the diagram of DaemonSet admission:
<p align="center"> <img src="images/daemonset_OPD/daemonset-support-DS-creating-DSadding.jpg"> </p>


#### Kubelet
Kubelet shall be able to identify the API resources (configmap, secret, service account token and service; however PVC is not in scape yet) from the same tenant partition as the daemon Pod to be started, in environment preparing process.

Below is the diagram of kubelet environment preparing process:
<p align="center"> <img src="images/daemonset_OPD/daemonset-support-DS-creating-podStarting-envPreparing.jpg"> </p>

There are following significant required functionalities for kubelet, at high level:
1. Keeping track of daemon Pod's TP origin.
   <p/>One trivial way is maintaining a map in memory for daemon Pod by UUID to TP origin.
   <p/>Pods, be of system tenant or others, identifies the original TP by the origin tracking (new introduction of pod-UUID to TP).
2. Retrieving supporting resources by TP origin.
   <p/>Secret and configmap resources each is managed via a local cached store being replenished by external API servers (TP's). Current store implementation simply identify TP by key of tenant name of the resource. To accomodate the DaemonSet design, the cache store should accept requests to get resource with specified TP origin; it should be able to keep system-tenanted secret or configmap without losing their TP origins; also it should use the resource-specific TP origin to replenish the staled resource instead of the simple tenant-name derived one.
   <p/>The specific detail the changes of the above-mentioned stores should be carefully considered before the kubelet module implementation.
   <p/>Service account token resources will be managed by the token manager, which will take multiple TP clients and be able to identify the one suited for SA token retrieval for the specific pod.
3. Keeping track of TP origin of service in Service lister, using services from proper TP when identifying services of the Pod.

#### Kube-proxy
In scale-out env, for services that are exposed from the system-tenanted Pods, kube-proxy shall be able to tell which TP a certain system-tenanted service is from and locate the endpoints from that TP only. Kube-proxy needs to have similar origin tracking mechanism.

The service and endpoint resources are of concern to be able to track their origins. Particularly, when identifying endpoints of a specific service, they must have same origin as of the service resource.
￼
￼The TP origin is the significant new property of system-tenanted service nd endpoint, as in 
￼<p align="center"> <img src="images/daemonset_OPD/kube-proxy-svc-ep-manage.jpg"> </p>
