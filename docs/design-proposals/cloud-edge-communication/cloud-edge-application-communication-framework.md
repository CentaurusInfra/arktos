# Arktos Cloud-edge Application Communication Framework Overview

## Motivation

As the next generation cloud platform architecture, arkto need to consider the edge cloud scenario in the architecture and scheme. In the edge cloud scenario, the computing nodes will be pulled to the edge site to run, some delay sensitive applications, such as manufacturing systems and real-time trading systems, need arktos cloud platform to provide cloud services locally to achieve local access or local data processing, as well as to achieve real-time response, which requires these cloud services to be pulled to the edge and requires arktos to provide secure communication framework between cloud and edges.

## Requirements

When the central cloud services are pulled to the edge, it also brings some problems, a big problem is how to ensure the security of cloud-edge communication. At present, the network is connected between cloud and edges through VPN and firewall, the security of edge-cloud communication is relatively low. This kind of communication control based on network layer, IP and port is too large to be attacked easily. Moreover, there is a lack of access control between cloud and edges, so it only needs to match IP and port to communicate.To solve this problem, arktos should provide higher-level communication capabilities between cloud and edges, such as providing communication access channels between cloud and edges based on application layer, and providing access control capabilities based on service/application and authentication; besides, it also needs to provide edge gateway services to support flow control and audit, support service discovery, service publishing, service routing control and other functions.

Based on the above discussion, in the edge cloud scenario, in order to ensure the security of cloud-edge communications, arktos needs to provide the following functions:

* Provide secure and encrypted application communication channel
* Service publishing mechanism and access agent: service registration, service API publishing, access control policy
* Automatic service discovery, Automatically route edge services through services and nodes to avoid exposing intranet IP
* Permission-based service routing, for example, to define access control strategies between services and which cloud services can access which edge services
* Flow control and audit of service access
