---
title: Support Openstack Compute APIs in Arktos
authors:
  - "@yb01"
---

# Support Openstack Compute APIs in Arktos
## Background and context
To boost Arktos VM adoption and to let Openstack users to easily explore Arktos VM support without a lot of changes from the users’ applications, it is decided to add a new set of APIs in Arktos to match Openstack Server related operations. Besides better user experiences, this new APIs will also help existing Openstack perf tests to be relatively easily converted to run against Arktos.

## Goal and non-goals
For the Arktos 130 release (1-30-2022):

Goals:

1. New set of REST interfaces in Arktos to support Openstack requests for Server CRUD
1. New set of REST interfaces in Arktos to support Openstack requests for Server Actions
1. Initial Batch support with Arktos deployments

Non-goals:

1. Nova or Nova test tool can switch to Arktos for basic VM operation, by switching to the Arktos URL. 
1. Openstack client-side SDK or other dev tools are not in the scope of this design doc.
1. Openstack API other than VM CRUD, VM Actions, is not in the scope of this design doc.

## New APIs in Arktos
### New API route paths:
The new route for servers

* 	{Arktos-service-url}/servers
* 	{Arktos-service-url}/servers/{server-id}/detail

The new route for server actions

* {Arktos-service-url}/servers/{server-id}/action


### API call chain
The API call chains of /servers will be the same as the other API requests, which is created in the DefaultBuildHandlerChain() function


### VM list, create, delete

|Methods |URL.     |Function                     |Supported in Arktos|
|--------|---------|-----------------------------|-------------------|
|GET	  |/servers  |	List all VMs for a tenant/namespace | yes |
|POST	  |/servers	 |Create a VM	                |yes |
|POST	  |/servers	 |Create multiple VMs.        | Initial support in Arktos 130 release, by calling the deployment API internally with replicaset of VM pods. Optimization will be in [future works](Optimize VM batch)|
|GET	  |/servers/detail	| List VM details for a tenant/namespace | Not for Arktos 130 release|
|GET	  |/servers/{server-id}	|Show VM details |Yes|
|PUT	  |/servers/{server-id} |	Update a VM | Not in Arktos 130 release |
|DELETE   |/servers/{server-id}	| Delete a VM |Yes|
|GET	  |/servers	 |List VM for all tenants |No in the Arktos 130 release|

### VM actions
|Methods |URL.     |Function                     |Supported in Arktos|
|--------|---------|-----------------------------|-------------------|
|POST	 |/servers/{server-id}/action	|Perform specific action on a given VM |	Yes (subset of actions) * |

*Currently only “reboot, stop, start, snapshot, restore” are supported in Arktos VM runtime. So the exposed action APIs will be limited to those actions for Arktos 130 release.
For a full list of Openstack VM actions, please refer to [Openstack compute API doc](https://docs.openstack.org/api-ref/compute/#servers-run-an-action-servers-action)

### Errors
Will relay whatever errors from Arktos, which is standard http errors.
TBD: Need to look into the Openstack client and service error handling. If there are Openstack specific error code and this affects
existing Openstack applications(tests) to run against Arktos, an error handling layer might be needed for this purpose.

### API Details
The following are the details for each API.

#### Create a VM
|Name	|In |	Type	| Description |
|------|---|--------|--------------|
|imageRef|body|string|			The os image url path where the image can be downloaded from|
|flavor|body  |string | The name of the flavor for the VM |
|name	|body	|string	|The server name|
|networks	|body	|array	|A list of network object. Required parameter when there are multiple networks defined for the tenant. When you do not specify the networks parameter, the server attaches to the only network created for the current tenant. Optionally, you can create one or more NICs on the server. To provision the server instance with a NIC for a network, specify the UUID of the network in the uuid attribute in a networks object. To provision the server instance with a NIC for an already existing port, specify the port-id in the port attribute in a networks object. If multiple networks are defined, the order in which they appear in the guest operating system will not necessarily reflect the order in which they are given in the server boot request. Guests should therefore not depend on device order to deduce any information about their network devices. Instead, device role tags should be used: introduced in 2.32, broken in 2.37, and re-introduced and fixed in 2.42, the tag is an optional, string attribute that can be used to assign a tag to a virtual network interface. This tag is then exposed to the guest in the metadata API and the config drive and is associated to hardware metadata for that network interface, such as bus (ex: PCI), bus address (ex: 0000:00:02.0), and MAC address. A bug has caused the tag attribute to no longer be accepted starting with version 2.37. Therefore, network interfaces could only be tagged in versions 2.32 to 2.36 inclusively. Version 2.42 has restored the tag attribute. Starting with microversion 2.37, this field is required and the special string values auto and none can be specified for networks. auto tells the Compute service to use a network that is available to the project, if one exists. If one does not exist, the Compute service will attempt to automatically allocate a network for the project (if possible). none tells the Compute service to not allocate a network for the instance. The auto and none values cannot be used with any other network values, including other network uuids, ports, fixed IPs or device tags. These are requested as strings for the networks value, not in a list. See the associated example.
|networks.uuid (Optional)	|body	|string |To provision the server instance with a NIC for a network, specify the UUID of the network in the uuid attribute in a networks object. Required if you omit the port attribute. Starting with microversion 2.37, this value is strictly enforced to be in UUID format.|
|networks.port (Optional)	|body	|string |To provision the server instance with a NIC for an already existing port, specify the port-id in the port attribute in a networks object. The port status must be DOWN. Required if you omit the uuid attribute. Requested security groups are not applied to pre-existing ports.|
|networks.fixed_ip (Optional)	|body	|string|A fixed IPv4 address for the NIC. Valid with a neutron or nova-networks network.|
|key_name (Optional)	|body	|string	|Key pair name.Note The null value was allowed in the Nova legacy v2 API, but due to strict input validation, it is not allowed in the Nova v2.1 API. |
|metadata (Optional)	|body	|object	|Metadata key and value pairs. The maximum size of the metadata key and value is 255 bytes each.|
|security_groups (Optional)	|body	|array |One or more security groups. Specify the name of the security group in the name attribute. If you omit this attribute, the API creates the server in the default security group. Requested security groups are not applied to pre-existing ports.|
|user_data (Optional)	|body	|string	|Configuration information or scripts to use upon launch. Must be Base64 encoded. Restricted to 65535 bytes. Note The null value allowed in Nova legacy v2 API, but due to the strict input validation, it isn’t allowed in Nova v2.1 API.|

Example of a simple server creation request body, to create a vm with default network under the tenant.

```
{
    "server": {
        "name": "auto-allocate-network",
        "imageRef": "cloud-images.ubuntu.com/releases/xenial/release/ubuntu-16.04-server-cloudimg-amd64-disk1.img",
        "flavor": "m1.tiny"
        "networks": "auto"
        “key_name”: "foobar"
    }
}
```

Response

|Name	|In	|Type	|Description|
|------|----|------|------------|
|id |	body	|string	|The UUID of the server.|
|links	|body	|array	|Links to the resources in question. See API Guide / Links and References for more info.|
|security_groups	|body	|array	|One or more security groups objects.|
|security_groups.name	|body	|string	|The security group name.|


Example of creating a server:

```
{
    "server": {
        "name": "the pod name",
        "id": "f5dc173b-6804-445a-a6d8-c705dad5b5eb",
        "links": [
            {
                # this will be the vm pod ref
                "href": "http://{arktos-url}/openstack/servers/f5dc173b-6804-445a-a6d8-c705dad5b5eb",
                "rel": "self"
            },
            {        ],
        "security_groups": [
            {
                "name": "default"
            }
        ]
    }
}
```

#### Get VM details
Request

|Name	  |In	  |Type	  |Description|
|---------|----   |-------|-----------|
|server_id|	path  |string	|The UUID of the server.|

Response

|Name	|In	  |Type	|Description|
|------|----|-------|-----------|
|accessIPv4	|body	|string	|IPv4 address that should be used to access this server. May be automatically set by the provider.|
|accessIPv6	|body	|string	|IPv6 address that should be used to access this server. May be automatically set by the provider.|
|created	|body	|string	|The date and time when the resource was created. The date and time stamp format is ISO 8601 CCYY-MM-DDThh:mm:ss±hh:mm For example, 2015-08-27T09:49:58-05:00. The ±hh:mm value, if included, is the time zone as an offset from UTC. In the previous example, the offset value is -05:00.|
|id	  |body	|string	|The UUID of the server.|
|image |body	|object	|The UUID and links for the image for your server instance. The image object will be an empty string when you boot the server from a volume.|
|key_name	|body	|string	|The name of associated key pair, if any.|
|links	|body	|array	|Links to the resources in question. See API Guide / Links and References for more info.|
|metadata	|body	|object	|A dictionary of metadata key-and-value pairs, which is maintained for backward compatibility.|
|name	|body	|string	|The server name.|
|OS-EXT-SRV-ATTR:user_data (Optional)	|body  |string |The user_data the instance was created with. By default, it appears in the response for administrative users only. New in version 2.3|
|OS-EXT-STS:power_state	 |body	|integer	|The power state of the instance. This is an enum value that is mapped as: 0: NOSTATE 1: RUNNING 3: PAUSED 4: SHUTDOWN 6: CRASHED 7: SUSPENDED |
|OS-EXT-STS:task_state	|body	|string	|The task state of the instance.|
|OS-EXT-STS:vm_state	|body	|string	|The VM state.|
|OS-SRV-USG:launched_at	|body	|string	|The date and time when the server was launched. The date and time stamp format is ISO 8601:CCYY-MM-DDThh:mm:ss±hh:mm For example, 015-08-27T09:49:58-05:00. The hh±:mm value, if included, is the time zone as an offset from UTC. If the deleted_at date and time stamp is not set, its value is null.
|OS-SRV-USG:terminated_at	|body	|string  |The date and time when the server was deleted.The date and time stamp format is ISO 8601:CCYY-MM-DDThh:mm:ss±hh:mm For example, 2015-08-27T09:49:58-05:00. The ±hh:mm value, if included, is the time zone as an offset from UTC. If the deleted_at date and time stamp is not set, its value is null.|
|status	|body	|string	|The server status.|
|tenant_id	|body	|string	|The UUID of the tenant in a multi-tenancy cloud.|
|updated	|body	|string	|The date and time when the resource was updated. The date and time stamp format is ISO 8601 CCYY-MM-DDThh:mm:ss±hh:mm For example, 2015-08-27T09:49:58-05:00. The ±hh:mm value, if included, is the time zone as an offset from UTC. In the previous example, the offset value is -05:00.|
|security_groups (Optional)	|body	|array |One or more security groups objects.|
|security_group.name	|body	|string	|The security group name.|


#### Delete a VM:
Request

|Name	    |In	  |Type	|Description|
|-----------|-----|-------|-----------|
|server_id	|path	|string	|The UUID of the server.|

Response

There is no body content for the response of a successful DELETE query


### VM Actions
The /servers/{server-id}/action will take an action defined in the request, the below uses reboot VM as an example.

Request

|Name	|In	  |Type	|Description|
|------|----|-------|-----------|
|server_id	|path	|string	|The UUID of the server.|
|reboot	|body	|object	|The action to reboot a server.|
|type	|body	|string	|The type of the reboot action. The valid values are HARD and SOFT. A SOFT reboot attempts a graceful shutdown and restart of the server. A HARD reboot attempts a forced shutdown and restart of the server. The HARD reboot corresponds to the power cycles of the server.|

Example Reboot Server (reboot Action)
```
{
    "reboot" : {
        "type" : "HARD"
    }
}
```
Response¶
If successful, this method does not return content in the response body.

## Implementation
List of changes in Arktos:
1.	New routes registered to the API server, for servers and actions
2.	New set of handlers for each route for VM and actions ( Reuse of existing pod handlers )
3.	Converting logic to convert Openstack requests to Arktos request and Response to Openstack response in the web service, for both Server and Action requests and responses
4.	API server logic to retrieve the tenant info from Openstack request header, to construct the desired PATH for the VM request to Arktos POD (or ACTION) routes.
5.	(Stretch goal) modify Openstack client SDK to for Arktos VM APIs

## API Security 

API authentication and authorization, and audit chain are built into the Arktos API server already. The new route will be part of the chain and leverage the security control there as the other objects such as PODs, actions built into Arktos.

### Handel Namespace
TBD -- ideally, the namespace in Arktos should map to Project in Openstack. for Arktos 130 release, it will be simply using the 
default namespace for tenant's VM workloads.

### VM flavor support:
Arktos to have a config map to preserve basic Openstack flavor

### VM imageRef support
Eventually there will be an image registry service to support this. For this current design, instead of a full flether image registry service, Arktos can have a config map to perserve commom images such as coros and Ubuntu etc.

## Future works
### Optimize VMs batch
In the Arktos 130 release, the VM batch creation essentially leverages the Arktos's deployment and replicaset.
Some future optimization effort on VM batch support can be:

1. Batch size: i.e. what is the max batch should we support, and for large batch, how to utilize batch scheduling logic?
2. How fast to create the VM pods in replicaset controller. i.e. dynamic QPS setting to create the VM pods into the API server for scheduling and orchestration
3. How to coordinate with network provider service for best performance for port allocation for the VMs
4. How to ensure scheduling fairness for the other tenants
 
### Support Openstack built in VM image and flavor with images and flavors routes
As indicated in the appendix section, Openstack client components make interactions with multiple Openstack services, including neutron, compute and keystone services for network, image, flavor, authentication purposes.
In order to have similar user experience when running those clients against Arktos, some extra routes will be added on:

	{Arktos-service-url}/images
	{Arktos-service-url}/flavors
	
## Milestones

|Date      |Description|
|------------|-----------|
|12/3/2021  | Support single VM creation/list/get/delete, with default network, image and flavor|
|12/10/2021 | Support vm actions|
|12/17/2021 | Add image registry and flavor route and config map for basic images, and basic Openstack flavors |
|1/14/2022  | Support create VMs in batch |


## Appendix: Debug output of creating a simple VM with DevStack
The below shows the openStack client interact with Identity service for AuthN,  neutron service for network, compute service for image, flavor during the request for creating a simple VM with DevStack.
```
stack@ip-172-31-10-216:~/devstack$ openstack server create --image cirros-0.5.2-x86_64-disk --flavor m1.tiny --network public test-2 --debug
/usr/lib/python3/dist-packages/secretstorage/dhcrypto.py:15: CryptographyDeprecationWarning: int_from_bytes is deprecated, use int.from_bytes instead
  from cryptography.utils import int_from_bytes
/usr/lib/python3/dist-packages/secretstorage/util.py:19: CryptographyDeprecationWarning: int_from_bytes is deprecated, use int.from_bytes instead
  from cryptography.utils import int_from_bytes
START with options: server create --image cirros-0.5.2-x86_64-disk --flavor m1.tiny --network public test-2 --debug
options: Namespace(access_key='', access_secret='***', access_token='***', access_token_endpoint='', access_token_type='', application_credential_id='', application_credential_name='', application_credential_secret='***', auth_methods='', auth_type='', auth_url='http://172.31.10.216/identity', cacert=None, cert='', client_id='', client_secret='***', cloud='', code='', consumer_key='', consumer_secret='***', debug=True, default_domain='default', default_domain_id='', default_domain_name='', deferred_help=False, discovery_endpoint='', domain_id='', domain_name='', endpoint='', identity_provider='', identity_provider_url='', insecure=None, interface='public', key='', log_file=None, openid_scope='', os_beta_command=False, os_compute_api_version='', os_dns_api_version='2', os_identity_api_version='3', os_image_api_version='', os_key_manager_api_version='1', os_network_api_version='', os_object_api_version='', os_placement_api_version='1', os_project_id=None, os_project_name=None, os_volume_api_version='', passcode='', password='***', profile='', project_domain_id='default', project_domain_name='', project_id='135f59752049436e95ea0642343340ae', project_name='demo', protocol='', redirect_uri='', region_name='RegionOne', remote_project_domain_id='', remote_project_domain_name='', remote_project_id='', remote_project_name='', service_provider='', service_provider_endpoint='', service_provider_entity_id='', system_scope='', timing=False, token='***', trust_id='', user_domain_id='', user_domain_name='Default', user_id='', username='admin', verbose_level=3, verify=None)
Auth plugin password selected
auth_config_hook(): {'api_timeout': None, 'verify': True, 'cacert': None, 'cert': None, 'key': None, 'baremetal_status_code_retries': '5', 'baremetal_introspection_status_code_retries': '5', 'image_status_code_retries': '5', 'disable_vendor_agent': {}, 'interface': 'public', 'floating_ip_source': 'neutron', 'image_api_use_tasks': False, 'image_format': 'qcow2', 'message': '', 'network_api_version': '2', 'object_store_api_version': '1', 'secgroup_source': 'neutron', 'status': 'active', 'auth': {'user_domain_name': 'Default', 'project_domain_id': 'default', 'project_id': '135f59752049436e95ea0642343340ae', 'project_name': 'demo'}, 'verbose_level': 3, 'deferred_help': False, 'debug': True, 'region_name': 'RegionOne', 'default_domain': 'default', 'timing': False, 'auth_url': 'http://172.31.10.216/identity', 'username': 'admin', 'password': '***', 'beta_command': False, 'identity_api_version': '3', 'dns_api_version': '2', 'placement_api_version': '1', 'key_manager_api_version': '1', 'auth_type': 'password', 'networks': []}
defaults: {'api_timeout': None, 'verify': True, 'cacert': None, 'cert': None, 'key': None, 'auth_type': 'password', 'baremetal_status_code_retries': 5, 'baremetal_introspection_status_code_retries': 5, 'image_status_code_retries': 5, 'disable_vendor_agent': {}, 'interface': None, 'floating_ip_source': 'neutron', 'image_api_use_tasks': False, 'image_format': 'qcow2', 'message': '', 'network_api_version': '2', 'object_store_api_version': '1', 'secgroup_source': 'neutron', 'status': 'active'}
cloud cfg: {'api_timeout': None, 'verify': True, 'cacert': None, 'cert': None, 'key': None, 'baremetal_status_code_retries': '5', 'baremetal_introspection_status_code_retries': '5', 'image_status_code_retries': '5', 'disable_vendor_agent': {}, 'interface': 'public', 'floating_ip_source': 'neutron', 'image_api_use_tasks': False, 'image_format': 'qcow2', 'message': '', 'network_api_version': '2', 'object_store_api_version': '1', 'secgroup_source': 'neutron', 'status': 'active', 'auth': {'user_domain_name': 'Default', 'project_domain_id': 'default', 'project_id': '135f59752049436e95ea0642343340ae', 'project_name': 'demo'}, 'verbose_level': 3, 'deferred_help': False, 'debug': True, 'region_name': 'RegionOne', 'default_domain': 'default', 'timing': False, 'auth_url': 'http://172.31.10.216/identity', 'username': 'admin', 'password': '***', 'beta_command': False, 'identity_api_version': '3', 'dns_api_version': '2', 'placement_api_version': '1', 'key_manager_api_version': '1', 'auth_type': 'password', 'networks': []}
compute API version 2.1, cmd group openstack.compute.v2
identity API version 3, cmd group openstack.identity.v3
image API version 2, cmd group openstack.image.v2
network API version 2, cmd group openstack.network.v2
object_store API version 1, cmd group openstack.object_store.v1
volume API version 3, cmd group openstack.volume.v3
dns API version 2, cmd group openstack.dns.v2
placement API version 1, cmd group openstack.placement.v1
/usr/local/lib/python3.8/dist-packages/barbicanclient/__init__.py:57: UserWarning: The secrets module is moved to barbicanclient/v1 directory, direct import of barbicanclient.secrets will be deprecated. Please import barbicanclient.v1.secrets instead.
  warnings.warn("The %s module is moved to barbicanclient/v1 "
key_manager API version 1, cmd group openstack.key_manager.v1
neutronclient API version 2, cmd group openstack.neutronclient.v2***
command: server create -> openstackclient.compute.v2.server.CreateServer (auth=True)
***Auth plugin password selected
auth_config_hook(): {'api_timeout': None, 'verify': True, 'cacert': None, 'cert': None, 'key': None, 'baremetal_status_code_retries': '5', 'baremetal_introspection_status_code_retries': '5', 'image_status_code_retries': '5', 'disable_vendor_agent': {}, 'interface': 'public', 'floating_ip_source': 'neutron', 'image_api_use_tasks': False, 'image_format': 'qcow2', 'message': '', 'network_api_version': '2', 'object_store_api_version': '1', 'secgroup_source': 'neutron', 'status': 'active', 'auth': {'user_domain_name': 'Default', 'project_domain_id': 'default', 'project_id': '135f59752049436e95ea0642343340ae', 'project_name': 'demo'}, 'additional_user_agent': [('osc-lib', '2.4.2')], 'verbose_level': 3, 'deferred_help': False, 'debug': True, 'region_name': 'RegionOne', 'default_domain': 'default', 'timing': False, 'auth_url': 'http://172.31.10.216/identity', 'username': 'admin', 'password': '***', 'beta_command': False, 'identity_api_version': '3', 'dns_api_version': '2', 'placement_api_version': '1', 'key_manager_api_version': '1', 'auth_type': 'password', 'networks': []}
Using auth plugin: password
Using parameters {'auth_url': 'http://172.31.10.216/identity', 'project_id': '135f59752049436e95ea0642343340ae', 'project_name': 'demo', 'project_domain_id': 'default', 'username': 'admin', 'user_domain_name': 'Default', 'password': '***'}
Get auth_ref
REQ: curl -g -i -X GET http://172.31.10.216/identity -H "Accept: application/json" -H "User-Agent: openstacksdk/0.59.0 keystoneauth1/4.4.0 python-requests/2.26.0 CPython/3.8.10"
Starting new HTTP connection (1): 172.31.10.216:80
http://172.31.10.216:80 "GET /identity HTTP/1.1" 300 272
RESP: [300] Connection: close Content-Length: 272 Content-Type: application/json Date: Tue, 16 Nov 2021 22:51:55 GMT Location: http://172.31.10.216/identity/v3/ Server: Apache/2.4.41 (Ubuntu) Vary: X-Auth-Token x-openstack-request-id: req-d70ce217-a953-46ec-b993-739ead6f7ae1
RESP BODY: {"versions": {"values": [{"id": "v3.14", "status": "stable", "updated": "2020-04-07T00:00:00Z", "links": [{"rel": "self", "href": "http://172.31.10.216/identity/v3/"}], "media-types": [{"base": "application/json", "type": "application/vnd.openstack.identity-v3+json"}]}]}}
GET call to http://172.31.10.216/identity used request id req-d70ce217-a953-46ec-b993-739ead6f7ae1
Making authentication request to http://172.31.10.216/identity/v3/auth/tokens
Resetting dropped connection: 172.31.10.216
http://172.31.10.216:80 "POST /identity/v3/auth/tokens HTTP/1.1" 201 2804
{"token": {"methods": ["password"], "user": {"domain": {"id": "default", "name": "Default"}, "id": "08c04c8e508c4a549df8302573e97dfb", "name": "admin", "password_expires_at": null}, "audit_ids": ["f2myGIZIQKeLKLNimwJVng"], "expires_at": "2021-11-16T23:51:55.000000Z", "issued_at": "2021-11-16T22:51:55.000000Z", "project": {"domain": {"id": "default", "name": "Default"}, "id": "135f59752049436e95ea0642343340ae", "name": "demo"}, "is_domain": false, "roles": [{"id": "95d20d42b5d14b948e8d11137e2190ee", "name": "member"}, {"id": "1815ff35acc048ddb4d3c6fd35c5de6c", "name": "reader"}, {"id": "d4630a1a711d4ac69293ef0baa43d252", "name": "admin"}], "catalog": [{"endpoints": [{"id": "3687c5308d4742f4bb4e7ad70e7f8bb6", "interface": "public", "region_id": "RegionOne", "url": "http://172.31.10.216/compute/v2/135f59752049436e95ea0642343340ae", "region": "RegionOne"}], "id": "4f081b278c3743628723d8e119b8282b", "type": "compute_legacy", "name": "nova_legacy"}, {"endpoints": [{"id": "082721b32fbf4f3c837ebca6191b253b", "interface": "public", "region_id": "RegionOne", "url": "http://172.31.10.216/volume/v3/135f59752049436e95ea0642343340ae", "region": "RegionOne"}], "id": "536a1188757048309abd305d1568df2e", "type": "volumev3", "name": "cinderv3"}, {"endpoints": [{"id": "7f669f37528c4b1d8720459b1a32d457", "interface": "public", "region_id": "RegionOne", "url": "http://172.31.10.216/image", "region": "RegionOne"}], "id": "8d7b5663e7754285bb2425089662a63c", "type": "image", "name": "glance"}, {"endpoints": [{"id": "edbd6484926b4112a056919b76e595bc", "interface": "public", "region_id": "RegionOne", "url": "http://172.31.10.216/identity", "region": "RegionOne"}], "id": "9575ac1b6bc4406098bc1d45489cb2ac", "type": "identity", "name": "keystone"}, {"endpoints": [{"id": "bf549b61941140178247c43d8a290ae5", "interface": "public", "region_id": "RegionOne", "url": "http://172.31.10.216:9696/", "region": "RegionOne"}], "id": "98afc299b17c472cb3fff3239a6ede55", "type": "network", "name": "neutron"}, {"endpoints": [{"id": "ff106d9ba60d4dbf9f7ecad5285c60c4", "interface": "public", "region_id": "RegionOne", "url": "http://172.31.10.216/volume/v3/135f59752049436e95ea0642343340ae", "region": "RegionOne"}], "id": "c7c9cce049424ea68b706858438122d8", "type": "block-storage", "name": "cinder"}, {"endpoints": [{"id": "64c3ca7bd7df4358ba3c8f36dc852d03", "interface": "public", "region_id": "RegionOne", "url": "http://172.31.10.216/compute/v2.1", "region": "RegionOne"}], "id": "c87dcbd40ef947f6803b0fd6bef89d33", "type": "compute", "name": "nova"}, {"endpoints": [{"id": "87c065f64a8d4a3084c0d56eda0af591", "interface": "public", "region_id": "RegionOne", "url": "http://172.31.10.216/placement", "region": "RegionOne"}], "id": "e7cd248eec5a4777a5704e3503523f68", "type": "placement", "name": "placement"}]}}
run(Namespace(availability_zone=None, block_device_mapping=[], block_devices=[], boot_from_volume=None, columns=[], config_drive=False, description=None, ephemerals=[], file=[], fit_width=False, flavor='m1.tiny', formatter='table', hint={}, host=None, hostname=None, hypervisor_hostname=None, image='cirros-0.5.2-x86_64-disk', image_properties=None, key_name=None, max=1, max_width=0, min=1, nics=[{'net-id': 'public', 'port-id': '', 'v4-fixed-ip': '', 'v6-fixed-ip': ''}], noindent=False, password=None, prefix='', print_empty=False, properties=None, security_group=[], server_name='test-2', snapshot=None, swap=None, tags=[], trusted_image_certs=None, user_data=None, variables=[], volume=None, wait=False))
Instantiating compute client for API Version Major: 2, Minor: 1
Instantiating compute api: <class 'openstackclient.api.compute_v2.APIv2'>
Instantiating volume client: <class 'cinderclient.v3.client.Client'>
REQ: curl -g -i -X GET http://172.31.10.216/image -H "Accept: application/json" -H "User-Agent: openstacksdk/0.59.0 keystoneauth1/4.4.0 python-requests/2.26.0 CPython/3.8.10"
Resetting dropped connection: 172.31.10.216
http://172.31.10.216:80 "GET /image HTTP/1.1" 300 993
RESP: [300] Connection: close Content-Length: 993 Content-Type: application/json Date: Tue, 16 Nov 2021 22:51:55 GMT Server: Apache/2.4.41 (Ubuntu)
RESP BODY: {"versions": [{"id": "v2.9", "status": "CURRENT", "links": [{"rel": "self", "href": "http://172.31.10.216/image/v2/"}]}, {"id": "v2.7", "status": "SUPPORTED", "links": [{"rel": "self", "href": "http://172.31.10.216/image/v2/"}]}, {"id": "v2.6", "status": "SUPPORTED", "links": [{"rel": "self", "href": "http://172.31.10.216/image/v2/"}]}, {"id": "v2.5", "status": "SUPPORTED", "links": [{"rel": "self", "href": "http://172.31.10.216/image/v2/"}]}, {"id": "v2.4", "status": "SUPPORTED", "links": [{"rel": "self", "href": "http://172.31.10.216/image/v2/"}]}, {"id": "v2.3", "status": "SUPPORTED", "links": [{"rel": "self", "href": "http://172.31.10.216/image/v2/"}]}, {"id": "v2.2", "status": "SUPPORTED", "links": [{"rel": "self", "href": "http://172.31.10.216/image/v2/"}]}, {"id": "v2.1", "status": "SUPPORTED", "links": [{"rel": "self", "href": "http://172.31.10.216/image/v2/"}]}, {"id": "v2.0", "status": "SUPPORTED", "links": [{"rel": "self", "href": "http://172.31.10.216/image/v2/"}]}]}
Image client initialized using OpenStack SDK: <openstack.image.v2._proxy.Proxy object at 0x7f1a4af77d00>
REQ: curl -g -i -X GET http://172.31.10.216/image/v2/images/cirros-0.5.2-x86_64-disk -H "User-Agent: openstacksdk/0.59.0 keystoneauth1/4.4.0 python-requests/2.26.0 CPython/3.8.10" -H "X-Auth-Token: {SHA256}47d331cb112e7ffd3549b9958f475e4a36ab4d7848770b6ca54f337a6ca9a37e"
Resetting dropped connection: 172.31.10.216
http://172.31.10.216:80 "GET /image/v2/images/cirros-0.5.2-x86_64-disk HTTP/1.1" 404 169
RESP: [404] Connection: close Content-Length: 169 Content-Type: text/html; charset=UTF-8 Date: Tue, 16 Nov 2021 22:51:55 GMT Server: Apache/2.4.41 (Ubuntu) x-openstack-request-id: req-f697db53-be35-46c0-9b58-468975a5ba91
RESP BODY: Omitted, Content-Type is set to text/html; charset=UTF-8. Only application/json responses have their bodies logged.
GET call to image for http://172.31.10.216/image/v2/images/cirros-0.5.2-x86_64-disk used request id req-f697db53-be35-46c0-9b58-468975a5ba91
REQ: curl -g -i -X GET "http://172.31.10.216/image/v2/images?name=cirros-0.5.2-x86_64-disk" -H "Accept: application/json" -H "User-Agent: openstacksdk/0.59.0 keystoneauth1/4.4.0 python-requests/2.26.0 CPython/3.8.10" -H "X-Auth-Token: {SHA256}47d331cb112e7ffd3549b9958f475e4a36ab4d7848770b6ca54f337a6ca9a37e"
Resetting dropped connection: 172.31.10.216
http://172.31.10.216:80 "GET /image/v2/images?name=cirros-0.5.2-x86_64-disk HTTP/1.1" 200 1075
RESP: [200] Connection: close Content-Length: 1075 Content-Type: application/json Date: Tue, 16 Nov 2021 22:51:55 GMT Server: Apache/2.4.41 (Ubuntu) x-openstack-request-id: req-d9c66ab0-7b28-40b3-a90a-494d0be592b1
RESP BODY: {"images": [{"owner_specified.openstack.object": "images/cirros-0.5.2-x86_64-disk", "owner_specified.openstack.sha256": "", "owner_specified.openstack.md5": "", "hw_rng_model": "virtio", "name": "cirros-0.5.2-x86_64-disk", "disk_format": "qcow2", "container_format": "bare", "visibility": "public", "size": 16300544, "virtual_size": 117440512, "status": "active", "checksum": "b874c39491a2377b8490f5f1e89761a4", "protected": false, "min_ram": 0, "min_disk": 0, "owner": "6e8819a82dda467b89e4499c2e2b1df6", "os_hidden": false, "os_hash_algo": "sha512", "os_hash_value": "6b813aa46bb90b4da216a4d19376593fa3f4fc7e617f03a92b7fe11e9a3981cbe8f0959dbebe36225e5f53dc4492341a4863cac4ed1ee0909f3fc78ef9c3e869", "id": "6db08272-a856-49da-8909-7c4c73ab0bac", "created_at": "2021-11-16T18:15:41Z", "updated_at": "2021-11-16T18:15:42Z", "tags": [], "self": "/v2/images/6db08272-a856-49da-8909-7c4c73ab0bac", "file": "/v2/images/6db08272-a856-49da-8909-7c4c73ab0bac/file", "schema": "/v2/schemas/image"}], "first": "/v2/images?name=cirros-0.5.2-x86_64-disk", "schema": "/v2/schemas/images"}
GET call to image for http://172.31.10.216/image/v2/images?name=cirros-0.5.2-x86_64-disk used request id req-d9c66ab0-7b28-40b3-a90a-494d0be592b1_REQ: curl -g -i -X GET http://172.31.10.216/compute/v2.1/flavors/m1.tiny -H "Accept: application/json" -H "User-Agent: python-novaclient" -H "X-Auth-Token: {SHA256}47d331cb112e7ffd3549b9958f475e4a36ab4d7848770b6ca54f337a6ca9a37e" -H "X-OpenStack-Nova-API-Version: 2.1"
Resetting dropped connection: 172.31.10.216
http://172.31.10.216:80 "GET /compute/v2.1/flavors/m1.tiny HTTP/1.1" 404 80
RESP: [404] Connection: close Content-Length: 80 Content-Type: application/json; charset=UTF-8 Date: Tue, 16 Nov 2021 22:51:55 GMT OpenStack-API-Version: compute 2.1 Server: Apache/2.4.41 (Ubuntu) Vary: OpenStack-API-Version,X-OpenStack-Nova-API-Version X-OpenStack-Nova-API-Version: 2.1 x-compute-request-id: req-aa4888f8-560e-466e-b954-4da0908f07cb x-openstack-request-id: req-aa4888f8-560e-466e-b954-4da0908f07cb
RESP BODY: {"itemNotFound": {"code": 404, "message": "Flavor m1.tiny could not be found."}}
GET call to compute for http://172.31.10.216/compute/v2.1/flavors/m1.tiny used request id req-aa4888f8-560e-466e-b954-4da0908f07cb
REQ: curl -g -i -X GET http://172.31.10.216/compute/v2.1/flavors -H "Accept: application/json" -H "User-Agent: python-novaclient" -H "X-Auth-Token: {SHA256}47d331cb112e7ffd3549b9958f475e4a36ab4d7848770b6ca54f337a6ca9a37e" -H "X-OpenStack-Nova-API-Version: 2.1"
Resetting dropped connection: 172.31.10.216
http://172.31.10.216:80 "GET /compute/v2.1/flavors HTTP/1.1" 200 2265
RESP: [200] Connection: close Content-Length: 2265 Content-Type: application/json Date: Tue, 16 Nov 2021 22:51:55 GMT OpenStack-API-Version: compute 2.1 Server: Apache/2.4.41 (Ubuntu) Vary: OpenStack-API-Version,X-OpenStack-Nova-API-Version X-OpenStack-Nova-API-Version: 2.1 x-compute-request-id: req-574a02e0-77b7-4f28-9870-3bd69ef84575 x-openstack-request-id: req-574a02e0-77b7-4f28-9870-3bd69ef84575
RESP BODY: {"flavors": [{"id": "1", "name": "m1.tiny", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/1"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/1"}]}, {"id": "2", "name": "m1.small", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/2"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/2"}]}, {"id": "3", "name": "m1.medium", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/3"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/3"}]}, {"id": "4", "name": "m1.large", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/4"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/4"}]}, {"id": "42", "name": "m1.nano", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/42"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/42"}]}, {"id": "5", "name": "m1.xlarge", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/5"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/5"}]}, {"id": "84", "name": "m1.micro", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/84"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/84"}]}, {"id": "c1", "name": "cirros256", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/c1"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/c1"}]}, {"id": "d1", "name": "ds512M", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/d1"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/d1"}]}, {"id": "d2", "name": "ds1G", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/d2"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/d2"}]}, {"id": "d3", "name": "ds2G", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/d3"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/d3"}]}, {"id": "d4", "name": "ds4G", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/d4"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/d4"}]}]}
GET call to compute for http://172.31.10.216/compute/v2.1/flavors used request id req-574a02e0-77b7-4f28-9870-3bd69ef84575
REQ: curl -g -i -X GET http://172.31.10.216/compute/v2.1/flavors/1 -H "Accept: application/json" -H "User-Agent: python-novaclient" -H "X-Auth-Token: {SHA256}47d331cb112e7ffd3549b9958f475e4a36ab4d7848770b6ca54f337a6ca9a37e" -H "X-OpenStack-Nova-API-Version: 2.1"
Resetting dropped connection: 172.31.10.216
http://172.31.10.216:80 "GET /compute/v2.1/flavors/1 HTTP/1.1" 200 366
RESP: [200] Connection: close Content-Length: 366 Content-Type: application/json Date: Tue, 16 Nov 2021 22:51:55 GMT OpenStack-API-Version: compute 2.1 Server: Apache/2.4.41 (Ubuntu) Vary: OpenStack-API-Version,X-OpenStack-Nova-API-Version X-OpenStack-Nova-API-Version: 2.1 x-compute-request-id: req-61ab3bb1-eb78-4b96-875f-cf58934a30b1 x-openstack-request-id: req-61ab3bb1-eb78-4b96-875f-cf58934a30b1
RESP BODY: {"flavor": {"id": "1", "name": "m1.tiny", "ram": 512, "disk": 1, "swap": "", "OS-FLV-EXT-DATA:ephemeral": 0, "OS-FLV-DISABLED:disabled": false, "vcpus": 1, "os-flavor-access:is_public": true, "rxtx_factor": 1.0, "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/1"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/1"}]}}
GET call to compute for http://172.31.10.216/compute/v2.1/flavors/1 used request id req-61ab3bb1-eb78-4b96-875f-cf58934a30b1
network endpoint in service catalog
Network client initialized using OpenStack SDK: <openstack.network.v2._proxy.Proxy object at 0x7f1a4af10a60>
REQ: curl -g -i -X GET http://172.31.10.216:9696/v2.0/networks/public -H "User-Agent: openstacksdk/0.59.0 keystoneauth1/4.4.0 python-requests/2.26.0 CPython/3.8.10" -H "X-Auth-Token: {SHA256}47d331cb112e7ffd3549b9958f475e4a36ab4d7848770b6ca54f337a6ca9a37e"
Starting new HTTP connection (1): 172.31.10.216:9696
http://172.31.10.216:9696 "GET /v2.0/networks/public HTTP/1.1" 404 108
RESP: [404] Connection: keep-alive Content-Length: 108 Content-Type: application/json Date: Tue, 16 Nov 2021 22:51:55 GMT X-Openstack-Request-Id: req-3c245f29-c7ac-4b66-8696-5c16cadf0911
RESP BODY: {"NeutronError": {"type": "NetworkNotFound", "message": "Network public could not be found.", "detail": ""}}
GET call to network for http://172.31.10.216:9696/v2.0/networks/public used request id req-3c245f29-c7ac-4b66-8696-5c16cadf0911
REQ: curl -g -i -X GET "http://172.31.10.216:9696/v2.0/networks?name=public" -H "Accept: application/json" -H "User-Agent: openstacksdk/0.59.0 keystoneauth1/4.4.0 python-requests/2.26.0 CPython/3.8.10" -H "X-Auth-Token: {SHA256}47d331cb112e7ffd3549b9958f475e4a36ab4d7848770b6ca54f337a6ca9a37e"
http://172.31.10.216:9696 "GET /v2.0/networks?name=public HTTP/1.1" 200 721
RESP: [200] Connection: keep-alive Content-Length: 721 Content-Type: application/json Date: Tue, 16 Nov 2021 22:51:55 GMT X-Openstack-Request-Id: req-0200819a-cabf-4b9f-b309-203320c6504f
RESP BODY: {"networks":[{"id":"2608d099-c5cb-45c9-a85e-c7daba9f95bf","name":"public","tenant_id":"6e8819a82dda467b89e4499c2e2b1df6","admin_state_up":true,"mtu":1500,"status":"ACTIVE","subnets":["a058780d-6f44-4eb8-b765-e731eb7baa66","f487652c-99d6-4ce1-81f9-c8d866688326"],"shared":false,"availability_zone_hints":[],"availability_zones":[],"ipv4_address_scope":null,"ipv6_address_scope":null,"router:external":true,"description":"","port_security_enabled":true,"is_default":true,"tags":[],"created_at":"2021-11-16T18:14:51Z","updated_at":"2021-11-16T18:15:06Z","revision_number":3,"project_id":"6e8819a82dda467b89e4499c2e2b1df6","provider:network_type":"flat","provider:physical_network":"public","provider:segmentation_id":null}]}
GET call to network for http://172.31.10.216:9696/v2.0/networks?name=public used request id req-0200819a-cabf-4b9f-b309-203320c6504f
network endpoint in service catalog
boot_args: ['test-2', openstack.image.v2.image.Image(hw_rng_model=virtio, name=cirros-0.5.2-x86_64-disk, disk_format=qcow2, container_format=bare, visibility=public, size=16300544, virtual_size=117440512, status=active, checksum=b874c39491a2377b8490f5f1e89761a4, protected=False, min_ram=0, min_disk=0, owner=6e8819a82dda467b89e4499c2e2b1df6, os_hidden=False, os_hash_algo=sha512, os_hash_value=6b813aa46bb90b4da216a4d19376593fa3f4fc7e617f03a92b7fe11e9a3981cbe8f0959dbebe36225e5f53dc4492341a4863cac4ed1ee0909f3fc78ef9c3e869, id=6db08272-a856-49da-8909-7c4c73ab0bac, created_at=2021-11-16T18:15:41Z, updated_at=2021-11-16T18:15:42Z, tags=[], file=/v2/images/6db08272-a856-49da-8909-7c4c73ab0bac/file, schema=/v2/schemas/image, properties={'owner_specified.openstack.object': 'images/cirros-0.5.2-x86_64-disk', 'owner_specified.openstack.sha256': '', 'owner_specified.openstack.md5': ''}, location=Munch({'cloud': '', 'region_name': 'RegionOne', 'zone': None, 'project': Munch({'id': '135f59752049436e95ea0642343340ae', 'name': 'demo', 'domain_id': 'default', 'domain_name': None})})), <Flavor: m1.tiny>]
boot_kwargs: {'meta': None, 'files': {}, 'reservation_id': None, 'min_count': 1, 'max_count': 1, 'security_groups': [], 'userdata': None, 'key_name': None, 'availability_zone': None, 'admin_pass': None, 'block_device_mapping_v2': [], 'nics': [{'net-id': '2608d099-c5cb-45c9-a85e-c7daba9f95bf', 'port-id': '', 'v4-fixed-ip': '', 'v6-fixed-ip': ''}], 'scheduler_hints': {}, 'config_drive': None}
REQ: curl -g -i -X POST http://172.31.10.216/compute/v2.1/servers -H "Accept: application/json" -H "Content-Type: application/json" -H "User-Agent: python-novaclient" -H "X-Auth-Token: {SHA256}47d331cb112e7ffd3549b9958f475e4a36ab4d7848770b6ca54f337a6ca9a37e" -H "X-OpenStack-Nova-API-Version: 2.1" -d '{"server": {"name": "test-2", "imageRef": "6db08272-a856-49da-8909-7c4c73ab0bac", "flavorRef": "1", "min_count": 1, "max_count": 1, "networks": [{"uuid": "2608d099-c5cb-45c9-a85e-c7daba9f95bf"}]}}'
Resetting dropped connection: 172.31.10.216
http://172.31.10.216:80 "POST /compute/v2.1/servers HTTP/1.1" 202 384
RESP: [202] Connection: close Content-Length: 384 Content-Type: application/json Date: Tue, 16 Nov 2021 22:51:55 GMT OpenStack-API-Version: compute 2.1 Server: Apache/2.4.41 (Ubuntu) Vary: OpenStack-API-Version,X-OpenStack-Nova-API-Version X-OpenStack-Nova-API-Version: 2.1 location: http://172.31.10.216/compute/v2.1/servers/bad005cf-17e1-48ad-b9f7-e9f9352beb13 x-compute-request-id: req-e1279ae9-2d96-4717-9935-6aa55797b1b1 x-openstack-request-id: req-e1279ae9-2d96-4717-9935-6aa55797b1b1
RESP BODY: {"server": {"id": "bad005cf-17e1-48ad-b9f7-e9f9352beb13", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/servers/bad005cf-17e1-48ad-b9f7-e9f9352beb13"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/servers/bad005cf-17e1-48ad-b9f7-e9f9352beb13"}], "OS-DCF:diskConfig": "MANUAL", "security_groups": [{"name": "default"}], "adminPass": "L3e7F8A8YrtC"}}
POST call to compute for http://172.31.10.216/compute/v2.1/servers used request id req-e1279ae9-2d96-4717-9935-6aa55797b1b1
REQ: curl -g -i -X GET http://172.31.10.216/compute/v2.1/servers/bad005cf-17e1-48ad-b9f7-e9f9352beb13 -H "Accept: application/json" -H "User-Agent: python-novaclient" -H "X-Auth-Token: {SHA256}47d331cb112e7ffd3549b9958f475e4a36ab4d7848770b6ca54f337a6ca9a37e" -H "X-OpenStack-Nova-API-Version: 2.1"
Resetting dropped connection: 172.31.10.216
http://172.31.10.216:80 "GET /compute/v2.1/servers/bad005cf-17e1-48ad-b9f7-e9f9352beb13 HTTP/1.1" 200 1290
RESP: [200] Connection: close Content-Length: 1290 Content-Type: application/json Date: Tue, 16 Nov 2021 22:51:56 GMT OpenStack-API-Version: compute 2.1 Server: Apache/2.4.41 (Ubuntu) Vary: OpenStack-API-Version,X-OpenStack-Nova-API-Version X-OpenStack-Nova-API-Version: 2.1 x-compute-request-id: req-f2d31e2e-dfdb-4fcb-bdbd-abe899ea6dd7 x-openstack-request-id: req-f2d31e2e-dfdb-4fcb-bdbd-abe899ea6dd7
RESP BODY: {"server": {"id": "bad005cf-17e1-48ad-b9f7-e9f9352beb13", "name": "test-2", "status": "BUILD", "tenant_id": "135f59752049436e95ea0642343340ae", "user_id": "08c04c8e508c4a549df8302573e97dfb", "metadata": {}, "hostId": "", "image": {"id": "6db08272-a856-49da-8909-7c4c73ab0bac", "links": [{"rel": "bookmark", "href": "http://172.31.10.216/compute/images/6db08272-a856-49da-8909-7c4c73ab0bac"}]}, "flavor": {"id": "1", "links": [{"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/1"}]}, "created": "2021-11-16T22:51:56Z", "updated": "2021-11-16T22:51:56Z", "addresses": {}, "accessIPv4": "", "accessIPv6": "", "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/servers/bad005cf-17e1-48ad-b9f7-e9f9352beb13"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/servers/bad005cf-17e1-48ad-b9f7-e9f9352beb13"}], "OS-DCF:diskConfig": "MANUAL", "progress": 0, "OS-EXT-AZ:availability_zone": "", "config_drive": "", "key_name": null, "OS-SRV-USG:launched_at": null, "OS-SRV-USG:terminated_at": null, "OS-EXT-SRV-ATTR:host": null, "OS-EXT-SRV-ATTR:instance_name": "", "OS-EXT-SRV-ATTR:hypervisor_hostname": null, "OS-EXT-STS:task_state": "scheduling", "OS-EXT-STS:vm_state": "building", "OS-EXT-STS:power_state": 0, "os-extended-volumes:volumes_attached": []}}
GET call to compute for http://172.31.10.216/compute/v2.1/servers/bad005cf-17e1-48ad-b9f7-e9f9352beb13 used request id req-f2d31e2e-dfdb-4fcb-bdbd-abe899ea6dd7
REQ: curl -g -i -X GET http://172.31.10.216/image/v2/images/6db08272-a856-49da-8909-7c4c73ab0bac -H "User-Agent: openstacksdk/0.59.0 keystoneauth1/4.4.0 python-requests/2.26.0 CPython/3.8.10" -H "X-Auth-Token: {SHA256}47d331cb112e7ffd3549b9958f475e4a36ab4d7848770b6ca54f337a6ca9a37e"
Resetting dropped connection: 172.31.10.216
http://172.31.10.216:80 "GET /image/v2/images/6db08272-a856-49da-8909-7c4c73ab0bac HTTP/1.1" 200 976
RESP: [200] Connection: close Content-Length: 976 Content-Type: application/json Date: Tue, 16 Nov 2021 22:51:56 GMT Server: Apache/2.4.41 (Ubuntu) x-openstack-request-id: req-d8be6e98-606b-4fcb-891a-4326c6fe18e9
RESP BODY: {"hw_rng_model": "virtio", "owner_specified.openstack.md5": "", "owner_specified.openstack.object": "images/cirros-0.5.2-x86_64-disk", "owner_specified.openstack.sha256": "", "name": "cirros-0.5.2-x86_64-disk", "disk_format": "qcow2", "container_format": "bare", "visibility": "public", "size": 16300544, "virtual_size": 117440512, "status": "active", "checksum": "b874c39491a2377b8490f5f1e89761a4", "protected": false, "min_ram": 0, "min_disk": 0, "owner": "6e8819a82dda467b89e4499c2e2b1df6", "os_hidden": false, "os_hash_algo": "sha512", "os_hash_value": "6b813aa46bb90b4da216a4d19376593fa3f4fc7e617f03a92b7fe11e9a3981cbe8f0959dbebe36225e5f53dc4492341a4863cac4ed1ee0909f3fc78ef9c3e869", "id": "6db08272-a856-49da-8909-7c4c73ab0bac", "created_at": "2021-11-16T18:15:41Z", "updated_at": "2021-11-16T18:15:42Z", "tags": [], "self": "/v2/images/6db08272-a856-49da-8909-7c4c73ab0bac", "file": "/v2/images/6db08272-a856-49da-8909-7c4c73ab0bac/file", "schema": "/v2/schemas/image"}
GET call to image for http://172.31.10.216/image/v2/images/6db08272-a856-49da-8909-7c4c73ab0bac used request id req-d8be6e98-606b-4fcb-891a-4326c6fe18e9
REQ: curl -g -i -X GET http://172.31.10.216/compute/v2.1/flavors/1 -H "Accept: application/json" -H "User-Agent: python-novaclient" -H "X-Auth-Token: {SHA256}47d331cb112e7ffd3549b9958f475e4a36ab4d7848770b6ca54f337a6ca9a37e" -H "X-OpenStack-Nova-API-Version: 2.1"
Resetting dropped connection: 172.31.10.216
http://172.31.10.216:80 "GET /compute/v2.1/flavors/1 HTTP/1.1" 200 366
RESP: [200] Connection: close Content-Length: 366 Content-Type: application/json Date: Tue, 16 Nov 2021 22:51:56 GMT OpenStack-API-Version: compute 2.1 Server: Apache/2.4.41 (Ubuntu) Vary: OpenStack-API-Version,X-OpenStack-Nova-API-Version X-OpenStack-Nova-API-Version: 2.1 x-compute-request-id: req-385bcf60-f80b-4a4d-8a6e-d55e1b78f711 x-openstack-request-id: req-385bcf60-f80b-4a4d-8a6e-d55e1b78f711
RESP BODY: {"flavor": {"id": "1", "name": "m1.tiny", "ram": 512, "disk": 1, "swap": "", "OS-FLV-EXT-DATA:ephemeral": 0, "OS-FLV-DISABLED:disabled": false, "vcpus": 1, "os-flavor-access:is_public": true, "rxtx_factor": 1.0, "links": [{"rel": "self", "href": "http://172.31.10.216/compute/v2.1/flavors/1"}, {"rel": "bookmark", "href": "http://172.31.10.216/compute/flavors/1"}]}}
GET call to compute for http://172.31.10.216/compute/v2.1/flavors/1 used request id req-385bcf60-f80b-4a4d-8a6e-d55e1b78f711
+-------------------------------------+-----------------------------------------------------------------+
| Field                               | Value                                                           |
+-------------------------------------+-----------------------------------------------------------------+
| OS-DCF:diskConfig                   | MANUAL                                                          |
| OS-EXT-AZ:availability_zone         |                                                                 |
| OS-EXT-SRV-ATTR:host                | None                                                            |
| OS-EXT-SRV-ATTR:hypervisor_hostname | None                                                            |
| OS-EXT-SRV-ATTR:instance_name       |                                                                 |
| OS-EXT-STS:power_state              | NOSTATE                                                         |
| OS-EXT-STS:task_state               | scheduling                                                      |
| OS-EXT-STS:vm_state                 | building                                                        |
| OS-SRV-USG:launched_at              | None                                                            |
| OS-SRV-USG:terminated_at            | None                                                            |
| accessIPv4                          |                                                                 |
| accessIPv6                          |                                                                 |
| addresses                           |                                                                 |
| adminPass                           | L3e7F8A8YrtC                                                    |
| config_drive                        |                                                                 |
| created                             | 2021-11-16T22:51:56Z                                            |
| flavor                              | m1.tiny (1)                                                     |
| hostId                              |                                                                 |
| id                                  | bad005cf-17e1-48ad-b9f7-e9f9352beb13                            |
| image                               | cirros-0.5.2-x86_64-disk (6db08272-a856-49da-8909-7c4c73ab0bac) |
| key_name                            | None                                                            |
| name                                | test-2                                                          |
| progress                            | 0                                                               |
| project_id                          | 135f59752049436e95ea0642343340ae                                |
| properties                          |                                                                 |
| security_groups                     | name='default'                                                  |
| status                              | BUILD                                                           |
| updated                             | 2021-11-16T22:51:56Z                                            |
| user_id                             | 08c04c8e508c4a549df8302573e97dfb                                |
| volumes_attached                    |                                                                 |
+-------------------------------------+-----------------------------------------------------------------+
clean_up CreateServer: 
END return value: 0
stack@ip-172-31-10-216:~/devstack$ 
```