#!/usr/bin/env bash

# Copyright 2014 The Kubernetes Authors.
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

# Bring up a Kubernetes cluster.
#
# If the full release name (gs://<bucket>/<release>) is passed in then we take
# that directly.  If not then we assume we are doing development stuff and take
# the defaults in the release config.

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

if [ -f "${KUBE_ROOT}/cluster/env.sh" ]; then
    source "${KUBE_ROOT}/cluster/env.sh"
fi

source "${KUBE_ROOT}/cluster/kube-util.sh"

export RESOURCE_DIRECTORY=${RESOURCE_DIRECTORY:-"${KUBE_ROOT}/cluster"}
export SHARED_CA_DIRECTORY=${SHARED_CA_DIRECTORY:-"/tmp/shared_ca"}

if [ -z "${ZONE-}" ]; then
  echo "... Starting cluster using provider: ${KUBERNETES_PROVIDER}" >&2
else
  echo "... Starting cluster in ${ZONE} using provider ${KUBERNETES_PROVIDER}" >&2
fi

echo "... calling verify-prereqs" >&2
verify-prereqs
echo "... calling verify-kube-binaries" >&2
verify-kube-binaries
echo "... calling verify-release-tars" >&2
verify-release-tars

echo "... calling kube-up" >&2
kube-up

if [[ "${PRESET_INSTANCES_ENABLED:-}" == $TRUE && "${IS_PRESET_INSTANCES_DRY_RUN:-}" == $TRUE ]]; then
  echo "Dry run of kube-up completed"
  exit 0
fi

#validate-cluster-status

if [[ "${ENABLE_PROXY:-}" == "true" ]]; then
  # shellcheck disable=SC1091
  . /tmp/kube-proxy-env
  echo ""
  echo "*** Please run the following to add the kube-apiserver endpoint to your proxy white-list ***"
  cat /tmp/kube-proxy-env
  echo "***                                                                                      ***"
  echo ""
fi

if [[ -d "${KUBE_ROOT}/partitionserver-config" ]]; then
  rm -r ${KUBE_ROOT}/partitionserver-config
fi
mkdir ${KUBE_ROOT}/partitionserver-config
if [[ "${APISERVERS_EXTRA_NUM:-0}" -gt "0" ]]; then
  echo "... configing apiserver datapartition" >&2
  for (( num=0; num<=${APISERVERS_EXTRA_NUM:-0}; num++ )); do
    APISERVER_RANGESTART=${APISERVER_RANGESTART:-"${APISERVER_DATAPARTITION_CONFIG:0:1}"}
    APISERVER_RANGEEND=${APISERVER_RANGEEND:-"${APISERVER_DATAPARTITION_CONFIG:$(( ${#APISERVER_DATAPARTITION_CONFIG}-1 )):1}"}
    APISERVER_ISRANGESTART_VALID=${APISERVER_ISRANGESTART_VALID:-false}
    APISERVER_ISRANGEEND_VALID=${APISERVER_ISRANGEEND_VALID:-false}
    set-apiserver-datapartition $num
    create-apiserver-datapartition-yml $num
    config-apiserver-datapartition $num
  done
fi 

if [[ "${ETCD_EXTRA_NUM:-0}" -gt "0" ]]; then
  echo "... applying etcd servers storagecluster" >&2
  config-etcd-storagecluster
fi

if [[ "${KUBERNETES_PROVIDER:-gce}" == "aws" ]]; then
  echo "Kubernetes master AWS Internal IP is ${MASTER_INTERNAL_IP}"
fi

if [[ "${SCALEOUT_CLUSTER:-false}" == "true" ]]; then
  # proxy setup expects a valid RP kubeconfig file to get master IP from;
    # rp-1 should be safe to assume here
  export ARKTOS_SCALEOUT_SERVER_TYPE="proxy" 
  KUBEMARK_CLUSTER_KUBECONFIG="${RESOURCE_DIRECTORY}/kubeconfig${KUBEMARK_PREFIX}.rp-1"
  setup_proxy
fi

exit 0
