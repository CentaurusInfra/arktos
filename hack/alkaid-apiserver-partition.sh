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

function detect_binary {
    # Detect the OS name/arch so that we can find our binary
    case "$(uname -s)" in
      Darwin)
        host_os=darwin
        ;;
      Linux)
        host_os=linux
        ;;
      *)
        echo "Unsupported host OS.  Must be Linux or Mac OS X." >&2
        exit 1
        ;;
    esac

    case "$(uname -m)" in
      x86_64*)
        host_arch=amd64
        ;;
      i?86_64*)
        host_arch=amd64
        ;;
      amd64*)
        host_arch=amd64
        ;;
      aarch64*)
        host_arch=arm64
        ;;
      arm64*)
        host_arch=arm64
        ;;
      arm*)
        host_arch=arm
        ;;
      i?86*)
        host_arch=x86
        ;;
      s390x*)
        host_arch=s390x
        ;;
      ppc64le*)
        host_arch=ppc64le
        ;;
      *)
        echo "Unsupported host arch. Must be x86_64, 386, arm, arm64, s390x or ppc64le." >&2
        exit 1
        ;;
    esac

   GO_OUT="${KUBE_ROOT}/_output/local/bin/${host_os}/${host_arch}"
}

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

  if [ "${GO_OUT}" == "" ]; then
    detect_binary
  fi

  echo "Copying apiserver.config..."

  cp "${KUBE_ROOT}/test/conf/apiserver-${APISERVER_PARTITION_TENANT}.config" "${GO_OUT}/apiserver.config"

  source $(dirname "$0")/alkaid-up.sh
}

"$@"

