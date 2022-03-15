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

# set up variables
KUBE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"

# Arktos specific network and service support is a feature that each
# pod is associated to certain network, which has its own DNS service.
# By default, this feature is enabled in the dev cluster started by this script.
export DISABLE_NETWORK_SERVICE_SUPPORT=${DISABLE_NETWORK_SERVICE_SUPPORT:-}

# flannel is the default cni plugin for scale-out env
export CNIPLUGIN=${CNIPLUGIN:-flannel}
if [ "${CNIPLUGIN}" == "flannel" ]
then
  echo "DBG: Flannel CNI plugin will be installed AFTER cluster is up"
  export ARKTOS_NO_CNI_PREINSTALLED="y"
fi

echo KUBE_ROOT ${KUBE_ROOT}
source "${KUBE_ROOT}/hack/lib/common-var-init.sh"

IS_RESOURCE_PARTITION=${IS_RESOURCE_PARTITION:-"false"}

# proxy is still used to start cloud KCM. Also useful for system tenant requests.
# However, don't use proxy to query node list as there is no aggregator for multiple RPs
# As we are tring to remove HA proxy, SCALE_OUT_PROXY_IP and SCALE_OUT_PROXY_PORT are both no longer
#  required in local cluster up. When they are not provided, they will be default to API server host
#  ip and port. If you need proxy to be running, please set environment variable SCALE_OUT_PROXY_IP
#  and SCALE_OUT_PROXY_PORT explicitly.
SCALE_OUT_PROXY_IP=${SCALE_OUT_PROXY_IP:-}
SCALE_OUT_PROXY_PORT=${SCALE_OUT_PROXY_PORT:-}
TENANT_SERVER=${TENANT_SERVER:-}
RESOURCE_SERVER=${RESOURCE_SERVER:-}
IS_SCALE_OUT=${IS_SCALE_OUT:-"true"}
IS_SECONDARY_TP=${IS_SECONDARY_TP:-"false"}

echo "IS_RESOURCE_PARTITION: |${IS_RESOURCE_PARTITION}|"
echo "TENANT_SERVER: |${TENANT_SERVER}|"
echo "RESOURCE_SERVER: |${RESOURCE_SERVER}|"
echo "IS_SCALE_OUT: |${IS_SCALE_OUT}|"

# scale-out specific env vars
# TP applicable vars
export TENANT_SERVER_NAME=${TENANT_SERVER_NAME:-${API_HOST}}
export TENANT_PARTITION_SERVICE_SUBNET=${TENANT_PARTITION_SERVICE_SUBNET:-10.0.0.0/16}
SERVICE_CLUSTER_IP_RANGE=${TENANT_PARTITION_SERVICE_SUBNET}

# get FIRST_SERVICE_CLUSTER_IP
IP_PREFIX=${TENANT_PARTITION_SERVICE_SUBNET%.*}
FIRST_SERVICE_CLUSTER_IP="${IP_PREFIX}.1"
echo "FIRST_SERVICE_CLUSTER_IP: |${FIRST_SERVICE_CLUSTER_IP}|"

# RP applicable vars
export RESOURCE_PARTITION_POD_CIDR=${RESOURCE_PARTITION_POD_CIDR:-10.244.0.0/16}
KUBE_CONTROLLER_MANAGER_CLUSTER_CIDR=${RESOURCE_PARTITION_POD_CIDR}

echo "TENANT_SERVER_NAME: |${TENANT_SERVER_NAME}|"
echo "TENANT_PARTITION_SERVICE_SUBNET: |${TENANT_PARTITION_SERVICE_SUBNET}|"
echo "RESOURCE_PARTITION_POD_CIDR: |${RESOURCE_PARTITION_POD_CIDR}|"

if [[ -z "${SCALE_OUT_PROXY_IP}" ]]; then
  echo SCALE_OUT_PROXY_IP is missing. Default to local host ip ${API_HOST}
  SCALE_OUT_PROXY_IP=${API_HOST}
fi

if [[ -z "${SCALE_OUT_PROXY_PORT}" ]]; then
  echo SCALE_OUT_PROXY_PORT is missing. Default to local host non secure port ${API_PORT}
  SCALE_OUT_PROXY_PORT=${API_PORT}
fi

SCALE_OUT_PROXY_ENDPOINT="https://${SCALE_OUT_PROXY_IP}:${SCALE_OUT_PROXY_PORT}/"

if [[ -z "${TENANT_SERVER}" ]]; then
  if [ IS_RESOURCE_PARTITION == "true" ]; then
    echo ERROR: Please set TENANT_SERVER for RP. For example: TENANT_SERVER=192.168.0.2 or TENANT_SERVER=192.168.0.3,192.168.0.5
    exit 1
  fi
else
  TENANT_SERVERS=(${TENANT_SERVER//,/ })
fi

if [[ -z "${RESOURCE_SERVER}" ]]; then
  if ! [ "${IS_RESOURCE_PARTITION}" == "true" ]; then
    echo ERROR: Please set RESOURCE_SERVER in tenant partition for RP. For example: RESOURCE_SERVER=192.168.0.2 or RESOURCE_SERVER=192.168.0.2,192.168.10.123
    exit 1
  fi
else
  RESOURCE_SERVERS=(${RESOURCE_SERVER//,/ })
fi

# sanity check for OpenStack provider
if [ "${CLOUD_PROVIDER}" == "openstack" ]; then
    if [ "${CLOUD_CONFIG}" == "" ]; then
        echo "Missing CLOUD_CONFIG env for OpenStack provider!"
        exit 1
    fi
    if [ ! -f "${CLOUD_CONFIG}" ]; then
        echo "Cloud config ${CLOUD_CONFIG} doesn't exist"
        exit 1
    fi
fi

# set feature gates if enable Pod priority and preemption
FEATURE_GATES="${FEATURE_GATES_COMMON_BASE}"
if [ "${ENABLE_POD_PRIORITY_PREEMPTION}" == true ]; then
    FEATURE_GATES="${FEATURE_GATES},PodPriority=true"
fi
if [[ "${CNIPLUGIN}" == "mizar" ]]; then
  FEATURE_GATES="${FEATURE_GATES},MizarVPCRangeNoOverlap=true"
fi

# check for network service support flags
if [ -z ${DISABLE_NETWORK_SERVICE_SUPPORT} ]; then # when enabled
  # kubelet enforces per-network DNS ip in pod
  FEATURE_GATES="${FEATURE_GATES},MandatoryArktosNetwork=true"
  # tenant controller automatically creates a default network resource for new tenant
  if [ "${CNIPLUGIN}" == "mizar" ]; then
    ARKTOS_NETWORK_TEMPLATE="${KUBE_ROOT}/hack/runtime/default_mizar_network.json"
  else
    ARKTOS_NETWORK_TEMPLATE="${KUBE_ROOT}/hack/testdata/default-flat-network.tmpl"
  fi
else # when disabled
  # kube-apiserver not to enforce deployment-network validation
  DISABLE_ADMISSION_PLUGINS="DeploymentNetwork"
fi

echo "DBG: effective feature gates ${FEATURE_GATES}"
echo "DBG: effective disabling admission plugins ${DISABLE_ADMISSION_PLUGINS}"
echo "DBG: effective default network template file is ${ARKTOS_NETWORK_TEMPLATE}"
echo "DBG: kubelet arg RESOLV_CONF is ${RESOLV_CONF}"

# warn if users are running with swap allowed
if [ "${FAIL_SWAP_ON}" == "false" ]; then
    echo "WARNING : The kubelet is configured to not fail even if swap is enabled; production deployments should disable swap."
fi

if [ "$(id -u)" != "0" ]; then
    echo "WARNING : This script MAY be run as root for docker socket / iptables functionality; if failures occur, retry as root." 2>&1
fi

# Stop right away if the build fails
set -e

# Do dudiligence to ensure containerd service and socket in a working state
# Containerd service should be part of docker.io installation or apt-get install containerd for Ubuntu OS
if ! sudo systemctl is-active --quiet containerd; then
  echo "Containerd is required for Arktos"
  exit 1
fi

if [[ ! -e "${CONTAINERD_SOCK_PATH}" ]]; then
  echo "Containerd socket file check failed. Please check containerd socket file path"
  exit 1
fi

# Install simple cni plugin based on env var CNIPLUGIN (bridge, alktron, mizar) before cluster is up.
# If more advanced cni like Flannel is desired, it should be installed AFTER the clsuter is up;
# in that case, please set ARKTOS-NO-CNI_PREINSTALLED to any no-empty value
echo "CNIPLUGIN=|${CNIPLUGIN}|, calling arktos-cni.rc"
[ "${IS_RESOURCE_PARTITION}" == "true" ] && source ${KUBE_ROOT}/hack/arktos-cni.rc
echo "CNIPLUGIN=|${CNIPLUGIN}|, calling arktos-cni.rc DONE"

source "${KUBE_ROOT}/hack/lib/init.sh"
source "${KUBE_ROOT}/hack/lib/common.sh"

kube::util::ensure-gnu-sed

function usage {
            echo "This script starts a local kube cluster. "
            echo "Example 0: hack/local-up-cluster.sh -h  (this 'help' usage description)"
            echo "Example 1: hack/local-up-cluster.sh -o _output/dockerized/bin/linux/amd64/ (run from docker output)"
            echo "Example 2: hack/local-up-cluster.sh -O (auto-guess the bin path for your platform)"
            echo "Example 3: hack/local-up-cluster.sh (build a local copy of the source)"
}

### Allow user to supply the source directory.
GO_OUT=${GO_OUT:-}
while getopts "ho:O" OPTION
do
    case ${OPTION} in
        o)
            echo "skipping build"
            GO_OUT="${OPTARG}"
            echo "using source ${GO_OUT}"
            ;;
        O)
            GO_OUT=$(kube::common::guess_built_binary_path)
            if [ "${GO_OUT}" == "" ]; then
                echo "Could not guess the correct output directory to use."
                exit 1
            fi
            ;;
        h)
            usage
            exit
            ;;
        ?)
            usage
            exit
            ;;
    esac
done

if [ "x${GO_OUT}" == "x" ]; then
    make -C "${KUBE_ROOT}" WHAT="cmd/kubectl cmd/hyperkube cmd/kube-apiserver cmd/kube-controller-manager cmd/workload-controller-manager cmd/cloud-controller-manager cmd/kubelet cmd/kube-proxy cmd/kube-scheduler cmd/arktos-network-controller"
else
    echo "skipped the build."
fi

# Shut down anyway if there's an error.
set +e


# name of the cgroup driver, i.e. cgroupfs or systemd
if [[ ${CONTAINER_RUNTIME} == "docker" ]]; then
  # default cgroup driver to match what is reported by docker to simplify local development
  if [[ -z ${CGROUP_DRIVER} ]]; then
    # match driver with docker runtime reported value (they must match)
    CGROUP_DRIVER=$(docker info | grep "Cgroup Driver:" |  sed -e 's/^[[:space:]]*//'|cut -f3- -d' ')
    echo "Kubelet cgroup driver defaulted to use: ${CGROUP_DRIVER}"
  fi
  if [[ -f /var/log/docker.log && ! -f "${LOG_DIR}/docker.log" ]]; then
    ln -s /var/log/docker.log "${LOG_DIR}/docker.log"
  fi
fi



# Ensure CERT_DIR is created for auto-generated crt/key and kubeconfig
mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"
CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")

cleanup()
{
  echo "Cleaning up..."
  # delete running images
  # if [[ "${ENABLE_CLUSTER_DNS}" == true ]]; then
  # Still need to figure why this commands throw an error: Error from server: client: etcd cluster is unavailable or misconfigured
  #     ${KUBECTL} --namespace=kube-system delete service kube-dns
  # And this one hang forever:
  #     ${KUBECTL} --namespace=kube-system delete rc kube-dns-v10
  # fi

  # Check if the API server is still running

  echo "Killing the following apiserver running processes"
  for APISERVER_PID_ITEM in "${APISERVER_PID_ARRAY[@]}" 
  do
      [[ -n "${APISERVER_PID_ITEM-}" ]] && mapfile -t APISERVER_PIDS < <(pgrep -P "${APISERVER_PID_ITEM}" ; ps -o pid= -p "${APISERVER_PID_ITEM}")
      [[ -n "${APISERVER_PIDS-}" ]] && sudo kill "${APISERVER_PIDS[@]}" 2>/dev/null
      echo "${APISERVER_PID_ITEM} has been killed"
  done
  #[[ -n "${APISERVER_PID-}" ]] && mapfile -t APISERVER_PIDS < <(pgrep -P "${APISERVER_PID}" ; ps -o pid= -p "${APISERVER_PID}")
  #[[ -n "${APISERVER_PIDS-}" ]] && sudo kill "${APISERVER_PIDS[@]}" 2>/dev/null

  # Check if the controller-manager is still running
  [[ -n "${CTLRMGR_PID-}" ]] && mapfile -t CTLRMGR_PIDS < <(pgrep -P "${CTLRMGR_PID}" ; ps -o pid= -p "${CTLRMGR_PID}")
  [[ -n "${CTLRMGR_PIDS-}" ]] && sudo kill "${CTLRMGR_PIDS[@]}" 2>/dev/null

  # Check if the arktos network controller is still running
  [[ -n "${ARKTOS_NETWORK_CONTROLLER_PID-}" ]] && mapfile -t ARKTOS_NETWORK_CONTROLLER_PID < <(pgrep -P "${ARKTOS_NETWORK_CONTROLLER_PID}" ; ps -o pid= -p "${ARKTOS_NETWORK_CONTROLLER_PID}")
  [[ -n "${ARKTOS_NETWORK_CONTROLLER_PID-}" ]] && sudo kill "${ARKTOS_NETWORK_CONTROLLER_PID[@]}" 2>/dev/null

  # Check if the kubelet is still running
  [[ -n "${KUBELET_PID-}" ]] && mapfile -t KUBELET_PIDS < <(pgrep -P "${KUBELET_PID}" ; ps -o pid= -p "${KUBELET_PID}")
  [[ -n "${KUBELET_PIDS-}" ]] && sudo kill "${KUBELET_PIDS[@]}" 2>/dev/null

  # Check if the proxy is still running
  [[ -n "${PROXY_PID-}" ]] && mapfile -t PROXY_PIDS < <(pgrep -P "${PROXY_PID}" ; ps -o pid= -p "${PROXY_PID}")
  [[ -n "${PROXY_PIDS-}" ]] && sudo kill "${PROXY_PIDS[@]}" 2>/dev/null

  # Check if the scheduler is still running
  [[ -n "${SCHEDULER_PID-}" ]] && mapfile -t SCHEDULER_PIDS < <(pgrep -P "${SCHEDULER_PID}" ; ps -o pid= -p "${SCHEDULER_PID}")
  [[ -n "${SCHEDULER_PIDS-}" ]] && sudo kill "${SCHEDULER_PIDS[@]}" 2>/dev/null

  # Check if the etcd is still running
  [[ -n "${ETCD_PID-}" ]] && kube::etcd::stop
  if [[ "${PRESERVE_ETCD}" == "false" ]]; then
    [[ -n "${ETCD_DIR-}" ]] && kube::etcd::clean_etcd_dir
  fi

  # Delete virtlet metadata and log directory
  if [[ -e "${VIRTLET_METADATA_DIR}" ]]; then
        echo "Cleanup runtime metadata folder"
        rm -f -r "${VIRTLET_METADATA_DIR}"
  fi

  if [[ -e "${VIRTLET_LOG_DIR}" ]]; then
       echo "Cleanup runtime log folder"
       rm -f -r "${VIRTLET_LOG_DIR}"
  fi

  [[ -n "${FLANNELD_PID-}" ]] && sudo kill "${FLANNELD_PID}" 2>/dev/null

  exit 0
}
# Check if all processes are still running. Prints a warning once each time
# a process dies unexpectedly.
function healthcheck {
  if [[ -n "${APISERVER_PID-}" ]] && ! sudo kill -0 "${APISERVER_PID}" 2>/dev/null; then
    warning_log "API server terminated unexpectedly, see ${APISERVER_LOG}"
    APISERVER_PID=
  fi

  if [[ -n "${CTLRMGR_PID-}" ]] && ! sudo kill -0 "${CTLRMGR_PID}" 2>/dev/null; then
    warning_log "kube-controller-manager terminated unexpectedly, see ${CTLRMGR_LOG}"
    CTLRMGR_PID=
  fi

  if [[ -n "${ARKTOS_NETWORK_CONTROLLER_PID-}" ]] && ! sudo kill -0 "${ARKTOS_NETWORK_CONTROLLER_PID}" 2>/dev/null; then
    warning_log "arktos network controller terminated unexpectedly, see ${ARKTOS_NETWORK_CONTROLLER_LOG}"
    ARKTOS_NETWORK_CONTROLLER_PID=
  fi

  if [[ -n "${KUBELET_PID-}" ]] && ! sudo kill -0 "${KUBELET_PID}" 2>/dev/null; then
    warning_log "kubelet terminated unexpectedly, see ${KUBELET_LOG}"
    KUBELET_PID=
  fi

  if [[ -n "${PROXY_PID-}" ]] && ! sudo kill -0 "${PROXY_PID}" 2>/dev/null; then
    warning_log "kube-proxy terminated unexpectedly, see ${PROXY_LOG}"
    PROXY_PID=
  fi

  if [[ -n "${SCHEDULER_PID-}" ]] && ! sudo kill -0 "${SCHEDULER_PID}" 2>/dev/null; then
    warning_log "scheduler terminated unexpectedly, see ${SCHEDULER_LOG}"
    SCHEDULER_PID=
  fi

  if [[ -n "${ETCD_PID-}" ]] && ! sudo kill -0 "${ETCD_PID}" 2>/dev/null; then
    warning_log "etcd terminated unexpectedly"
    ETCD_PID=
  fi
}

function print_color {
  message=$1
  prefix=${2:+$2: } # add colon only if defined
  color=${3:-1}     # default is red
  echo -n "$(tput bold)$(tput setaf "${color}")"
  echo "${prefix}${message}"
  echo -n "$(tput sgr0)"
}

function warning_log {
  print_color "$1" "W$(date "+%m%d %H:%M:%S")]" 1
}

function start_etcd {
    echo "Starting etcd"
    export ETCD_LOGFILE=${LOG_DIR}/etcd.log
    kube::etcd::start
}

function start_cloud_controller_manager {
    if [ -z "${CLOUD_CONFIG}" ]; then
      echo "CLOUD_CONFIG cannot be empty!"
      exit 1
    fi
    if [ ! -f "${CLOUD_CONFIG}" ]; then
      echo "Cloud config ${CLOUD_CONFIG} doesn't exist"
      exit 1
    fi

    node_cidr_args=()
    if [[ "${NET_PLUGIN}" == "kubenet" ]]; then
      node_cidr_args=("--allocate-node-cidrs=true" "--cluster-cidr=10.1.0.0/16")
    fi

    CLOUD_CTLRMGR_LOG=${LOG_DIR}/cloud-controller-manager.log
    ${CONTROLPLANE_SUDO} "${EXTERNAL_CLOUD_PROVIDER_BINARY:-"${GO_OUT}/hyperkube" cloud-controller-manager}" \
      --v="${LOG_LEVEL}" \
      --vmodule="${LOG_SPEC}" \
      "${node_cidr_args[@]:-}" \
      --feature-gates="${FEATURE_GATES}" \
      --cloud-provider="${CLOUD_PROVIDER}" \
      --cloud-config="${CLOUD_CONFIG}" \
      --kubeconfig "${CERT_DIR}"/controller.kubeconfig \
      --leader-elect=false \
      --master=${SCALE_OUT_PROXY_ENDPOINT} >"${CLOUD_CTLRMGR_LOG}" 2>&1 &
    export CLOUD_CTLRMGR_PID=$!
}

function start_nodelocaldns {
  cp "${KUBE_ROOT}/cluster/addons/dns/nodelocaldns/nodelocaldns.yaml" nodelocaldns.yaml
  sed -i -e "s/__PILLAR__DNS__DOMAIN__/${DNS_DOMAIN}/g" nodelocaldns.yaml
  sed -i -e "s/__PILLAR__DNS__SERVER__/${DNS_SERVER_IP}/g" nodelocaldns.yaml
  sed -i -e "s/__PILLAR__LOCAL__DNS__/${LOCAL_DNS_IP}/g" nodelocaldns.yaml
  # use kubectl to create nodelocaldns addon
  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" --namespace=kube-system create -f nodelocaldns.yaml
  echo "NodeLocalDNS addon successfully deployed."
  rm nodelocaldns.yaml
}

function start_kubedashboard {
    if [[ "${ENABLE_CLUSTER_DASHBOARD}" = true ]]; then
        echo "Creating kubernetes-dashboard"
        # use kubectl to create the dashboard
        ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/cluster/addons/dashboard/dashboard-secret.yaml"
        ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/cluster/addons/dashboard/dashboard-configmap.yaml"
        ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/cluster/addons/dashboard/dashboard-rbac.yaml"
        ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/cluster/addons/dashboard/dashboard-controller.yaml"
        ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/cluster/addons/dashboard/dashboard-service.yaml"
        echo "kubernetes-dashboard deployment and service successfully deployed."
    fi
}

function create_psp_policy {
    echo "Create podsecuritypolicy policies for RBAC."
    ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" create -f "${KUBE_ROOT}/examples/podsecuritypolicy/rbac/policies.yaml"
    ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" create -f "${KUBE_ROOT}/examples/podsecuritypolicy/rbac/roles.yaml"
    ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" create -f "${KUBE_ROOT}/examples/podsecuritypolicy/rbac/bindings.yaml"
}

function create_storage_class {
    if [ -z "${CLOUD_PROVIDER}" ]; then
        CLASS_FILE=${KUBE_ROOT}/cluster/addons/storage-class/local/default.yaml
    else
        CLASS_FILE=${KUBE_ROOT}/cluster/addons/storage-class/${CLOUD_PROVIDER}/default.yaml
    fi

    if [ -e "${CLASS_FILE}" ]; then
        echo "Create default storage class for ${CLOUD_PROVIDER}"
        ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" create -f "${CLASS_FILE}"
    else
        echo "No storage class available for ${CLOUD_PROVIDER}."
    fi
}

function print_success {
if [[ "${START_MODE}" != "kubeletonly" ]]; then
  if [[ "${ENABLE_DAEMON}" = false ]]; then
    echo "Local Kubernetes cluster is running. Press Ctrl-C to shut it down."
  else
    echo "Local Kubernetes cluster is running."
  fi
  cat <<EOF

Logs:
  ${APISERVER_LOG:-}
  ${CTLRMGR_LOG:-}
  ${CLOUD_CTLRMGR_LOG:-}
  ${PROXY_LOG:-}
  ${SCHEDULER_LOG:-}
EOF
fi

if [[ "${START_MODE}" == "all" ]]; then
  echo "  ${KUBELET_LOG}"
elif [[ "${START_MODE}" == "nokubelet" ]]; then
  echo
  echo "No kubelet was started because you set START_MODE=nokubelet"
  echo "Run this script again with START_MODE=kubeletonly to run a kubelet"
fi

if [[ "${START_MODE}" != "kubeletonly" ]]; then
  echo
  if [[ "${ENABLE_DAEMON}" = false ]]; then
    echo "To start using your cluster, you can open up another terminal/tab and run:"
  else
    echo "To start using your cluster, run:"
  fi
  cat <<EOF

  export KUBECONFIG=${CERT_DIR}/admin.kubeconfig
Or
  export KUBECONFIG=${CERT_DIR}/adminN(N=0,1,...).kubeconfig

  cluster/kubectl.sh

Alternatively, you can write to the default kubeconfig:

  export KUBERNETES_PROVIDER=local

  cluster/kubectl.sh config set-cluster local --server=https://${API_HOST}:${API_SECURE_PORT} --certificate-authority=${ROOT_CA_FILE}
  cluster/kubectl.sh config set-credentials myself ${AUTH_ARGS}
  cluster/kubectl.sh config set-context local --cluster=local --user=myself
  cluster/kubectl.sh config use-context local
  cluster/kubectl.sh
EOF
else
  cat <<EOF
The kubelet was started.

Logs:
  ${KUBELET_LOG}
EOF
fi

if [ "${IS_RESOURCE_PARTITION}" == "true" ]; then
  printf "\033[0;32mResource Partition Cluster is Running ... \033[0m\n"
else
  printf "\033[0;33mTenant Partition Cluster is Running ... \033[0m\n"
fi

}

# install etcd if necessary
if ! [[ $(which etcd) ]]; then
  if ! [ -f "${KUBE_ROOT}/third_party/etcd/etcd" ]; then
     echo "cannot find etcd locally. will install one."
     ${KUBE_ROOT}/hack/install-etcd.sh
  fi

  export PATH=$PATH:${KUBE_ROOT}/third_party/etcd
fi

# If we are running in the CI, we need a few more things before we can start
if [[ "${KUBETEST_IN_DOCKER:-}" == "true" ]]; then
  echo "Preparing to test ..."
  "${KUBE_ROOT}"/hack/install-etcd.sh
  export PATH="${KUBE_ROOT}/third_party/etcd:${PATH}"
  KUBE_FASTBUILD=true make ginkgo cross

  apt-get update && apt-get install -y sudo
  apt-get remove -y systemd

  # configure shared mounts to prevent failure in DIND scenarios
  mount --make-rshared /

  # kubekins has a special directory for docker root
  DOCKER_ROOT="/docker-graph"
fi

# validate that etcd is: not running, in path, and has minimum required version.
if [[ "${START_MODE}" != "kubeletonly" ]]; then
  kube::etcd::validate
fi

if [ "${CONTAINER_RUNTIME}" == "docker" ] && ! kube::util::ensure_docker_daemon_connectivity; then
  exit 1
fi

if [[ "${START_MODE}" != "kubeletonly" ]]; then
  kube::common::test_apiserver_off
fi

kube::util::test_openssl_installed
kube::util::ensure-cfssl

### IF the user didn't supply an output/ for the build... Then we detect.
if [ "${GO_OUT}" == "" ]; then
  kube::common::detect_binary
fi
echo "Detected host and ready to start services.  Doing some housekeeping first..."
echo "Using GO_OUT ${GO_OUT}"
export KUBELET_CIDFILE=/tmp/kubelet.cid
if [[ "${ENABLE_DAEMON}" = false ]]; then
  trap cleanup EXIT
fi

echo "Starting services now!"
if [[ "${START_MODE}" != "kubeletonly" ]]; then
  start_etcd
  kube::common::set_service_accounts
  echo "Starting ${APISERVER_NUMBER} kube-apiserver instances. If you want to make changes to the kube-apiserver nubmer, please run export APISERVER_SERVER=n(n=1,2,...). "
  APISERVER_PID_ARRAY=()
  previous=
  for ((i = $((APISERVER_NUMBER - 1)) ; i >= 0 ; i--)); do
    kube::common::start_apiserver $i
  done
  #remove workload controller manager cluster role and rolebinding applying per this already be added to bootstrappolicy
  
  # If there are other resources ready to sync thru workload-controller-mananger, please add them to the following clusterrole file
  #cluster/kubectl.sh create -f hack/runtime/workload-controller-manager-clusterrole.yaml

  #cluster/kubectl.sh create -f hack/runtime/workload-controller-manager-clusterrolebinding.yaml
  KCM_TENANT_SERVER_KUBECONFIG_FLAG="--tenant-server-kubeconfig="
  kubeconfig_filename="tenant-server-controller"
  if [ "${IS_RESOURCE_PARTITION}" == "true" ]; then
    serverCount=${#TENANT_SERVERS[@]}
    for (( pos=0; pos<${serverCount}; pos++ ));
    do
      # here generate kubeconfig for remote API server. Only work in non secure mode for now
      kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "" "${TENANT_SERVERS[${pos}]}" "${API_PORT}" tenant-server-controller "" "http"
      ${CONTROLPLANE_SUDO} mv "${CERT_DIR}/${kubeconfig_filename}.kubeconfig" "${CERT_DIR}/${kubeconfig_filename}${pos}.kubeconfig"
      ${CONTROLPLANE_SUDO} chown "$(whoami)" "${CERT_DIR}/${kubeconfig_filename}${pos}.kubeconfig"

      KCM_TENANT_SERVER_KUBECONFIG_FLAG="${KCM_TENANT_SERVER_KUBECONFIG_FLAG}${CERT_DIR}/${kubeconfig_filename}${pos}.kubeconfig,"
    done
    KCM_TENANT_SERVER_KUBECONFIG_FLAG=${KCM_TENANT_SERVER_KUBECONFIG_FLAG::-1}
  fi

  kube::common::start_controller_manager

  # Unless arktos service support is disabled, ensure start arktos network controller
  # on TENANT PARTITION nodes other than RESOURCE_PARTITION nodes
  if [ "${IS_RESOURCE_PARTITION}" != "true" ]; then
    [ ${DISABLE_NETWORK_SERVICE_SUPPORT} ] ||  kube::common::start_arktos_network_ontroller ${TENANT_SERVER_NAME}
  fi

  if [[ "${EXTERNAL_CLOUD_PROVIDER:-}" == "true" ]]; then
    start_cloud_controller_manager
  fi
  if [[ "${START_MODE}" != "nokubeproxy" ]]; then
    kube::common::start_kubeproxy
  fi
  if [ "${IS_RESOURCE_PARTITION}" != "true" ]; then
     kube::common::start_kubescheduler
  fi
  if [[ "${ENABLE_NODELOCAL_DNS:-}" == "true" ]]; then
    start_nodelocaldns
  fi
  start_kubedashboard
fi

if [[ "${START_MODE}" != "nokubelet" ]]; then
  ## TODO remove this check if/when kubelet is supported on darwin
  # Detect the OS name/arch and display appropriate error.
    case "$(uname -s)" in
      Darwin)
        print_color "kubelet is not currently supported in darwin, kubelet aborted."
        KUBELET_LOG=""
        ;;
      Linux)
        KUBELET_LOG=/tmp/kubelet.log
        kube::common::start_kubelet
        ;;
      *)
        print_color "Unsupported host OS.  Must be Linux or Mac OS X, kubelet aborted."
        ;;
    esac
fi

if [[ -n "${PSP_ADMISSION}" && "${AUTHORIZATION_MODE}" = *RBAC* ]]; then
  create_psp_policy
fi

if [[ "${DEFAULT_STORAGE_CLASS}" == "true" && "${IS_RESOURCE_PARTITION}" != "true" ]]; then
  create_storage_class
fi

if [ "${IS_RESOURCE_PARTITION}" != "true" ]; then
  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/pkg/controller/artifacts/crd-network.yaml"
  # refresh the resource discovery cache after the CRD is created
  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" api-resources &>/dev/null
fi
echo "*******************************************"
echo "Setup Arktos components ..."
echo ""

# todo: start flannel daemon deterministically, instead of waiting for arbitrary time
if [[ "${CNIPLUGIN}" == "flannel" && "${IS_RESOURCE_PARTITION}" == "true" ]]; then
  echo "Installing Flannel cni plugin... "
  sleep 30  #need sometime for KCM to be fully functioning
  install_flannel "${RESOURCE_PARTITION_POD_CIDR}"
fi

if [ "${IS_RESOURCE_PARTITION}" == "true" ]; then
  while ! cluster/kubectl.sh get nodes --no-headers | grep -i -w Ready; do sleep 3; echo "Waiting for node ready"; done

  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" label node ${HOSTNAME_OVERRIDE} extraRuntime=virtlet

  # Verify whether podCIDR on RESOURCE_PARTITION node is set correctly
  ${KUBECTL} get nodes -o yaml |grep podCIDR
fi

if [ "${IS_RESOURCE_PARTITION}" != "true" ]; then
  # Applying mizar cni
  if [[ "${CNIPLUGIN}" == "mizar" ]]; then
    ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" create configmap system-source --namespace=kube-system --from-literal=name=arktos --from-literal=company=futurewei

    # Creating mizar crds
    echo "Creating mizar crds ......."
    cp "${KUBE_ROOT}/third_party/mizar/mizar-crds.yaml" mizar-crds.yaml
    ${KUBECTL} apply -f mizar-crds.yaml
    rm mizar-crds.yaml

    if [[ "${IS_SECONDARY_TP}" == "false" ]]; then
      # Deploying mizar daemonset
      echo "Deploying mizar daemonset ......."
      cp "${KUBE_ROOT}/third_party/mizar/mizar-daemon.yaml" mizar-daemon.yaml
      sed -i -e "s@{{network_provider_version}}@${MIZAR_VERSION}@g" mizar-daemon.yaml
      ${KUBECTL} apply -f mizar-daemon.yaml
      rm mizar-daemon.yaml
    fi

    # Deploying mizar daemon to tp master
    echo "Deploying mizar daemon to TP master ......."
    cp "${KUBE_ROOT}/third_party/mizar/mizar-daemon-tpmaster.yaml" mizar-daemon-tpmaster.yaml
    sed -i -e "s@{{network_provider_version}}@${MIZAR_VERSION}@g" mizar-daemon-tpmaster.yaml
    sed -i -e "s@{{tp_master_name}}@${API_HOST}@g" mizar-daemon-tpmaster.yaml
    ${KUBECTL} apply -f mizar-daemon-tpmaster.yaml
    rm mizar-daemon-tpmaster.yaml

    # Place mizar operator
    echo "Starting mizar operator......."
    # # For starting mizar-operator pods on scale-out TP servers successfully on Ubuntu 20.04
    MIZAR_OPERATOR_HOST_PATH=${MIZAR_OPERATOR_HOST_PATH:-"/etc/kubernetes"}
    if [ ! -d "${MIZAR_OPERATOR_HOST_PATH}"  ]; then
      sudo mkdir -p ${MIZAR_OPERATOR_HOST_PATH}
    fi
    CLUSTER_VPC_VNI_ID="${RANDOM}"
    cp "${KUBE_ROOT}/third_party/mizar/mizar-operator.yaml" mizar-operator.yaml
    sed -i -e "s@{{network_provider_version}}@${MIZAR_VERSION}@g" mizar-operator.yaml
    sed -i -e "s@{{tp_master_name}}@${API_HOST}@g" mizar-operator.yaml
    sed -i -e "s@{{cluster_vpc_vni_id}}@${CLUSTER_VPC_VNI_ID}@g" mizar-operator.yaml
    ${KUBECTL} apply -f mizar-operator.yaml
    rm mizar-operator.yaml
  fi

  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" create configmap -n kube-system virtlet-image-translations --from-file ${VIRTLET_DEPLOYMENT_FILES_DIR}/images.yaml

  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" create -f ${VIRTLET_DEPLOYMENT_FILES_DIR}/vmruntime.yaml

  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" get ds --namespace kube-system

  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/cluster/addons/rbac/kubelet-network-reader/kubelet-network-reader.yaml"
fi
 
echo ""
echo "Arktos Setup done."
#echo "*******************************************"
#echo "Setup Kata Containers components ..."
#KUBECTL=${KUBECTL} "${KUBE_ROOT}"/hack/install-kata.sh
#echo "Kata Setup done."
#echo "*******************************************"

if [ "${CNIPLUGIN}" == "mizar" ] && [ "${IS_RESOURCE_PARTITION}" == "false" ]; then
  kube::common::wait-until-mizar-ready
fi

print_success

if [[ "${ENABLE_DAEMON}" = false ]]; then
  while true; do sleep 1; healthcheck; done
fi

if [[ "${KUBETEST_IN_DOCKER:-}" == "true" ]]; then
  cluster/kubectl.sh config set-cluster local --server=https://${API_HOST_IP}:6443 --certificate-authority=/var/run/kubernetes/server-ca.crt
  cluster/kubectl.sh config set-credentials myself --client-key=/var/run/kubernetes/client-admin.key --client-certificate=/var/run/kubernetes/client-admin.crt
  cluster/kubectl.sh config set-context local --cluster=local --user=myself
  cluster/kubectl.sh config use-context local
fi
