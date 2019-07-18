---
title: VM and Container Common Fields for CloudFabric
authors:
  - "@vinaykul"
  - "@yb01"
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

TBD

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

