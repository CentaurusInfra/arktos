### Brief 

This introduces the tool to set up the tenant-partition cluster and the resource-partition cluster.

### On the TENANT partition:

Run 
```bash
IS_RESOURCE_PARTITION=false INSTALL_SCALE_OUT_PROXY=true SECOND_PARTITION_IP=[resource-partition-ip] hack/arktos-up-scale-out-poc.sh
```

This will 
1. install the nginx proxy in the local host of tenant-partition, 
2. start all the components except kubelet, kubeproxy,
3. disable nodelifecycle and nodeipam controllers.

### On the RESOURCE partition:

Run 
```bash
IS_RESOURCE_PARTITION=true SCALE_OUT_PROXY_ENDPOINT=http://[tenant-partition-ip]:8888 hack/arktos-up-scale-out-poc.sh
```

This will start all the components except scheduler, and only nodelifecycle and nodeipam controllers are enabled

### Tests done
This is verified on my local setups with two AWS machines. 
