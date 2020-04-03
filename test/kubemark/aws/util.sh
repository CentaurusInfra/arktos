#!/usr/bin/env bash

# Copyright 2020 Authors of Arktos.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/../../..

source "${KUBE_ROOT}/test/kubemark/common/util.sh"

function authenticate-docker {
  echo "Configuring AWS registry authentication"

  local aws_account_id=""
  local awscliv1=false
  if [[ $(aws --version) == "aws-cli/1."* ]]; then
    awscliv1=true
  fi

  if [ "$awscliv1" = true ]; then
    local docker_login_str="$(aws ecr get-login | sed 's/-e none //')"
    KUBEMARK_IMAGE_REGISTRY=$(echo $docker_login_str | awk '{print $7}' | sed 's/https:\/\///')
    eval $docker_login_str
    aws_account_id=$(echo $KUBEMARK_IMAGE_REGISTRY | cut -d. -f1)
  else
    aws_account_id=$(aws iam get-user --query "User.Arn" | awk -F ":" '{print $5}')
    KUBEMARK_IMAGE_REGISTRY=$aws_account_id.dkr.ecr.$AWS_REGION.amazonaws.com
    aws ecr get-login-password | sudo -E docker login --username AWS --password-stdin $KUBEMARK_IMAGE_REGISTRY
  fi
  set +e
  # TODO: remove kubemark hard-coding
  aws ecr describe-repositories --registry-id $aws_account_id --repository-names "kubemark"
  local repository_exists=$?
  set -e
  if [ $repository_exists -ne 0 ]; then
    aws ecr create-repository --repository-name "kubemark"
    if [ $? -eq 0 ]; then
      echo "Created ECR repository kubemark in AWS"
    fi
  fi
  echo "AWS registry auth successful. Registry is: $KUBEMARK_IMAGE_REGISTRY"
}

# Delete the image given by $1.
function delete-image() {
  # TODO: remove kubemark hard-coding
  local imgTag=$(echo $1 | cut -d: -f2)
  aws ecr batch-delete-image --repository-name "kubemark" --image-ids imageTag=$imgTag
}

function get-kubemark-ssh-key {
  if [[ ! -f "$AWS_SSH_KEY" ]]; then
    echo "ERROR: Run aws-kube-up.sh to generated SSH key for AWS"
    exit 1
  fi

  # Note that we use get-ssh-fingerprint, so this works on OSX Mavericks
  # get-aws-fingerprint gives the same fingerprint that AWS computes,
  # but OSX Mavericks ssh-keygen can't compute it
  AWS_SSH_KEY_FINGERPRINT=$(get-ssh-fingerprint ${AWS_SSH_KEY}.pub)
  echo "Using SSH key with (AWS) fingerprint: ${AWS_SSH_KEY_FINGERPRINT}"
  AWS_SSH_KEY_NAME="kubemark-${AWS_SSH_KEY_FINGERPRINT//:/}"

  import-public-key ${AWS_SSH_KEY_NAME} ${AWS_SSH_KEY}.pub
}

function get-kubemark-master-sg {
  MASTER_SG_ID=$($AWS_CMD describe-security-groups \
                 --filters "Name=group-name,Values=kubernetes-master-${INSTANCE_PREFIX}" \
                 --query "SecurityGroups[0].GroupId")
  echo "Using master security group ID $MASTER_SG_ID"
}

function detect-kubemark-root-device {
  local master_image=${AWS_IMAGE}
  ROOT_DEVICE_MASTER=$($AWS_CMD describe-images --image-ids ${master_image} --query 'Images[].RootDeviceName')
  MASTER_BLOCK_DEVICE_MAPPINGS="[{\"DeviceName\":\"${ROOT_DEVICE_MASTER}\",\"Ebs\":{\"DeleteOnTermination\":true,\"VolumeSize\":${MASTER_ROOT_DISK_SIZE},\"VolumeType\":\"${MASTER_ROOT_DISK_TYPE}\"}} ${EPHEMERAL_BLOCK_DEVICE_MAPPINGS}]"
}

function ensure-kubemark-temp-dir {
  if [[ -z ${KUBEMARK_TEMP-} ]]; then
    KUBEMARK_TEMP=$(mktemp -d -t kubemark.XXXXXX)
    trap 'rm -rf "${KUBEMARK_TEMP}"' EXIT
  fi
}

function create-kubemark-bootstrap-script {
  ensure-kubemark-temp-dir

  KUBEMARK_BOOTSTRAP_SCRIPT="${KUBEMARK_TEMP}/kubemark-bootstrap-script"

  (
    # Include the default functions from the configure-vm script
    sed '/^#+AWS_OVERRIDES_HERE/,$d' "${KUBE_ROOT}/cluster/aws/configure-vm.sh"
    # Include the configure-vm directly-executed code
    sed -e '1,/^#+AWS_OVERRIDES_HERE/d' "${KUBE_ROOT}/cluster/aws/configure-vm.sh"
  ) > "${KUBEMARK_BOOTSTRAP_SCRIPT}"
}

function upload-kubemark-bootstrap-script {
  if [[ -z ${AWS_S3_BUCKET-} ]]; then
      local project_hash=
      local key=$(aws configure get aws_access_key_id)
      if which md5 > /dev/null 2>&1; then
        project_hash=$(md5 -q -s "${USER} ${key} ${INSTANCE_PREFIX}")
      else
        project_hash=$(echo -n "${USER} ${key} ${INSTANCE_PREFIX}" | md5sum | awk '{ print $1 }')
      fi
      AWS_S3_BUCKET="kubernetes-staging-${project_hash}"
  fi

  if ! aws s3api get-bucket-location --bucket ${AWS_S3_BUCKET} > /dev/null 2>&1 ; then
    echo "AWS bucket location not found. Please run aws-kube-up prior to aws-kubemark."
    exit 1
  fi

  echo "Uploading to Amazon S3 bucket $AWS_S3_BUCKET"
  local s3_bucket_location=$(aws s3api get-bucket-location --bucket ${AWS_S3_BUCKET})
  local s3_url_base=https://s3-${s3_bucket_location}.amazonaws.com
  if [[ "${s3_bucket_location}" == "None" ]]; then
    # "US Classic" does not follow the pattern
    s3_url_base=https://s3.amazonaws.com
    s3_bucket_location=us-east-1
  elif [[ "${s3_bucket_location}" == "cn-north-1" ]]; then
    s3_url_base=https://s3.cn-north-1.amazonaws.com.cn
  fi

  local -r staging_path="devel"
  local -r local_dir="${KUBEMARK_TEMP}/s3/"
  mkdir ${local_dir}

  echo "+++ Staging server tars to S3 Storage: ${AWS_S3_BUCKET}/${staging_path}"
  cp -a "${KUBEMARK_BOOTSTRAP_SCRIPT}" ${local_dir}

  aws s3 sync --region ${s3_bucket_location} --exact-timestamps ${local_dir} "s3://${AWS_S3_BUCKET}/${staging_path}/"

  local server_binary_path="${staging_path}/${SERVER_BINARY_TAR##*/}"
  SERVER_BINARY_TAR_URL="${s3_url_base}/${AWS_S3_BUCKET}/${server_binary_path}"

  local kubemark_bootstrap_script_path="${staging_path}/${KUBEMARK_BOOTSTRAP_SCRIPT##*/}"
  aws s3api put-object-acl --region ${s3_bucket_location} --bucket ${AWS_S3_BUCKET} --key "${kubemark_bootstrap_script_path}" --grant-read 'uri="http://acs.amazonaws.com/groups/global/AllUsers"'
  KUBEMARK_BOOTSTRAP_SCRIPT_URL="${s3_url_base}/${AWS_S3_BUCKET}/${kubemark_bootstrap_script_path}"

  echo "Uploaded kubemark bootstrap script:"
  echo "  SERVER_BINARY_TAR_URL: ${SERVER_BINARY_TAR_URL}"
  echo "  KUBEMARK_BOOTSTRAP_SCRIPT_URL: ${KUBEMARK_BOOTSTRAP_SCRIPT_URL}"

  SERVER_BINARY_TAR_HASH=$(sha1sum-file "${SERVER_BINARY_TAR}")
  BOOTSTRAP_SCRIPT_HASH=$(sha1sum-file "${KUBEMARK_BOOTSTRAP_SCRIPT}")
}

function setup-kubemark-master-env {
  write-master-env
  cat >>${KUBE_TEMP}/master-kube-env.yaml <<EOF
KUBEMARK_MASTER: $(yaml-quote "true")
EOF

  (
    # We pipe this to the ami as a startup script in the user-data field.  Requires a compatible ami
    echo "#! /bin/bash"
    echo "mkdir -p /var/cache/kubernetes-install"
    echo "cd /var/cache/kubernetes-install"

    echo "cat > kube_env.yaml << __EOF_MASTER_KUBE_ENV_YAML"
    cat ${KUBE_TEMP}/master-kube-env.yaml
    echo "AUTO_UPGRADE: 'true'"
    # TODO: get rid of these exceptions / harmonize with common or GCE
    echo "DOCKER_STORAGE: $(yaml-quote ${DOCKER_STORAGE:-})"
    echo "API_SERVERS: $(yaml-quote ${MASTER_INTERNAL_IP:-})"
    echo "MASTER_EIP: $(yaml-quote ${MASTER_IP:-})"
    echo "__EOF_MASTER_KUBE_ENV_YAML"
    echo ""
    echo "wget -O bootstrap ${KUBEMARK_BOOTSTRAP_SCRIPT_URL}"
    echo "chmod +x bootstrap"
    echo "mkdir -p /etc/kubernetes"
    echo "mv kube_env.yaml /etc/kubernetes"
    echo "mv bootstrap /etc/kubernetes/"
    echo "cat > /etc/rc.local << EOF_RC_LOCAL"
    echo "#!/bin/sh -e"
    # We want to be sure that we don't pass an argument to bootstrap
    echo "/etc/kubernetes/bootstrap"
    echo "exit 0"
    echo "EOF_RC_LOCAL"
    echo "/etc/kubernetes/bootstrap"
  ) > "${KUBEMARK_TEMP}/kubemark-master-user-data"

  # Compress the data to fit under the 16KB limit (cloud-init accepts compressed data)
  gzip "${KUBEMARK_TEMP}/kubemark-master-user-data"

}

# Generate certs/keys for CA, master, kubelet and kubecfg, and tokens for kubelet
# and kubeproxy.
function generate-pki-config {
  kube::util::ensure-temp-dir
  gen-kube-bearertoken
  gen-kube-basicauth
  if [ "${CLOUD_PROVIDER}" = "aws" ]; then
    create-certs "${MASTER_INTERNAL_IP}" "${KUBEMARK_MASTER_IP}"
  else
    create-certs "${MASTER_IP}"
  fi
  create-etcd-apiserver-certs "etcd-${MASTER_NAME}" "${MASTER_NAME}"
  KUBELET_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
  KUBE_PROXY_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
  NODE_PROBLEM_DETECTOR_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
  HEAPSTER_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
  CLUSTER_AUTOSCALER_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
  KUBE_DNS_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
  echo "Generated PKI authentication data for kubemark."
}

# Write kubeconfig to ${RESOURCE_DIRECTORY}/kubeconfig.kubemark in order to
# use kubectl locally.
function write-local-kubeconfig {
  LOCAL_KUBECONFIG="${RESOURCE_DIRECTORY}/kubeconfig.kubemark"
  cat > "${LOCAL_KUBECONFIG}" << EOF
apiVersion: v1
kind: Config
users:
- name: kubecfg
  user:
    client-certificate-data: "${KUBECFG_CERT_BASE64}"
    client-key-data: "${KUBECFG_KEY_BASE64}"
    username: ubuntu
    password: ubuntu
clusters:
- name: kubemark
  cluster:
    certificate-authority-data: "${CA_CERT_BASE64}"
    server: https://${MASTER_IP}
contexts:
- context:
    cluster: kubemark
    user: kubecfg
  name: kubemark-context
current-context: kubemark-context
EOF
  echo "Kubeconfig file for kubemark master written to ${LOCAL_KUBECONFIG}."
}

function get-or-create-master-ip {
  ZONE=$KUBE_AWS_ZONE
  ensure-master-pd
  find-tagged-master-ip
  if [[ ! -z "${KUBE_MASTER_IP:-}" ]]; then
    MASTER_IP=$KUBE_MASTER_IP
  fi

  if [[ -z "${MASTER_IP:-}" ]]; then
    # Check if MASTER_RESERVED_IP looks like an IPv4 address
    # Note that we used to only allocate an elastic IP when MASTER_RESERVED_IP=auto
    # So be careful changing the IPV4 test, to be sure that 'auto' => 'allocate'
    if [[ "${MASTER_RESERVED_IP}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      MASTER_IP="${MASTER_RESERVED_IP}"
      echo "Using reserved Elastic IP for master: ${MASTER_IP}"
    else
      MASTER_IP=`$AWS_CMD allocate-address --domain vpc --query PublicIp`
      echo "Allocated Elastic IP for master: ${MASTER_IP}"
    fi

    # We can't tag elastic ips.  Instead we put the tag on the persistent disk.
    # It is a little weird, perhaps, but it sort of makes sense...
    # The master mounts the master PD, and whoever mounts the master PD should also
    # have the master IP
    add-tag ${MASTER_DISK_ID} ${TAG_KEY_MASTER_IP} ${MASTER_IP}
  fi

  MASTER_IP_SUFFIX=.10
  MASTER_INTERNAL_IP="${SUBNET_CIDR%.*}${MASTER_IP_SUFFIX}"
  KUBEMARK_MASTER_IP=$MASTER_IP
}

function get-master-ip-and-create-config() {
  find-release-tars
  # We need master IP to generate PKI and kubeconfig for cluster.
  get-or-create-master-ip
  # TODO: Get rid of this hack
  if [ "${CLOUD_PROVIDER}" = "aws" ]; then
    MASTER_PUBLIC_IP=$MASTER_IP
    KUBE_MASTER_IP=$MASTER_INTERNAL_IP
    MASTER_IP=$MASTER_INTERNAL_IP
  fi
  generate-pki-config
  write-local-kubeconfig
}

# Write all environment variables that we need to pass to the kubemark master,
# locally to the file ${RESOURCE_DIRECTORY}/kubemark-master-env.sh.
function create-master-environment-file {
  cat > "${RESOURCE_DIRECTORY}/kubemark-master-env.sh" <<EOF
# Generic variables.
INSTANCE_PREFIX="${INSTANCE_PREFIX:-}"
SERVICE_CLUSTER_IP_RANGE="${SERVICE_CLUSTER_IP_RANGE:-}"
EVENT_PD="${EVENT_PD:-}"

# Etcd related variables.
ETCD_IMAGE="${ETCD_IMAGE:-3.3.10-1}"
ETCD_VERSION="${ETCD_VERSION:-}"

# Controller-manager related variables.
CONTROLLER_MANAGER_TEST_ARGS="${CONTROLLER_MANAGER_TEST_ARGS:-}"
ALLOCATE_NODE_CIDRS="${ALLOCATE_NODE_CIDRS:-}"
CLUSTER_IP_RANGE="${CLUSTER_IP_RANGE:-}"
TERMINATED_POD_GC_THRESHOLD="${TERMINATED_POD_GC_THRESHOLD:-}"

# Scheduler related variables.
SCHEDULER_TEST_ARGS="${SCHEDULER_TEST_ARGS:-}"

# Apiserver related variables.
APISERVER_TEST_ARGS="${APISERVER_TEST_ARGS:-}"
STORAGE_MEDIA_TYPE="${STORAGE_MEDIA_TYPE:-}"
STORAGE_BACKEND="${STORAGE_BACKEND:-etcd3}"
ETCD_SERVERS="${ETCD_SERVERS:-}"
ETCD_SERVERS_OVERRIDES="${ETCD_SERVERS_OVERRIDES:-}"
ETCD_COMPACTION_INTERVAL_SEC="${ETCD_COMPACTION_INTERVAL_SEC:-}"
RUNTIME_CONFIG="${RUNTIME_CONFIG:-}"
NUM_NODES="${KUBEMARK_NUM_NODES:-}"
CUSTOM_ADMISSION_PLUGINS="${CUSTOM_ADMISSION_PLUGINS:-}"
FEATURE_GATES="${FEATURE_GATES:-}"
KUBE_APISERVER_REQUEST_TIMEOUT="${KUBE_APISERVER_REQUEST_TIMEOUT:-}"
ENABLE_APISERVER_ADVANCED_AUDIT="${ENABLE_APISERVER_ADVANCED_AUDIT:-}"
EOF
  echo "Created the environment file for master."
}

function create-kubemark-master-with-resources {
  detect-image
  get-kubemark-ssh-key
  get-kubemark-master-sg
  detect-kubemark-root-device
  create-kubemark-bootstrap-script
  upload-kubemark-bootstrap-script
  setup-kubemark-master-env

  echo "Starting kubemark master $MASTER_NAME, Role '$MASTER_TAG', KubernetesCluster '$CLUSTER_ID'"

  master_id=$($AWS_CMD run-instances \
    --image-id $AWS_IMAGE \
    --iam-instance-profile Name=$IAM_PROFILE_MASTER \
    --instance-type $MASTER_SIZE \
    --subnet-id $SUBNET_ID \
    --private-ip-address $MASTER_INTERNAL_IP \
    --key-name ${AWS_SSH_KEY_NAME} \
    --security-group-ids ${MASTER_SG_ID} \
    --no-associate-public-ip-address \
    --block-device-mappings "${MASTER_BLOCK_DEVICE_MAPPINGS}" \
    --user-data fileb://${KUBEMARK_TEMP}/kubemark-master-user-data.gz \
    --query Instances[].InstanceId)
  add-tag $master_id Name $MASTER_NAME
  add-tag $master_id Role $MASTER_TAG
  add-tag $master_id KubernetesCluster ${CLUSTER_ID}

  echo "Waiting for kubemark master to be ready"
  local attempt=0

  local ip=""
  while true; do
    echo -n Attempt "$(($attempt+1))" to check for master node
    if [[ -z "${ip}" ]]; then
      # We are not able to add an elastic ip, a route or volume to the instance until that instance is in "running" state.
      wait-for-instance-state ${master_id} "running"

      #KUBE_MASTER=${MASTER_NAME}
      echo -e " ${color_green}[master running]${color_norm}"

      attach-ip-to-instance ${KUBEMARK_MASTER_IP} ${master_id}

      # This is a race between instance start and volume attachment.  There appears to be no way to start an AWS instance with a volume attached.
      # To work around this, we wait for volume to be ready in setup-master-pd.sh
      echo "Attaching persistent data volume (${MASTER_DISK_ID}) to master"
      $AWS_CMD attach-volume --volume-id ${MASTER_DISK_ID} --device /dev/sdb --instance-id ${master_id}

      # Get route table of cluster-one master
      ROUTE_TABLE_ID=$($AWS_CMD describe-route-tables \
                            --filters Name=vpc-id,Values=${VPC_ID} \
                                      Name=tag:KubernetesCluster,Values=${CLUSTER_ID} \
                            --query RouteTables[].RouteTableId)

      if [[ -z "${ROUTE_TABLE_ID}" ]]; then
        echo "Cluster-one route table not found, please run kube-up.sh before starting kubemark"
        exit 1
      fi

      sleep 4
      $AWS_CMD create-route --route-table-id $ROUTE_TABLE_ID --destination-cidr-block ${MASTER_IP_RANGE} --instance-id $master_id > $LOG

      break
    fi
    echo -e " ${color_yellow}[master not working yet]${color_norm}"
    attempt=$(($attempt+1))
    if (( attempt > 10 )); then
      echo
      echo -e "${color_red}master failed to start. Your cluster is unlikely" >&2
      echo "to work correctly. Please run ./cluster/kube-down.sh and re-create the" >&2
      echo -e "cluster. (sorry!)${color_norm}" >&2
      exit 1
    fi
    sleep 10
  done
  local ip=$(get_instance_public_ip ${master_id})
  local prip=$(get_instance_private_ip ${master_id})
  echo "Started kubemark master: public_ip= $ip, private_ip: $prip"
  MASTER_INTERNAL_IP=$prip
  if [[ -f "$HOME/.ssh/known_hosts" ]]; then
    ssh-keygen -f "$HOME/.ssh/known_hosts" -R $ip
  fi
}

# Command to be executed is '$1'.
# No. of retries is '$2' (if provided) or 1 (default).
function execute-cmd-on-master-with-retries() {
  # TODO: Add retries
  local attempt=0
  local sshSts=0
  while true; do
    echo 'Attempt "$(($attempt+1))" to execute command on master'
    set +e
    ssh -o "StrictHostKeyChecking no" -t ubuntu@${KUBEMARK_MASTER_IP} "$1"
    sshSts=$?
    set -e
    if [ $sshSts -ne 0 ]; then
      echo -e " ${color_yellow}[master command not successful yet]${color_norm}"
      attempt=$(($attempt+1))
      if (( attempt > 10 )); then
        echo
        echo -e "${color_red}master command execute failed" >&2
        exit 1
      fi
    else
      break
    fi
    sleep 5
  done
}

function copy-files() {
  array=( "$@" )
  dest=${@: -1}
  dest=`echo $dest | cut -d: -f2`
  unset "array[${#array[@]}-1]"
  ssh -o "StrictHostKeyChecking no" -t ubuntu@${KUBEMARK_MASTER_IP} "sudo mkdir -p /tmp/foo1234 && sudo chmod 777 /tmp/foo1234"
  for fl in "${array[@]}"; do
    scp -o "StrictHostKeyChecking no" -rp $fl ubuntu@${KUBEMARK_MASTER_IP}:/tmp/foo1234
    fn=`basename $fl`
    ssh -o "StrictHostKeyChecking no" -t ubuntu@${KUBEMARK_MASTER_IP} "sudo mv /tmp/foo1234/$fn $dest"
  done
  ssh -o "StrictHostKeyChecking no" -t ubuntu@${KUBEMARK_MASTER_IP} "sudo rm -rf /tmp/foo1234"
}

# Wait for the master to be reachable for executing commands on it. We do this by
# trying to run the bash noop(:) on the master, with 10 retries.
function wait-for-master-reachability {
  execute-cmd-on-master-with-retries ":" 10
  echo "Checked master reachability for remote command execution."
}

# Write all the relevant certs/keys/tokens to the master.
function write-pki-config-to-master {
  PKI_SETUP_CMD="sudo mkdir /home/kubernetes/k8s_auth_data -p && \
    sudo bash -c \"echo ${CA_CERT_BASE64} | base64 --decode > /home/kubernetes/k8s_auth_data/ca.crt\" && \
    sudo bash -c \"echo ${MASTER_CERT_BASE64} | base64 --decode > /home/kubernetes/k8s_auth_data/server.cert\" && \
    sudo bash -c \"echo ${MASTER_KEY_BASE64} | base64 --decode > /home/kubernetes/k8s_auth_data/server.key\" && \
    sudo bash -c \"echo ${ETCD_APISERVER_CA_KEY_BASE64} | base64 --decode > /home/kubernetes/k8s_auth_data/etcd-apiserver-ca.key\" && \
    sudo bash -c \"echo ${ETCD_APISERVER_CA_CERT_BASE64} | base64 --decode | gunzip > /home/kubernetes/k8s_auth_data/etcd-apiserver-ca.crt\" && \
    sudo bash -c \"echo ${ETCD_APISERVER_SERVER_KEY_BASE64} | base64 --decode > /home/kubernetes/k8s_auth_data/etcd-apiserver-server.key\" && \
    sudo bash -c \"echo ${ETCD_APISERVER_SERVER_CERT_BASE64} | base64 --decode | gunzip > /home/kubernetes/k8s_auth_data/etcd-apiserver-server.crt\" && \
    sudo bash -c \"echo ${ETCD_APISERVER_CLIENT_KEY_BASE64} | base64 --decode > /home/kubernetes/k8s_auth_data/etcd-apiserver-client.key\" && \
    sudo bash -c \"echo ${ETCD_APISERVER_CLIENT_CERT_BASE64} | base64 --decode | gunzip > /home/kubernetes/k8s_auth_data/etcd-apiserver-client.crt\" && \
    sudo bash -c \"echo ${REQUESTHEADER_CA_CERT_BASE64} | base64 --decode > /home/kubernetes/k8s_auth_data/aggr_ca.crt\" && \
    sudo bash -c \"echo ${PROXY_CLIENT_CERT_BASE64} | base64 --decode > /home/kubernetes/k8s_auth_data/proxy_client.crt\" && \
    sudo bash -c \"echo ${PROXY_CLIENT_KEY_BASE64} | base64 --decode > /home/kubernetes/k8s_auth_data/proxy_client.key\" && \
    sudo bash -c \"echo ${KUBECFG_CERT_BASE64} | base64 --decode > /home/kubernetes/k8s_auth_data/kubecfg.crt\" && \
    sudo bash -c \"echo ${KUBECFG_KEY_BASE64} | base64 --decode > /home/kubernetes/k8s_auth_data/kubecfg.key\" && \
    sudo bash -c \"echo \"${KUBE_BEARER_TOKEN},ubuntu,ubuntu\" > /home/kubernetes/k8s_auth_data/known_tokens.csv\" && \
    sudo bash -c \"echo \"${KUBELET_TOKEN},system:node:node-name,uid:kubelet,system:nodes\" >> /home/kubernetes/k8s_auth_data/known_tokens.csv\" && \
    sudo bash -c \"echo \"${KUBE_PROXY_TOKEN},system:kube-proxy,uid:kube_proxy\" >> /home/kubernetes/k8s_auth_data/known_tokens.csv\" && \
    sudo bash -c \"echo \"${HEAPSTER_TOKEN},system:heapster,uid:heapster\" >> /home/kubernetes/k8s_auth_data/known_tokens.csv\" && \
    sudo bash -c \"echo \"${CLUSTER_AUTOSCALER_TOKEN},system:cluster-autoscaler,uid:cluster-autoscaler\" >> /home/kubernetes/k8s_auth_data/known_tokens.csv\" && \
    sudo bash -c \"echo \"${NODE_PROBLEM_DETECTOR_TOKEN},system:node-problem-detector,uid:system:node-problem-detector\" >> /home/kubernetes/k8s_auth_data/known_tokens.csv\" && \
    sudo bash -c \"echo \"${KUBE_DNS_TOKEN},system:kube-dns,uid:kube-dns\" >> /home/kubernetes/k8s_auth_data/known_tokens.csv\" && \
    sudo bash -c \"echo ${KUBE_PASSWORD},ubuntu,ubuntu > /home/kubernetes/k8s_auth_data/basic_auth.csv\""
  execute-cmd-on-master-with-retries "${PKI_SETUP_CMD}" 3
  echo "Wrote PKI certs, keys, tokens and admin password to master."
}

# Copy all the necessary resource files (scripts/configs/manifests) to the master.
function copy-resource-files-to-master {
  copy-files \
    "${SERVER_BINARY_TAR}" \
    "${RESOURCE_DIRECTORY}/kubemark-master-env.sh" \
    "${RESOURCE_DIRECTORY}/start-kubemark-master.sh" \
    "${RESOURCE_DIRECTORY}/kubeconfig.kubemark" \
    "${KUBEMARK_DIRECTORY}/configure-kubectl.sh" \
    "${RESOURCE_DIRECTORY}/manifests/etcd.yaml" \
    "${RESOURCE_DIRECTORY}/manifests/etcd-events.yaml" \
    "${RESOURCE_DIRECTORY}/manifests/kube-apiserver.yaml" \
    "${RESOURCE_DIRECTORY}/manifests/kube-scheduler.yaml" \
    "${RESOURCE_DIRECTORY}/manifests/kube-controller-manager.yaml" \
    "${RESOURCE_DIRECTORY}/manifests/kube-addon-manager.yaml" \
    "${RESOURCE_DIRECTORY}/manifests/addons/kubemark-rbac-bindings" \
    "kubernetes@${MASTER_NAME}":/home/kubernetes/
  echo "Copied server binary, master startup scripts, configs and resource manifests to master."
}

# Make startup scripts executable and run start-kubemark-master.sh.
function start-master-components {
  echo ""
  MASTER_STARTUP_CMD="sudo bash -c 'export CLOUD_PROVIDER=${CLOUD_PROVIDER} && /home/kubernetes/start-kubemark-master.sh'"
  execute-cmd-on-master-with-retries "${MASTER_STARTUP_CMD}"
  echo "The master has started and is now live."
}

function create-kubemark-master {
  get-master-ip-and-create-config
  create-master-environment-file
  create-kubemark-master-with-resources
  wait-for-master-reachability
  write-pki-config-to-master
  copy-resource-files-to-master
  start-master-components
}

# Generate secret and configMap for the hollow-node pods to work, prepare
# manifests of the hollow-node and heapster replication controllers from
# templates, and finally create these resources through kubectl.
function create-kube-hollow-node-resources-aws {
  echo "Creating config for $NUM_NODES hollow nodes.."
  # Create kubeconfig for Kubelet.
  KUBELET_KUBECONFIG_CONTENTS="apiVersion: v1
kind: Config
users:
- name: kubelet
  user:
    client-certificate-data: ${KUBELET_CERT_BASE64}
    client-key-data: ${KUBELET_KEY_BASE64}
clusters:
- name: kubemark
  cluster:
    certificate-authority-data: ${CA_CERT_BASE64}
    server: https://${MASTER_INTERNAL_IP}
contexts:
- context:
    cluster: kubemark
    user: kubelet
  name: kubemark-context
current-context: kubemark-context"

  # Create kubeconfig for Kubeproxy.
  KUBEPROXY_KUBECONFIG_CONTENTS="apiVersion: v1
kind: Config
users:
- name: kube-proxy
  user:
    token: ${KUBE_PROXY_TOKEN}
clusters:
- name: kubemark
  cluster:
    insecure-skip-tls-verify: true
    server: https://${MASTER_INTERNAL_IP}
contexts:
- context:
    cluster: kubemark
    user: kube-proxy
  name: kubemark-context
current-context: kubemark-context"

  # Create kubeconfig for Heapster.
  HEAPSTER_KUBECONFIG_CONTENTS="apiVersion: v1
kind: Config
users:
- name: heapster
  user:
    token: ${HEAPSTER_TOKEN}
clusters:
- name: kubemark
  cluster:
    insecure-skip-tls-verify: true
    server: https://${MASTER_INTERNAL_IP}
contexts:
- context:
    cluster: kubemark
    user: heapster
  name: kubemark-context
current-context: kubemark-context"

  # Create kubeconfig for Cluster Autoscaler.
  CLUSTER_AUTOSCALER_KUBECONFIG_CONTENTS="apiVersion: v1
kind: Config
users:
- name: cluster-autoscaler
  user:
    token: ${CLUSTER_AUTOSCALER_TOKEN}
clusters:
- name: kubemark
  cluster:
    insecure-skip-tls-verify: true
    server: https://${MASTER_INTERNAL_IP}
contexts:
- context:
    cluster: kubemark
    user: cluster-autoscaler
  name: kubemark-context
current-context: kubemark-context"

  # Create kubeconfig for NodeProblemDetector.
  NPD_KUBECONFIG_CONTENTS="apiVersion: v1
kind: Config
users:
- name: node-problem-detector
  user:
    token: ${NODE_PROBLEM_DETECTOR_TOKEN}
clusters:
- name: kubemark
  cluster:
    insecure-skip-tls-verify: true
    server: https://${MASTER_INTERNAL_IP}
contexts:
- context:
    cluster: kubemark
    user: node-problem-detector
  name: kubemark-context
current-context: kubemark-context"

  # Create kubeconfig for Kube DNS.
  KUBE_DNS_KUBECONFIG_CONTENTS="apiVersion: v1
kind: Config
users:
- name: kube-dns
  user:
    token: ${KUBE_DNS_TOKEN}
clusters:
- name: kubemark
  cluster:
    insecure-skip-tls-verify: true
    server: https://${MASTER_INTERNAL_IP}
contexts:
- context:
    cluster: kubemark
    user: kube-dns
  name: kubemark-context
current-context: kubemark-context"

  # Create kubemark namespace.
  "${KUBECTL}" create -f "${RESOURCE_DIRECTORY}/kubemark-ns.json"

  # Create configmap for configuring hollow- kubelet, proxy and npd.
  "${KUBECTL}" create configmap "node-configmap" --namespace="kubemark" \
    --from-literal=content.type="${TEST_CLUSTER_API_CONTENT_TYPE}" \
    --from-file=kernel.monitor="${RESOURCE_DIRECTORY}/kernel-monitor.json"

  # Create secret for passing kubeconfigs to kubelet, kubeproxy and npd.
  "${KUBECTL}" create secret generic "kubeconfig" --type=Opaque --namespace="kubemark" \
    --from-literal=kubelet.kubeconfig="${KUBELET_KUBECONFIG_CONTENTS}" \
    --from-literal=kubeproxy.kubeconfig="${KUBEPROXY_KUBECONFIG_CONTENTS}" \
    --from-literal=heapster.kubeconfig="${HEAPSTER_KUBECONFIG_CONTENTS}" \
    --from-literal=cluster_autoscaler.kubeconfig="${CLUSTER_AUTOSCALER_KUBECONFIG_CONTENTS}" \
    --from-literal=npd.kubeconfig="${NPD_KUBECONFIG_CONTENTS}" \
    --from-literal=dns.kubeconfig="${KUBE_DNS_KUBECONFIG_CONTENTS}"

  # Create addon pods.
  # Heapster.
  mkdir -p "${RESOURCE_DIRECTORY}/addons"
  sed "s/{{MASTER_IP}}/${MASTER_INTERNAL_IP}/g" "${RESOURCE_DIRECTORY}/heapster_template.json" > "${RESOURCE_DIRECTORY}/addons/heapster.json"
  metrics_mem_per_node=4
  metrics_mem=$((200 + metrics_mem_per_node*NUM_NODES))
  sed -i'' -e "s/{{METRICS_MEM}}/${metrics_mem}/g" "${RESOURCE_DIRECTORY}/addons/heapster.json"
  metrics_cpu_per_node_numerator=${NUM_NODES}
  metrics_cpu_per_node_denominator=2
  metrics_cpu=$((80 + metrics_cpu_per_node_numerator / metrics_cpu_per_node_denominator))
  sed -i'' -e "s/{{METRICS_CPU}}/${metrics_cpu}/g" "${RESOURCE_DIRECTORY}/addons/heapster.json"
  eventer_mem_per_node=500
  eventer_mem=$((200 * 1024 + eventer_mem_per_node*NUM_NODES))
  sed -i'' -e "s/{{EVENTER_MEM}}/${eventer_mem}/g" "${RESOURCE_DIRECTORY}/addons/heapster.json"

  # Cluster Autoscaler.
  if [[ "${ENABLE_KUBEMARK_CLUSTER_AUTOSCALER:-}" == "true" ]]; then
    echo "Setting up Cluster Autoscaler"
    KUBEMARK_AUTOSCALER_MIG_NAME="${KUBEMARK_AUTOSCALER_MIG_NAME:-${NODE_INSTANCE_PREFIX}-group}"
    KUBEMARK_AUTOSCALER_MIN_NODES="${KUBEMARK_AUTOSCALER_MIN_NODES:-0}"
    KUBEMARK_AUTOSCALER_MAX_NODES="${KUBEMARK_AUTOSCALER_MAX_NODES:-10}"
    NUM_NODES=${KUBEMARK_AUTOSCALER_MAX_NODES}
    echo "Setting maximum cluster size to ${NUM_NODES}."
    KUBEMARK_MIG_CONFIG="autoscaling.k8s.io/nodegroup: ${KUBEMARK_AUTOSCALER_MIG_NAME}"
    sed "s/{{master_ip}}/${MASTER_INTERNAL_IP}/g" "${RESOURCE_DIRECTORY}/cluster-autoscaler_template.json" > "${RESOURCE_DIRECTORY}/addons/cluster-autoscaler.json"
    sed -i'' -e "s/{{kubemark_autoscaler_mig_name}}/${KUBEMARK_AUTOSCALER_MIG_NAME}/g" "${RESOURCE_DIRECTORY}/addons/cluster-autoscaler.json"
    sed -i'' -e "s/{{kubemark_autoscaler_min_nodes}}/${KUBEMARK_AUTOSCALER_MIN_NODES}/g" "${RESOURCE_DIRECTORY}/addons/cluster-autoscaler.json"
    sed -i'' -e "s/{{kubemark_autoscaler_max_nodes}}/${KUBEMARK_AUTOSCALER_MAX_NODES}/g" "${RESOURCE_DIRECTORY}/addons/cluster-autoscaler.json"
  fi

  # Kube DNS.
  if [[ "${ENABLE_KUBEMARK_KUBE_DNS:-}" == "true" ]]; then
    echo "Setting up kube-dns"
    sed "s/{{dns_domain}}/${KUBE_DNS_DOMAIN}/g" "${RESOURCE_DIRECTORY}/kube_dns_template.yaml" > "${RESOURCE_DIRECTORY}/addons/kube_dns.yaml"
  fi

  "${KUBECTL}" create -f "${RESOURCE_DIRECTORY}/addons" --namespace="kubemark"

  # Create the replication controller for hollow-nodes.
  # We allow to override the NUM_REPLICAS when running Cluster Autoscaler.
  NUM_REPLICAS=${NUM_REPLICAS:-${NUM_NODES}}
  sed "s/{{numreplicas}}/${NUM_REPLICAS}/g" "${RESOURCE_DIRECTORY}/hollow-node_template.yaml" > "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  proxy_cpu=20
  if [ "${NUM_NODES}" -gt 1000 ]; then
    proxy_cpu=50
  fi
  proxy_mem_per_node=50
  proxy_mem=$((100 * 1024 + proxy_mem_per_node*NUM_NODES))
  sed -i'' -e "s/{{HOLLOW_PROXY_CPU}}/${proxy_cpu}/g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s/{{HOLLOW_PROXY_MEM}}/${proxy_mem}/g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s'{{kubemark_image_registry}}'${KUBEMARK_IMAGE_REGISTRY}'g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s/{{kubemark_image_tag}}/${KUBEMARK_IMAGE_TAG}/g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s/{{master_ip}}/${MASTER_INTERNAL_IP}/g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s/{{hollow_kubelet_params}}/${HOLLOW_KUBELET_TEST_ARGS}/g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s/{{hollow_proxy_params}}/${HOLLOW_PROXY_TEST_ARGS}/g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s'{{kubemark_mig_config}}'${KUBEMARK_MIG_CONFIG:-}'g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  "${KUBECTL}" create -f "${RESOURCE_DIRECTORY}/hollow-node.yaml" --namespace="kubemark"

  echo "Created secrets, configMaps, replication-controllers required for hollow-nodes."
}

function delete-kubemark-master {
  ZONE=$KUBE_AWS_ZONE
  route_table_ids=$($AWS_CMD describe-route-tables \
                             --filters Name=vpc-id,Values=$VPC_ID \
                                       Name=route.destination-cidr-block,Values=$MASTER_IP_RANGE \
                             --query RouteTables[].RouteTableId \
                    | tr "\t" "\n")
  for route_table_id in ${route_table_ids}; do
    echo "Deleting route for $MASTER_IP_RANGE from table $route_table_id"
    $AWS_CMD delete-route --route-table-id $route_table_id --destination-cidr-block $MASTER_IP_RANGE > $LOG
  done

  instance_ids=$($AWS_CMD describe-instances \
                          --filters Name=vpc-id,Values=$VPC_ID \
                                    Name=tag:KubernetesCluster,Values=${CLUSTER_ID} \
                                    Name=tag:Name,Values=${MASTER_NAME} \
                          --query Reservations[].Instances[].InstanceId)

  echo "Terminating cluster instances in VPC: $VPC_ID, Instances: $instance_ids"
  if [[ -n "${instance_ids}" ]]; then
    $AWS_CMD terminate-instances --instance-ids ${instance_ids} > $LOG
    echo "Waiting for instances to be deleted"
    for instance_id in ${instance_ids}; do
      wait-for-instance-state ${instance_id} "terminated"
    done
    echo "All instances deleted"
  fi

  find-master-pd
  find-tagged-master-ip

  if [[ -n "${KUBE_MASTER_IP:-}" ]]; then
    echo "Releasing Kubemark Master EIP $KUBE_MASTER_IP"
    release-elastic-ip ${KUBE_MASTER_IP}
  fi

  if [[ -n "${MASTER_DISK_ID:-}" ]]; then
    echo "Deleting kubemark Master volume ${MASTER_DISK_ID}"
    $AWS_CMD delete-volume --volume-id ${MASTER_DISK_ID} > $LOG
  fi
}
