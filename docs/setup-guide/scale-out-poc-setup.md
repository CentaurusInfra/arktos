### Brief 

This introduces the tool to set up the the nginx proxy, tenant-partition cluster(s), and the resource-partition cluster(s).

### Environment variables setup
```bash
export TENANT_PARTITION_IP=[tenant-partition-ip]
export RESOURCE_PARTITION_IP=[resource-partition-ip]
export SCALE_OUT_PROXY_ENDPOINT=http://[nginx-proxy-ip]:8888
```

### nginx proxy setup
On the machine to run nginx proxy (it could be the tenant partition machine, the resource partition machine, or a third machine), run:

```bash
./hack/scale_out_poc/setup_nginx_proxy.sh
```

### On the TENANT partition:

Run 
```bash
export IS_RESOURCE_PARTITION=false 
./hack/arktos-up-scale-out-poc.sh
```

### On the RESOURCE partition:

Run 
```bash
export IS_RESOURCE_PARTITION=true 
./hack/arktos-up-scale-out-poc.sh
```

### Tests done
This is verified on my local setups with two AWS machines. 
