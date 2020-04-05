# Roadmap

## Arktos Release v0.1 (1/30/2020)
Arktos is an open source cluster management system designed for large scale clouds. It is evolved from the open source Kubernetes v1.15 codebase with some fundamental improvements.
Arktos aims to be an open source solution to address key challenges of large scale clouds, including system scalability, resource efficiency, multitenancy, cross-AZ resiliency, and the native support for the fast-growing modern workloads such as containers and serverless functions.

Today we announce the v0.1 release of Arktos.

### Repos
1. Arktos: https://github.com/futurewei-cloud/arktos 
1. Arktos-cniplugins: https://github.com/futurewei-cloud/arktos-cniplugins
1. Arktos-vm-runtime: https://github.com/futurewei-cloud/arktos-vm-runtime

### Released Components
1. Unified Node Agent
1. Unified Scheduler
1. Partitioned and Scalable Controller Manager
1. API Server/Core Server with Multi-Tenancy and Unified Pod Support
1. Arktos VM Runtime Server

### Features
1. Multi-tenancy Features 
     1. Introduce a new layer “tenant” before “namespace” in API resource URL schema, to provide a clear and isolated resource hierarchy for tenants.
     1. Introduce a new API resource “tenant”, to keep tenant-level configurations and properties.
     1. The metadata section of all exiting API resources has a new member: tenantName.
     1. API Server, ClientGo, Scheduler, Controllers and CLI changes for the new resource model.
1. Unified VM/Container Support:
     1. Extend “pod” definition to both containers and VM. Now a pod can contain one VM, or one or more containers.
     1. Enhance scheduler to schedule container pods and VM pods in the same way (unified scheduling).
     1. Enhance kubelet to support multiple CRI runtimes (unified agent).
     1. Implement a VM runtime server evolved from project Virtlet, with new features like VM reboot, snapshot, restore, etc.
     1. Enhance kubelet to handle VM state changes and configuration changes.
     1. Introduce a new API resource “action” and the corresponding handles (action framework) to support some VM specific actions which are not appropriate to be expressed as state machine changes, like reboot and snapshot.
1. Artkos Integration with OpenStack Neutron
     1. Arktos network controller integrate with neutron.
     1. Arktos CNI plugins for neutron. 
1. Arktos integration with Mizar
     1. Arktos CNI plugins for mizar-mp.
1. Scalability
     1. Partitioned and scalable controller managers with active-active support.
     1. API Server Partitioning (in progress)
     1. ETCD partitioning (in progress)
### Known Issues
1. Go 1.13 is not supported yet.
### Features in Planning
1. Intelligent scheduling
1. in-place resource update 
1. QoS enforcement

## Arktos Release v0.2 (3/30/2020)

### Features Added
1. Multi-tenancy Features
     1. Tenancy short-path support
     2. Add Tenant Controller  
     3. Tenancy-aware token Authenticator 
     4. Tenancy-aware Cert Authenticator 
     5. Tenancy-aware RBAC Authorizer  
     6. Tenancy in kubeconfig context  
     7. Stabilization on multi-tenancy API Model  
     8. More test and workaround fixes Added  

2. Scalability Features
     1. API Server Data Partitioning
     2. Active-active controller framework - new Kubernetes master component: Workload Controller Manager
     3. Set up test environments for data partitioned environment 
     4. Add kube-up and start-kubemark for AWS
     
3. Unifed VM/containers
     1. Add support for primary runtime 
     2. Add volume driver for OpenStack Cinder 
     3. Fix issues related to VM pod vCPU settings 
   
4. Documentation
   1. New documentation readthedocs page

### Known Issues

   1. Create new tenant make events related to the tenant populated to all api servers
   2. Performance testing: Scheduling Throughput is one fourth of pre-arktos
   3. AWS: Register kubemark master as a node
   4. AWS: Start-kubemark failed to run without sudo password
   5. Get coredns working with kubeadm
   6. AWS: Add workload-controller-manager to aws kube-up and start-kubemark

### Future Releases 
   1. Performance test result  
   2. ETCD partitioning
   3. Intelligent scheduling
   4. In-place resource update
   5. QoS enforcement


