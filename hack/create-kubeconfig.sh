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

set -o errexit
set -o nounset
set -o pipefail

LOG_DIR=${LOG_DIR:-"/tmp"}
KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CERT_DIR=${CERT_DIR:-"/var/run/kubernetes"}
CERT_ROOTNAME=${CERT_ROOTNAME:-"host"}

source "${KUBE_ROOT}/hack/lib/util.sh"

# Ensure CERT_DIR is created for crt/key and kubeconfig
mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"
CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")

# There are following commands to run the script
# hack/create-kubeconfig.sh is to create a kubeconfig giving the following params
# hack/create-kubeconfig.sh extract key is to extract content from an existing kubeconfig.
if [ $# -lt 1 ] ; then
  if [ -z "${KUBECONFIG_SERVER}" ]; then
    echo "KUBECONFIG_SERVER has not been set"
    exit 1
  fi
  if [ -z "${KUBECONFIG_CA}" ]; then
    echo "KUBECONFIG_CA has not been set"
    exit 1
  fi
  if [ -z "${KUBECONFIG_CERT}" ]; then
    echo "KUBECONFIG_CERT has not been set"
    exit 1
  fi
  if [ -z "${KUBECONFIG_KEY}" ]; then
    echo "KUBECONFIG_KEY has not been set"
    exit 1
  fi
  server=${KUBECONFIG_SERVER}
  protocal="$(cut -d':' -f1 <<<"$server")"
  url="$(cut -d':' -f2<<<"$server")"
  ports="$(cut -d':' -f3<<<"$server")"
  port="$(cut -d'/' -f1<<<"$ports")"

  ${CONTROLPLANE_SUDO}  sh -c 'echo ${KUBECONFIG_CA}   | base64 -d > ${CERT_DIR}/ca-${CERT_ROOTNAME}.crt'
  ${CONTROLPLANE_SUDO}  sh -c 'echo ${KUBECONFIG_CERT} | base64 -d > ${CERT_DIR}/client-${CERT_ROOTNAME}.crt'
  ${CONTROLPLANE_SUDO}  sh -c 'echo ${KUBECONFIG_KEY}  | base64 -d > ${CERT_DIR}/client-${CERT_ROOTNAME}.key'

  kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "ca-${CERT_ROOTNAME}.crt" "$protocal:$url" $port ${CERT_ROOTNAME}

else
  echo "The current operation is $1"
  case $1 in
    extract)
      echo "Here is $2"
      echo "${CERT_DIR}/admin.kubeconfig"
      if [ -n "$2" ]; then
        ${CONTROLPLANE_SUDO} grep $2 ${CERT_DIR}/admin.kubeconfig | awk '{print $2}'
      fi
      ;;
    *)
      echo "Unknown operation"
      ;;
  esac
fi