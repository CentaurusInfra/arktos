#!/bin/bash

if [ $# -ne 1 ]; then
  echo "MASTER IP parameter is not set"  
  exit 1
fi

MASTER_IP=$1
echo $MASTER_IP

export PATH=$PATH
AWS_PRIVATE_KEY="/home/ubuntu/AWS/keypair/CarlXieKeyPairAccessFromWin.pem"
SECRET_DIR="/var/run/kubernetes"

if [ -f /tmp/kubelet.worker.log ]; then
  mv /tmp/kubelet.worker.log /tmp/kubelet.worker.log.old
fi

if [ -d $SECRET_DIR ]; then
  sudo rm $SECRET_DIR/*
else
  sudo mkdir -p $SECRET_DIR
  sudo chown ubuntu $SECRET_DIR
fi 	
sudo ls -alg $SECRET_DIR

echo "Copy secret files from master node $MASTER_IP ......"
for secret in kubelet.kubeconfig client-ca.crt kube-proxy.kubeconfig
do
  scp -i "$AWS_PRIVATE_KEY" ubuntu@$MASTER_IP:$SECRET_DIR/$secret $SECRET_DIR/$secret
done

sudo ls -alg $SECRET_DIR
echo "Sleep 5 seconds ......"
sleep 5

OPT_CNI_BIN=/opt/cni/bin
ETC_CNI_NET_DIR=/etc/cni/net.d
echo "Clean up two directories $OPT_CNI_BIN and $ETC_CNI_NET_DIR ......"

sudo rm -f $OPT_CNI_BIN/*
sudo ls -alg $OPT_CNI_BIN

sudo rm -f $ETC_CNI_NET_DIR/bridge.conf
sudo rm -f $ETC_CNI_NET_DIR/10-flannel.conflist
sudo ls -alg $ETC_CNI_NET_DIR

export ARKTOS_NO_CNI_PREINSTALLED=y
export KUBELET_IP=`hostname -i`; echo $KUBELET_IP

DOCKER_SOCK=/var/run/docker.sock
sudo chmod o+rw $DOCKER_SOCK; sudo ls -alg $DOCKER_SOCK

# If there is no code changes, uncomment out the following 1st line, 
# comment out the following 2nd line
#API_HOST_IP_EXTERNAL=$MASTER_IP CNIPLUGIN=flannel ./hack/arktos-worker-up.sh -O
make clean; API_HOST_IP_EXTERNAL=$MASTER_IP CNIPLUGIN=flannel ./hack/arktos-worker-up.sh

sleep 10
echo "Checking kubelet process ......"
ps -ef |grep kubelet |grep -v grep

ls -al /tmp/*.log

echo "Checking flannel process ......"
ps -ef |grep flannel |grep -v grep

cat /tmp/flanneld.log
cat /run/flannel/subnet.env
cat /tmp/net-conf.json
sudo cat /etc/cni/net.d/10-flannel.conflist
ip route
route -n
ifconfig -a
arp -n

sleep 5
echo "Checking kube-proxy process ......"
ps -ef |grep kube-proxy |grep -v grep
cat /tmp/kube-proxy.yaml

exit 0
