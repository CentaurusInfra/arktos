# Arktos to Enforce the Multi-tenant with Mizar Network Feature

This document captures the steps applied to an Arktos cluster lab enabling the unique multi-tenant network feature. The machines in this lab used are AWS EC2 t2-large (2 CPUs, 8GB mem), Ubuntu 18.04 LTS.

The steps might change with the progress of development.
  
1. Update Kernel
To check kernel, run following command

```bash
uname -a
```

To update Kernel, download and run:
If it is `5.6.0-rc2` then you can skip downloading ```kernelupdate.sh```
```bash
wget https://raw.githubusercontent.com/CentaurusInfra/mizar/dev-next/kernelupdate.sh
bash kernelupdate.sh
```

2. Start Arktos cluster
```bash
CNIPLUGIN=mizar ./hack/arktos-up.sh
```

3. Leave the "arktos-up.sh" terminal and opened a another terminal to the master node. Run the following command to confirm that the first network, "default", in system tenant, has already been created. Its state is empty at this moment.
```bash
./cluster/kubectl.sh get net
NAME      TYPE    VPC                      PHASE   DNS
default   mizar   system-default-network    
```

Now, the default network of system tenant should be Ready.
```bash
./cluster/kubectl.sh get net
NAME      TYPE   VPC                       PHASE   DNS
default   mizar  system-default-network    Ready   10.0.0.207
```

From now on, you should be able to play with multi-tenant and the network features.
