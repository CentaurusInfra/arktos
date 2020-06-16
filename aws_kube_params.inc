export KUBERNETES_PROVIDER=aws
export CLOUD_PROVIDER=${KUBERNETES_PROVIDER}
export KUBE_AWS_INSTANCE_PREFIX=arktos

export KUBE_AWS_ZONE=us-east-1a
export VPC_ID=vpc-2d43fc57
export SUBNET_ID=subnet-5d076e73
export NUM_NODES=2
export KUBEMARK_NUM_NODES=100

export KUBEMARK_MASTER_SIZE=t2.xlarge
export MASTER_SIZE=t2.xlarge
export NODE_SIZE=t2.xlarge
export MASTER_DISK_SIZE=50
export MASTER_ROOT_DISK_SIZE=${MASTER_DISK_SIZE}
export KUBEMARK_MASTER_ROOT_DISK_SIZE=${MASTER_DISK_SIZE}
export KUBEMARK_AUTOSCALER_MAX_NODES=${KUBEMARK_NUM_NODES}
export NUM_REPLICAS=${KUBEMARK_NUM_NODES}
export NUM_WINDOWS_NODES=0
export AWS_SSH_KEY=$HOME/.ssh/id_rsa

export PRESET_INSTANCES_ENABLED=false
export IS_PRESET_INSTANCES_DRY_RUN=false  # In Preset instance, if it's dry run, then it only generates script but does not deploy. 
export PRESET_KUBE_MASTER_IP=172.31.6.63
export PRESET_KUBE_MINION1_IP=172.31.11.206
# Uncomment next line and set minion2 ip to enable deploy 2-nodes cluster
# export PRESET_KUBE_MINION2_IP=172.31.11.XXX

export PRESET_KUBE_SUBNET_CIDR=172.31.0.0/16
export PRESET_KUBE_VPC_CIDR_BASE=172.31.0.0/
export PRESET_IGW_ID=igw-ff4ae997
export PRESET_ROUTE_TABLE_ID=rtb-43539d28
export PRESET_MASTER_SG_ID=sg-0454a7fee09174cdb
export PRESET_NODE_SG_ID=sg-0f04c3466926ecd72

export PRESET_SERVER_BINARY_TAR_URL=/tmp/kubernetes-server-linux-amd64.tar.gz
export PRESET_BOOTSTRAP_SCRIPT_URL=/tmp/bootstrap-script
