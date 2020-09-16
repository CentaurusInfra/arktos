# Arktos Cloud-Edge Communication Framework

## Motivation

In edge cloud scenario, cloud infrastructure components are deployed in the cloud and edge. For example, AWS Outposts, the outpost rack(include the compute, network, storage servers which are used for edge cloud infrastructure) will be deployed in customer data center, and it can be managed by AWS Outposts, customers can run vms or containers on it through AWS Outposts services. Another example is Aliyun ENS, it can help customers to deploy vms on the edge(in CDN datacenter where is closer to customers than Aliyun public regions). 

Cloud is doing edge server management, and the services running in the cloud and edge need to communicate with each other. For example, the compute agent/service running on the edge servers may need to report compute server info to the cloud for vm scheduling, and the vm services(like openstack nova) running in the cloud may send vm deploy request to compute agent/service running the edge, to do the vm deploying.

There are some ways to provide the service communication between the cloud and edge. For example, AWS Outposts recommend that customers can use direct connect or vpn for the remote services management.
In most cases, use vpn or direct connect for the cloud-edge communication may cause some problems:
* IP address conflict. We need to do the ip addresses planning before we deploy the services to the edge if we use vpn, all the ip of the services need to be different. In some edge scenario, the edge server is deployed in customers' data center, and the available IP addresses may be fiexd or conflict with the cloud or other edge site.  
* Security problem. Because edge sites are usually located in customers' data center or carriers' data center. For cloud provider, they are defined as an untrusted environment. So relying on network perimeter technolygies(VPNs etc.) is a security risk, it expose the whole network of the cloud to the untrusted edge site.

This proposal aims to outline a design for cloud-edge communication framework and it can solve the above problems

## Goals

* Support communication for the services between the cloud and edge
* There is no need to do ip planning for these services between the cloud and edge, no ip conflict problems
* Security proposal, support zero trust

## Proposal A

In this proposal, we want to implement it by service proxy. For solve the problems, we do not expose network to the cloud or the edge, we can "expose service". For example, if there is a service in the cloud, and the edge server need to visit it through our communication framework.
We can add a proxy A in the edge, and it can proxy the service request from the edge server to the cloud service.
But the target service in the cloud is usually not exposed to the Internet. So we need add another proxy B in the cloud, and the proxy A can proxy the service request to proxy B, and the proxy B can proxy the service request to the target service.

edge server -----> Proxy A(in the edge) <----------> Proxy B(in the cloud) ------> target service

If we want to support all the communication between the cloud and edge, we need to implement the "proxy" to proxy the service request between the cloud and edge.
Usually, there are many kinds of services communication between the cloud and edge. For example, the communication may be rest API calls, gprc, message call(kafka/rabbitmq) or something else.

The proxy may work like a nginx(work in network layer 4 or 7), to support all kinds of the communications in the edge cloud. And the proxy A and Proxy B need to communicat to each other with mTLS for security.

Problem of proposal A(to be supplemented)

## Proposal B

We want to implement it based on network layer 3, and we also need to solve the ip address conflict and security problems.

We want to introduce a new concept virtual presence:
* It is a local virtual presentation of remote services
* It is full isolation of remote servers
* It exposes services to remote sites, do not expose networks
* It uses locally assigned virtual presence ips to avoid exposure of site internal addresses
* It use translation mechanisms to emulate local traffic flow
* There is no man-in-the-middle, no session termination, it provide end-to-end encryption

![](Proposal-B1.png)


![](Proposal-B2.png)

Proposal B detail description here

## Implementation Steps：


