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
KUBEMARK_INSTALL="/var/cache/kubernetes-install/kubemark"

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

# Wait for the master to be reachable for executing commands on it. We do this by
# trying to run the bash noop(:) on the master, with 10 retries.
function wait-for-master-reachability {
  execute-cmd-on-master-with-retries ":" 10
  echo "Checked master reachability for remote command execution."
}

function create-kubemark-master {
  # We intentionally override env vars in subshell to preserve original values.
  # shellcheck disable=SC2030,SC2031
  (
    kube::util::ensure-temp-dir
    export KUBE_TEMP="${KUBE_TEMP}"

    export KUBECONFIG="${RESOURCE_DIRECTORY}/kubeconfig.kubemark"
    export KUBE_AWS_INSTANCE_PREFIX="${KUBE_AWS_INSTANCE_PREFIX:-e2e-test-${USER}}-kubemark"
    export CLUSTER_NAME="${KUBE_AWS_INSTANCE_PREFIX}"
    export KUBE_CREATE_NODES=false
    export LOCAL_KUBECONFIG="${RESOURCE_DIRECTORY}/kubeconfig.kubemark"
    export KUBE_API_BIND_PORT=443
    export NETWORK_PROVIDER="bridge"

    # Disable all addons. They are running outside of the kubemark cluster.
    export KUBE_ENABLE_CLUSTER_AUTOSCALER=false
    export KUBE_ENABLE_CLUSTER_DNS=false
    export KUBE_ENABLE_NODE_LOGGING=false
    export KUBE_ENABLE_METRICS_SERVER=false
    export KUBE_ENABLE_CLUSTER_MONITORING="none"
    export KUBE_ENABLE_L7_LOADBALANCING="none"

    unset MASTER_ROOT_DISK_SIZE

    MASTER_IP_SUFFIX=.10
    export MASTER_INTERNAL_IP="${SUBNET_CIDR%.*}${MASTER_IP_SUFFIX}"

    # Set kubemark-specific overrides:
    # for each defined env KUBEMARK_X=Y call export X=Y.
    for var in ${!KUBEMARK_*}; do
      dst_var=${var#KUBEMARK_}
      val=${!var}
      echo "Setting ${dst_var} to '${val}'"
      export "${dst_var}"="${val}"
    done
    "${KUBE_ROOT}/hack/e2e-internal/e2e-up.sh"
  )

  KUBE_AWS_INSTANCE_PREFIX="${KUBE_AWS_INSTANCE_PREFIX:-e2e-test-${USER}}-kubemark"
  CLUSTER_ID="${KUBE_AWS_INSTANCE_PREFIX}"
  MASTER_IP_SUFFIX=.10
  MASTER_INTERNAL_IP="${SUBNET_CIDR%.*}${MASTER_IP_SUFFIX}"
  find-master-pd
  find-tagged-master-ip
  MASTER_IP=$KUBE_MASTER_IP
  MASTER_PUBLIC_IP=$KUBE_MASTER_IP
  KUBEMARK_MASTER_IP=$KUBE_MASTER_IP
  KUBE_MASTER_IP=$MASTER_INTERNAL_IP
  create-kubemark-resources
}

function delete-kubemark-master {
  # We intentionally override env vars in subshell to preserve original values.
  # shellcheck disable=SC2030,SC2031
  (
    ## reset CLUSTER_NAME to avoid multi kubemark added after e2e.
    export KUBE_AWS_INSTANCE_PREFIX="${KUBE_AWS_INSTANCE_PREFIX:-e2e-test-${USER}}-kubemark"
    export CLUSTER_NAME="${KUBE_AWS_INSTANCE_PREFIX}"
    export KUBE_DELETE_NETWORK=false

    # Disable all addons. They are running outside of the kubemark cluster.
    export KUBE_ENABLE_CLUSTER_AUTOSCALER=false
    export KUBE_ENABLE_CLUSTER_DNS=false
    export KUBE_ENABLE_NODE_LOGGING=false
    export KUBE_ENABLE_METRICS_SERVER=false
    export KUBE_ENABLE_CLUSTER_MONITORING="none"
    export KUBE_ENABLE_L7_LOADBALANCING="none"

    "${KUBE_ROOT}/hack/e2e-internal/e2e-down.sh"
  )
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

function create-master-env-file {
  cat > "${RESOURCE_DIRECTORY}/kubemark-master-env.sh" <<EOF
# Generic variables.
INSTANCE_PREFIX="${INSTANCE_PREFIX:-}"
SERVICE_CLUSTER_IP_RANGE="${SERVICE_CLUSTER_IP_RANGE:-}"
EVENT_PD="${EVENT_PD:-}"

# Etcd related variables.
ETCD_IMAGE="${ETCD_IMAGE:-3.3.10-1}"
ETCD_VERSION="${ETCD_VERSION:-}"
EOF
  echo "Created environment file for kubemark master."
}

# Copy all the necessary resource files (scripts/configs/manifests) to the master.
function copy-resource-files-to-master {
  copy-files \
    "${RESOURCE_DIRECTORY}/kubemark-master-env.sh" \
    "${RESOURCE_DIRECTORY}/start-kubemark-master-aws.sh" \
    "${KUBEMARK_DIRECTORY}/configure-kubectl.sh" \
    "${RESOURCE_DIRECTORY}/manifests/etcd-events.yaml" \
    "${RESOURCE_DIRECTORY}/manifests/kube-addon-manager.yaml" \
    "kubernetes@${MASTER_NAME}":${KUBEMARK_INSTALL}
  copy-files \
    "${RESOURCE_DIRECTORY}/manifests/addons/kubemark-rbac-bindings/cluster-autoscaler-binding.yaml" \
    "${RESOURCE_DIRECTORY}/manifests/addons/kubemark-rbac-bindings/heapster-binding.yaml" \
    "${RESOURCE_DIRECTORY}/manifests/addons/kubemark-rbac-bindings/kubecfg-binding.yaml" \
    "${RESOURCE_DIRECTORY}/manifests/addons/kubemark-rbac-bindings/npd-binding.yaml" \
    "kubernetes@${MASTER_NAME}":${KUBEMARK_INSTALL}/kubemark-rbac-bindings/
  echo "Copied resource manifests to master."
}

function create-kubemark-resources {
  create-master-env-file
  CREATE_DIR_CMD="sudo mkdir -p ${KUBEMARK_INSTALL}/kubemark-rbac-bindings"
  execute-cmd-on-master-with-retries "${CREATE_DIR_CMD}" 3
  copy-resource-files-to-master
  MASTER_STARTUP_CMD="sudo bash -c 'export CLOUD_PROVIDER=${CLOUD_PROVIDER} && ${KUBEMARK_INSTALL}/start-kubemark-master-aws.sh'"
  execute-cmd-on-master-with-retries "${MASTER_STARTUP_CMD}"
  echo "The master has started and is now live."

}
