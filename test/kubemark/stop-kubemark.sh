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

# Script that destroys Kubemark cluster and deletes all master resources.

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/../..

source "${KUBE_ROOT}/test/kubemark/skeleton/util.sh"
source "${KUBE_ROOT}/test/kubemark/cloud-provider-config.sh"
source "${KUBE_ROOT}/test/kubemark/${CLOUD_PROVIDER}/util.sh"
source "${KUBE_ROOT}/cluster/kubemark/${CLOUD_PROVIDER}/config-default.sh"

if [[ -f "${KUBE_ROOT}/test/kubemark/${CLOUD_PROVIDER}/shutdown.sh" ]] ; then
  source "${KUBE_ROOT}/test/kubemark/${CLOUD_PROVIDER}/shutdown.sh"
fi

source "${KUBE_ROOT}/cluster/kubemark/util.sh"

KUBECTL="${KUBE_ROOT}/cluster/kubectl.sh"
KUBEMARK_DIRECTORY="${KUBE_ROOT}/test/kubemark"
RESOURCE_DIRECTORY="${KUBEMARK_DIRECTORY}/resources"

detect-project &> /dev/null

"${KUBECTL}" delete -f "${RESOURCE_DIRECTORY}/addons" &> /dev/null || true
"${KUBECTL}" delete -f "${RESOURCE_DIRECTORY}/hollow-node.yaml" &> /dev/null || true
"${KUBECTL}" delete -f "${RESOURCE_DIRECTORY}/kubemark-ns.json" &> /dev/null || true

rm -rf "${RESOURCE_DIRECTORY}/addons" \
	"${RESOURCE_DIRECTORY}/kubeconfig.kubemark" \
    "${RESOURCE_DIRECTORY}/hollow-node.yaml"  &> /dev/null || true

if [[ "${SCALEOUT_CLUSTER:-false}" == "true" ]]; then
  export ENABLE_APISERVER_INSECURE_PORT=true
  export KUBERNETES_TENANT_PARTITION=true
  for (( tp_num=1; tp_num<=${SCALEOUT_TP_COUNT}; tp_num++ ))
  do
    export TENANT_PARTITION_SEQUENCE=${tp_num}
    delete-kubemark-master
  done


  export KUBERNETES_TENANT_PARTITION=false
  export KUBERNETES_RESOURCE_PARTITION=true
  export KUBERNETES_SCALEOUT_PROXY=true
  delete-kubemark-master
  rm -rf ${RESOURCE_DIRECTORY}/kubeconfig.kubemark-proxy
  rm -rf ${RESOURCE_DIRECTORY}/kubeconfig.kubemark-rp
  rm -rf ${RESOURCE_DIRECTORY}/kubeconfig.kubemark-tp-*
else
  delete-kubemark-master
fi
