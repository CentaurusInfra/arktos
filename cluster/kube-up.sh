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

echo "... calling validate-cluster" >&2
# Override errexit
(validate-cluster) && validate_result="$?" || validate_result="$?"

# We have two different failure modes from validate cluster:
# - 1: fatal error - cluster won't be working correctly
# - 2: weak error - something went wrong, but cluster probably will be working correctly
# We just print an error message in case 2).
if [[ "${validate_result}" == "1" ]]; then
	exit 1
elif [[ "${validate_result}" == "2" ]]; then
	echo "...ignoring non-fatal errors in validate-cluster" >&2
fi

if [[ "${ENABLE_PROXY:-}" == "true" ]]; then
  # shellcheck disable=SC1091
  . /tmp/kube-proxy-env
  echo ""
  echo "*** Please run the following to add the kube-apiserver endpoint to your proxy white-list ***"
  cat /tmp/kube-proxy-env
  echo "***                                                                                      ***"
  echo ""
fi

if [[ "${APISERVERS_EXTRA_NUM:-0}" -gt "0" ]]; then
  echo "... configing apiserver datapartition" >&2
  if [[ -d "${KUBE_ROOT}/apiserverdatapartition" ]]; then
    rm -r ${KUBE_ROOT}/apiserverdatapartition
  fi
  mkdir ${KUBE_ROOT}/apiserverdatapartition
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

echo -e "\nDone, listing cluster services:\n" >&2
"${KUBE_ROOT}/cluster/kubectl.sh" cluster-info
echo

if [[ "${KUBERNETES_PROVIDER:-gce}" == "aws" ]]; then
  echo "Kubernetes master AWS Internal IP is ${MASTER_INTERNAL_IP}"
fi

exit 0
