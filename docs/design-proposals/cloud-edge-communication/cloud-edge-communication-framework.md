# Arktos Cloud-Edge Communication Framework

## Motivation

In the edge cloud scenario, worker node of arktos can be run on the edge closer to the user, such as CDN machine room of the operator, user data center and other edge sites, and virtual machines, containers and other resources are provided at the edge. Through the edge network with low delay, the operation of delay sensitive application is satisfied, such as interactive live broadcasting, cloud games and real-time trading systems etc..

In this way, arktos and other cloud management infrastructure components will remotely manage the service components in these edge sites across the public network or private line network. It is necessary to have a way to provide communication access for these basic management components or services between cloud and edge.

Because the cloud and edge is located in the network boundary of different security levels, the management components or services in the edge site and the central cloud can be connected through the VPN layer of the management plane, and then cross boundary security protection can be provided through the firewall ACL.

However, there may be many problems when VPN is used to directly connect the components/services of different security levels:

* It is too risky to directly expose the central cloud management network/IP to users

* IP needs to be planned globally in advance to ensure that the management components/service IP of the central cloud and each edge site do not conflict

* Network changes can be difficult

This proposal aims to outline a design for cloud-edge communication framework/service to solve the above problems

## Goals

* Support communication access capability of cloud-edge basic management components

* Support independent IP addressing and IP address convergence of edge management components/services

* Minimize attack surface, zero trust security

