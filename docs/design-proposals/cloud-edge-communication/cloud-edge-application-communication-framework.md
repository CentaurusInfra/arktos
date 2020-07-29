# Arktos Cloud-edge Application Communication Framework Overview

## Motivation

As the next generation cloud platform architecture, Arkto needs to consider the edge cloud scenario as part of the overall architecture. In the edge cloud scenario, computing nodes will be extended to the edge sites. Delay sensitive applications, such as manufacturing systems and real-time trading systems etc., would require cloud services running in the Arktos cloud platform to perform data processing locally in order to achieve desired real-time response time.

All this requires cloud services processing to be pulled towards the edge site. This, in turn, results in a requirement for secure communication framework between central cloud and the edge sites.

## Requirements

Performing computation at the edge cloud sites poses a big challenge as far as how to secure the cloud-edge communication. At present, network connectivity between cloud and edges is through coarse grain VPN and firewall based mechanisms. When VPN is used to connect cloud edge communication, if there are too many edge sites connected to the central cloud, the IP address will be insufficient. Moreover, in order to make the edge access to the center, it is necessary to configure complex ACL policies on the central firewall and modify them according to the network changes of the edge sites. Such a communication mechanism has a much larger attack surface area and is highly prone to vulnerabilities.

To address all these challenges, Arktos should provide higher-level communication capabilities between central cloud and edges at the application layer level instead, rather than based on VPN. Which can converge the access from the edge to the center, and shield the IP information of the edge site and the central cloud, so as to minimize the attack surface. Additionally, provide granular access control capabilities in conjunction with the authentication process for services/applications. Besides this, it also needs to provide edge gateway services to support flow control and audit trail, support service discovery, service publishing, service routing control and various other functions.

Based on all this, as part of the edge cloud scenario, in order to ensure the security of cloud-edge communications, Arktos needs to provide the following capabilities:

* Provide secure and encrypted application communication channel
* Service publishing mechanism and access agent: service registration, service API publishing, access control policy
* Automatic service discovery, it does not need to expose the intranet IP, and finds the edge service to be accessed through the name of edge node and service name
* Permission-based service routing, for example, to define access control strategies between services as well as which central cloud services can access which edge services
* Flow control and audit trail of service access

