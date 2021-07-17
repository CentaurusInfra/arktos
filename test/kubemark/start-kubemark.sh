#!/usr/bin/env bash

# Copyright 2015 The Kubernetes Authors.
# Copyright 2020 Authors of Arktos - file modified.
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

# Script that creates a Kubemark cluster for any given cloud provider.

set -o errexit
set -o nounset
set -o pipefail

export USE_INSECURE_SCALEOUT_CLUSTER_MODE="${USE_INSECURE_SCALEOUT_CLUSTER_MODE:-false}"

### UPLOAD_TAR_DONE serves as global flag whether image upload has already been done
#   empty string is not done yet; non-empty is for done.
export UPLOAD_TAR_DONE="${UPLOAD_TAR_DONE-}"

TMP_ROOT="$(dirname "${BASH_SOURCE[@]}")/../.."
KUBE_ROOT=$(readlink -e "${TMP_ROOT}" 2> /dev/null || perl -MCwd -e 'print Cwd::abs_path shift' "${TMP_ROOT}")

source "${KUBE_ROOT}/test/kubemark/skeleton/util.sh"
source "${KUBE_ROOT}/test/kubemark/cloud-provider-config.sh"
source "${KUBE_ROOT}/test/kubemark/${CLOUD_PROVIDER}/util.sh"
source "${KUBE_ROOT}/cluster/kubemark/${CLOUD_PROVIDER}/config-default.sh"

if [[ -f "${KUBE_ROOT}/test/kubemark/${CLOUD_PROVIDER}/startup.sh" ]] ; then
  source "${KUBE_ROOT}/test/kubemark/${CLOUD_PROVIDER}/startup.sh"
fi

source "${KUBE_ROOT}/cluster/kubemark/util.sh"

KUBECTL="${KUBE_ROOT}/cluster/kubectl.sh"
KUBEMARK_DIRECTORY="${KUBE_ROOT}/test/kubemark"
export RESOURCE_DIRECTORY="${KUBEMARK_DIRECTORY}/resources"
export SHARED_CA_DIRECTORY=${SHARED_CA_DIRECTORY:-"/tmp/shared_ca"}
export SCALEOUT_TP_COUNT="${SCALEOUT_TP_COUNT:-1}"
export SCALEOUT_RP_COUNT="${SCALEOUT_RP_COUNT:-1}"
export HAPROXY_TLS_MODE=${HAPROXY_TLS_MODE:-"bridging"}

### the list of kubeconfig files to TP masters
export TENANT_SERVER_KUBECONFIGS=""

### kubeconfig files
# SCALEOUT_CLUSTER_KUBECONFIG is used to convey the kubeconfig name to the subprocess to create the kubeconfig file with desired names
# this is used in scale up env
KUBEMARK_CLUSTER_KUBECONFIG="${RESOURCE_DIRECTORY}/kubeconfig.kubemark"

### following kubeconfigs are used in scale-out env
##
# TP_KUBECONFIG are saved with the below file names with TP number as suffix
# ${RESOURCE_DIRECTORY}/kubeconfig.kubemark.tp-${tp_num}"

# RP_KUBECONFIG are sav with the below file names with RP number as suffix
# ${RESOURCE_DIRECTORY}/kubeconfig.kubemark.rp-${rp_num}"

if [[ "${SCALEOUT_CLUSTER:-false}" == "true" ]]; then
  RP_KUBECONFIG="${RESOURCE_DIRECTORY}/kubeconfig.kubemark.rp"
  TP_KUBECONFIG="${RESOURCE_DIRECTORY}/kubeconfig.kubemark.tp"
else
  RP_KUBECONFIG="${KUBEMARK_CLUSTER_KUBECONFIG}"
  TP_KUBECONFIG="${KUBEMARK_CLUSTER_KUBECONFIG}"
fi

### files to explicitly save the kubeconfig to different cluster or proxy
### those are used for targeted operations such as tenant creation etc on
### desired cluster directly
export SCALEOUT_CLUSTER=${SCALEOUT_CLUSTER:-false}
export KUBE_APISERVER_EXTRA_ARGS=${KUBE_APISERVER_EXTRA_ARGS:-}
export KUBE_CONTROLLER_EXTRA_ARGS=${KUBE_CONTROLLER_EXTRA_ARGS:-}
export KUBE_SCHEDULER_EXTRA_ARGS=${KUBE_SCHEDULER_EXTRA_ARGS:-}

# Generate a random 6-digit alphanumeric tag for the kubemark image.
# Used to uniquify image builds across different invocations of this script.
KUBEMARK_IMAGE_TAG=$(head /dev/urandom | tr -dc 'a-z0-9' | fold -w 6 | head -n 1)

# Create a docker image for hollow-node and upload it to the appropriate docker registry.
function create-and-upload-hollow-node-image {
  authenticate-docker
  KUBEMARK_IMAGE_REGISTRY="${KUBEMARK_IMAGE_REGISTRY:-${CONTAINER_REGISTRY}/${PROJECT}}"
  if [[ "${KUBEMARK_BAZEL_BUILD:-}" =~ ^[yY]$ ]]; then
    # Build+push the image through bazel.
    touch WORKSPACE # Needed for bazel.
    build_cmd=("bazel" "run" "//cluster/images/kubemark:push" "--define" "REGISTRY=${KUBEMARK_IMAGE_REGISTRY}" "--define" "IMAGE_TAG=${KUBEMARK_IMAGE_TAG}")
    run-cmd-with-retries "${build_cmd[@]}"
  else
    # Build+push the image through makefile.
    build_cmd=("make" "${KUBEMARK_IMAGE_MAKE_TARGET}")
    MAKE_DIR="${KUBE_ROOT}/cluster/images/kubemark"
    KUBEMARK_BIN="$(kube::util::find-binary-for-platform kubemark linux/amd64)"
    if [[ -z "${KUBEMARK_BIN}" ]]; then
      echo 'Cannot find cmd/kubemark binary'
      exit 1
    fi
    echo "Copying kubemark binary to ${MAKE_DIR}"
    cp "${KUBEMARK_BIN}" "${MAKE_DIR}"
    CURR_DIR=$(pwd)
    cd "${MAKE_DIR}"
    REGISTRY=${KUBEMARK_IMAGE_REGISTRY} IMAGE_TAG=${KUBEMARK_IMAGE_TAG} run-cmd-with-retries "${build_cmd[@]}"
    rm kubemark
    cd "$CURR_DIR"
  fi
  echo "Created and uploaded the kubemark hollow-node image to docker registry."
  # Cleanup the kubemark image after the script exits.
  if [[ "${CLEANUP_KUBEMARK_IMAGE:-}" == "true" ]]; then
    trap delete-kubemark-image EXIT
  fi
}

function delete-kubemark-image {
  delete-image "${KUBEMARK_IMAGE_REGISTRY}/kubemark:${KUBEMARK_IMAGE_TAG}"
}

RESOURCE_SERVER_KUBECONFIG=""
RP_NUM=""
# Generate secret and configMap for the hollow-node pods to work, prepare
# manifests of the hollow-node and heapster replication controllers from
# templates, and finally create these resources through kubectl.
function create-kube-hollow-node-resources {
  # Create kubemark namespace.
  if [[ "${SCALEOUT_CLUSTER:-false}" == "true" ]]; then
    echo "DBG: RP number: ${RP_NUM}"
    local -r current_rp_kubeconfig=${RP_KUBECONFIG}-${RP_NUM}
    echo "DBG: PR kubeconfig: ${current_rp_kubeconfig}"

    if [[ ${RP_NUM} == 1 ]]; then
    "${KUBECTL}" create -f "${RESOURCE_DIRECTORY}/kubemark-ns.json"
    fi
  else
    local -r current_rp_kubeconfig=${KUBEMARK_CLUSTER_KUBECONFIG}
    "${KUBECTL}" create -f "${RESOURCE_DIRECTORY}/kubemark-ns.json"
  fi

  # Create secret for passing kubeconfigs to kubelet, kubeproxy and npd.
  # It's bad that all component shares the same kubeconfig.
  # TODO(https://github.com/kubernetes/kubernetes/issues/79883): Migrate all components to separate credentials.
  if [[ "${SCALEOUT_CLUSTER:-false}" == "false" ]]; then
    # Create configmap for configuring hollow- kubelet, proxy and npd.
    "${KUBECTL}" create configmap "node-configmap" --namespace="kubemark" \
      --from-literal=content.type="${TEST_CLUSTER_API_CONTENT_TYPE}" \
      --from-file=kernel.monitor="${RESOURCE_DIRECTORY}/kernel-monitor.json" \
      --from-literal=resource.server.kubeconfig="${KUBEMARK_CLUSTER_KUBECONFIG:-}" \
      --from-literal=tenant.server.kubeconfigs="${KUBEMARK_CLUSTER_KUBECONFIG:-}"

    "${KUBECTL}" create secret generic "kubeconfig" --type=Opaque --namespace="kubemark" \
      --from-file=kubelet.kubeconfig="${KUBEMARK_CLUSTER_KUBECONFIG}" \
      --from-file=kubeproxy.kubeconfig="${KUBEMARK_CLUSTER_KUBECONFIG}" \
      --from-file=npd.kubeconfig="${KUBEMARK_CLUSTER_KUBECONFIG}" \
      --from-file=heapster.kubeconfig="${KUBEMARK_CLUSTER_KUBECONFIG}" \
      --from-file=cluster_autoscaler.kubeconfig="${KUBEMARK_CLUSTER_KUBECONFIG}" \
      --from-file=dns.kubeconfig="${KUBEMARK_CLUSTER_KUBECONFIG}"
  else
    # Create configmap for configuring hollow- kubelet, proxy and npd.
    "${KUBECTL}" create configmap "node-configmap-${RP_NUM}" --namespace="kubemark" \
      --from-literal=content.type="${TEST_CLUSTER_API_CONTENT_TYPE}" \
      --from-file=kernel.monitor="${RESOURCE_DIRECTORY}/kernel-monitor.json" \
      --from-literal=resource.server.kubeconfig="${RESOURCE_SERVER_KUBECONFIG:-}" \
      --from-literal=tenant.server.kubeconfigs="${TENANT_SERVER_KUBECONFIGS:-}"

    # TODO: DNS to proxy to get the service info
    # TODO: kubeproxy to proxy to get the serivce info
    #
    echo "DBG setting up secrets for hollow nodes"
    create_secret_args="--from-file=kubelet.kubeconfig="${current_rp_kubeconfig}
    create_secret_args=${create_secret_args}"  --from-file=kubeproxy.kubeconfig="${current_rp_kubeconfig}
    create_secret_args=${create_secret_args}"  --from-file=npd.kubeconfig="${current_rp_kubeconfig}
    create_secret_args=${create_secret_args}"  --from-file=heapster.kubeconfig="${current_rp_kubeconfig}
    create_secret_args=${create_secret_args}" --from-file=cluster_autoscaler.kubeconfig="${current_rp_kubeconfig}
    create_secret_args=${create_secret_args}" --from-file=dns.kubeconfig="${current_rp_kubeconfig}

    for (( tp_num=1; tp_num<=${SCALEOUT_TP_COUNT}; tp_num++ ))
    do
      create_secret_args=${create_secret_args}" --from-file=tp${tp_num}.kubeconfig="${TP_KUBECONFIG}-${tp_num}
    done

    create_secret_args=${create_secret_args}" --from-file=rp.kubeconfig-${RP_NUM}="${current_rp_kubeconfig}


    "${KUBECTL}" create secret generic "kubeconfig-${RP_NUM}" --type=Opaque --namespace="kubemark" ${create_secret_args}
  fi

  ## host level addons, set up for scaleup or first RP
  ## note that the objects are created on the admin cluster
  #
  if [[ ${RP_NUM} == 1 ]] || [[ "${SCALEOUT_CLUSTER:-false}" == "false" ]]; then
    # Create addon pods.
    ## TODO: update addons if they need to run at hollow-node level. currently treat them as host level
    # Heapster.
    mkdir -p "${RESOURCE_DIRECTORY}/addons"
    MASTER_IP=$(grep server "${KUBEMARK_CLUSTER_KUBECONFIG}" | awk -F "/" '{print $3}')
    sed "s@{{MASTER_IP}}@${MASTER_IP}@g" "${RESOURCE_DIRECTORY}/heapster_template.json" > "${RESOURCE_DIRECTORY}/addons/heapster.json"
    metrics_mem_per_node=4
    metrics_mem=$((200 + metrics_mem_per_node*NUM_NODES))
    sed -i'' -e "s@{{METRICS_MEM}}@${metrics_mem}@g" "${RESOURCE_DIRECTORY}/addons/heapster.json"
    metrics_cpu_per_node_numerator=${NUM_NODES}
    metrics_cpu_per_node_denominator=2
    metrics_cpu=$((80 + metrics_cpu_per_node_numerator / metrics_cpu_per_node_denominator))
    sed -i'' -e "s@{{METRICS_CPU}}@${metrics_cpu}@g" "${RESOURCE_DIRECTORY}/addons/heapster.json"
    eventer_mem_per_node=500
    eventer_mem=$((200 * 1024 + eventer_mem_per_node*NUM_NODES))
    sed -i'' -e "s@{{EVENTER_MEM}}@${eventer_mem}@g" "${RESOURCE_DIRECTORY}/addons/heapster.json"

    # Cluster Autoscaler.
    if [[ "${ENABLE_KUBEMARK_CLUSTER_AUTOSCALER:-}" == "true" ]]; then
      echo "Setting up Cluster Autoscaler"
      KUBEMARK_AUTOSCALER_MIG_NAME="${KUBEMARK_AUTOSCALER_MIG_NAME:-${NODE_INSTANCE_PREFIX}-group}"
      KUBEMARK_AUTOSCALER_MIN_NODES="${KUBEMARK_AUTOSCALER_MIN_NODES:-0}"
      KUBEMARK_AUTOSCALER_MAX_NODES="${KUBEMARK_AUTOSCALER_MAX_NODES:-10}"
      NUM_NODES=${KUBEMARK_AUTOSCALER_MAX_NODES}
      echo "Setting maximum cluster size to ${NUM_NODES}."
      KUBEMARK_MIG_CONFIG="autoscaling.k8s.io/nodegroup: ${KUBEMARK_AUTOSCALER_MIG_NAME}"
      sed "s/{{master_ip}}/${MASTER_IP}/g" "${RESOURCE_DIRECTORY}/cluster-autoscaler_template.json" > "${RESOURCE_DIRECTORY}/addons/cluster-autoscaler.json"
      sed -i'' -e "s@{{kubemark_autoscaler_mig_name}}@${KUBEMARK_AUTOSCALER_MIG_NAME}@g" "${RESOURCE_DIRECTORY}/addons/cluster-autoscaler.json"
      sed -i'' -e "s@{{kubemark_autoscaler_min_nodes}}@${KUBEMARK_AUTOSCALER_MIN_NODES}@g" "${RESOURCE_DIRECTORY}/addons/cluster-autoscaler.json"
      sed -i'' -e "s@{{kubemark_autoscaler_max_nodes}}@${KUBEMARK_AUTOSCALER_MAX_NODES}@g" "${RESOURCE_DIRECTORY}/addons/cluster-autoscaler.json"
    fi

    # Kube DNS.
    if [[ "${ENABLE_KUBEMARK_KUBE_DNS:-}" == "true" ]]; then
      echo "Setting up kube-dns"
      sed "s@{{dns_domain}}@${KUBE_DNS_DOMAIN}@g" "${RESOURCE_DIRECTORY}/kube_dns_template.yaml" > "${RESOURCE_DIRECTORY}/addons/kube_dns.yaml"
    fi

    "${KUBECTL}" create -f "${RESOURCE_DIRECTORY}/addons" --namespace="kubemark"
  fi

  ## the replication controller is per RP cluster
  ##
  # Create the replication controller for hollow-nodes.
  # We allow to override the NUM_REPLICAS when running Cluster Autoscaler.
  NUM_REPLICAS=${NUM_REPLICAS:-${NUM_NODES}}
  if [[ "${SCALEOUT_CLUSTER:-false}" == "true" ]]; then
    sed "s@{{numreplicas}}@${NUM_REPLICAS}@g" "${RESOURCE_DIRECTORY}/hollow-node_template_scaleout.yaml" > "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  else
    sed "s@{{numreplicas}}@${NUM_REPLICAS}@g" "${RESOURCE_DIRECTORY}/hollow-node_template.yaml" > "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  fi

  proxy_cpu=20
  if [ "${NUM_NODES}" -gt 1000 ]; then
    proxy_cpu=50
  fi
  proxy_mem_per_node=50
  proxy_mem=$((100 * 1024 + proxy_mem_per_node*NUM_NODES))
  if [[ "${SCALEOUT_CLUSTER:-false}" == "true" ]]; then
    sed -i'' -e "s@{{rp_num}}@${RP_NUM}@g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  fi

  sed -i'' -e "s@{{HOLLOW_PROXY_CPU}}@${proxy_cpu}@g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s@{{HOLLOW_PROXY_MEM}}@${proxy_mem}@g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s@{{kubemark_image_registry}}@${KUBEMARK_IMAGE_REGISTRY}@g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s@{{kubemark_image_tag}}@${KUBEMARK_IMAGE_TAG}@g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s@{{master_ip}}@${RESOURCE_SERVER_KUBECONFIG:-}@g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s@{{hollow_kubelet_params}}@${HOLLOW_KUBELET_TEST_ARGS}@g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s@{{hollow_proxy_params}}@${HOLLOW_PROXY_TEST_ARGS}@g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s@{{kubemark_mig_config}}@${KUBEMARK_MIG_CONFIG:-}@g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s@{{tenant_server_kubeconfigs}}@${TENANT_SERVER_KUBECONFIGS:-}@g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  sed -i'' -e "s@{{resource_server_kubeconfig}}@${RESOURCE_SERVER_KUBECONFIG:-}@g" "${RESOURCE_DIRECTORY}/hollow-node.yaml"
  "${KUBECTL}" create -f "${RESOURCE_DIRECTORY}/hollow-node.yaml" --namespace="kubemark"

  echo "Created secrets, configMaps, replication-controllers required for hollow-nodes."
}

# Wait until all hollow-nodes are running or there is a timeout.
function wait-for-hollow-nodes-to-run-or-timeout {
  if [[ "${SCALEOUT_CLUSTER:-false}" == "true" ]]; then
    echo "DBG: RP number: ${RP_NUM}"
    local -r current_rp_kubeconfig=${RP_KUBECONFIG}-${RP_NUM}
    echo "DBG: PR kubeconfig: ${current_rp_kubeconfig}"
  else
    local -r current_rp_kubeconfig=${KUBEMARK_CLUSTER_KUBECONFIG}
  fi

  timeout_seconds=$1
  echo -n "Waiting for all hollow-nodes to become Running"
  start=$(date +%s)
  nodes=$("${KUBECTL}" --kubeconfig="${current_rp_kubeconfig}" get node 2> /dev/null) || true
  ready=$(($(echo "${nodes}" | grep -vc "NotReady") - 1))

  until [[ "${ready}" -ge "${NUM_REPLICAS}" ]]; do
    echo -n "."
    sleep 1
    now=$(date +%s)
    if [ $((now - start)) -gt ${timeout_seconds:-1800} ]; then
      echo ""
      # shellcheck disable=SC2154 # Color defined in sourced script
      echo -e "${color_red} Timeout waiting for all hollow-nodes to become Running. ${color_norm}"
      # Try listing nodes again - if it fails it means that API server is not responding
      if "${KUBECTL}" --kubeconfig="${current_rp_kubeconfig}" get node &> /dev/null; then
        echo "Found only ${ready} ready hollow-nodes while waiting for ${NUM_NODES}."
      else
        echo "Got error while trying to list hollow-nodes. Probably API server is down."
      fi
      if [[ "${SCALEOUT_CLUSTER:-false}" == "true" ]]; then
        pods=$("${KUBECTL}" get pods -l name=hollow-node-${RP_NUM} --namespace=kubemark) || true
      else
        pods=$("${KUBECTL}" get pods -l name=hollow-node --namespace=kubemark) || true
      fi

      running=$(($(echo "${pods}" | grep -c "Running")))
      echo "${running} hollow-nodes are reported as 'Running'"
      not_running=$(($(echo "${pods}" | grep -vc "Running") - 1))
      echo "${not_running} hollow-nodes are reported as NOT 'Running'"
      echo "${pods}" | grep -v Running
      exit 1
    fi
    nodes=$("${KUBECTL}" --kubeconfig="${current_rp_kubeconfig}" get node 2> /dev/null) || true
    ready=$(($(echo "${nodes}" | grep -vc "NotReady") - 1))
  done
  # shellcheck disable=SC2154 # Color defined in sourced script
  echo -e "${color_green} Done!${color_norm}"
}

############################### Main Function ########################################

# Setup for hollow-nodes.
function start-hollow-nodes {
  # shellcheck disable=SC2154 # Color defined in sourced script
  echo -e "${color_yellow}STARTING SETUP FOR HOLLOW-NODES${color_norm}"
  create-and-upload-hollow-node-image
  if [ "${CLOUD_PROVIDER}" = "aws" ]; then
    (
      MASTER_IP=${MASTER_INTERNAL_IP}
      create-kube-hollow-node-resources
    )
  else
    create-kube-hollow-node-resources
  fi

  # the timeout value is set based on default QPS of 20/sec and a buffer time of 15 minutes
  let timeout_seconds=${KUBEMARK_NUM_NODES:-10}/20+900
  echo -e "$(date): Wait ${timeout_seconds} seconds for ${KUBEMARK_NUM_NODES:-10} hollow nodes to be ready."

  wait-for-hollow-nodes-to-run-or-timeout ${timeout_seconds}
}

function generate-shared-ca-cert {
  echo "Create the shared CA for kubemark test"
  rm -f -r "${SHARED_CA_DIRECTORY}"
  mkdir -p "${SHARED_CA_DIRECTORY}"

  local -r cert_create_debug_output=$(mktemp "/tmp/cert_create_debug_output.XXXXX")
  (set -x
    cd "${SHARED_CA_DIRECTORY}"
    curl -L -O --connect-timeout 30 --retry 10 --retry-delay 2 https://storage.googleapis.com/kubernetes-release/easy-rsa/easy-rsa.tar.gz
    tar xzf easy-rsa.tar.gz
    cd easy-rsa-master/easyrsa3
    ./easyrsa init-pki
    ./easyrsa --batch "--req-cn=kubemarktestca" build-ca nopass ) &>${cert_create_debug_output} || {
    cat "${cert_create_debug_output}" >&2
    echo "=== Failed to generate shared CA certificates: Aborting ===" >&2
    exit 2
  }

  cp -f ${SHARED_CA_DIRECTORY}/easy-rsa-master/easyrsa3/pki/ca.crt ${SHARED_CA_DIRECTORY}/ca.crt
  cp -f ${SHARED_CA_DIRECTORY}/easy-rsa-master/easyrsa3/pki/private/ca.key ${SHARED_CA_DIRECTORY}/ca.key
}

# master machine name format: {KUBE_GCE_ZONE}-kubemark-tp-1-master
# destination file " --resource-providers=/etc/srv/kubernetes/kube-scheduler/rp-kubeconfig"
# TODO: avoid calling GCE compute from here
# TODO: currently the same kubeconfig is used by both scheduler and kube-controller-managers on RP clusters
#       Pending design on how RP kubeconfigs to be used on the TP cluster:
#       if the current approach continue to be used,  modify the kubeconfigs so the scheduler and controllers
#       can have different identity for refined RBAC and logging purposes
#       if future design is to let scheduler and controller managers to point to a generic server to get those RP
#       kubeconfigs, the generic service should generate different ones for them.
function restart_tp_scheduler_and_controller {
  for (( tp_num=1; tp_num<=${SCALEOUT_TP_COUNT}; tp_num++ ))
  do
    tp_vm="${RUN_PREFIX}-kubemark-tp-${tp_num}-master"
    echo "DBG: copy rp kubeconfigs for scheduler and controller manager to TP master: ${tp_vm}"
    for (( rp_num=1; rp_num<=${SCALEOUT_RP_COUNT}; rp_num++ ))
    do
      rp_kubeconfig="${RP_KUBECONFIG}-${rp_num}"
      gcloud compute scp --zone="${KUBE_GCE_ZONE}" "${rp_kubeconfig}" "${tp_vm}:/tmp/rp-kubeconfig-${rp_num}"
    done

    echo "DBG: copy rp kubeconfigs to destinations on TP master: ${tp_vm}"
    cmd="sudo cp /tmp/rp-kubeconfig-* /etc/srv/kubernetes/kube-scheduler/ && sudo cp /tmp/rp-kubeconfig-* /etc/srv/kubernetes/kube-controller-manager/"
    gcloud compute ssh --ssh-flag="-o LogLevel=quiet" --ssh-flag="-o ConnectTimeout=30" --project "${PROJECT}" --zone="${KUBE_GCE_ZONE}" "${tp_vm}" --command "${cmd}"

    echo "DBG: restart scheduler on TP master: ${tp_vm}"
    cmd="sudo pkill -f kube-scheduler"
    gcloud compute ssh --ssh-flag="-o LogLevel=quiet" --ssh-flag="-o ConnectTimeout=30" --project "${PROJECT}" --zone="${KUBE_GCE_ZONE}" "${tp_vm}" --command "${cmd}"

    echo "DBG: restart controller manager on TP master: ${tp_vm}"
    cmd="sudo pkill -f kube-controller-manager"
    gcloud compute ssh --ssh-flag="-o LogLevel=quiet" --ssh-flag="-o ConnectTimeout=30" --project "${PROJECT}" --zone="${KUBE_GCE_ZONE}" "${tp_vm}" --command "${cmd}"
  done
}

### start the hollow nodes for scaleout env
###
# TENANT_SERVER_KUBECONFIGS and RESOURCE_SERVER_KUBECONFIG are mounted secrets
# to the hollow-node kubelet containers
#
function start_hollow_nodes_scaleout {
  echo "DBG: start hollow nodes for scaleout env"
  export TENANT_SERVER_KUBECONFIGS="/kubeconfig/tp1.kubeconfig"
  for (( tp_num=2; tp_num<=${SCALEOUT_TP_COUNT}; tp_num++ ))
  do
    export TENANT_SERVER_KUBECONFIGS="${TENANT_SERVER_KUBECONFIGS},/kubeconfig/tp${tp_num}.kubeconfig"
  done

  echo "DBG: TENANT_SERVER_KUBECONFIGS: ${TENANT_SERVER_KUBECONFIGS}"

  for (( rp_num=1; rp_num<=${SCALEOUT_RP_COUNT}; rp_num++ ))
  do
    RP_NUM=${rp_num}
    RESOURCE_SERVER_KUBECONFIG="/kubeconfig/rp.kubeconfig-${rp_num}"
    echo "DBG: RESOURCE_SERVER_KUBECONFIG: ${RESOURCE_SERVER_KUBECONFIG}"
    start-hollow-nodes
  done
}

# setup_proxy setups the proxy service for the scale-out deployment
# and creates kubeconfig with the proxy service IP and port
function setup_proxy {
  local -r tp1_ip=$(grep server "${TP_KUBECONFIG}-1" | awk -F "/" '{print $3}')
  export TPIP="${tp1_ip}"
  for (( tp_num=2; tp_num<=${SCALEOUT_TP_COUNT}; tp_num++ ))
  do
    tp_ip=$(grep server "${TP_KUBECONFIG}-${tp_num}" | awk -F "/" '{print $3}')
    export TPIP=${TPIP},"${tp_ip}"
  done

  # currently proxy only supports 1 RP
  rp1_ip=$(grep server "${RP_KUBECONFIG}-1" | awk -F "/" '{print $3}')
  export RPIP="${rp1_ip}"

  export KUBERNETES_SCALEOUT_PROXY=true
  export KUBE_BEARER_TOKEN=${SHARED_APISERVER_TOKEN}
  export SCALEOUT_PROXY_NAME="${KUBE_GCE_INSTANCE_PREFIX}-proxy"
  export PROXY_KUBECONFIG="${RESOURCE_DIRECTORY}/kubeconfig.kubemark-proxy"
  export KUBERNETES_SCALEOUT_PROXY_APP=${KUBERNETES_SCALEOUT_PROXY_APP:-haproxy}
  export PROXY_CONFIG_FILE=${PROXY_CONFIG_FILE:-"haproxy.cfg"}
  export PROXY_CONFIG_FILE_TMP="${RESOURCE_DIRECTORY}/${PROXY_CONFIG_FILE}.tmp"
  create-arktos-proxy
  export KUBERNETES_SCALEOUT_PROXY=false
}

function create_TP() {
  local id=${1:-0}
  echo "DBG: creating TP ${id}"
  export TENANT_PARTITION_SEQUENCE=${id}
  export KUBEMARK_CLUSTER_KUBECONFIG="${TP_KUBECONFIG}-${id}"
  create-kubemark-master
  echo "DBG: TP ${id} created"
}

function create_RP() {
  local id=${1:-0}
  echo "DBG: creating RP ${id}"
  export RESOURCE_PARTITION_SEQUENCE=${id}
  export KUBEMARK_CLUSTER_KUBECONFIG="${RP_KUBECONFIG}-${id}"
  create-kubemark-master
  echo "DBG: RP ${id} created"
}

detect-project &> /dev/null

### master_metadata is used in the cloud-int script to create the GCE VMs
#
MASTER_METADATA=""

### to upload etcd image / binary tar once
#   this used to be part of create-kubemark-master; separate it out as preliminary
#   step to ensure it only be done once when starting kubemark cluster.
if [[ -z ${UPLOAD_TAR_DONE:-} ]]; then
  echo "DBG: uploading image + tar files..."
  export SERVER_BINARY_TAR_URL
  export SERVER_BINARY_TAR_HASH
  export KUBE_MANIFESTS_TAR_URL
  export KUBE_MANIFESTS_TAR_HASH
  export NODE_BINARY_TAR_URL
  export NODE_BINARY_TAR_HASH
  test_resource_upload
  echo "DBG: image + tar files uploaded"
  UPLOAD_TAR_DONE="done by kubemark"
fi

if [[ "${SCALEOUT_CLUSTER:-false}" == "true" ]]; then
  echo "DBG: Generating shared CA certificates"
  generate-shared-ca-cert

  ### calculate the expected TENANT_SERVER_KUBECONFIGS and MASTER_METADATA,
  #   which will be used by components that need to talk to all TPs.
  export TENANT_SERVER_KUBECONFIGS
  export MASTER_METADATA
  for (( tp_num=1; tp_num<=${SCALEOUT_TP_COUNT}; tp_num++ ))
  do
      # TODO: fix the hardcoded path
      # the path is what the controller used in master init script on the master machines
      if [[ ${tp_num} == 1 ]]; then
          export TENANT_SERVER_KUBECONFIGS="/etc/srv/kubernetes/tp-kubeconfigs/tp-${tp_num}-kubeconfig"
      else
          export TENANT_SERVER_KUBECONFIGS="${TENANT_SERVER_KUBECONFIGS},/etc/srv/kubernetes/tp-kubeconfigs/tp-${tp_num}-kubeconfig"
      fi

      if [[ ${tp_num} == 1 ]]; then
          MASTER_METADATA="tp-${tp_num}=${TP_KUBECONFIG}-${tp_num}"
      else
          MASTER_METADATA=${MASTER_METADATA},"tp-${tp_num}=${TP_KUBECONFIG}-${tp_num}"
      fi
  done

  echo "DBG: Starting ${SCALEOUT_TP_COUNT} tenant partitions ..."
  export USE_INSECURE_SCALEOUT_CLUSTER_MODE="${USE_INSECURE_SCALEOUT_CLUSTER_MODE:-false}"
  export KUBE_ENABLE_APISERVER_INSECURE_PORT="${KUBE_ENABLE_APISERVER_INSECURE_PORT:-false}"
  export KUBERNETES_TENANT_PARTITION=true
  export KUBERNETES_RESOURCE_PARTITION=false
  export PROXY_KUBECONFIG

  # the (shared) bearer token is needed for all TP/RP provisioning
  export SHARED_APISERVER_TOKEN="$(secure_random 32)"
  export KUBE_BEARER_TOKEN=${SHARED_APISERVER_TOKEN}
  echo "DBG: shared bearer token: ${KUBE_BEARER_TOKEN}"

  for (( tp_num=1; tp_num<=${SCALEOUT_TP_COUNT}; tp_num++ ))
  do
    # TODO: each background call has dedicated log
    create_TP ${tp_num} &
  done

  echo "DBG: waiting for all TP to be created, at $(date)..."
  wait
  echo "DBG: all TP created, at $(date)"

  echo "DBG: Starting resource partition ..."
  export KUBE_MASTER_EXTRA_METADATA="${MASTER_METADATA}"
  echo "DBG: KUBE_MASTER_EXTRA_METADATA:  ${KUBE_MASTER_EXTRA_METADATA}"

  export KUBERNETES_TENANT_PARTITION=false
  export KUBERNETES_RESOURCE_PARTITION=true

  for (( rp_num=1; rp_num<=${SCALEOUT_RP_COUNT}; rp_num++ ))
  do
    # TODO: each background call has dedicated log
    create_RP ${rp_num} &
  done

  echo "DBG: waiting for all RP to be created, at $(date)..."
  wait
  echo "DBG: all RP created, at $(date)"

  export KUBERNETES_RESOURCE_PARTITION=false

  # proxy setup expects a valid RP kubeconfig file to get master IP from;
  # rp-1 should be safe to assume here
  KUBEMARK_CLUSTER_KUBECONFIG="${RP_KUBECONFIG}-1"

  restart_tp_scheduler_and_controller
  start_hollow_nodes_scaleout
  setup_proxy
else
  # scale-up, just create the master servers
  export KUBEMARK_CLUSTER_KUBECONFIG="${RESOURCE_DIRECTORY}/kubeconfig.kubemark"
  create-kubemark-master
  start-hollow-nodes
fi

### display and verify cluster info
###
echo ""
if [ "${CLOUD_PROVIDER}" = "aws" ]; then
  echo "Master Public IP: ${MASTER_PUBLIC_IP}"
  echo "Master Internal IP: ${MASTER_INTERNAL_IP}"
else
  echo "Master IP: ${MASTER_IP}"
fi

sleep 5
echo -e "\nListing kubeamrk cluster details:" >&2

if [[ "${SCALEOUT_CLUSTER:-false}" == "true" ]]; then
  for (( rp_num=1; rp_num<=${SCALEOUT_RP_COUNT}; rp_num++ ))
  do
    rp_kubeconfig="${RP_KUBECONFIG}-${rp_num}"
    echo
    echo -e "Getting total hollow-nodes number for RP-${rp_num}" >&2
    "${KUBECTL}" --kubeconfig="${rp_kubeconfig}" get node | grep "hollow-node" | wc -l
    echo
  done

  for (( tp_num=1; tp_num<=${SCALEOUT_TP_COUNT}; tp_num++ ))
  do
    tp_kubeconfig="${TP_KUBECONFIG}-${tp_num}"
    echo
    echo -e "Getting endpoints status for TP-${tp_num}:" >&2
    "${KUBECTL}" --kubeconfig="${tp_kubeconfig}" get endpoints -A
    echo
    echo -e "Getting workload controller co status for TP-${tp_num}:" >&2
    "${KUBECTL}" --kubeconfig="${tp_kubeconfig}" get co
    echo
    echo -e "Getting apiserver data partition status for TP-${tp_num}:" >&2
    "${KUBECTL}" --kubeconfig="${tp_kubeconfig}" get datapartition
    echo
    echo -e "Getting ETCD data partition status for TP-${tp_num}:" >&2
    "${KUBECTL}" --kubeconfig="${tp_kubeconfig}" get etcd
    echo
  done
else
  KUBEMARK_KUBECONFIG="${KUBEMARK_CLUSTER_KUBECONFIG}"
  "${KUBECTL}" --kubeconfig="${KUBEMARK_KUBECONFIG}" get node | wc -l
  echo
  echo -e "Getting total hollow-nodes number:" >&2
  "${KUBECTL}" --kubeconfig="${KUBEMARK_KUBECONFIG}" get node | grep "hollow-node" | wc -l
  echo
  echo -e "Getting endpoints status:" >&2
  "${KUBECTL}" --kubeconfig="${KUBEMARK_KUBECONFIG}" get endpoints -A
  echo
  echo -e "Getting workload controller co status:" >&2
  "${KUBECTL}" --kubeconfig="${KUBEMARK_KUBECONFIG}" get co
  echo
  echo -e "Getting apiserver data partition status:" >&2
  "${KUBECTL}" --kubeconfig="${KUBEMARK_KUBECONFIG}" get datapartition
  echo
  echo -e "Getting ETCD data partition status:" >&2
  "${KUBECTL}" --kubeconfig="${KUBEMARK_KUBECONFIG}" get etcd
echo
fi
