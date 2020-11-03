### Setup proxy

Please follow [these steps](https://github.com/futurewei-cloud/arktos-perftest/wiki/How-to-setup-and-config-nginx-proxy-for-scale-out-design#setting-up-proxy). 

The proxy can run on a machine of your choice. Make sure to update the ip addresses in the Nginx config for your partition servers.

This is a one-time setup and pretty straightforward so it's not included in this PR as a script. 

Will automate this in another PR.

### Set proxy endpoint:

This to to be done on ***both tenant and resource partitions***.

Set the proxy url like

```bash
export SCALE_OUT_PROXY_ENDPOINT="http://192.168.0.120:8888"
```

### On the TENANT partition:

Run 
```bash
IS_RESOURCE_PARTITION=false arktos-up-scale-out-poc.sh
```

This will start all the components except kubelet, kubeproxy, and nodelifecycle and nodeipam controllers are disabled

### On the RESOURCE partition:

Run 
```bash
IS_RESOURCE_PARTITION=true arktos-up-scale-out-poc.sh
```

This will start all the components except scheduler, and only nodelifecycle and nodeipam controllers are enabled

### Tests done
This is verified on my local setups with two VMs. Proxy running on one of the VMs.
