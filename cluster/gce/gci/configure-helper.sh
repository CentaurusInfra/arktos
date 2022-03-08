#!/usr/bin/env bash

# Copyright 2016 The Kubernetes Authors.
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

# This script is for configuring kubernetes master and node instances. It is
# uploaded in the manifests tar ball.

# TODO: this script duplicates templating logic from cluster/saltbase/salt
# using sed. It should use an actual template parser on the manifest
# files.

set -o errexit
set -o nounset
set -o pipefail

########### Main Function ###########
function main() {
  # redirect stdout/stderr to a file
  exec >> /var/log/master-init.log 2>&1
  echo "Start to configure master instance for kubernetes"
  source "/home/kubernetes/bin/configure-helper-common.sh"
  readonly UUID_MNT_PREFIX="/mnt/disks/by-uuid/google-local-ssds"
  readonly UUID_BLOCK_PREFIX="/dev/disk/by-uuid/google-local-ssds"
  readonly COREDNS_AUTOSCALER="Deployment/coredns"
  readonly KUBEDNS_AUTOSCALER="Deployment/kube-dns"

  # Resource requests of master components.
  KUBE_CONTROLLER_MANAGER_CPU_REQUEST="${KUBE_CONTROLLER_MANAGER_CPU_REQUEST:-200m}"
  WORKLOAD_CONTROLLER_MANAGER_CPU_REQUEST="${WORKLOAD_CONTROLLER_MANAGER_CPU_REQUEST:-200m}"
  ARKTOS_NETWORK_CONTROLLER_CPU_REQUEST="${ARKTOS_NETWORK_CONTROLLER_CPU_REQUEST:-200m}"
  KUBE_SCHEDULER_CPU_REQUEST="${KUBE_SCHEDULER_CPU_REQUEST:-75m}"

  # Use --retry-connrefused opt only if it's supported by curl.
  CURL_RETRY_CONNREFUSED=""
  if curl --help | grep -q -- '--retry-connrefused'; then
    CURL_RETRY_CONNREFUSED='--retry-connrefused'
  fi

  KUBE_HOME="/home/kubernetes"
  CONTAINERIZED_MOUNTER_HOME="${KUBE_HOME}/containerized_mounter"
  PV_RECYCLER_OVERRIDE_TEMPLATE="${KUBE_HOME}/kube-manifests/kubernetes/pv-recycler-template.yaml"
  DEFAULT_NETWORK_TEMPLATE="${KUBE_HOME}/kube-manifests/kubernetes/default-network.tmpl"

  if [[ ! -e "${KUBE_HOME}/kube-env" ]]; then
    echo "The ${KUBE_HOME}/kube-env file does not exist!! Terminate cluster initialization."
    exit 1
  fi

  source "${KUBE_HOME}/kube-env"


  if [[ -f "${KUBE_HOME}/kubelet-config.yaml" ]]; then
    echo "Found Kubelet config file at ${KUBE_HOME}/kubelet-config.yaml"
    KUBELET_CONFIG_FILE_ARG="--config ${KUBE_HOME}/kubelet-config.yaml"
  fi

  if [[ -e "${KUBE_HOME}/kube-master-certs" ]]; then
    source "${KUBE_HOME}/kube-master-certs"
  fi

  if [[ -n "${KUBE_USER:-}" ]]; then
    if ! [[ "${KUBE_USER}" =~ ^[-._@a-zA-Z0-9]+$ ]]; then
      echo "Bad KUBE_USER format."
      exit 1
    fi
  fi

  setup-os-params
  config-ip-firewall
  create-dirs
  setup-kubelet-dir
  ensure-local-ssds
  setup-logrotate
  if [[ "${KUBERNETES_MASTER:-}" == "true" ]]; then
    mount-master-pd
    create-node-pki
    create-master-pki
    create-master-auth
    ensure-master-bootstrap-kubectl-auth
    create-master-kubelet-auth
    create-master-etcd-auth
    create-master-etcd-apiserver-auth
    override-pv-recycler
    create-default-network-template-volume-mount
    gke-master-start
    if [[ "${NETWORK_PROVIDER:-}" == "mizar" ]]; then
      create-kubeproxy-user-kubeconfig
    fi
  else
    create-node-pki
    if [[ "${USE_INSECURE_SCALEOUT_CLUSTER_MODE:-false}" == "true" ]]; then
      create-kubelet-kubeconfig ${KUBERNETES_MASTER_NAME} "8080" "http"
    else
      create-kubelet-kubeconfig ${KUBERNETES_MASTER_NAME}
    fi
    if [[ "${KUBE_PROXY_DAEMONSET:-}" != "true" ]]; then
      create-kubeproxy-user-kubeconfig
    fi
    if [[ "${ENABLE_NODE_PROBLEM_DETECTOR:-}" == "standalone" ]]; then
      create-node-problem-detector-kubeconfig ${KUBERNETES_MASTER_NAME}
    fi
  fi

  override-kubectl
  container_runtime="${CONTAINER_RUNTIME:-docker}"
  # Run the containerized mounter once to pre-cache the container image.
  if [[ "${container_runtime}" == "docker" ]]; then
    assemble-docker-flags
  elif [[ "${container_runtime}" == "containerd" ]]; then
    setup-containerd
  fi
  start-kubelet

  if [[ "${KUBERNETES_MASTER:-}" == "true" ]]; then
    compute-master-manifest-variables
    if [[ -z "${ETCD_SERVERS:-}" ]]; then
      start-etcd-servers
      start-etcd-empty-dir-cleanup-pod

      echo "variable START_ETCD_GRPC_PROXY is ${START_ETCD_GRPC_PROXY:-}"
      if [[ "${START_ETCD_GRPC_PROXY:-}" == "true" ]]; then
        start-etcd-grpc-proxy
      fi
    fi

    start-kube-apiserver
    start-kube-controller-manager
    if [[ "${NETWORK_PROVIDER:-}" == "mizar" ]]; then
      start-kube-proxy
    fi

    if [[ "${ARKTOS_SCALEOUT_SERVER_TYPE:-}" != "rp" ]]; then
      start-kube-scheduler
    fi
    wait-till-apiserver-ready
    # start-workload-controller-manager
    start-kube-addons
    start-cluster-autoscaler
    start-lb-controller
    update-legacy-addon-node-labels &
    apply-encryption-config &
    apply-network-crd 
    if [[ "${SCALEOUT_CLUSTER:-false}" == "false" ]]; then
      if [[ -z "${DISABLE_NETWORK_SERVICE_SUPPORT:-}" ]]; then
        start-arktos-network-controller
      fi
      start-cluster-networking   ####start cluster networking if not using default kubenet
    else
      if [[ "${ARKTOS_SCALEOUT_SERVER_TYPE:-}" == "tp" ]]; then
        if [[ -z "${DISABLE_NETWORK_SERVICE_SUPPORT:-}" ]]; then
          start-arktos-network-controller ${CLUSTER_NAME}
        fi
        start-cluster-networking   ####start cluster networking if not using default kubenet
      fi
    fi

  else
    if [[ "${KUBE_PROXY_DAEMONSET:-}" != "true" ]]; then
      start-kube-proxy
    fi
    if [[ "${ENABLE_NODE_PROBLEM_DETECTOR:-}" == "standalone" ]]; then
      start-node-problem-detector
    fi
  fi
  if [[ "${NETWORK_PROVIDER:-}" == "mizar" ]]; then
    #TODO: This is a hack for arktos runtime hard-coding of /home/kubernetes/bin path. Remove when arktos is fixed.
    until [ -f "/opt/cni/bin/mizarcni" ]; do
      sleep 5
    done
    cp -f "/opt/cni/bin/mizarcni" "/home/kubernetes/bin/"
  fi
  reset-motd
  prepare-mounter-rootfs
  if [[ "${KUBE_GCI_VERSION}" == "cos"* ]]; then
    modprobe configs
  fi
  if [[ "${ENABLE_PPROF_DEBUG:-false}" == "true" ]]; then
    start-collect-pprof &  #### start collect profiling files
  fi
  if [[ "${ENABLE_PROMETHEUS_DEBUG:-false}" == "true" ]]; then
    start-prometheus &  #####start prometheus
  fi 
  ulimit -c unlimited

  echo "Done for the configuration for kubernetes"
}

echo "${@}"
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "${@}"
fi
