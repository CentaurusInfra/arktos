#!/bin/bash
if [ $# -ne 1 ]; then
  echo "RESOURCE_SERVER IP parameter is not set"
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


echo "Setting up environment parameters ......"
export SCALE_OUT_TP_ENABLE_DAEMONSET=false
export IS_RESOURCE_PARTITION=false
#export RESOURCE_SERVER=172.31.5.191
export RESOURCE_SERVER=$1

export TENANT_PARTITION_SERVICE_SUBNET=10.0.0.0/16

DOCKER_SOCK=/var/run/docker.sock
sudo chmod o+rw $DOCKER_SOCK; sudo ls -alg $DOCKER_SOCK

echo "Start scale-out cluster TP1 node ......"
# If there is no code changes, uncomment out the following 1st line,
# comment out the following 2nd line
#./hack/arktos-up-scale-out-poc.sh -O
make clean; ./hack/arktos-up-scale-out-poc.sh

exit 0
