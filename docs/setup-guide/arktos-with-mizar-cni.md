# Arktos deployment with Mizar CNI 

The machines in this lab used are AWS EC2 t2-large (2 CPUs, 8GB mem), Ubuntu 18.04 LTS.

The steps might change with the progress of development.

1. To install dependencies required for Arktos, run the following commands: 
```bash
wget https://raw.githubusercontent.com/CentaurusInfra/arktos/master/hack/setup-dev-node.sh
sudo bash setup-dev-node.sh
git clone https://github.com/CentaurusInfra/arktos.git ~/go/src/k8s.io/arktos
echo export PATH=$PATH:/usr/local/go/bin\ >> ~/.profile
echo cd \$HOME/go/src/k8s.io/arktos >> ~/.profile
source ~/.profile
```
  
2. To check kernel, run the following command:

```bash
uname -a
```

If kernel version is `5.6.0-rc2` or higher then you can skip the step 3.

3. To update Kernel, run following commands:
```bash
wget https://raw.githubusercontent.com/CentaurusInfra/mizar/dev-next/kernelupdate.sh
sudo bash kernelupdate.sh
```

4. Start Arktos cluster
```bash
CNIPLUGIN=mizar ./hack/arktos-up.sh
```

5. Leave the "arktos-up.sh" terminal and open another terminal to the master node. Run the following command to confirm that the first network, "default", in system tenant, has already been created. 

Now, the default network of system tenant should be `Ready`.
```bash
./cluster/kubectl.sh get net
NAME      TYPE   VPC                       PHASE   DNS
default   mizar  system-default-network    Ready   10.0.0.207
```

Now you can use the Arktos cluster with Mizar CNI.
