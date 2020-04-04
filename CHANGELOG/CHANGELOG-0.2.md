# Release Summary

This release focuses on the stabilization of Arktos as well as new features in multi-tenancy, scalability and unified VM/Container. Major improvements include: 

* Multi-tenancy: virtualized multi-tenancy cluster based on short path and access control.
* Scalability: API server data partitioning and performance test in AWS.  
* Unified VM/Container: Partial runtime services readiness and storage volume support.  

# Key Features or Improvements 

Multi-tenancy:  

* Multi-tenancy design update [#101](https://github.com/futurewei-cloud/arktos/pull/101)  
* Tenancy short-path support [#50](https://github.com/futurewei-cloud/arktos/pull/50) 
* Add Tenant Controller [#124](https://github.com/futurewei-cloud/arktos/pull/124)
* Tenancy-aware token Authenticator [#129](https://github.com/futurewei-cloud/arktos/pull/129)
* Tenancy-aware Cert Authenticator [#99](https://github.com/futurewei-cloud/arktos/pull/99) 
* Tenancy-aware RBAC Authorizer [#20](https://github.com/futurewei-cloud/arktos/pull/20)  
* Tenancy in kubeconfig context [#69](https://github.com/futurewei-cloud/arktos/pull/69) 
* Stabilization, more test and workaround fixes [#92](https://github.com/futurewei-cloud/arktos/pull/92)  

Scalability:  

* API Server Data Partitioning [#105](https://github.com/futurewei-cloud/arktos/pull/105),  [#65](https://github.com/futurewei-cloud/arktos/pull/65) 
* Tools and guidance for setting up data partitioned Arktos cluster [#62](https://github.com/futurewei-cloud/arktos/pull/62)  
* Add kube-up and start-kubemark for AWS [#127](https://github.com/futurewei-cloud/arktos/pull/127)   

Unified VM/Container:
 
* Add support for primary runtime [#126](https://github.com/futurewei-cloud/arktos/pull/126)  
* Add volume driver for OpenStack Cinder [#93](https://github.com/futurewei-cloud/arktos/pull/93)  
* Fix issues on VM pod vCPU settings [#139](https://github.com/futurewei-cloud/arktos/pull/139)  

# Known Issues 

Below is a list of known issues of this release:

* Create new tenant make events related to the tenant populated to all api servers [#64](https://github.com/futurewei-cloud/arktos/issues/64) 
* Performance testing: Scheduling Throughput is one fourth of pre-Alkaid [#96](https://github.com/futurewei-cloud/arktos/issues/96) 
* AWS: Register kubemark master as a node [#171](https://github.com/futurewei-cloud/arktos/issues/171) 
* AWS: Start-kubemark failed to run without sudo password [#170](https://github.com/futurewei-cloud/arktos/issues/170) 
* Get coredns working with kubeadm [#169](https://github.com/futurewei-cloud/arktos/issues/169) 
* AWS: Add workload-controller-manager to aws-kube-up and start-kubemark [#168](https://github.com/futurewei-cloud/arktos/issues/168) 
