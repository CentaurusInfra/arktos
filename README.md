# Arktos



[![Go Report Card](https://goreportcard.com/badge/github.com/CentaurusInfra/arktos)](https://goreportcard.com/report/github.com/CentaurusInfra/arktos)
[![LICENSE](https://img.shields.io/badge/license-apache%202.0-green)](https://github.com/CentaurusInfra/arktos/blob/master/LICENSE)


## What is Arktos

Arktos is an open source project designed for large scale cloud compute infrastructure. It is evolved from the open source project [Kubernetes](https://github.com/kubernetes/kubernetes) codebase with core design changes. 

Arktos aims to be an open source solution to address key challenges of large-scale clouds, including system scalability, resource efficiency, multitenancy, edge computing, and the native support for the fast-growing modern workloads such as containers and serverless functions. 

## Architecture
![Architecture Diagram](https://raw.githubusercontent.com/CentaurusInfra/arktos/master/docs/design-proposals/arch/project_architecture.png)
## Key Features

### Large Scalability

Arktos achieves a scalable architecture by partitioning and scaling out components, including API Server, storage, controllers and data plane. The eventual goal of Arktos is to support 300K nodes with a single regional control plane.

### Multitenancy

Arktos implements a hard multitenancy model to meet the strict isolation requirement highly desired by public cloud environment. It's based on the virtual cluster idea and all isolations are transparent to tenants. Each tenant feels it's a dedicated cluster for them. 

### Unified Container/VM Orchestration

In addition to container orchestration, Arktos implements a built-in support for VMs. In Arktos a pod can contain either containers or a VM. They are scheduled the same way in a same resource pool. This enables cloud providers use a single converged stack to manage all cloud hosts.

### More Features

There are more features under development, such as cloud-edge scheduling, in-place vertical scaling, etc. Check out [the project introduction](https://docs.google.com/presentation/d/1PG1m27MYRh4kuq654W9HvdoZ5QDX9tWxoCMCfeOZUrE/edit#slide=id.g8a27d34398_8_0) for more information.


## Build Arktos

Arktos requires a few dependencies to build and run, and [a bash script](https://github.com/CentaurusInfra/arktos/tree/master/hack/setup-dev-node.sh) is provided to install them.

After the prerequisites are installed, you just need to clone the repo and run "make":

##### Note: you need to have a working [Go 1.13 environment](https://golang.org/doc/install). Go 1.14 and above is not supported yet.

```
mkdir -p $GOPATH/src/github.com
cd $GOPATH/src/github.com
git clone https://github.com/CentaurusInfra/arktos
cd arktos
make
```

## Run Arktos
The easiest way to run Arktos is to bring up a single-node cluster in your local development box:

```
cd $GOPATH/src/github.com/arktos
hack/arktos-up.sh
```

After the Arktos cluster is up, you can access the cluster with [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) tool just like what you do with a Kubernetes cluster. For example:

```
cd $GOPATH/src/github.com/arktos
cluster/kubectl.sh get nodes
```

To setup a multi-node cluster, please refer to [Arktos Cluster Setup Guide](docs/setup-guide/multi-node-dev-cluster.md). And [this guide](docs/setup-guide/arktos-apiserver-partition.md) gives detailed instructions if you want to enable partitions in the cluster.

## Community Meetings 

 Pacific Time: **Tuesday, 6:00PM PT (Biweekly), starting 7/20/2021.** Please join our slack channel/email group for the latest update. 

 Resources: [Meeting Link](https://futurewei.zoom.us/j/92636035970) | [Meeting Summary](https://docs.google.com/document/d/1Cwpp44pQhMZ_MQ4ebralDHCt0AZHqhSkj14kNAzA7lY/edit#)

## Documents and Support

The [design document folder](https://github.com/CentaurusInfra/arktos/tree/master/docs/design-proposals/) contains the detailed design of already implemented features, and also some thoughts for planned features.

The [user guide folder](https://github.com/CentaurusInfra/arktos/tree/master/docs/user-guide/) provides information about these features from users' perspective.

To report a problem, please [create an issue](https://github.com/CentaurusInfra/arktos/issues) in the project repo. 

To ask a question, here is [the invitation](https://join.slack.com/t/arktosworkspace/shared_invite/zt-cmak5gjq-rBxX4vX2TGMyNeU~jzAMLQ) to join [Arktos slack channels](http://arktosworkspace.slack.com/). You can also post in the [email group](https://groups.google.com/forum/#!forum/arktos-user), or [create an issue](https://github.com/CentaurusInfra/arktos/issues) of question type in the repo.
