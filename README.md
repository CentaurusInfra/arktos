# Alkaid

<br/>


## What Alkaid is

Alkaid is an open source cluster management system designed for large scale clouds. It is evolved from the open source [Kubernetes](https://github.com/kubernetes/kubernetesh) v1.15 codebase with some fundamental improvements. 

Alkaid aims to be an open source solution to address key challenges of large scale clouds, including system scalability, resource efficiency, multitenancy, cross-AZ resiliency, and the native support for the fast-growing modern workloads such as containers and serverless functions. 

----

## Key Features of Alkaid


### Large Scalability

Alkaid achieves a scalable architecture by partitioning and replicating system components, including API Server, storage, controllers and data plane. The eventual goal of Alkaid is to support 100K nodes with a single cross-AZ control plane.

### Multitenancy

Alkaid implements a hard multi-tenancy model to meet the strict isolation requirement highly desired by a typical public cloud environment. Tenant is a built-in object in the system, and some flexible multi-tenancy models can also be supported with customized resource authorization rules.

### Native VM Support

In addition to container orchestration, Alkaid implements a built-in support for VMs. In Alkaid a pod can contain either containers or a VM. They are scheduled the same way and launched by node agent using different runtime servers. VMs and containers are both the first-class citizens in Alkaid.


### More Planned Features

More features are planned but not started yet, including intelligent scheduling, in-place resource update, QoS enforcement, etc.


## Build Alkaid


To build Alkaid, you just need to clone the repo and run "make":

##### Note: you need to have a working [Go environment](https://golang.org/doc/install).

```
mkdir -p $GOPATH/src/github.com
cd $GOPATH/src/github.com
git clone https://github.com/futurewei-cloud/alkaid
cd alkaid
make
```

## Run Alkaid

To run a single-node Alkaid cluster in your local development box:

```
cd $GOPATH/src/github.com/alkaid
hack/alkaid-up.sh
```

After the Alkaid cluster is up, you can access the cluster with [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) tool just like what you do with a Kubernetes cluster. For example:

```
cd $GOPATH/src/github.com/alkaid
cluster/kubectl.sh get nodes
```

For more complicated cluster setups, an automated deployment tool is not available yet. Some manually work is required before the tool is ready. You need to manually setup more worker nodes and register them to a same master server.   

## Documents and Support

The [design document folder](https://github.com/futurewei-cloud/alkaid/tree/master/docs/design-proposals) contains the detailed design of already implemented features, and also some thoughts for planned features.

The [user guide folder](https://github.com/futurewei-cloud/alkaid/tree/master/docs/user-guide) provides information about these features from users' perspective.

To report a problem, please [create an issue](https://github.com/futurewei-cloud/alkaid/issues) in the project repo. 

To ask a question, you can either chat with project members in the [slack channel](https://app.slack.com/client/TMNECBVT5/CRRUU7137), post in the [email group](https://groups.google.com/forum/#!forum/alkaid-user), or [create an issue](https://github.com/futurewei-cloud/alkaid/issues) of question type in the repo.
