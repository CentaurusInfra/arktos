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

# Kube-apiserver partitioned by tenant

BINARY_DIR=${BINARY_DIR:-}

source ${KUBE_ROOT}/hack/arktos-cni.rc

source "${KUBE_ROOT}/hack/lib/init.sh"

source "${KUBE_ROOT}/hack/lib/common.sh"

kube::util::test_openssl_installed
kube::util::ensure-cfssl

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

function build_binary {
   exists="false"
   if [[ -n ${BINARY_DIR} ]] ; then
       exists="true"
       for file in "$@"; do
           echo "The file is $file"
           if [[ -f ${BINARY_DIR}$file ]] ; then
               echo "${BINARY_DIR}$file does exist."
           else
               echo "${BINARY_DIR}$file does not exist."
               exists="false"
               break
           fi
       done
       if  [ "$exists" == "true" ]; then
           echo "GO_OUT has to be set"
           GO_OUT=${BINARY_DIR}
       fi
   fi
   if [ "$exists" == "false" ]; then
       echo "starting to build binaries"
       kube::common::build
   fi
   ### IF the user didn't supply an output/ for the build... Then we detect.
   if [ "${GO_OUT}"=="" ]; then
      kube::common::detect_binary
   fi
}

function start_apiserver {
  kill_process kube-apiserver
  # Ensure CERT_DIR is created for auto-generated crt/key and kubeconfig
  mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"

  # install cni plugin based on env var CNIPLUGIN (bridge, alktron)
  kube::util::ensure-gnu-sed

  build_binary kube-apiserver hyperkube
  kube::common::set_service_accounts
  echo "starting apiserver"
  if [ $# -gt 0 ] ; then
    ETCD_HOST=$1
  fi
  kube::common::start_apiserver 0

}

function start_workload_controller_manager {
  kill_process workload-controller-manager
  # Ensure CERT_DIR is created for auto-generated crt/key and kubeconfig
  mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"

  # install cni plugin based on env var CNIPLUGIN (bridge, alktron)
  kube::util::ensure-gnu-sed

  build_binary workload-controller-manager hyperkube
  kube::common::set_service_accounts

  ### IF the user didn't supply an output/ for the build... Then we detect.
  if [ "${GO_OUT}" == "" ]; then
    kube::common::detect_binary
  fi
  echo "starting workload controller managers"
  kube::common::start_workload_controller_manager $@

}

function start_kube_controller_manager {
  kill_process kube-controller-manager
  # Ensure CERT_DIR is created for auto-generated crt/key and kubeconfig
  mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"

  # install cni plugin based on env var CNIPLUGIN (bridge, alktron)
  kube::util::ensure-gnu-sed

  build_binary kube-controller-manager hyperkube
  kube::common::set_service_accounts

  ### IF the user didn't supply an output/ for the build... Then we detect.
  if [ "${GO_OUT}" == "" ]; then
    kube::common::detect_binary
  fi
  echo "starting kube controller managers"
  kube::common::start_controller_manager $@

}

function start_kube_scheduler {
  kill_process kube-scheduler
  # Ensure CERT_DIR is created for auto-generated crt/key and kubeconfig
  mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"

  # install cni plugin based on env var CNIPLUGIN (bridge, alktron)
  kube::util::ensure-gnu-sed

  build_binary kube-scheduler hyperkube
  kube::common::set_service_accounts

  ### IF the user didn't supply an output/ for the build... Then we detect.
  if [ "${GO_OUT}" == "" ]; then
    kube::common::detect_binary
  fi
  echo "starting scheduler"
  kube::common::start_kubescheduler $@

}

"$@"



