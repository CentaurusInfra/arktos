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

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

source "${KUBE_ROOT}/hack/lib/common-var-init.sh"

BINARY_DIR=${BINARY_DIR:-}

source ${KUBE_ROOT}/hack/arktos-cni.rc

source "${KUBE_ROOT}/hack/lib/init.sh"

source "${KUBE_ROOT}/hack/lib/common.sh"

kube::util::test_openssl_installed
kube::util::ensure-cfssl

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
  make -C "${KUBE_ROOT}" WHAT="cmd/hyperkube cmd/kube-apiserver"
else
  echo "skipped the build."
fi

### IF the user didn't supply an output/ for the build... Then we detect.
if [ "${GO_OUT}" == "" ]; then
  kube::common::detect_binary
fi

function apply_test_data {
  echo "Creating apiserver partion test data"
  # use kubectl to create the dashboard
  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/test/yaml/apiserver_partition/${APISERVER_PARTITION_TENANT}.yaml"
  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/test/yaml/apiserver_partition/${APISERVER_PARTITION_TENANT}-ns-a.yaml"
  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/test/yaml/apiserver_partition/${APISERVER_PARTITION_TENANT}-ns-b.yaml"
  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/test/yaml/apiserver_partition/${APISERVER_PARTITION_TENANT}-ns-a-deployment.yaml"
  ${KUBECTL} --kubeconfig="${CERT_DIR}/admin.kubeconfig" apply -f "${KUBE_ROOT}/test/yaml/apiserver_partition/${APISERVER_PARTITION_TENANT}-ns-b-deployment.yaml"
  echo "Apiserver partion test data have been successfully applied."
}
function kill_process {
  if [[ $# -eq 0 ]] ; then
     echo "The process to kill is not specified."
  else
     for process_name in "$@"; do
        process=$(ps aux|grep " ${process_name} "| wc -l)

        if [[ $process > 1 ]]; then
           process=$(ps aux|grep " ${process_name} "| grep arktos |awk '{print $2}')
           for i in ${process[@]}; do
               sudo kill -9 $i
           done
        fi
     done
  fi
}

function start_apiserver {
  kill_process kube-apiserver
  # Ensure CERT_DIR is created for auto-generated crt/key and kubeconfig
  mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"

  # install cni plugin based on env var CNIPLUGIN (bridge, alktron)
  kube::util::ensure-gnu-sed

  kube::common::set_service_accounts

  echo "starting apiserver"
  if [ $# -eq 2 ] ; then
    ETCD_HOST=$2
  fi
  if [ $# -eq 3 ] ; then
    ETCD_HOST=$3
  fi

  kube::common::start_apiserver 0

}

start_apiserver $@



