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

set -e
die() { echo "$*" 1>&2 ; exit 1; }

# install cni plugin related packages
KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source ${KUBE_ROOT}/hack/lib/common-var-init.sh
source ${KUBE_ROOT}/hack/arktos-cni.rc

# creates a self-contained kubeconfig: args are sudo, dest-dir, ca file, host, port, client id, token(optional)
function write_client_kubeconfig {
    local sudo=$1
    local dest_dir=$2
    local ca_file=$3
    local api_host=$4
    local api_port=$5
    local client_id=$6
    local token=${7:-}
    local protocol=${8:-"http"}

    cat <<EOF | ${sudo} tee "${dest_dir}"/"${client_id}".kubeconfig > /dev/null
apiVersion: v1
kind: Config
clusters:
  - cluster:
      server: ${protocol}://${api_host}:${api_port}/
    name: local-up-cluster
users:
  - user:
    name: local-up-cluster
contexts:
  - context:
      cluster: local-up-cluster
      user: local-up-cluster
    name: local-up-cluster
current-context: local-up-cluster
EOF
}


# Ensure CERT_DIR is created for auto-generated kubeconfig
mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"
CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")

# Generate kubeconfig
write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "" "${API_HOST}" "${API_PORT}" kubelet "" "http"
${CONTROLPLANE_SUDO} chown "$(whoami)" "${CERT_DIR}/kubelet.kubeconfig"

KUBELET_CLIENTCA=${KUBELET_CLIENTCA:-"${CERT_DIR}/client-ca.crt"}
${CONTROLPLANE_SUDO} chown "$(whoami)" "${KUBELET_CLIENTCA}"

HOSTNAME_OVERRIDE=${HOSTNAME_OVERRIDE:-"$(hostname)"}
CLUSTER_DNS=${CLUSTER_DNS:-"10.0.0.10"}

if [[ ! -s "${KUBELET_CLIENTCA}" ]]; then
    echo "kubelet client ca cert not found at ${KUBELET_CLIENTCA}."
    echo "Please copy this file from Arktos master, e.g. in GCP run:"
    echo "gcloud compute scp <master-vm-node>:/var/run/kubernetes/client-ca.crt /tmp/arktos/"
    die "arktos worker node failed to start."
fi

make all WHAT=cmd/hyperkube

if [[ "${IS_SCALE_OUT}" != "true" ]]; then
echo "IS_SCALE_OUT false"

sudo ./_output/local/bin/linux/amd64/hyperkube kubelet \
--v="${LOG_LEVEL}" \
--container-runtime=remote \
--hostname-override=${HOSTNAME_OVERRIDE} \
--address='0.0.0.0' \
--kubeconfig="${CERT_DIR}/kubelet.kubeconfig" \
--authorization-mode=Webhook \
--authentication-token-webhook \
--client-ca-file=${KUBELET_CLIENTCA} \
--feature-gates=AllAlpha=false \
--cpu-cfs-quota=true \
--enable-controller-attach-detach=true \
--cgroups-per-qos=true \
--cgroup-driver= --cgroup-root= \
--cluster-dns=${CLUSTER_DNS} \
--cluster-domain=cluster.local \
--container-runtime-endpoint="containerRuntime,container,/run/containerd/containerd.sock;vmRuntime,vm,/run/virtlet.sock" \
--runtime-request-timeout=2m \
--port=10250 \
--resolv-conf=/run/systemd/resolve/resolv.conf \
> /tmp/kubelet.worker.log 2>&1 &

else

echo "IS_SCALE_OUT true"
# generate kubeconfig fpr tenant partition
write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "" "${API_TENANT_SERVER}" "${API_PORT}" "tenant-server-kubelet0" "" "http"
${CONTROLPLANE_SUDO} chown "$(whoami)" "${CERT_DIR}/tenant-server-kubelet0.kubeconfig"

sudo ./_output/local/bin/linux/amd64/hyperkube kubelet \
--v="${LOG_LEVEL}" \
--container-runtime=remote \
--hostname-override=${HOSTNAME_OVERRIDE} \
--address='0.0.0.0' \
--kubeconfig="${CERT_DIR}/kubelet.kubeconfig" \
--tenant-server-kubeconfig="${CERT_DIR}/tenant-server-kubelet0.kubeconfig" \
--authorization-mode=Webhook \
--authentication-token-webhook \
--client-ca-file=${KUBELET_CLIENTCA} \
--feature-gates=AllAlpha=false \
--cpu-cfs-quota=true \
--enable-controller-attach-detach=true \
--cgroups-per-qos=true \
--cgroup-driver= --cgroup-root= \
--cluster-dns=${CLUSTER_DNS} \
--cluster-domain=cluster.local \
--container-runtime-endpoint="containerRuntime,container,/run/containerd/containerd.sock;vmRuntime,vm,/run/virtlet.sock" \
--runtime-request-timeout=2m \
--port=10250 \
--resolv-conf=/run/systemd/resolve/resolv.conf \
> /tmp/kubelet.worker.log 2>&1 &

fi

echo "kubelet has been started. please check /tmp/kubelet.worker.log for its running log."

