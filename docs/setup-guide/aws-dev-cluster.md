# How to Setup a multi-node Arktos cluster in AWS

Use cases for a Arktos multi-node dev cluster on AWS are to test features in cloud deployments, and to test performance and scaling with tools such as kubemark. This document outlines the steps to deploy such a cluster on AWS, and to deploy for kubemark based performance testing.

## Prerequisites

1. You will need an AWS account, and awscli configured in your bash profile. Please refer to AWS CLI configuration documentation.

1. You will need python, golang, docker, and build-essential installed.

## Arktos cluster start-up

1. Build the Arktos release binaries from a bash terminal from your Arktos source root directory.
```bash
make clean
source $PWD/aws_build_version && make quick-release
```

2. Create a VPC and subnet in AWS. Note down the aws zone, vpd-id, subnet-id to use.

3. Edit aws_kube_params.inc file and set the KUBE_AWS_ZONE, VPC_ID, SUBNET_ID parameters. Optionally, configure NUM_NODES to change the number of Arktos worker nodes, and KUBEMARK_NUM_NODES to change the number of hollow-nodes for kubemark.

4. To deploy the admin cluster in AWS, run kube-up script as follows:
```bash
source $PWD/aws_kube_params.inc && ./cluster/kube-up.sh
```
kube-up script displays the admin cluster details upon successful deployment.

5. To start kubemark master, run the following:
```bash
source $PWD/aws_kube_params.inc && ./test/kubemark/start-kubemark.sh
```
start-kubemark script will print the kubemark master details upon successful deployment. Note down the External IP of kubemark master node.

## Using the admin cluster and kubemark cluster

1. To use admin cluster, just use kubectl. The build node is setup with the config to access the admin cluster. e.g:
```bash
./cluster/kubectl.sh get nodes -o wide
```

2. To use kubemark cluster, use the cluster kubectl.sh with kubemark local kubeconfig as follows:
```bash
./cluster/kubectl.sh --kubeconfig=$PWD/test/kubemark/resources/kubeconfig.kubemark get no
```

## Running kubemark perf tests
```bash
root@node:~/go/src/k8s.io# git clone https://github.com/kubernetes/perf-tests
root@node:~/go/src/k8s.io# MASTER_IP=<Kubemark_Master_External_IP> ./perf-tests/run-e2e.sh cluster-loader2 --provider=kubemark --report-dir=/tmp/perflogs/ --kubeconfig=<Repo_Root>/test/kubemark/resources/kubeconfig.kubemark --testconfig=testing/load/config.yaml
```

## Arktos cluster tear-down

1. To stop kubemark master, run the following:
```bash
source $PWD/aws_kube_params.inc && ./test/kubemark/stop-kubemark.sh
```

2. To terminate admin cluster, run the following:
```bash
source $PWD/aws_kube_params.inc && ./cluster/kube-down.sh
```

### Configuration Options

These AWS specific options can be set as environment variables to customize how your cluster is created.

**KUBERNETES_PROVIDER**, **CLOUD_PROVIDER**:
The cloud provider for kube-up, start-kubemark VMs. Defaults to gce.

**KUBE_AWS_INSTANCE_PREFIX**:
The instance prefix for naming VMs. Defaults to 'kubernetes'.

**KUBE_AWS_ZONE**:
The AWS availability zone to deploy to. Defaults to us-east-2a.

**AWS_IMAGE**:
The AMI to use. If not specified, the image will be selected based on the AWS region.

**AWS_S3_BUCKET**, **AWS_S3_REGION**:
The bucket name to use, and the region where the bucket should be created, or where the bucket is located if it exists already.
If not specified, defaults to AWS_S3_REGION us-east-2. AWS_S3_BUCKET will default to a uniquely generated name.
AWS_S3_REGION is useful for people that want to control their data location, because of regulatory restrictions for example.

**AWS_SSH_KEY**:
Location of AWS ssh public key file to allow authorization to cluster API.

**MASTER_SIZE**, **NODE_SIZE**, **KUBEMARK_MASTER_SIZE**:
The instance type to use for creating the master/worker/kubemark-master.

**NUM_NODES**:
The number of worker nodes to deploy using kube-up.

**KUBEMARK_NUM_NODES**:
The number of kubemark hollow nodes to deploy using start-kubemark.

**MASTER_DISK_SIZE**, **MASTER_ROOT_DISK_SIZE**, **KUBEMARK_MASTER_ROOT_DISK_SIZE**:
Disk size for kubernetes and kubemark master nodes.
