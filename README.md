# Arktos

test CI 505

[![Go Report Card](https://goreportcard.com/badge/github.com/futurewei-cloud/arktos)](https://goreportcard.com/report/github.com/futurewei-cloud/arktos)
[![LICENSE](https://img.shields.io/badge/license-apache%202.0-green)](https://github.com/futurewei-cloud/arktos/blob/master/LICENSE)


## What Arktos is

Arktos is an open source cluster management system designed for large scale clouds. It is evolved from the open source [Kubernetes](https://github.com/kubernetes/kubernetesh) v1.15 codebase with some fundamental improvements. 

Arktos aims to be an open source solution to address key challenges of large scale clouds, including system scalability, resource efficiency, multitenancy, cross-AZ resiliency, and the native support for the fast-growing modern workloads such as containers and serverless functions. 

----

## Key Features of Arktos


### Large Scalability

Arktos achieves a scalable architecture by partitioning and replicating system components, including API Server, storage, controllers and data plane. The eventual goal of Arktos is to support 100K nodes with a single cross-AZ control plane.

### Multitenancy

Arktos implements a hard multi-tenancy model to meet the strict isolation requirement highly desired by a typical public cloud environment. Tenant is a built-in object in the system, and some flexible multi-tenancy models can also be supported with customized resource authorization rules.

### Native VM Support

In addition to container orchestration, Arktos implements a built-in support for VMs. In Arktos a pod can contain either containers or a VM. They are scheduled the same way and launched by node agent using different runtime servers. VMs and containers are both the first-class citizens in Arktos.


### More Planned Features

More features are planned but not started yet, including intelligent scheduling, in-place resource update, QoS enforcement, etc.


## Build Arktos


To build Arktos, you just need to clone the repo and run "make":

##### Note: you need to have a working [Go 1.12 environment](https://golang.org/doc/install). Go 1.13 is not supported yet.

```
mkdir -p $GOPATH/src/github.com
cd $GOPATH/src/github.com
git clone https://github.com/futurewei-cloud/arktos
cd arktos
make
```

## Run Arktos
Arktos requires a few dependencies to run, and [a bash script](hack/setup-dev-node.sh) is provided to install them.

With the dependencies in place, to run a single-node Arktos cluster in your local development box:

```
cd $GOPATH/src/github.com/arktos
hack/arktos-up.sh
```

After the Arktos cluster is up, you can access the cluster with [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) tool just like what you do with a Kubernetes cluster. For example:

```
cd $GOPATH/src/github.com/arktos
cluster/kubectl.sh get nodes
```

For more complicated cluster setups, please refer to [Arktos Cluster Setup Guide](https://github.com/futurewei-cloud/arktos/blob/master/docs/setup-guide/multi-node-dev-cluster.md).   

## Setup Data Partitioned Arktos Environment

[Data Partitioned Arktos Environment Setting Up Guide](docs/setup-guide/arktos-apiserver-partition.md) gives detailed instruction on how to set up Arktos cluster with data partition

## Documents and Support

The [design document folder](https://github.com/futurewei-cloud/arktos/tree/master/docs/design-proposals/) contains the detailed design of already implemented features, and also some thoughts for planned features.

The [user guide folder](https://github.com/futurewei-cloud/arktos/tree/master/docs/user-guide/) provides information about these features from users' perspective.

To report a problem, please [create an issue](https://github.com/futurewei-cloud/arktos/issues) in the project repo. 

To ask a question, here is [the invitation](https://join.slack.com/t/arktosworkspace/shared_invite/zt-cmak5gjq-rBxX4vX2TGMyNeU~jzAMLQ) to join [Arktos slack channels](http://arktosworkspace.slack.com/). You can also post in the [email group](https://groups.google.com/forum/#!forum/arktos-user), or [create an issue](https://github.com/futurewei-cloud/arktos/issues) of question type in the repo.
