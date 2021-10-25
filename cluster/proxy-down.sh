#!/usr/bin/env bash

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

# Delete proxy in Arktos clusters

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

if [ -f "${KUBE_ROOT}/cluster/env.sh" ]; then
    source "${KUBE_ROOT}/cluster/env.sh"
fi

source "${KUBE_ROOT}/cluster/kube-util.sh"

detect-project
detect-subnetworks

if [ -z "${ZONE-}" ]; then
  echo "... Using provider: ${KUBERNETES_PROVIDER}" >&2
else
  echo "... In ${ZONE} using provider ${KUBERNETES_PROVIDER}" >&2
fi

if gcloud compute instances describe "${SCALEOUT_PROXY_NAME}" --zone "${ZONE}" --project "${PROJECT}" &>/dev/null; then
  gcloud compute instances delete \
    --project "${PROJECT}" \
    --quiet \
    --delete-disks all \
    --zone "${ZONE}" \
    "${SCALEOUT_PROXY_NAME}"
fi
# Delete firewall rule for the proxy.
delete-firewall-rules "${SCALEOUT_PROXY_NAME}-https"

if [[ "${ENABLE_PROMETHEUS_DEBUG:-false}" == "true" ]]; then
  delete-firewall-rules "promethues-${SCALEOUT_PROXY_NAME}"
fi

echo "Deleting proxy ${SCALEOUT_PROXY_NAME} reserved IP"
if gcloud compute addresses describe "${SCALEOUT_PROXY_NAME}-ip" --region "${REGION}" --project "${PROJECT}" &>/dev/null; then
  gcloud compute addresses delete \
    --project "${PROJECT}" \
    --region "${REGION}" \
    --quiet \
    "${SCALEOUT_PROXY_NAME}-ip"
fi
if gcloud compute addresses describe "${SCALEOUT_PROXY_NAME}-internalip" --region "${REGION}" --project "${PROJECT}" &>/dev/null; then
  gcloud compute addresses delete \
    --project "${PROJECT}" \
    --region "${REGION}" \
    --quiet \
    "${SCALEOUT_PROXY_NAME}-internalip"
fi

echo "Done. deleted arktos-proxy"

exit 0
