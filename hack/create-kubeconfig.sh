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

# Kube-apiserver service group id
APISERVER_SERVICEGROUPID=${APISERVER_SERVICEGROUPID:-"1"}
CLUSTER_UP_NAME="service_group_${APISERVER_SERVICEGROUPID}"

source "${KUBE_ROOT}/hack/lib/util.sh"

display_usage() {
  echo "The script can be run with the following formats:"
  echo "hack/create-kubeconfig.sh command filepath to create a command of building new kubeconfig filepath from default admin.kubeconfig"
  echo "hack/create-kubeconfig.sh command filepath1 filepath2 to create a command of building a new kubeconfig filepath2 from the specified kubeconfig filepath1"
  echo "hack/create-kubeconfig.sh create server, certificate-authority-data client-certificate-data client-key-data filepath to the command generated to build a new kubeconfig filepath"
}

# Ensure CERT_DIR is created for crt/key and kubeconfig
mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"
CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")
# There are following commands to run the script
# hack/create-kubeconfig.sh is to create a kubeconfig giving the following params
# hack/create-kubeconfig.sh extract key is to extract content from an existing kubeconfig.
if [ $# -lt 1 ] ; then
  display_usage
else
  case $1 in
    extract)
      if [ -n "$2" ]; then
        ${CONTROLPLANE_SUDO} grep $2 ${CERT_DIR}/admin.kubeconfig | awk '{print $2}'
      fi
      ;;
    command)
      filepath=${CERT_DIR}/admin.kubeconfig
      newfilepath=${CERT_DIR}/new.kubeconfig
      if [[ $# -eq 2 ]] ; then
         newfilepath=$2
      elif [[ $# -eq 3 ]] ; then
         filepath=$2
         newfilepath=$3
      else 
         display_usage
         exit 0
      fi
      host="$(${CONTROLPLANE_SUDO} grep server $filepath | awk '{print $2}')"
      ca="$(${CONTROLPLANE_SUDO} grep certificate-authority-data $filepath | awk '{print $2}')"
      cert="$(${CONTROLPLANE_SUDO} grep client-certificate-data $filepath | awk '{print $2}')"
      key="$(${CONTROLPLANE_SUDO} grep client-key-data $filepath | awk '{print $2}')"
      echo "hack/create-kubeconfig.sh create $host $ca $cert $key $newfilepath"
      ;;
    create)
      if [[ $# -ne 6 ]] ; then
        display_usage
      else 
        cat <<EOF |  ${CONTROLPLANE_SUDO} tee -a $6
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: $3  
    server: $2
  name: $CLUSTER_UP_NAME
contexts:
- context:
    cluster: $CLUSTER_UP_NAME
    user: $CLUSTER_UP_NAME
  name: $CLUSTER_UP_NAME
current-context: $CLUSTER_UP_NAME
kind: Config
preferences: {}
users:
- name: $CLUSTER_UP_NAME
  user:
    client-certificate-data: $4
    client-key-data: $5
EOF
     fi
       ;;
    *)
      display_usage
      ;;
  esac
fi

