---
title: In-place Update of Pod Resources for Container and VM Pods
authors:
  - "@vinaykul"
  - "@yb01"
---

# In-place Update of Pod Resources

## Table of Contents

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [API Changes](#api-changes)
    - [Resize Policy](#resize-policy)
    - [CRI Changes](#cri-changes)
  - [Kubelet and API Server Interaction](#kubelet-and-api-server-interaction)
    - [Kubelet Restart Tolerance](#kubelet-restart-tolerance)
  - [Scheduler and API Server Interaction](#scheduler-and-api-server-interaction)
  - [Flow Control](#flow-control)
    - [Container resource limit update ordering](#container-resource-limit-update-ordering)
    - [Container resource limit update failure handling](#container-resource-limit-update-failure-handling)
    - [Notes](#notes)
  - [Affected Components](#affected-components)
  - [Changes in Arktos-vm-runtime-service](Changes in Arktos-vm-runtime-service)
- [References](References)  

<!-- /toc -->

## Summary

This proposal aims at allowing Pod resource requests & limits to be updated
in-place, without a need to restart the Pod or its Containers or VMs.

The **core idea** behind the proposal is to make PodSpec mutable with regards to
Resources, denoting **desired** resources. Additionally, PodSpec is extended to
reflect resources **allocated** to a Pod, and PodStatus is extended to provide
information about **actual** resources applied to the Pod and its containers or VMs.

## Motivation

Resources allocated to a Pod's can require a change for various reasons:
* load handled by the Pod has increased significantly, and current resources
  are not sufficient,
* load has decreased significantly, and allocated resources are unused,
* resources have simply been set improperly.

Currently, changing resource allocation requires the Pod to be recreated since
the PodSpec's Resources are immutable.

While many stateless workloads are designed to withstand such a disruption,
some, like VMs are more sensitive, especially when using low number of Pod replicas.

Moreover, for stateful VMs or batch workloads, Pod restart is a serious disruption,
resulting in lower availability or higher cost of running.

Allowing resources to be changed without recreating the Pod or restarting the
containers or VMs addresses this issue directly.

### Goals

* Primary: allow to change Pod resource requests & limits without restarting
  its Containers or VMs.
* Secondary: allow actors (users, VPA, StatefulSet, JobController) to decide
  how to proceed if in-place resource resize is not possible.
* For VM, NUMA awareness of memory increase 
* For VM, x86 CPU architecture must be supported, ARM can be stretch goal
* For VM, major vendors of linux guest OS,

### Non-Goals
* For VM, fractional CPU increase is not supported. i.e. CPU change must be entire vCPU.


The explicit non-goal of this KEP is to avoid controlling full lifecycle of a
Pod which failed in-place resource resizing. This should be handled by actors
which initiated the resizing.

Other identified non-goals are:
* allow to change Pod QoS class without a restart,
* to change resources of Init Containers without a restart,
* eviction of lower priority Pods to facilitate Pod resize,
* updating extended resources or any other resource types besides CPU, memory.
* For VM, GPU, hugepage memory, not in current KEP scope

## Proposal

### API Changes

Arktos PodSpec becomes mutable with regards to resources requests and limits for
Containers as well as VMs. PodSpec is extended with information of resources
allocated on node for the Pod. PodStatus is extended to show the actual resources
applied to the Pod and its Containers or VM.

Thanks to the above:
* Pod.Spec.Containers[i].Resources and Pod.Spec.VirtualMachine.Resources become
  purely a declaration, denoting the **desired** state of Pod resources,
* Pod.Spec.Containers[i].ResourcesAllocated (new object, type v1.ResourceList)
  denotes the Node resources **allocated** to the Pod and its Containers,
* Pod.Spec.VirtualMachine.ResourcesAllocated (new object, type v1.ResourceList)
  denotes the Node resources **allocated** to the Pod and its VM,
* Pod.Status.ContainerStatuses[i].Resources (new object, type
  v1.ResourceRequirements) shows the **actual** resources held by the Pod and
  its Containers.
* Pod.Status.VirtualMachineStatus.Resources (new object, type
  v1.ResourceRequirements) shows the **actual** resources held by the Pod and
  its VM.

A new admission controller named 'PodResourceAllocation' is introduced in order
to limit access to ResourcesAllocated field such that only Kubelet can update
this field.

Additionally, Kubelet is authorized to update PodSpec, and NodeRestriction
admission plugin is extended to limit Kubelet's update access only to Pod's
ResourcesAllocated field for CPU and memory resources.

#### Resize Policy

To provide fine-grained user control, PodSpec.Containers is extended with
ResizePolicy - a list of named subobjects (new object) that supports 'cpu'
and 'memory' as names. It supports the following policy values:
* NoRestart - the default value; resize Container without restarting it,
* RestartContainer - restart the Container in-place to apply new resource
  values. (e.g. Java process needs to change its Xmx flag)

By using ResizePolicy, user can mark Containers as safe (or unsafe) for
in-place resource update. Kubelet uses it to determine the required action.

Setting the flag to separately control CPU & memory is due to an observation
that usually CPU can be added/removed without much problem whereas changes to
available memory are more probable to require restarts.

If more than one resource type with different policies are updated, then
RestartContainer policy takes precedence over NoRestart policy.

Additionally, if RestartPolicy is 'Never', ResizePolicy should be set to
NoRestart in order to pass validation.

#### CRI Changes

ContainerStatus CRI API is extended to hold *runtimeapi.ContainerResources*
so that it allows Kubelet to query Container's or VM's CPU and memory limit
configurations from runtime.

These CRI changes are a separate effort that does not affect the design
proposed in this KEP.

### Kubelet and API Server Interaction

When a new Pod is created, Scheduler is responsible for selecting a suitable
Node that accommodates the Pod.

For a newly created Pod, Spec.Containers[i].ResourcesAllocated must match
Spec.Containers[i].Resources.Requests. When Kubelet admits a new Pod, values in
Spec.Containers[i].ResourcesAllocated are used to determine if there is enough
room to admit the Pod. Kubelet does not set Pod's ResourcesAllocated after
admitting a new Pod.

When a Pod resize is requested, Kubelet attempts to update the resources
allocated to the Pod and its Containers. Kubelet first checks if the new
desired resources can fit the Node allocable resources by computing the sum of
resources allocated (Pod.Spec.Containers[i].ResourcesAllocated) for all Pods in
the Node, except the Pod being resized. For the Pod being resized, it adds the
new desired resources (i.e Spec.Containers[i].Resources.Requests) to the sum.
* If new desired resources fit, Kubelet accepts the resize by updating
  Pod.Spec.Containers[i].ResourcesAllocated, and then proceeds to invoke
  UpdateContainerResources CRI API to update Container resource limits. Once
  all Containers are successfully updated, it updates
  Pod.Status.ContainerStatuses[i].Resources to reflect new resource values.
* If new desired resources don't fit, Kubelet rejects the resize, and no
  further action is taken.
  - Kubelet retries the Pod resize at a later time.

If multiple Pods need resizing, they are handled sequentially in the order in
which Pod additions and updates arrive at Kubelet.

Scheduler may, in parallel, assign a new Pod to the Node because it uses cached
Pods to compute Node allocable values. If this race condition occurs, Kubelet
resolves it by rejecting that new Pod if the Node has no room after Pod resize.

#### Kubelet Restart Tolerance

If Kubelet were to restart amidst handling a Pod resize, then upon restart, all
Pods are admitted at their current Pod.Spec.Containers[i].ResourcesAllocated
values, and resizes are handled after all existing Pods have been added. This
ensures that resizes don't affect previously admitted existing Pods.

### Scheduler and API Server Interaction

Scheduler continues to use Pod's Spec.Containers[i].Resources.Requests for
scheduling new Pods, and continues to watch Pod updates, and updates its cache.
It uses the cached Pod's Spec.Containers[i].ResourcesAllocated values to
compute the Node resources allocated to Pods. This ensures that it always uses
the most recently available resource allocations in making new Pod scheduling
decisions.

### Flow Control

The following steps denote a typical flow of an in-place resize operation for a
Pod with ResizePolicy set to NoRestart for all its Containers.

1. Initiating actor updates Pod's Spec.Containers[i].Resources via PATCH verb.
1. API Server validates the new Resources. (e.g. Limits are not below
   Requests, QoS class doesn't change, ResourceQuota not exceeded...)
1. API Server calls all Admission Controllers to verify the Pod Update.
   * If any of the Controllers reject the update, API Server responds with an
     appropriate error message.
1. API Server updates PodSpec object with the new desired Resources.
1. Kubelet observes that Pod's Spec.Containers[i].Resources.Requests and
   Spec.Containers[i].ResourcesAllocated differ. It checks its Node allocable
   resources to determine if the new desired Resources fit the Node.
   * _Case 1_: Kubelet finds new desired Resources fit. It accepts the resize
     and sets Spec.Containers[i].ResourcesAllocated equal to the values of
     Spec.Containers[i].Resources.Requests. It then applies the new cgroup
     limits to the Pod and its Containers, and once successfully done, sets
     Pod's Status.ContainerStatuses[i].Resources to reflect desired resources.
     - If at the same time, a new Pod was assigned to this Node against the
       capacity taken up by this resource resize, that new Pod is rejected by
       Kubelet during admission if Node has no more room.
   * _Case 2_: Kubelet finds that the new desired Resources does not fit.
     - If Kubelet determines there isn't enough room, it simply retries the Pod
       resize at a later time.
1. Scheduler uses cached Pod's Spec.Containers[i].ResourcesAllocated to compute
   resources available on the Node while a Pod resize may be in progress.
   * If a new Pod is assigned to that Node in parallel, it can temporarily
     result in actual sum of Pod resources for the Node exceeding Node's
     allocable resources. This is resolved when Kubelet rejects that new Pod
     during admission due to lack of room.
   * Once Kubelet that accepted a parallel Pod resize updates that Pod's
     Spec.Containers[i].ResourcesAllocated, and subsequently the Scheduler
     updates its cache, accounting will reflect updated Pod resources for
     future computations and scheduling decisions.
1. The initiating actor (e.g. VPA) observes the following:
   * _Case 1_: Pod's Spec.Containers[i].ResourcesAllocated values have changed
     and matches Spec.Containers[i].Resources.Requests, signifying that desired
     resize has been accepted, and Pod is being resized. The resize operation
     is complete when Pod's Status.ContainerStatuses[i].Resources and
     Spec.Containers[i].Resources match.
   * _Case 2_: Pod's Spec.Containers[i].ResourcesAllocated remains unchanged,
     and continues to differ from desired Spec.Containers[i].Resources.Requests.
     After a certain (user defined) timeout, initiating actor may take alternate
     action. For example, based on Retry policy, initiating actor may:
     - Evict the Pod to trigger a replacement Pod with new desired resources,
     - Do nothing and let Kubelet back off and later retry the in-place resize.

#### Container resource limit update ordering

When in-place resize is requested for multiple Containers in a Pod, Kubelet
updates resource limit for the Pod and its Containers in the following manner:
  1. If resource resizing results in net-increase of a resource type (CPU or
     Memory), Kubelet first updates Pod-level cgroup limit for the resource
     type, and then updates the Container or VM resource limit.
  1. If resource resizing results in net-decrease of a resource type, Kubelet
     first updates the Container resource limit, and then updates Pod-level
     cgroup limit.
  1. If resource update results in no net change of a resource type, only the
     Container or VM resource limits are updated.

In all the above cases, Kubelet applies Container resource limit decreases
before applying limit increases.

Please noticed that the container or VM CGroup property update is done by the CRI implementation, when 
Kubelet calls the UpdateContainerResources() API.

#### Container resource limit update failure handling

If multiple Containers in a Pod are being updated, and UpdateContainerResources
CRI API fails for any of the containers, Kubelet will backoff and retry at a
later time. Kubelet does not attempt to update limits for containers that are
lined up for update after the failing container. This ensures that sum of the
container limits does not exceed Pod-level cgroup limit at any point. Once all
the container limits have been successfully updated, Kubelet updates the Pod's
Status.ContainerStatuses[i].Resources to match the desired limit values.

#### Notes

* If CPU Manager policy for a Node is set to 'static', then only integral
  values of CPU resize are allowed. If non-integral CPU resize is requested
  for a Node with 'static' CPU Manager policy, that resize is rejected, and
  an error message is logged to the event stream.
* To avoid races and possible gamification, all components will use Pod's
  Spec.Containers[i].ResourcesAllocated when computing resources used by Pods.
* If additional resize requests arrive when a Pod is being resized, those
  requests are handled after completion of the resize that is in progress. And
  resize is driven towards the latest desired state.
* Lowering memory limits may not always take effect quickly if the application
  is holding on to pages. Kubelet will use a control loop to set the memory
  limits near usage in order to force a reclaim, and update the Pod's
  Status.ContainerStatuses[i].Resources only when limit is at desired value.
* Impact of Pod Overhead: Kubelet adds Pod Overhead to the resize request to
  determine if in-place resize is possible.
* Impact of memory-backed emptyDir volumes: If memory-backed emptyDir is in
  use, Kubelet will clear out any files in emptyDir upon Container restart.
* At this time, Vertical Pod Autoscaler should not be used with Horizontal Pod
  Autoscaler on CPU, memory. This enhancement does not change that limitation.

### Affected Components

Pod v1 core API:
* extended model,
* modify RBAC bootstrap policy authorizing Node to update PodSpec,
* extend NodeRestriction plugin limiting Node's update access to PodSpec only
  to the ResourcesAllocated field,
* new admission controller to limit update access to ResourcesAllocated field
  only to Node, and mutates any updates to ResourcesAllocated & ResizePolicy
  fields to maintain compatibility with older versions of clients,
* added validation allowing only CPU and memory resource changes,
* setting defaults for ResourcesAllocated and ResizePolicy fields.

Admission Controllers: LimitRanger, ResourceQuota need to support Pod Updates:
* for ResourceQuota, podEvaluator.Handler implementation is modified to allow
  Pod updates, and verify that sum of Pod.Spec.Containers[i].Resources for all
  Pods in the Namespace don't exceed quota,
* PodResourceAllocation admission plugin is ordered before ResourceQuota.
* for LimitRanger we check that a resize request does not violate the min and
  max limits specified in LimitRange for the Pod's namespace.

Kubelet:
* set Pod's Status.ContainerStatuses[i].Resources for Containers upon placing
  a new Pod on the Node,
* update Pod's Spec.Containers[i].ResourcesAllocated upon resize,
* change UpdateContainerResources CRI API to work for both Linux & Windows.

Scheduler:
* compute resource allocations using Pod.Spec.Containers[i].ResourcesAllocated.

Other components:
* check how the change of meaning of resource requests influence other
  Kubernetes components.

### Changes in Arktos-vm-runtime-service

#### Background and context
Currently in Arktos, the VM workload POD is defined the same way as container pod in terms of resources in the spec, i.e.

    spec:
      resource:
       limits:
         cpu:
         memory:
       request:
         cpu:
         memory:
     
Arktos controls the VM pod resources the same way as for containers, i.e. schedule based on requested resource level, and evict 
POD based on the limits values, per some defined eviction policies.

Unlike the container, which is essentially the isolated process on the host, there are quite some differences for VM that
are worth to describe here to lay some common ground before the design for supporting vertical scaling for VMs.

For VM types, the resources for VM are also controlled by the Hypervisor and the management layer i.e. the Libvirt and the 
Qemu/KVM/Xen. For memory resource allocation, a few essential configurations are in the Libvirt domain definition:

    <domain>
      ...
      <maxMemory slots='16' unit='KiB'>1524288</maxMemory>
      <memory unit='KiB'>524288</memory>
      <currentMemory unit='KiB'>524288</currentMemory>
      ...
    </domain>

Memory defines the max memory allocation at the boot time, while the currentMemory defines the real memory allocation for
the guest OS. The delta between the currentMemory and memory is the space for memory ballooning to scale-up memory, and the
delta between least memory a guestOS requires (say 512Mi) to the currentMemory is the space for memory ballooning to scale-down
memory between VMs and the hypervisor. The maxMemory defines the max memory allocation for a VM at runtime, which can 
be achieved with memory hotplug.

Please be noted that there are a few limitations in current design and workflow:
1. Only spec.resource.limit was passed to the runtime from Kubelet via the linuxContainerResources.MemoryLimitInBytes
2. The linuxContainerResources.MemoryLimitInBytes was used to set the "memory" in the VM domain xml definition. currentMemory
   is set to the "memory" setting by default.
3. With Ballooning, to adjust the currentMemory, currently there is no auto adjust for currentMemory based on the 
   application running in the VM. For example, with Libvirt, the "setmem" API has to be called to adjust the currentMemory
   for a VM.
4. Direct change "memory" config can only be done after VM is restarted. i.e. setMaxMemory() cannot be done without restart
   the VM.
5. Memory hotplug/unplug can be done live to a running VM. this is hypervisor specific. Both QEMU and KVM supports it.

The aforementioned is some essentials to lay the common ground before we enter the design sessions. For more details, please
refer to the libvirt doc in the references[2]. 

For better clarity and communication, these terminologies were used in this document:

1. MaxBootTimeMemoryAllocation to refer the the memory setting in VM domain
2. CurrentMemoryAllocation to refer the currentMemory setting in the VM domain
3. MaxExpandableMemoryAllocation to refer the maxMemory setting in the VM domain

#### Goals and requirements

Let's summarize the goals in VM to support vertical scaling
1. Resource change must be live-update to running VMs
2. Support major distributions of Linux OS
3. Windows guest OS is a stretch goal pending support in Arktos runtime
4. NUMA aware memory increase/decrease
5. All VM flavors in Arktos must be supported

#### Design considerations

1. HotPlug(Unplug) vs Memory Ballooning for memory resizing

   HotPlug(Unplug) and Memory Ballooning are the two technologies that can be used for memory resizing for VMs. Both have 
   been there for years, and both has pros and cons in supporting memory resizing for VMs. The reference[3,4] has very detailed
   introduction and comparision between them.
   
   Memory Ballooning is primarily used by the hypervisor to reclaim some "unused" memory blocks from the guest OS. Combined
   with hypervisor logic and the "ballooning driver" added into the guest OS, some unused memory blocks can be reclaimed from
   a guest OS and the hypervisor can reused them to other VMs that require more memory allocation. 
   
   Memory Ballooning provides a good way for supporting memory over-subscription scenarios. Because the "free" memory is tracked
   by the ballooning driver before the memory reclaim request from the hypervisor, they can be reclaimed fairly quick when 
   more memory is needed at the host.
   
   However,  using Memory Ballooning supporting vertical scaling feature, there are few hard limitations:
   
     1. Memory Ballooning is limited between the maxBootTimeMemoryAllocation and the currentMemoryAllocation defined in the 
        domain with hard cap for memory scale-up.
        The maxBootTimeMemoryAllocation cannot be reset without restarting the VM. For example, given a VM with
        requested and limit of memory as 4G, essentially there is no space to scale up the VM with ballooning. The current 
        design would have to be changed to either increase the maxBootTimeMemoryAllocation or lower the currentMemoryAllocation
        to have ballooning to work. Even though, either way will not solve this limitation. Increasing maxBootTimeMemoryAllocation
        too high can introduce some unpredicted behavior of the VM ( and host as well) when it is restarted since it will 
        use this setting to boot the VM up before ballooning driver kicks in to reset the memory allocation to the 
        currentMemoryAllocation level. Some changes might be made into the hypervisor layer to address this situation. 
        However it will take quite some logic to achieve this goal. For example, how to differentiate the intention and 
        boot the VM with different settings( future work [2]).
        
        One possible way to mitigate this limitation is to control the scale up of a VM by some policies so that the 
        MaxBootTimeMemoryAllocation can be controlled to some level. however, this is not a reliable solution and has to be
        coordinated with other features such as number of VMs scheduled on the host, level of over-subscription on the host etc.
      
     2. Memory fragmentation
        The nature of the ballooning mechanism is to reclaim unused memory blocks of the guest OS and render them to host 
        hypervisor. Given time and with repeated memory increase and decrease, it can left the VM memory fragmentation 
        and affect its performance.
      
     3. Ballooning is not NUMA aware
        Ballooning driver just keeps tracking the unused memory blocks in the guest OS and render them back to the hypervisor
        as needed. It is not NUMA aware. Ballooning cannot support cases where applications are placed to a particular 
        NUMA node, and need more memory, 
   
   On the other hand, memory hotplug/unplug simulates the physical memory device being plugged into the physical machines 
   for a guest OS. It can increase the memory for the guest OS at runtime without rebooting the VM. The size of the memory 
   device can be configured as well from Megabytes to Gigabytes. It provides potentially large scale-up space for a VM, 
   along with NUMA aware memory plugin, faster memory increase to the VM etc. 
   
   However it also carries some limitations, especially with scale-down:
   
     1. Unplug device to scale down the VM could be slow or failed
        Unlike the ballooning mechanism where the free memory blocks are tracked in the memory maps before they were released
        to the hypervisor, the unplug operation has to be done until all memory allocations on the device are migrated out. 
        This, sometimes could be slow or failed.
     2. Implementation complexity
        Due to the above reason, the memory scale-down operation has to be implemented with retry and error check, and also,
        the logic in the runtime service to handle repeated same request from Kubelet due to this enlarged time window for 
        decrease the memory of a VM. In addition, to support different flavor of VM types, the multiple sizes of memory devices
        must be supported as well.
     
   
   The limitations are considered at the implementation level and not blocking for VM memory vertical scaling. 
   In Arktos VM runtime, it is proposed to use hotplug(hotUnplug) in current release with the de-coupled internal implementation 
   in mind for future works ( combined or new auto-scalar at VM runtime ).

2. VM memory live update

   Both attach and detach device operations are done via the AttachDeviceFlag and DetachDeviceFlag API, with flag to
   affect the running VM and persist the changes to the domain configuration for next reboot as well.
       
       AttachDeviceFlags(memoryDeviceDefinition, libvirt.DOMAIN_DEVICE_MODIFY_CONFIG|libvirt.DOMAIN_DEVICE_MODIFY_LIVE)
       DetachDeviceFlags(memoryDeviceDefinition, libvirt.DOMAIN_DEVICE_MODIFY_CONFIG|libvirt.DOMAIN_DEVICE_MODIFY_LIVE)

3. Some limitations/constraints for VM workload types
       
    1. Guest OS Kernel requirements
       Guest OS kernel must support device hotPlug and hotUnplug.

#### Handle the async operation for memory unplug

Once after the kubelet issued the patch to change the resource settings for a VM pod, it will periodically check the POD
status for the reported VM configurations to ensure the desired setting is set. Kubelet will resend the request via the 
UpdateContainerResource() API call to the runtime till the desired setting is done.

The below logic implements the approach to avoid the repeated updateContainerResource() calls from Kubelet for the VM being updated:

Expand the VMConfig struct with two new fileds. So it can be persisted to runtime metadata in case of runtime service restarts.

    type VMConfig struct {
        // currently a resource is being updated
   	    ResourceUpdateInProgress bool
   	    // Resource being updated
   	    MemoryLimitInBytesBeingUpdated int64
   	    ......
   	    ......
   	    }

Add a handlers for the VIR_DOMAIN_EVENT_ID_DEVICE_REMOVED and VIR_DOMAIN_EVENT_ID_DEVICE_REMOVAL_FAILED events in Libvirt.

In UpdateContainerResource(), the workflow is as follows:

    1. if the VMConfig.ResourceupdteInProgress is set, ignore the call from Kubelet for VM update to the same resource 
    2. In the AdjustDomainMemory() function, Set the VMConfig to set the ResourceUpdateInProgress
    3. The two aforementioned handlers reset the VMConfig.ResourceupdteInProgress
   
   
#### Multiple memory device sizes support
Different flavors of VMs will likely need different level of memory scale-up to supports it peak resource consumptions. Cloud
providers, like AWS, has VM flavors from nano type ( 1vcpu, 500MiB RAM ) to extra large or memory optimized VMs with TB level
of RAMs. Some customer data or VM usage analysis are needed to determine what size memory device will be needed. Currently Arktos
starts with the below sizes of devices, that supports from 128MiB to 2GiB for each device. NUMA node can be set as needed.

    const memoryDeviceDefinition1 = `<memory model='dimm'>
							<target>
								<size unit='MiB'>128</size>
								<node>0</node>
							</target>
						</memory>`

    const memoryDeviceDefinition2 = `<memory model='dimm'>
							<target>
								<size unit='MiB'>512</size>
								<node>0</node>
							</target>
						</memory>`

    const memoryDeviceDefinition3 = `<memory model='dimm'>
							<target>
								<size unit='MiB'>1024</size>
								<node>0</node>
							</target>
						</memory>`

    const memoryDeviceDefinition4 = `<memory model='dimm'>
							<target>
								<size unit='MiB'>2048</size>
								<node>0</node>
							</target>
						</memory>`
 
The VM runtime internal function, determineNumberOfDeviceNeeded(),  will be modified with the workflow as:
 
    If AttachDevice
       GetVMSize
       GetFreeSlot
       if VMsizeSmall and FreeSlotEnough and numberOfNewDevice < "some limits"
          DetermineNumberDevicesAndSize
          
    If Detach
       QueryCurrentAttachedDevices
       DetermineDevicesToDetach
          
    return ArrayOfDevieXml
     
 
#### Arktos-vm-runtime internal API changes
As described in the "CRI changes" section, the Arttos-vm-runtime UpdateContainerResource() were implemented to update
the VM CGroup settings and update the VM domain definition with the new resource configuration.

If Kubelet enables its CPU manager and set the cpus to a particular pod. the cpuset is sent to the runtime via the 
UpdateContainerResource() and the runtime will honor the cpusets in both VM definition and its CGroup.

Two new APIs, SetVcpus() and AdjustDomainMemory() were added to support VM vertical scaling, which extends the call to 
Libvirt APIs to update the VM domain xml.

	// Update vcpu for a give domain
	SetVcpus(int) error
	// Update the domain memory with request to scale up or scale down the VM
    AdjustDomainMemory(int64) error
	
Both of them has similar workflow a below:

    1. Get the linux resource from the UpdateContainerResource request
    2. Get the current configuration from the VM 
    3. Calculate the delta and determine Attach or Detach operation ( for CPU it will be enable and disable operation)
    4. Call the Libvirt API to update the domain.
    
The ContainerStatus() implementation is also changed to return the current configured resources in the VM to Kubelet. Only
change for the vertical scale feature is to sync the VM domain configuration with the runtime metadata in containerInfo.Config
data structure.

#### Type conversions
There are a few data struct that handles, transports or stores the resource info as metadata in Kubernetes, VM runtime 
and Libvirt. It is important to list here so during conversion the data will not be truncated.

|item	|Kubelet|linuxContainerResource in CRI|LinuxResource & CG	| Runtime vmConfig	|Runtime DomainSettings	|Libvirt domain |
|-------|-------|-----------------------------|-----------------------|-----------------------|-----------------------|---------------|
|vcpus	|int64	|-	                      |-	              |-	              |int	              |domainVcpu.Value, int
|cpuShares|-	|int64, default 0	      |LinuxCPU.Shares, *uint64|Int64	              |uint |domainCpuTuneShares.Value |uint
|cpuQuota |-	|Int64, default 0.            |LinuxCPU.Quota, *int64	|Int64	|int64	|domainCPUTTuneQuota.Value |int64
|cpuPeriod|-	|Int64, default 0.            |LinuxCPU.Period, *uint64	|Int64	|Uint64	|domainCPUTunePeriod.Value |uint64
|Memory	| Int64	|Int64 InBytes, default 0.    |LinuxMemory.Limit *int64, inBytes|Int64	|Int, with Unit def	|domainMemory.Value uint

#### Keep metadata in-sync
The Arktos-vm-runtime service has to handle three pieces metadata, VM CGroup, vmConfig, and Libvirt domain definition, 
and ensure they are synchronized when they were used.

All three of them were persisted in different stores, CGroup in files, vmConfig in runtime's metadata store and domain 
definition in the Libvirt metadata store. it is runtime's responsibility to keep them in sync. When VM resource is 
created or updated, CGroup, vmConfig, domain definition are persisted in such order:

1. Update CGroup
2. Update VMResources( vmConfig and domainDef)

If update VMResources operation failed, then the CGroup update operation will be rolled back and error will be raised.

During UpdateVmResources(), no rollback is enforced. The ContainerStatus() function will sync up the vmConfig and 
Libvirt domain definition if they are out of sync.

#### Future works
Currently as described in this proposal, the memory resizing for VMs is done via the memory hotplug and hotunplug technology
supported in Linux kernel. In the future, a few things can be improved:

1. Combine with the ballooning technology and provide better, refined resizing for memory
2. Qemu (other hypervisor) dynamically determine boot VM with its previously allocated memory instead of bootTimeMemoryAllocation
3. Vertical scaling with Hugepage memory

#### Test Cases
For a existing VM/container workload, with current vcpu C, memory X, currentMemory Y, and MaxMemory Z

##### Single resource (memory or CPUs) changes
1. set VM memory to X + Delta; then reduce it to X
2. set memory to X + Delta; repeat to increase memory to X + 3*Delta; then reduce memory to X
3. set VM vcpus to C+1, then reduce it to C
4. set VM vcpus to C + 3, then reduce it to C+1

##### Mixed resources (memory and CPUs) changes
1. do combined changes in #1 and #3;
2. do combined changes in #2 and #4

##### Memory changes with NUMA node on VM
1. set VM with 4 NUMA nodes, do #2 for NUMA node 1

##### VM reboot cases
1. set VM memory to X + Delta; reboot VM; verify the memory is X + Delta after reboot
2. set VM memory to X + Delta, vcpus to C+1; reboot VM; verify memory X+Delta and vcpus C+1 after reboot

##### VM migration cases
1. set VM memory to X + Delta; migrate VM to another node; verify the memory is X + Delta after reboot
2. set VM memory to X + Delta, vcpus to C+1; migrate VM to another node; verify memory X+Delta and vcpus C+1 after reboot
3. set VM memory to X + Delta on NUMA node 2; migrate VM to another node, verify memory X+Delta on NUMA node 2.

## References
1. Linux kernerl doc:
   https://www.kernel.org/doc/html/latest/admin-guide/mm/memory-hotplug.html
2. Libvirt doc:
   https://libvirt.org/html/libvirt-libvirt-domain.html#virDomainDetachDevice
3. Resizing Memory With Balloons and Hotplug:
   https://www.landley.net/kdocs/ols/2006/ols2006v2-pages-313-320.pdf
4. virtio-mem: Paravirtualized memory:
   https://events19.linuxfoundation.org/wp-content/uploads/2017/12/virtio-mem-Paravirtualized-Memory-David-Hildenbrand-Red-Hat-1.pdf

