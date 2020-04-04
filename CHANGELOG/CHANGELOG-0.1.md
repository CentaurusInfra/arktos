## Release Summary

This release is the first release of the project. It includes the following new components:

* Unified Node Agent
* Unified Scheduler
* Partitioned and Scalable Controller Manager
* API Server with Multi-Tenancy and Unified Pod Support
* Arktos VM Runtime Server

## Key Features or Improvements:

* Multi-tenancy
	* Introduce a new layer “tenant” before “namespace” in API resource URL schema, to provide a clear and isolated resource hierarchy for tenants.
	* Introduce a new API resource “tenant”, to keep tenant-level configurations and properties.
	* The metadata section of all exiting API resources has a new member: tenantName.
	* API Server, ClientGo, Scheduler, Controllers and CLI changes for the new resource model.

* Unified VM/Container:
	* Extend “pod” definition to both containers and VM. Now a pod can contain one VM, or one or more containers.
	* Enhance scheduler to schedule container pods and VM pods in the same way (unified scheduling).
	* Enhance kubelet to support multiple CRI runtimes (unified agent).
	* Implement a VM runtime server evolved from project Virtlet, with new features like VM reboot, snapshot, restore, etc.
	* Enhance kubelet to handle VM state changes and configuration changes.
	* Introduce a new API resource “action” and the corresponding handles (action framework) to support some VM specific actions which are not appropriate to be expressed as state machine changes, like reboot and snapshot.
	* Artkos Integration with OpenStack Neutron.
	* Arktos integration with Mizar.

* Scalability
	* Partitioned and scalable controller managers with active-active support.
	* API Server Partitioning (in progress).
