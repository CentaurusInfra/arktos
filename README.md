# Global Scheduler

[![Go Report Card](https://goreportcard.com/badge/github.com/futurewei-cloud/arktos)](https://goreportcard.com/report/github.com/futurewei-cloud/arktos)
[![LICENSE](https://img.shields.io/badge/license-apache%202.0-green)](https://github.com/futurewei-cloud/arktos/blob/master/LICENSE)

## What Global Scheduler is

Global Scheduler is an open source large scale cloud resource orchestration and scheduling platform.
It breaks resource boundary, extends resource limitation, and optimizes resource utilization among cloud data centers and edge sites.
It has a global view of all the DCs' available resource capacity and edge sites' available resource capacity, and intelligently schedules
VMs/Containers on a DC/edge site based on traffic patterns and optimal global resource utilization. It is evolved from the open source [Kubernetes](https://github.com/kubernetes/kubernetesh) v1.15 codebase with some fundamental redesign of the scheduler.

Global Scheduler aims to address key scheduling challenges of compute units (e.g. VM and containers) across
a large number of DC clouds and edge clouds---large scalability, low scheduling latency, global resource sharing, high resource utilization, 
application-performance-aware, etc.  

----

## Key Features of Global Scheduler


### Large Scalability

Global Scheduler achieves a scalable architecture by partitioning system components, such as API Server, storage, scheduler, etc. 
It achieves large scalability through an end-to-end dynamic geolocation and resource profile-based partition scheme which enables multiple schedulers running in a real concurrent mode. The eventual goal of Global Scheduler is to support 10K clusters with a single cross-AZ control plane.

### Low Scheduling Latency

Global Scheduler achieves low shceduling latency through the use of lock-free concurrent scheduling design.
It supports full parallelism without any inter-scheduler head of line blocking.  All the Schedulers operate completely in parallel and do not have to wait for each other.
Each scheduler has its own local cache for fast information retrieval.

### Intelligent Scheduling Algorithm

Global Scheduler implements a multi-dimension optimization model based scheduling algorithm. Its weight based scoring mechanism allows flexible scheduling policy. The scheudling algorithm design allows easy extension of more dimensions in the future

### High Resource Utilization

Global Scheduler achieves high resource utitlization through global view of resources across all the DCs and edge sites as well as a scheduling algorithm that considers resource equivalance which avoids depletion of one type of resource while leaving other types of resources wasted. 

### Application Aware Scheduling

The Global Scheduler Monitors each application’s input flow characteristics and automatically scale out/in
VMs/Containers or migrate the hosting VMs/Containers to a better geo-location to meet the application QOS requirement


## Build Global Scheduler


To build Global Scheduler, you just need to clone the repo and run "make":

##### Note: you need to have a working [Go 1.12 environment](https://golang.org/doc/install). Go 1.13 is not supported yet.

```
mkdir -p $GOPATH/src/github.com
cd $GOPATH/src/github.com
git clone https://github.com/futurewei-cloud/global-resource-scheduler
cd global-resource-scheduler
make
```

## Run Global Scheduler

To run a single-node Global Scheduler cluster in your local development box:

```
cd $GOPATH/src/github.com/global-resource-scheduler
hack/global-scheduler-up.sh
```

## Documents and Support

The [design document folder](https://github.com/futurewei-cloud/global-resource-scheduler/tree/master/docs/design-proposals/) contains the detailed design of already implemented features, and also some thoughts for planned features.

The [user guide folder](https://github.com/futurewei-cloud/global-resource-scheduler/tree/master/docs/user-guide/) provides information about these features from users' perspective.

To report a problem, please [create an issue](https://github.com/futurewei-cloud/global-resource-scheduler/issues) in the project repo. 

To ask a question, here is [the invitation](https://join.slack.com/t/arktosworkspace/shared_invite/zt-cmak5gjq-rBxX4vX2TGMyNeU~jzAMLQ) to join [Arktos slack channels](http://arktosworkspace.slack.com/). You can also post in the [email group](https://groups.google.com/forum/#!forum/arktos-user), or [create an issue](https://github.com/futurewei-cloud/arktos/issues) of question type in the repo.
