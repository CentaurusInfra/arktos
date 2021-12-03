#!/bin/bash

export PATH=$PATH

OPT_CNI_BIN_DIR=/opt/cni/bin
ETC_CNI_NETD_DIR=/etc/cni/net.d
echo "Clean up two directories $OPT_CNI_BIN_DIR and $ETC_CNI_NETD_DIR ......"

sudo rm -f $OPT_CNI_BIN_DIR/*
sudo ls -alg $OPT_CNI_BIN_DIR

sudo rm -f $ETC_CNI_NETD_DIR/bridge.conf
sudo rm -f $ETC_CNI_NETD_DIR/10-flannel.conflist
sudo ls -alg $ETC_CNI_NET_DIR

export ARKTOS_NO_CNI_PREINSTALLED=y

DOCKER_SOCK=/var/run/docker.sock
sudo chmod o+rw $DOCKER_SOCK; sudo ls -alg $DOCKER_SOCK

echo "Start scale-up cluster master node ......"
# If there is no code changes, uncomment out the following 1st line,
# comment out the following 2nd line
#CNIPLUGIN=flannel ./hack/arktos-up.sh -O
make clean; CNIPLUGIN=flannel ./hack/arktos-up.sh


exit 0
