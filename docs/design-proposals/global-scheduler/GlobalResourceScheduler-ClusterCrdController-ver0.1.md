# Global Resource Scheduler – Cluster CRD

Oct-20-2020, Hong Zhang, Eunju Kim

## 1. Module Description

This module allows global resource scheduler register, deregister (or
unregister), get and list existing clusters.

-   Cluster consists of a group of nodes and nodes are host machines for
    hosting/running VM/Container PODs.

-   Namespace is a virtual cluster inside a Kubernetes cluster. There can be
    multiple namespaces inside a single Kubernetes cluster, and they are all
    logically isolated from each other.

-   Relationship among objects:

    Cluster \> Namespace (multiple name space in a cluster) \> Node (Host or VM)
    \> Pod \> Container

## 2. Requirements

-   In milestone1, The global-scheduler can interface with different types of
    clusters (openstack cluster, kubernetes cluster, etc.) through a generic
    southbound API. They share one single cluster resoruce data strcuture.

-   The cluster resource controller does not create a cluster. it just registers
    existing clusters which are already deployed through other approaches. The
    deployment of clusters themselves is out of the scope of this project.

![](/images/global-scheduler-cluster-diagram.png)

[Picture1] Flow of registration of a cluster

**2.1** [API
Implementation](https://github.com/futurewei-cloud/global-resource-scheduler/issues/23)

(1) CLI: command line APIs like kubectl

-   Register cluster, Unregister cluster, List cluster, Get cluster

(2) REST APIs: REST WEB APIs

-   Register cluster, Unregister cluster, List cluster, Get cluster

**2.2** [Create the CRD Controller code framework via
code-generator](https://github.com/futurewei-cloud/global-resource-scheduler/issues/25)

-   Global resource scheduler defines a cluster CRD definition.

**2.3** [Implement the controller
logic](https://github.com/futurewei-cloud/global-resource-scheduler/issues/27)

-   List/watch the cluster object and scheduler object through Informer.

-   Run consistent hashing algorithm to set the scheduler-cluster
    binding/association.

-   Save the binding to ETCD

## 3. Cluster CRD Definition & Data Structure

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
type Cluster struct {  
    apiversion:         string          //v1
    kind:               string          //Cluster
    Name:               string          
    Spec                ClusterSpec     // Spec is the custom resource spec 
} 
// MyResourceSpec is the spec for a MyResource resource. 
//This is where you would put your custom resource data
type ClusterSpec struct { 
    ipAdrress           string 
    GeoLocation         GeolocationInfo 
    Region              RegionInfo 
    Operator            OperatorInfo 
    flavors             []FlavorInfo 
    storage             []StorageSpec 
    EipCapacity         int64 
    CPUCapacity         int64 
    MemCapacity         int64 
    ServerPrice         int64 
    HomeScheduler       string 
} 
type FlavorInfo struct { 
    FlavorID            string 
    TotalCapacity       int64 
} 
type StorageSpec struct { 
    TypeID              string      //(sata, sas, ssd) 
    StorageCapacity     int64 
} 
type GeolocationInfo struct { 
    city                string 
    province            string 
    area                string 
    country             string 
} 
type RegionInfo { 
    region              string 
    AvailabilityZone    string 
} 
type OperatorInfo { 
    operator            string 
}
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

## 4. APIs Design

**4.1 CLI APIs**

-   register cluster -filename FILENAME

    `Example: register -filename ./cluster1.yaml`

-   unregister cluster -name CLUSTERNAME

    `Example: unregister -name cluster1`

-   unregister cluster -id CLUSTERID

    `Example: unregister -id 3dda2801-d675-4688-a63f-dcda8d327f51`

-   list clusters

    `Example: list clusters`

-   get cluster -name CLUSTERNAME

    `Example: get cluster -name cluster1`

-   get cluster -id CLUSTERID

    `Example: get cluster -id 3dda2801-d675-4688-a63f-dcda8d327f51`

**4.2 REST APIs & Error Codes Design**

| **Group** | **API Name**            | **Method** | **Request**                                      |
|-----------|-------------------------|------------|--------------------------------------------------|
| Cluster   | registerCluster         | POST       | /globalscheduler/v1/clusters                     |
|           | unregisterClusterById   | DELETE     | /globalscheduler/v1/clusters/id/{cluster_id}     |
|           | unregisterClusterByName | DELETE     | /globalscheduler/v1/clusters/name/{cluster_name} |
|           | listCluster             | GET        | /globalscheduler/v1/clusters                     |
|           | getClusterById          | GET        | /globalscheduler/v1/clusters/id/{cluster_id}     |
|           | getClusterByName        | GET        | /globalscheduler/v1/clusters/name/{cluster_name} |

(1) Register Cluster

-   Method: POST

-   Request: /globalscheduler/v1/clusters

-   Request Parameter:

-   Response: cluster profile

    -   Normal response codes: 201

    -   Error response codes: 400, 409, 412, 500, 503

-   Example

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Request: 
    http://127.0.0.1:8080/globalscheduler/v1/clusters 
Body: 
    {   
        "cluster_profile": { 
            "cluster_name": "cluster1", 
            “cluster_spec”: { 
                “ipAdrress”: “10.0.0.3”, 
                “GeoLocation”: { “city”: “Bellevue”, “province”: “Washington”, “area”: “West”, “country”: “US” }, 
                “Region”: { “region”: “us-west”, “AvailabilityZone”: “us-west-1” }, 
                “Operator”: { “operator”: “globalscheduler”, }, 
                “flavors”: [ {“FlavorID”: “small”, “TotalCapacity”: 5}, { “FlavorID”: “medium”, “TotalCapacity”: 10}, { “FlavorID”: “large”, “TotalCapacity”: 20}, { “FlavorID”: “xlarge”,“TotalCapacity”: 10}, { “FlavorID”: “2xlarge”, “TotalCapacity”: 5 }], 
                “storage”: [ {“TypeID”: “sata”, “StorageCapacity”: 2000}, { “TypeID”: “sas”, “StorageCapacity”: 1000}, { “TypeID”: “ssd”, “StorageCapacity”: 3000}, }], 
                “EipCapacity”: 3, 
                “CPUCapacity”: 8, 
                “MemCapacity”: 256, 
                “ServerPrice”: 10, 
                “HomeScheduler”: “scheduler1” 
            } 
        } 
    } 

Response: 
    { "cluster_profile": { 
        "cluster_id": "3dda2801-d675-4688-a63f-dcda8d327f51", 
        "cluster_name": "cluster1", 
        “cluster_spec”: { 
            “ipAdrress”: “10.0.0.3”, 
            “GeoLocation”: { “city”: “Bellevue”, “province”: “Washington”, “area”: “West”, “country”: “US” }, 
            “Region”: { “region”: “us-west”, “AvailabilityZone”: “us-west-1” }, 
            “Operator”: { “operator”: “globalscheduler”, }, 
            “flavors”: [ {“FlavorID”: “small”, “TotalCapacity”: 5}, { “FlavorID”: “medium”, “TotalCapacity”: 10}, { “FlavorID”: “large”, “TotalCapacity”: 20}, { “FlavorID”: “xlarge”,“TotalCapacity”: 10}, { “FlavorID”: “2xlarge”, “TotalCapacity”: 5 }], 
            “storage”: [ {“TypeID”: “sata”, “StorageCapacity”: 2000}, { “TypeID”: “sas”, “StorageCapacity”: 1000}, { “TypeID”: “ssd”, “StorageCapacity”: 3000}, }], 
            “EipCapacity”: 3, 
            “CPUCapacity”: 8, 
            “MemCapacity”: 256, 
            “ServerPrice”: 10, 
            “HomeScheduler”: “scheduler1” 
            } 
        } 
    } 
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

(2) Unregister Cluster By Id

-   Method: DELETE

-   Request: /globalscheduler/v1/clusters/id/{cluster_id}

-   Request Parameter: \@PathVariable String cluster_id

-   Response: cluster_id

    -   Normal response codes: 200

    -   Error response codes: 400, 412, 500

-   Example

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Request: 
    http://127.0.0.1:8080/globalscheduler/v1/clusters/3dda2801-d675-4688-a63f-dcda8d327f51

Response: 
    deleted: 3dda2801-d675-4688-a63f-dcda8d327f50 
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

(3) Unregister Cluster By Name

-   Method: DELETE

-   Request: /globalscheduler/v1/clusters/name/{cluster_name}

-   Request Parameter: \@PathVariable String cluster_name

-   Response: cluster_name

    -   Normal response codes: 200

    -   Error response codes: 400, 412, 500

-   Example

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Request: 
    http://127.0.0.1:8080/globalscheduler/v1/clutsers/name/cluster1

Response: 
    Deleted: cluster1
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

(4) List Clusters

-   Method: GET

-   Request: v1/clusters

-   Request Parameter:

-   Response: clusters list

    -   Normal response codes: 200

    -   Error response codes: 400, 409, 412, 500, 503

-   Example

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Request: 
    http://127.0.0.1:8080/globalscheduler/v1/clusters

Response: 
    { 
        [ 
            { 
                "cluster_id": "3dda2801-d675-4688-a63f-dcda8d327f51", 
                "cluster_name": "cluster1", “cluster_spec”: {…} 
            }, 
            { "cluster_id": "3dda2801-d675-4688-a63f-dcda8d327f52", 
                "cluster_name": "cluster2", “cluster_spec”: {…} 
            },
            …
        ] 
    } 
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

(5) Get ClusterById

-   Method: GET

-   Request: v1/clusters/id/{cluster_id}

-   Request Parameter: \@PathVariable String cluster_id

-   Response: cluster profile

    -   Normal response codes: 200

    -   Error response codes: 400, 412, 500

-   Example

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Request: 
    http://127.0.0.1:8080/globalscheduler/v1/clusters/3dda2801-d675-4688-a63f-dcda8d327f50 

Response: 
    { "cluster_profile": { 
        "cluster_id": "3dda2801-d675-4688-a63f-dcda8d327f51", 
        "cluster_name": "cluster1", 
        “cluster_spec”: { 
            “ipAdrress”: “10.0.0.3”, 
            “GeoLocation”: { “city”: “Bellevue”, “province”: “Washington”, “area”: “West”, “country”: “US” }, 
            “Region”: { “region”: “us-west”, “AvailabilityZone”: “us-west-1” }, 
            “Operator”: { “operator”: “globalscheduler”, }, 
            “flavors”: [ {“FlavorID”: “small”, “TotalCapacity”: 5}, { “FlavorID”: “medium”, “TotalCapacity”: 10}, { “FlavorID”: “large”, “TotalCapacity”: 20}, { “FlavorID”: “xlarge”,“TotalCapacity”: 10}, { “FlavorID”: “2xlarge”, “TotalCapacity”: 5 }], 
            “storage”: [ {“TypeID”: “sata”, “StorageCapacity”: 2000}, { “TypeID”: “sas”, “StorageCapacity”: 1000}, { “TypeID”: “ssd”, “StorageCapacity”: 3000}, }], 
            “EipCapacity”: 3, 
            “CPUCapacity”: 8, 
            “MemCapacity”: 256, 
            “ServerPrice”: 10, 
            “HomeScheduler”: “scheduler1” 
            } 
        } 
    } 
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

(6) Get Cluster By Name

-   Method: GET

-   Request: /clusters/name/{cluster_name}

-   Request Parameter: \@PathVariable String cluster_name

-   Response: cluster profile

    -   Normal response codes: 200

    -   Error response codes: 400, 412, 500

-   Example

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Request: 
    http://127.0.0.1:8080/globalscheduler/v1/clusters/cluster1 
Response: 
    { "cluster_profile": { 
        "cluster_id": "3dda2801-d675-4688-a63f-dcda8d327f51", 
        "cluster_name": "cluster1", 
        “cluster_spec”: { 
            “ipAdrress”: “10.0.0.3”, 
            “GeoLocation”: { “city”: “Bellevue”, “province”: “Washington”, “area”: “West”, “country”: “US” }, 
            “Region”: { “region”: “us-west”, “AvailabilityZone”: “us-west-1” }, 
            “Operator”: { “operator”: “globalscheduler”, }, 
            “flavors”: [ {“FlavorID”: “small”, “TotalCapacity”: 5}, { “FlavorID”: “medium”, “TotalCapacity”: 10}, { “FlavorID”: “large”, “TotalCapacity”: 20}, { “FlavorID”: “xlarge”,“TotalCapacity”: 10}, { “FlavorID”: “2xlarge”, “TotalCapacity”: 5 }], 
            “storage”: [ {“TypeID”: “sata”, “StorageCapacity”: 2000}, { “TypeID”: “sas”, “StorageCapacity”: 1000}, { “TypeID”: “ssd”, “StorageCapacity”: 3000}, }], 
            “EipCapacity”: 3, 
            “CPUCapacity”: 8, 
            “MemCapacity”: 256, 
            “ServerPrice”: 10, 
            “HomeScheduler”: “scheduler1” 
            } 
        } 
    } 
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

## References

[1] Kubernetes Cluster, https://kubernetes.io/docs/tutorials/kubernetes

[2] Openstack Cluster,
https://docs.openstack.org/senlin/latest/user/clusters.html\#creating-a-cluster-basics/create-cluster/cluster-intro
