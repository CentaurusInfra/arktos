#!/usr/bin/env bash


# Copyright 2014 The Kubernetes Authors.
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

# Kube-apiserver partitioned by tenant
APISERVER_PARTITION_TENANT=${APISERVER_PARTITION_TENANT:-"tenant1"}

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

CERT_DIR=${CERT_DIR:-"/var/run/kubernetes"}

KUBECTL=${KUBECTL:-"${KUBE_ROOT}/cluster/kubectl.sh"}


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

function start_apiserver {

  echo "Copying apiserver.config..."

  cp "${KUBE_ROOT}/test/conf/apiserver-${APISERVER_PARTITION_TENANT}.config" "apiserver.config"

  echo "Running alkaid-up.sh..."
  source $(dirname "$0")/alkaid-up.sh
}

"$@"


