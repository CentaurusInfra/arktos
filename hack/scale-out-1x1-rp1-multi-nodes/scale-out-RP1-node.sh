#!/bin/bash

if [ $# -ne 1 ]; then
  echo "TENANT_SERVER IP parameter is not set"
  exit 1
fi

echo $1

export PATH=$PATH

OPT_CNI_BIN_DIR=/opt/cni/bin
ETC_CNI_NETD_DIR=/etc/cni/net.d
echo "Clean up two directories $OPT_CNI_BIN_DIR and $ETC_CNI_NETD_DIR ......"

sudo rm -f $OPT_CNI_BIN_DIR/*
sudo ls -alg $OPT_CNI_BIN_DIR

sudo rm -f $ETC_CNI_NETD_DIR/bridge.conf
sudo rm -f $ETC_CNI_NETD_DIR/10-flannel.conflist
sudo ls -alg $ETC_CNI_NETD_DIR


export IS_RESOURCE_PARTITION=true
#export TENANT_SERVER=172.31.3.192
export TENANT_SERVER=$1

export RESOURCE_PARTITION_POD_CIDR=10.244.0.0/16

DOCKER_SOCK=/var/run/docker.sock
sudo chmod o+rw $DOCKER_SOCK; sudo ls -alg $DOCKER_SOCK

echo "Start scale-out cluster RP1 node ......"
# If there is no code changes, uncomment out the following 1st line,
# comment out the following 2nd line
#./hack/arktos-up-scale-out-poc.sh -O
make clean; ./hack/arktos-up-scale-out-poc.sh

exit 0

