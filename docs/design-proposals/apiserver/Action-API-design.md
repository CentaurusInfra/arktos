---
title: Action API Design for CloudFabric
authors:
  - "@vinaykul"
  - "@yb01"
---

# Action API Design for CloudFabric

## Table of Contents

   * [Action API for CloudFabric](#action-api-for-cloud-fabric)
      * [Table of Contents](#table-of-contents)
      * [Motivation](#motivation)
         * [Goals](#goals)
         * [Non-Goals](#non-goals)
      * [Proposal](#proposal)
         * [API Changes](#api-changes)
         * [API Structs](#api-structs)
         * [Action Object Persistence](#action-object-persistence)
         * [Action Object Watch](#action-object-watch)
         * [Flow Control](#flow-control)
         * [Affected Components](#affected-components)

## Motivation

One of the goals of CloudFabric is to unify management of Pods and VMs and use
the Kubernetes way of doing things. However, unlike Pods, VMs have some
requrements that are not applicable to Pods. e.g. Reboot, Snapshot,...

These translate to actions that user/ECS would invoke on a VM workload/pod.
This proposal aims to outline a design for user to invoke actions for VMs in
particular, and on other cluster resources in general.

### Goals

* Primary: Define a mechanism for user/ECS to invoke specific actions on a VM
  Pod that cannot be defined in the Pod Spec (desired state)
* Primary: Allow users to query the status of actions by specifying the action
  UID that was created.
* Secondary: Allow users to query the status of actions by specifying the VM
  Pod(s) for which they were invoked.

### Non-Goals

The explicit non-goal of this design is not to specify how actions are
implemented. It is upto the node-agent or other entity responsible for
executing the action, and posting the outcome of the execution to the action
status.

## Proposal

We use the Kubernetes subresource pattern to let user/ECS specify or invoke
actions on a VM Pod. Subresource pattern allows modification of a specific
part of a resource e.g. pods/binding subresource allows scheduler to set the
NodeName field in PodSpec (of pod resource).

We create a new subresource of Pod named **pods/action** which user/ECS
will invoke in order to request CloudFabric infrastructure to take a specified action.

Action subresource for a Pod will be invoked by POST to a URL that specifies
the namespace, the podID, and the action data. Action data contains the
actionName which describes the action desired, and action specific paramters
in JSON format.

API Server responds with action UID if the requested action passes validation
and was accepted. If not, it responds with an appropriate error code, reason,
and message.

The above POST results in a call to the action subresource handler in API
Server which receives the action specification. API Server handles the
specified action by creating an **Action** object which is persisted in
store/etcd.

Action object is a top level Kubernetes resource similar to Pod or Service.
This allows interested components such as Node agenet or Kubelet to create a
watch for Action objects, with filters to allow them to only receive those
actions that they care about. The component that handles the action can then
take steps to implement that action, and post the status back to API Server.

### API Changes

* We add a new top level Kubernetes resource named '**actions**' with path
'**/api/v1/actions**'.
  - This allows user/ECS to list and get actions, and optionally create
    actions for a Pod or other cluster resource.
    - Creating Action this way takes JSON object of Kind 'Action'. Name of the
      action describes what the user wants to do, and the Spec of the action
      provides data specific to the action requested.
  - This allows Node agent / Kubelet or other intersted component to watch
    for Actions by specifying filters.
* We introduce a pods subresource named '**pods/action**' that allows user to
specify a desired action on Pod object. This is the recommended way for user
have the infrastructure create Action object for a specific action for a (VM)
Pod. Programmatically, this is done by invoking Action() subresource on Pod.

Having pods/action subresource allows user/ECS to easily create actions on a
specific Pod as illustrated in the below reboot example:
```
root@fw0000358:~/ActAlk# cat ../YML/reboot.json
{
  "apiVersion": "v1",
  "kind": "CustomAction",
  "actionName": "Reboot",
  "rebootParams": {
    "delayInSeconds": 10
  }
}

root@fw0000358:~/ActAlk# curl -X POST http://127.0.0.1:8001/api/v1/namespaces/default/pods/1pod/action -H "Content-Type: application/json" -d @../YML/reboot.json
```

Similarly, user/ECS can request snapshot of (VM) Pod as follows:
```
root@fw0000358:~/ActAlk# cat ../YML/snapshot.json
{
  "apiVersion": "v1",
  "kind": "CustomAction",
  "actionName": "Snapshot",
  "snapshotParams": {
    "snapshotLocation": "/var/tmp/"
  }
}

root@fw0000358:~/ActAlk# curl -X POST http://127.0.0.1:8001/api/v1/namespaces/default/pods/1pod/action -H "Content-Type: application/json" -d @../YML/snapshot.json
```

Having top level actions resource allows user/ECS to directly create and list
actions as illustrated below:
NOTE: This is not supported, it is here as historical reference.
```
root@fw0000358:~/ActAlk# cat ~/YML/action_reboot.json
{
  "apiVersion": "v1",
  "kind": "Action",
  "metadata": {
    "name": "reboot"
  },
  "spec": {
    "podAction": {
      "podName": "1pod",
      "rebootAction": {
        "delayInSeconds": 13
      }
    }
  }
}

root@fw0000358:~/ActAlk# curl -X POST http://127.0.0.1:8001/api/v1/actions -H "Content-Type: application/json" -d @../YML/action_reboot.json
{
  "kind": "Action",
  "apiVersion": "v1",
  "metadata": {
    "name": "reboot",
    "selfLink": "/api/v1/actions/reboot",
    "uid": "5f2fb321-9fcc-412e-bd2a-08041ab1075b",
    "resourceVersion": "338",
    "creationTimestamp": "2019-09-12T21:24:25Z"
  },
  "spec": {
    "podAction": {
      "podName": "1pod",
      "rebootAction": {
        "delayInSeconds": 13
      }
    }
  },
  "status": {

  }
}

root@fw0000358:~/ActAlk# curl http://127.0.0.1:8001/api/v1/actions
{
  "kind": "ActionList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/actions",
    "resourceVersion": "360"
  },
  "items": [
    {
      "metadata": {
        "name": "reboot",
        "selfLink": "/api/v1/actions/reboot",
        "uid": "5f2fb321-9fcc-412e-bd2a-08041ab1075b",
        "resourceVersion": "338",
        "creationTimestamp": "2019-09-12T21:24:25Z"
      },
      "spec": {
        "podAction": {
          "podName": "1pod",
          "rebootAction": {
            "delayInSeconds": 13
          }
        }
      },
      "status": {

      }
    }
  ]
}
```

### API Structs

Using pod subresource mechanism, user/ECS can specify supported actions by
specifying actionName, and parameters specific to the actionName. For e.g.
if actionName is 'reboot', then rebootParams should be specified, and will be
looked at by API server when creating Reboot action object.

The pattern of structs defined to handle actions posted by user/ECS via
subresource mechanism is illustrated below:
```
type RebootParams struct {
        DelayInSeconds int32
}

type SnapshotParams struct {
        SnapshotLocation string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomAction object - Specified when invoking action subresource for Pod
type CustomAction struct {
        metav1.TypeMeta
        // ObjectMeta describes the object to which this Action applies
        // +optional
        metav1.ObjectMeta

        // Name of the action e.g. Reboot, Snapshot, ...
        ActionName string

        // Action specific parameters
        RebootParams   RebootParams
        SnapshotParams SnapshotParams
}
```

The pattern for creating and storing Action objects in etcd is shown below:
```
type RebootAction struct {
        DelayInSeconds int32
}

type RebootStatus struct {
        RebootSuccessful bool
}

type SnapshotAction struct {
        SnapshotLocation string
}

type SnapshotStatus struct {
        SnapshotSizeInBytes int64
}

type PodAction struct {
        PodName        string
        PodID          string
        RebootAction   *RebootAction
        SnapshotAction *SnapshotAction
}

type PodActionStatus struct {
        PodName        string
        PodID          string
        RebootStatus   *RebootStatus
        SnapshotStatus *SnapshotStatus
}

type NodeAction struct {
        RebootAction *RebootAction
}

type NodeActionStatus struct {
        RebootStatus *RebootStatus
}

type ActionSpec struct {
        NodeName   string
        PodAction  *PodAction
        NodeAction *NodeAction
}

type ActionStatus struct {
        Complete         bool
        PodActionStatus  *PodActionStatus
        NodeActionStatus *NodeActionStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//  Action object - Created and persisted in etcd in response to invocation of pods/action subresource
type Action struct {
        metav1.TypeMeta
        // +optional
        metav1.ObjectMeta

        // Spec defines the Action desired by the caller.
        // +optional
        Spec ActionSpec

        // Status represents the current information about a pod. This data may not be up
        // to date.
        // +optional
        Status ActionStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActionList is a list of events.
type ActionList struct {
        metav1.TypeMeta
        // +optional
        metav1.ListMeta

        Items []Action
}
```

### Action Object Persistence

API Server uses the store interface to persist Action objects to etcd. Action
objects are cleaned up by a Action controller (details TBD) based on
completion status, after certain duration. (user configurable via configmap)

### Action Object Watch

Action objects can be watched by Node agent or other components interested in
the actions as illustrated below:
```
// NewSourceApiserver creates a config source that watches and pulls from the apiserver.
func NewSourceApiserver(c clientset.Interface, nodeName types.NodeName, updates chan<- interface{}) {
        lwActions := cache.NewListWatchFromClient(c.CoreV1(), "actions", metav1.NamespaceAll, fields.Everything())
        newSourceApiserverFromLWActions(lwActions, updates)
}

func newSourceApiserverFromLWActions(lw cache.ListerWatcher, updates chan<- interface{}) {
        send := func(objs []interface{}) {
                var actions []*v1.Action
                for _, o := range objs {
                        actions = append(actions, o.(*v1.Action))
                }
                updates <- kubetypes.PodUpdate{Actions: actions, Op: kubetypes.ACTION, Source: kubetypes.ApiserverSource}
        }
        r := cache.NewReflector(lw, &v1.Action{}, cache.NewUndeltaStore(send, cache.MetaNamespaceKeyFunc), 0)
        go r.Run(wait.NeverStop)
}

```

#### Action Flow Diagram

Below diagram illustrates how user/ECS invokes action for a Pod, and how
APIServer and Node agent (kubelet) work to see the action. Once node agent
completes executing the action, it writes the status of the action back to
etcd via APIServer

```text
                                 +---------+
                                 |         |
                                 |  etcd   |
                                 |         |
                                 +----^----+
                                      |
                                      | Create("ActionObject")
                                      |
 +-------+       subresource    +-----+-----+
 |       |  ../pods/foo/action  |           |
 |  ECS  |----------------------> APIServer |
 |       |                      |           |
 +-------+                      +-----^-----+
                                      |
                                      | Watch("action", nodeName=="foo")
                                      |
                                +-----+-----+                 +-------+
                                |           |                 |       |
                                |  Kubelet  +----------------->  VM   |
                                |  Node=foo |  Execute Action |  Pod  |
                                |           |                 |       |
                                +-----------+                 +-------+
```

### Affected Components

APISerer and registry:

Pod v1 core API:

Kubelet:

Scheduler:

Controllers:

Other components:

## Implementation History

- 2019-11-12 - initial draft design

