
# Arktos Apiserver Partition

[![Go Report Card](https://goreportcard.com/badge/github.com/futurewei-cloud/arktos)](https://goreportcard.com/report/github.com/futurewei-cloud/arktos)
[![LICENSE](https://img.shields.io/badge/license-apache%202.0-green)](https://github.com/futurewei-cloud/arktos/blob/master/LICENSE)


## What Arktos Apiserver Partition is
Arktos Apiserver Partition is a lightweight componizing script to deploy Arktos by its function declarations. It runs like bash hack/arktos-apiserver-partition.sh function_name to achieve deployement flexiblities.

## Key Features of Arktos Apiserver Partiion

### Partition
Feel free to assgin kube-apiserver service group id by export APISERVER_SERVICEGROUPID=n (n=1,2,3,...). A data partition yaml is to define the data range to watch

### Deploy across hosts
Since we build a shared ETCD cluster, all the data can be shared across hosts

## Set up environment

### Clone repo
```
mkdir -p $GOPATH/src/
cd $GOPATH/src/
git clone https://github.com/futurewei-cloud/arktos
```

### Install Goland 
```
sudo apt-get update
sudo apt-get -y upgrade
cd /tmp
wget https://dl.google.com/go/go1.12.9.linux-amd64.tar.gz
sudo tar -xvf go1.12.9.linux-amd64.tar.gz
sudo mv go /usr/local
```
Add the following lines to .profile
```
GOROOT=/usr/local/go
GOPATH=$HOME/go
PATH=$GOPATH/bin:$GOROOT/bin:$PATH
```
Update the current shell session
```
source ~/.profile
```

### Install gcc and make. There might be an issue to build images. It can be fixed by running "git tag -a v2.7.4"
```
sudo apt update
sudo apt install build-essential
sudo apt-get install manpages-dev
sudo apt install make
make clean
```

### Install Containerd 
```
sudo apt-get update
sudo apt-get install libseccomp2
wget https://storage.googleapis.com/cri-containerd-release/cri-containerd-1.1.8.linux-amd64.tar.gz
sudo tar --no-overwrite-dir -C / -xzf cri-containerd-1.1.8.linux-amd64.tar.gz
sudo systemctl start containerd
```

Please make it sure run make clean if you have any changes to apply


## Deployment Steps


### Start apiserver on a master server

```
cd $GOPATH/src/github.com/arktos
hack/arktos-up.sh
```

### Start apiserver on a worker host

Verify we can access etcd from the worker host
```
curl -L http://[master server ip]:2379/v3beta/kv/put -X POST -d '{"key":"key1","value":"YmFy"}'
curl -L http://[master server ip]::2379/v3beta/kv/range -X POST -d '{"key":"key1"}'

```
If the key1's value is verfied to save in etcd, run the following command to start another apiserver on a worker host
```
export APISERVER_SERVICEGROUPID=n (n=2,3,..)
bash hack/arktos-apiserver-partition.sh start_apiserver [etcd ip]
```

### Copy kubeconfig from the master server to the worker host
```
./hack/create-kubeconfig.sh command kubeconfig_target_filepath
```
It will create a command to create a kubeconfig file for the worker host using the filepath kubeconfig_target_filepath and run the command on the worker host to get different kubeconfig to access different apiservers

### Run the workload controller managers with multiple apiserver configs
```
bash ./hack/arktos-apiserver-partition.sh start_workload_controller_manager kubeconfig_target_filepath1 kubeconfig_target_filepath2 kubeconfig_target_filepath3, ...
```

### Run the kube controller managers with multiple apiserver configs
```
bash ./hack/arktos-apiserver-partition.sh start_kube_controller_manager kubeconfig_target_filepath1 kubeconfig_target_filepath2 kubeconfig_target_filepath3, ...
```

### Run the kube scheduler with multiple apiserver configs
```
bash ./hack/arktos-apiserver-partition.sh start_kube_scheduler kubeconfig_target_filepath1 kubeconfig_target_filepath2 kubeconfig_target_filepath3, ...
```

### Skip building steps
Copy existing binaries to a folder, for example, /home/ubuntu/output/
Run the following commands to skip building
```
export BINARY_DIR=/home/ubuntu/output/
bash ./hack/arktos-apiserver-partition.sh start_XXX ...
```

## Test Senarios
1. No matter what partitions have been set, we always load default tenant
```
cluster/kubectl.sh  get tenants -T default
tail -f /tmp/kube-apiserver.log | grep default
```

2. Create different tenants
```
cluster/kubectl.sh apply -f tenant1.yaml
tail -f /tmp/kube-apiserver.log | grep tenant1
```

3. Create different namespaces under tenants 
```
cluster/kubectl.sh apply -f namespace1.yaml
tail -f /tmp/kube-apiserver.log | grep namespace1
```

4. Create different deployments under tenants 
```
cluster/kubectl.sh apply -f deployment1.yaml
tail -f /tmp/kube-apiserver.log | grep deployment1
```

5. Create different replicatesets under tenants 
```
cluster/kubectl.sh apply -f replicateset1.yaml
tail -f /tmp/kube-apiserver.log | grep replicateset1
```

