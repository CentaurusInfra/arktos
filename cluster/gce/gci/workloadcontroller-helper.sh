#!/usr/bin/env bash

# Copyright 2020 Authors of Arktos - file created.
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

# A library of helper functions and constant for GCI distro
source "${KUBE_ROOT}/cluster/gce/gci/helper.sh"

# create-workloadcontroller-instance creates the workloadcontroller instance. If called with

#
# It requires a whole slew of assumed variables, partially due to to
# the call to write-apiservr-env. Listing them would be rather
# futile. Instead, we list the required calls to ensure any additional
#
# variables are set:
#   ensure-temp-dir
#   detect-project
#   get-bearer-token
function create-workloadcontroller-instance {
  local name=""
  [[ -n ${1:-} ]] && name="${1}"
  write-workloadcontroller-env
  ensure-gci-metadata-files
  create-workloadcontroller-instance-internal "${name}"
}

function create-workloadcontroller-instance-internal() {
  local gcloud="gcloud"
  local retries=5
  local sleep_sec=10
  if [[ "${MASTER_SIZE##*-}" -ge 64 ]]; then  # remove everything up to last dash (inclusive)
    # Workaround for #55777
    retries=30
    sleep_sec=60
  fi

  local -r server_name="${1}"

  local enable_ip_aliases
  if [[ "${NODE_IPAM_MODE:-}" == "CloudAllocator" ]]; then
    enable_ip_aliases=true
  else
    enable_ip_aliases=false
  fi

  local network=$(make-gcloud-network-argument \
    "${NETWORK_PROJECT}" "${REGION}" "${NETWORK}" "${SUBNETWORK:-}" \
    "" "" "${enable_ip_aliases:-}" "${IP_ALIAS_SIZE:-}")

  local metadata="kube-env=${KUBE_TEMP}/master-kube-env.yaml"
  metadata="${metadata},kubelet-config=${KUBE_TEMP}/master-kubelet-config.yaml"
  metadata="${metadata},user-data=${KUBE_ROOT}/cluster/gce/gci/workloadcontroller.yaml"
  metadata="${metadata},configure-sh=${KUBE_ROOT}/cluster/gce/gci/configure.sh"
  metadata="${metadata},apiserver-config=${KUBE_ROOT}/hack/apiserver.config"
  metadata="${metadata},cluster-location=${KUBE_TEMP}/cluster-location.txt"
  metadata="${metadata},cluster-name=${KUBE_TEMP}/cluster-name.txt"
  metadata="${metadata},gci-update-strategy=${KUBE_TEMP}/gci-update.txt"
  metadata="${metadata},gci-ensure-gke-docker=${KUBE_TEMP}/gci-ensure-gke-docker.txt"
  metadata="${metadata},gci-docker-version=${KUBE_TEMP}/gci-docker-version.txt"
  metadata="${metadata},kube-master-certs=${KUBE_TEMP}/kube-master-certs.yaml"
  metadata="${metadata},cluster-location=${KUBE_TEMP}/cluster-location.txt"
  metadata="${metadata},controllerconfig=${KUBE_TEMP}/controllerconfig.json"
  metadata="${metadata},${MASTER_EXTRA_METADATA}"

  local disk="name=${server_name}-pd"
  disk="${disk},device-name=server-pd"
  disk="${disk},mode=rw"
  disk="${disk},boot=no"
  disk="${disk},auto-delete=no"

  for attempt in $(seq 1 ${retries}); do
    if result=$(${gcloud} compute instances create "${server_name}" \
      --project "${PROJECT}" \
      --zone "${ZONE}" \
      --machine-type "${WORKLOADCONTROLLER_SIZE}" \
      --image-project="${WORKLOADCONTROLLER_IMAGE_PROJECT:-${MASTER_IMAGE_PROJECT}}" \
      --image "${WORKLOADCONTROLLER_IMAGE:-${MASTER_IMAGE}}" \
      --tags "${server_name}" \
      --scopes "storage-ro,compute-rw,monitoring,logging-write" \
      --metadata-from-file "${metadata}" \
      --disk "${disk}" \
      --boot-disk-size "${WORKLOADCONTROLLER_ROOT_DISK_SIZE}" \
      ${WORKLOADCONTROLLER_MIN_CPU_ARCHITECTURE:+"--min-cpu-platform=${WORKLOADCONTROLLER_MIN_CPU_ARCHITECTURE}"} \
      ${network} 2>&1); then
      echo "${result}" >&2
      return 0
    else
      echo "${result}" >&2
      if [[ ! "${result}" =~ "try again later" ]]; then
        echo "Failed to create workload controller instance due to non-retryable error" >&2
        return 1
      fi
      sleep $sleep_sec
    fi
  done

  echo "Failed to create workload controller instance despite ${retries} attempts" >&2
  return 1
}