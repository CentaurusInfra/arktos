---
title: VM and Container Common Fields for CloudFabric
authors:
  - "@vinaykul"
  - "@yb01"
  - "@penggu"
---

# VM and Container Common Fields Design for CloudFabric

## Table of Contents

   * [VM and Container Common Fields for CloudFabric](#vm-and-container-common-fields-for-cloud-fabric)
      * [Table of Contents](#table-of-contents)
      * [Motivation](#motivation)
         * [Goals](#goals)
         * [Non-Goals](#non-goals)
      * [Proposal](#proposal)
         * [API Changes](#api-changes)
         * [CommonInfo struct definition](#commoninfo-struct-definition)
         * [Container struct definition](#container-struct-definition)
            * [Container YAML definition](#container-yaml-definition)
         * [VirtualMachine struct definition](#virtualmachine-struct-definition)
            * [VM YAML definition](#vm-yaml-definition)
      * [Affected Components](#affected-components)
      * [Risks and Mitigations](#risks-and-mitigations)
      * [Graduation Criteria](#graduation-criteria)

## Motivation

One of the goals of CloudFabric is to unify management of Containers and VMs,
and use the Kubernetes way of doing things. Today, PodSpec and PodStatus only
let you specify Containers and ContainerStatuses.

Containers and VMs have common attributes such as Name, Image, etc. They also
differ in many aspects. For example WorkingDir, Command etc are applicable
to containers, but not VMs. Likewise, fields such as VMFlavor is applicable to
VMs and not Containers.

We could add a VirtualMachine struct that duplicates the common fields, but
existing code wouldn't work without siginficant code changes - something like:
```text
if VM then
  Start(Pod.Spec.VM.Image)
else
  Start(Pod.Spec.Container.Image)
endif
```

We need a way to define the Container and VirtualMachine structs without the
ugly if-else as shown above, and ideally with minimal code changes.

### Goals

* Primary: Identify all the fields that are common to Container and VM types.
* Primary: Existing code continues to work (No regression with Pods that
  describe Container workloads.
* Primary: Existing code works for VMs if no VM specific logic is needed.

### Non-Goals


## Proposal

### API Changes

A good approach that allows existing code to work involves refactoring shared
fields into a common struct, and embed the common struct Container and
VirtualMachine structs.

```text
type CommonInfo struct {
       // Name of the container specified as a DNS_LABEL.
       // Each container in a pod must have a unique name (DNS_LABEL).
       // Cannot be updated.
       Name string
       // Required.
       Image string
}

type VirtualMachine struct {
       // Common information
       CommonInfo
       // VM flavor
       VmFlavor string
}

// Container represents a single container that is expected to be run on the host.
type Container struct {
        // Common information
        CommonInfo
        // +optional
        Command []string
        // +optional
        Args []string
        // Optional: Defaults to Docker's default.
        // +optional
        WorkingDir string
	...
	...
}
```

This leverages a feature of golang which allows CommonInfo to become a field
in Container and VirtualMachine structs, whithout needing to change usage of
the common fields in existing code. For e.g, the following code does not
need to be updated:

```text
    glog.Infof("Container name is: %s", pod.Spec.Container[0].Name)
```

This also allows current Pod specification YAMLs to continue to work without
any change. For example, the below works end-to-end:

```text
# cat 1job2do.yaml 
apiVersion: batch/v1
kind: Job
metadata:
  name: 1job2do
spec:
  template:
    spec:
      restartPolicy: OnFailure
      containers:
      - name: stress
        image: skiibum/ubuntu-stress:18.10
        imagePullPolicy: IfNotPresent
        command: ["tail", "-f", "/dev/null"]
        workingDir: /var/log
```

One issue with this modification is that initializers that reference fields
from CommonInfo fail to compile. The following is disallowed.

```text
    c := v1.Container{
            Name: name,
            WorkingDir: "/root",
            Image: imageString
         }
```

They have to be modified to be initialized as below:

```text
    c := v1.Container{
            CommonInfo: v1.CommonInfo{
                Name: name,
                Image: imageString
            },
	    WorkingDir: "/tmp"
         }
```

There are around 900 such initializations in the current Kubernetes codebase,
most of them are in unit test code. We modify these initializations as shown
above.

### CommonInfo struct definition

We define CommonInfo struct as follows:

```text
type CommonInfo struct {
        // Name of the container specified as a DNS_LABEL.
        // Each container in a pod must have a unique name (DNS_LABEL).
        // Cannot be updated.
        Name string
        // Required.
        Image string
        // Compute resource requirements.
        // +optional
        Resources ResourceRequirements
        // +optional
        VolumeMounts []VolumeMount
        // Required: Policy for pulling images for this container
        ImagePullPolicy PullPolicy
}
```

### Container struct definition

We modify Container struct as follows:

```text
        // Common information
        CommonInfo
        // Optional: The docker image's entrypoint is used if this is not provided; cannot be updated.
        // Variable references $(VAR_NAME) are expanded using the container's environment.  If a variable
        // cannot be resolved, the reference in the input string will be unchanged.  The $(VAR_NAME) syntax
        // can be escaped with a double $$, ie: $$(VAR_NAME).  Escaped references will never be expanded,
        // regardless of whether the variable exists or not.
        // +optional
        Command []string
        // Optional: The docker image's cmd is used if this is not provided; cannot be updated.
        // Variable references $(VAR_NAME) are expanded using the container's environment.  If a variable
        // cannot be resolved, the reference in the input string will be unchanged.  The $(VAR_NAME) syntax
        // can be escaped with a double $$, ie: $$(VAR_NAME).  Escaped references will never be expanded,
        // regardless of whether the variable exists or not.
        // +optional
        Args []string
        // Optional: Defaults to Docker's default.
        // +optional
        WorkingDir string
        // +optional
        Ports []ContainerPort
        // List of sources to populate environment variables in the container.
        // The keys defined within a source must be a C_IDENTIFIER. All invalid keys
        // will be reported as an event when the container is starting. When a key exists in multiple
        // sources, the value associated with the last source will take precedence.
        // Values defined by an Env with a duplicate key will take precedence.
        // Cannot be updated.
        // +optional
        EnvFrom []EnvFromSource
        // +optional
        Env []EnvVar
        // volumeDevices is the list of block devices to be used by the container.
        // This is a beta feature.
        // +optional
        VolumeDevices []VolumeDevice
        // +optional
        LivenessProbe *Probe
        // +optional
        ReadinessProbe *Probe
        // +optional
        Lifecycle *Lifecycle
        // Required.
        // +optional
        TerminationMessagePath string
        // +optional
        TerminationMessagePolicy TerminationMessagePolicy
        // Optional: SecurityContext defines the security options the container should be run with.
        // If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
        // +optional
        SecurityContext *SecurityContext

        // Variables for interactive containers, these have very specialized use-cases (e.g. debugging)
        // and shouldn't be used for general purpose containers.
        // +optional
        Stdin bool
        // +optional
        StdinOnce bool
        // +optional
        TTY bool

```

#### Container YAML definition

No change required. Current YAML specs should continue to just work.

### VirtualMachine struct definition

We define VirtualMachine struct as follows:

```text
type VirtualMachine struct {
        // Common information
        CommonInfo
        // VM flavor
        VmFlavor string
	... TBD ...
}
```

#### VM YAML definition

```text
apiVersion: batch/v1
kind: Job
spec:
  template:
    spec:
      restartPolicy: OnFailure
      virtualMachine:
      - name: vm1
        vmFlavor: "ARM64"
        image: <someURL>/vm1.img
        resources:
          limits:
            cpu: "1"
            memory: "2Gi"
          requests:
            cpu: "1"
            memory: "2Gi"
```

## Affected Components

Pod v1 core API:
* create CommonInfo struct and embed in Container and VirtualMachine structs,
* modify initialization code to compile and run without regressions.

Kubelet:
* TBD

Scheduler:
* TBD

Controllers:
* TBD

Other components:
* TBD

## Affected Files (Files that use v1.Container{} initializer)
|File name| Number of occurrences|
|---------|----------------------|
|cmd/kubeadm/app/util/staticpod/utils_test.go|2|
|cmd/kubeadm/app/util/staticpod/utils.go|1|
|cmd/kubeadm/app/phases/upgrade/compute_test.go|1|
|cmd/kubeadm/app/phases/upgrade/prepull.go|1|
|cmd/kubeadm/app/phases/selfhosting/podspec_mutation_test.go|10|
|cmd/kubeadm/app/phases/controlplane/manifests.go|3|
|cmd/kubeadm/app/phases/etcd/local.go|1|
|staging/src/k8s.io/client-go/examples/create-update-delete-deployment/main.go|1|
|staging/src/k8s.io/sample-controller/controller.go|1|
|staging/src/k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle/admission_test.go|1|
|test/images/agnhost/webhook/patch_test.go|2|
|test/e2e_node/container_log_rotation_test.go|1|
|test/e2e_node/security_context_test.go|6|
|test/e2e_node/image_id_test.go|1|
|test/e2e_node/runtime_conformance_test.go|1|
|test/e2e_node/volume_manager_test.go|2|
|test/e2e_node/node_problem_detector_linux.go|1|
|test/e2e_node/eviction_test.go|4|
|test/e2e_node/apparmor_test.go|1|
|test/e2e_node/dockershim_checkpoint_test.go|1|
|test/e2e_node/resource_collector.go|3|
|test/e2e_node/cpu_manager_test.go|1|
|test/e2e_node/log_path_test.go|2|
|test/e2e_node/docker_test.go|1|
|test/e2e_node/critical_pod_test.go|1|
|test/e2e_node/hugepages_test.go|2|
|test/e2e_node/pids_test.go|2|
|test/e2e_node/quota_lsci_test.go|1|
|test/e2e_node/garbage_collector_test.go|2|
|test/e2e_node/perf/workloads/npb_is.go|1|
|test/e2e_node/perf/workloads/npb_ep.go|1|
|test/e2e_node/perf/workloads/tf_wide_deep.go|1|
|test/e2e_node/summary_test.go|1|
|test/e2e_node/container_manager_test.go|3|
|test/e2e_node/pods_container_manager_test.go|5|
|test/e2e_node/device_plugin.go|1|
|test/integration/replicationcontroller/replicationcontroller_test.go|2|
|test/integration/defaulttolerationseconds/defaulttolerationseconds_test.go|1|
|test/integration/scale/scale_test.go|1|
|test/integration/garbagecollector/garbage_collector_test.go|2|
|test/integration/evictions/evictions_test.go|1|
|test/integration/auth/svcaccttoken_test.go|2|
|test/integration/auth/node_test.go|2|
|test/integration/daemonset/daemonset_test.go|2|
|test/integration/statefulset/util.go|1|
|test/integration/secrets/secrets_test.go|1|
|test/integration/scheduler/extender_test.go|1|
|test/integration/scheduler/taint_test.go|1|
|test/integration/scheduler/util.go|1|
|test/integration/scheduler/priorities_test.go|1|
|test/integration/scheduler/predicates_test.go|31|
|test/integration/scheduler/volume_binding_test.go|1|
|test/integration/serviceaccount/service_account_test.go|1|
|test/integration/cronjob/cronjob_test.go|1|
|test/integration/pods/pods_test.go|2|
|test/integration/master/synthetic_master_test.go|1|
|test/integration/master/kube_apiserver_test.go|1|
|test/integration/deployment/util.go|2|
|test/integration/apiserver/admissionwebhook/broken_webhook_test.go|1|
|test/integration/apiserver/admissionwebhook/reinvocation_test.go|2|
|test/integration/apiserver/apiserver_test.go|1|
|test/integration/quota/quota_test.go|2|
|test/integration/configmap/configmap_test.go|1|
|test/integration/replicaset/replicaset_test.go|2|
|test/integration/disruption/disruption_test.go|1|
|test/integration/volume/attach_detach_test.go|2|
|test/integration/client/dynamic_client_test.go|1|
|test/integration/client/client_test.go|6|
|test/utils/runners.go|6|
|test/soak/serve_hostnames/serve_hostnames.go|1|
|test/e2e/servicecatalog/podpreset.go|4|
|test/e2e/scheduling/predicates.go|1|
|test/e2e/scheduling/nvidia-gpus.go|2|
|test/e2e/scheduling/ubernetes_lite.go|2|
|test/e2e/scheduling/priorities.go|1|
|test/e2e/scheduling/equivalence_cache_predicates.go|1|
|test/e2e/scheduling/taints.go|3|
|test/e2e/framework/pod/resource.go|3|
|test/e2e/framework/statefulset_utils.go|1|
|test/e2e/framework/ingress/ingress_utils.go|1|
|test/e2e/framework/pv_util.go|3|
|test/e2e/framework/service_util.go|3|
|test/e2e/framework/networking_utils.go|2|
|test/e2e/framework/util.go|5|
|test/e2e/framework/deployment/fixtures.go|2|
|test/e2e/framework/job/fixtures.go|1|
|test/e2e/framework/replicaset/fixtures.go|1|
|test/e2e/framework/rc_util.go|2|
|test/e2e/framework/volume/fixtures.go|3|
|test/e2e/auth/audit_dynamic.go|2|
|test/e2e/auth/pod_security_policy.go|1|
|test/e2e/auth/node_authz.go|1|
|test/e2e/auth/audit.go|1|
|test/e2e/auth/node_authn.go|1|
|test/e2e/auth/service_accounts.go|3|
|test/e2e/auth/metadata_concealment.go|1|
|test/e2e/network/networking_perf.go|2|
|test/e2e/network/scale/ingress.go|1|
|test/e2e/network/kube_proxy.go|4|
|test/e2e/network/service.go|1|
|test/e2e/network/dns_common.go|5|
|test/e2e/network/no_snat.go|2|
|test/e2e/gke_local_ssd.go|1|
|test/e2e/network/network_policy.go|3|
|test/e2e/storage/ephemeral_volume.go|1|
|test/e2e/storage/persistent_volumes-local.go|1|
|test/e2e/storage/persistent_volumes.go|1|
|test/e2e/storage/drivers/in_tree.go|1|
|test/e2e/storage/utils/utils.go|2|
|test/e2e/storage/detach_mounted.go|1|
|test/e2e/storage/empty_dir_wrapper.go|3|
|test/e2e/storage/vsphere/vsphere_utils.go|2|
|test/e2e/storage/testsuites/volumes.go|1|
|test/e2e/storage/testsuites/volume_io.go|2|
|test/e2e/storage/testsuites/subpath.go|5|
|test/e2e/storage/testsuites/provisioning.go|1|
|test/e2e/storage/csi_mock_volume.go|2|
|test/e2e/storage/regional_pd.go|1|
|test/e2e/storage/volume_provisioning.go|1|
|test/e2e/instrumentation/monitoring/custom_metrics_deployments.go|4|
|test/e2e/instrumentation/monitoring/accelerator.go|1|
|test/e2e/instrumentation/logging/generic_soak.go|1|
|test/e2e/instrumentation/logging/utils/logging_pod.go|2|
|test/e2e/common/kubelet.go|4|
|test/e2e/common/container_probe.go|3|
|test/e2e/common/downward_api.go|4|
|test/e2e/common/security_context.go|5|
|test/e2e/common/projected_downwardapi.go|2|
|test/e2e/common/docker_containers.go|1|
|test/e2e/common/runtime.go|7|
|test/e2e/common/downwardapi_volume.go|5|
|test/e2e/common/expansion.go|10|
|test/e2e/common/privileged.go|1|
|test/e2e/common/projected_combined.go|1|
|test/e2e/common/projected_secret.go|4|
|test/e2e/common/secrets.go|2|
|test/e2e/common/projected_configmap.go|5|
|test/e2e/common/configmap.go|2|
|test/e2e/common/secrets_volume.go|6|
|test/e2e/common/container.go|1|
|test/e2e/common/pods.go|11|
|test/e2e/common/kubelet_etc_hosts.go|2|
|test/e2e/common/host_path.go|1|
|test/e2e/common/runtimeclass.go|1|
|test/e2e/common/empty_dir.go|2|
|test/e2e/common/init_container.go|8|
|test/e2e/common/sysctl.go|1|
|test/e2e/common/lifecycle_hook.go|2|
|test/e2e/scalability/density.go|1|
|test/e2e/common/configmap_volume.go|8|
|test/e2e/upgrades/secrets.go|1|
|test/e2e/upgrades/configmaps.go|1|
|test/e2e/upgrades/apps/daemonsets.go|1|
|test/e2e/upgrades/sysctl.go|1|
|test/e2e/node/security_context.go|1|
|test/e2e/node/kubelet.go|1|
|test/e2e/node/pre_stop.go|3|
|test/e2e/node/events.go|1|
|test/e2e/node/mount_propagation.go|1|
|test/e2e/node/pod_gc.go|1|
|test/e2e/node/pods.go|2|
|test/e2e/windows/memory_limits.go|1|
|test/e2e/windows/volumes.go|2|
|test/e2e/windows/density.go|1|
|test/e2e/windows/hybrid_network.go|1|
|test/e2e/windows/gmsa.go|1|
|test/e2e/kubectl/portforward.go|1|
|test/e2e/apps/rc.go|2|
|test/e2e/apps/statefulset.go|1|
|test/e2e/apps/network_partition.go|1|
|test/e2e/apps/daemon_set.go|1|
|test/e2e/apps/disruption.go|3|
|test/e2e/apps/replica_set.go|2|
|test/e2e/apps/cronjob.go|1|
|test/e2e/apimachinery/crd_conversion_webhook.go|1|
|test/e2e/apimachinery/chunking.go|1|
|test/e2e/apimachinery/table_conversion.go|2|
|test/e2e/apimachinery/webhook.go|5|
|test/e2e/apimachinery/aggregator.go|1|
|test/e2e/apimachinery/generated_clientset.go|2|
|test/e2e/apimachinery/namespace.go|1|
|test/e2e/apimachinery/resource_quota.go|4|
|test/e2e/apimachinery/garbage_collector.go|3|
|plugin/pkg/admission/exec/admission_test.go|1|
|docs/design-proposals/apiserver/VM-Container-Common.md|2|
|pkg/apis/core/v1/defaults_test.go|23|
|pkg/apis/core/v1/conversion_test.go|1|
|pkg/apis/core/v1/helper/qos/qos_test.go|14|
|pkg/apis/core/v1/validation/validation_test.go|4|
|pkg/apis/extensions/v1beta1/defaults_test.go|1|
|pkg/apis/apps/v1/defaults_test.go|1|
|pkg/apis/apps/v1beta2/defaults_test.go|1|
|pkg/security/apparmor/validate_test.go|3|
|pkg/security/podsecuritypolicy/provider_test.go|1|
|pkg/scheduler/scheduler_test.go|2|
|pkg/scheduler/util/utils_test.go|5|
|pkg/scheduler/core/generic_scheduler_test.go|7|
|pkg/scheduler/nodeinfo/node_info_test.go|15|
|pkg/scheduler/internal/cache/cache_test.go|5|
|pkg/scheduler/algorithm/predicates/predicates_test.go|15|
|pkg/scheduler/algorithm/priorities/resource_limits_test.go|4|
|pkg/scheduler/algorithm/priorities/balanced_resource_allocation_test.go|7|
|pkg/scheduler/algorithm/priorities/most_requested_test.go|4|
|pkg/scheduler/algorithm/priorities/requested_to_capacity_ratio_test.go|1|
|pkg/scheduler/algorithm/priorities/least_requested_test.go|3|
|pkg/scheduler/algorithm/priorities/image_locality_test.go|3|
|pkg/scheduler/algorithm/priorities/metadata_test.go|3|
|pkg/controller/statefulset/stateful_set_utils_test.go|2|
|pkg/controller/controller_utils_test.go|2|
|pkg/controller/controller_ref_manager_test.go|1|
|pkg/controller/ttlafterfinished/ttlafterfinished_controller_test.go|1|
|pkg/controller/cronjob/utils_test.go|2|
|pkg/controller/cronjob/cronjob_controller_test.go|1|
|pkg/controller/daemon/daemon_controller_test.go|13|
|pkg/controller/daemon/util/daemonset_util_test.go|1|
|pkg/controller/resourcequota/resource_quota_controller_test.go|9|
|pkg/controller/deployment/util/deployment_util_test.go|2|
|pkg/controller/deployment/deployment_controller_test.go|1|
|pkg/controller/endpoint/endpoints_controller_test.go|2|
|pkg/controller/history/controller_history_test.go|1|
|pkg/controller/job/job_controller_test.go|1|
|pkg/controller/replicaset/replica_set_test.go|1|
|pkg/controller/volume/attachdetach/testing/testvolumespec.go|2|
|pkg/controller/podautoscaler/metrics/legacy_metrics_client_test.go|1|
|pkg/controller/podautoscaler/replica_calculator_test.go|1|
|pkg/controller/podautoscaler/horizontal_test.go|1|
|pkg/controller/podautoscaler/legacy_replica_calculator_test.go|1|
|pkg/controller/podautoscaler/legacy_horizontal_test.go|1|
|pkg/kubelet/prober/prober_test.go|5|
|pkg/kubelet/prober/common_test.go|2|
|pkg/kubelet/prober/prober_manager_test.go|5|
|pkg/kubelet/volumemanager/populator/desired_state_of_world_populator_test.go|8|
|pkg/kubelet/custommetrics/custom_metrics_test.go|2|
|pkg/kubelet/apis/podresources/server_test.go|2|
|pkg/kubelet/types/types_test.go|4|
|pkg/kubelet/util/manager/cache_based_manager_test.go|1|
|pkg/kubelet/kubelet_pods_windows_test.go|1|
|pkg/kubelet/config/http_test.go|9|
|pkg/kubelet/config/common_test.go|2|
|pkg/kubelet/config/file_linux_test.go|2|
|pkg/kubelet/config/apiserver_test.go|5|
|pkg/kubelet/config/config_test.go|5|
|pkg/kubelet/images/image_manager_test.go|1|
|pkg/kubelet/runonce_test.go|1|
|pkg/kubelet/qos/policy_test.go|8|
|pkg/kubelet/kubelet_pods_test.go|34|
|pkg/kubelet/lifecycle/handlers_test.go|13|
|pkg/kubelet/lifecycle/predicate_test.go|1|
|pkg/kubelet/kubelet_pods_linux_test.go|10|
|pkg/kubelet/server/server_test.go|1|
|pkg/kubelet/kuberuntime/security_context_test.go|1|
|pkg/kubelet/kuberuntime/labels_test.go|6|
|pkg/kubelet/kuberuntime/kuberuntime_gc_test.go|4|
|pkg/kubelet/kuberuntime/kuberuntime_container_linux_test.go|2|
|pkg/kubelet/kuberuntime/kuberuntime_container_test.go|3|
|pkg/kubelet/kuberuntime/kuberuntime_manager_test.go|10|
|pkg/kubelet/kuberuntime/kuberuntime_sandbox_test.go|1|
|pkg/kubelet/kuberuntime/helpers_test.go|2|
|pkg/kubelet/kuberuntime/kuberuntime_container.go|1|
|pkg/kubelet/status/generate_test.go|13|
|pkg/kubelet/status/status_manager_test.go|1|
|pkg/kubelet/container/ref_test.go|11|
|pkg/kubelet/container/helpers_test.go|19|
|pkg/kubelet/kubelet_test.go|22|
|pkg/kubelet/preemption/preemption_test.go|1|
|pkg/kubelet/mountpod/mount_pod_test.go|1|
|pkg/kubelet/secret/secret_manager_test.go|1|
|pkg/kubelet/cm/helpers_linux_test.go|24|
|pkg/kubelet/cm/cpumanager/cpu_manager_test.go|7|
|pkg/kubelet/cm/devicemanager/manager_test.go|3|
|pkg/kubelet/configmap/configmap_manager_test.go|1|
|pkg/kubelet/kubelet_resources_test.go|1|
|pkg/kubelet/eviction/helpers_test.go|53|
|pkg/kubelet/eviction/eviction_manager_test.go|3|
|pkg/api/v1/pod/util_test.go|15|
|pkg/api/v1/resource/helpers_test.go|2|
|pkg/kubectl/cmd/util/helpers_test.go|6|
|pkg/kubectl/cmd/get/customcolumn_test.go|1|
|pkg/kubectl/cmd/attach/attach_test.go|3|
|pkg/kubectl/cmd/logs/logs_test.go|1|
|pkg/kubectl/cmd/exec/exec_test.go|1|
|pkg/kubectl/cmd/set/set_resources_test.go|15|
|pkg/kubectl/cmd/set/set_env_test.go|16|
|pkg/kubectl/cmd/set/helper.go|2|
|pkg/kubectl/cmd/set/set_serviceaccount_test.go|4|
|pkg/kubectl/cmd/set/set_image_test.go|34|
|pkg/kubectl/cmd/create/create_cronjob_test.go|2|
|pkg/kubectl/cmd/create/create_job_test.go|4|
|pkg/kubectl/cmd/create/create_job.go|1|
|pkg/kubectl/cmd/create/create_cronjob.go|1|
|pkg/kubectl/cmd/portforward/portforward_test.go|22|
|pkg/kubectl/util/pod_port_test.go|2|
|pkg/kubectl/util/service_port_test.go|7|
|pkg/kubectl/polymorphichelpers/portsforobject_test.go|4|
|pkg/kubectl/polymorphichelpers/logsforobject_test.go|7|
|pkg/kubectl/polymorphichelpers/protocolsforobject_test.go|5|
|pkg/kubectl/describe/versioned/describe_test.go|29|
|pkg/kubectl/rollback_test.go|1|
|pkg/kubectl/rolling_updater_test.go|4|
|pkg/kubectl/generate/versioned/deployment.go|2|
|pkg/kubectl/generate/versioned/run.go|2|
|pkg/kubectl/generate/versioned/run_test.go|20|
|pkg/kubectl/generate/versioned/deployment_test.go|1|
|pkg/volume/plugins.go|1|
|pkg/volume/util/nested_volumes_test.go|7|
|pkg/volume/util/operationexecutor/operation_executor_test.go|2|
|pkg/volume/plugins_test.go|1|
|pkg/volume/emptydir/empty_dir_test.go|6|


## Risks and Mitigations

1. When we migrate to higher versions of Kubernetes to integrate newer
   features, we would need to modify code that initializes promoted fields.
   We write a simple parser tool that can find and rewrite initialization of
   promoted fields from embedded structs, and avoid time-consuming manual
   work.

## Graduation Criteria

TODO

## Implementation History

- 2019-07-18 - initial design document

