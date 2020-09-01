This release contains some new core features, scalability improvement and lots of stabilization improvements.

Some highlights include:   

 * New core features such as **etcd partitioning**, **multi-tenancy controllers & CRD**, **in-place vertical scaling of both container and VM**, **multi networking**, etc.  
 * Verified scalability improvement of **300K pods and 10K nodes**.
 * Stabilization, improved build&test infrastructure and more test coverage.

The detailed enhancements are listed below.

## Key Features and Improvements 

**Unified VM/Container:** 

* Add features of **in-place container vertical scaling**.
* Add the initial implementation of **in-place VM vertical scaling**, based on vcpu and memory hotplug.
* Bump to new libvirt version in Arktos VM runtime.
* Enable standalone deployment of Arktos VM runtime for edge scenarios.
* Use same cgroup hierarchy for container pods and VM pods.
* Refactor and simplify the runtime manager code in kubelet.

**Multi Tenancy:**

* Add the new feature of **per-tenant CRD**. Each tenant can install their own CRDs without impacting each other.
* Add the new feature of **tenant-shared CRD**. Tenants can share a CRD installed in system space.
* Add support of tenant.All in client-go.
* Add support of patch and more other commands for "--tenant" option in kubelet.
* Update most commonly-used controllers to **tenancy-aware controllers**:
    * Job controller
    * Volume controller, include corresponding changes in scheduler and kubelet
    * StatefulSet controller 
    * Service controller
    * Resource quota controller
    * Daemonset controller
    * Cronjob controller
* Update **Tenant controller** to:
    * Initialize default tenant role and role binding during tenant creation.
    * Initialize default network object during tenant creation. 
    * Support tenant deletion.
* Stabilization and various bug fixes. 

**Multi-Tenancy Networking:**

* Add the new **CRD object "Network"**.
* Initial support of **per-network service IP** allocation.
* Add pod network annotations and check readiness in kubelet.
* Initial implementation of flat network controller, with flannel network provider.

**Scalability:**

* Verified support of **300K pods with 10k nodes** within performance constraint.
* Support **multiple etcd clusters** to shard cluster data.
* Bump to latest etcd version and customized it for multi-etcd partitioning.
* Stabilization and enhancement of multiple API Server partitioning.
* Stabilization and enhancement of multiple controller partitioning.
* Add test infrastructure support for AWS.
* Improved build and test infrastructure.

