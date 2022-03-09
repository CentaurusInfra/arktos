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

echo "DBG: Flannel CNI plugin will be installed AFTER cluster is up"
[ "${CNIPLUGIN}" == "flannel" ] && ARKTOS_NO_CNI_PREINSTALLED="y"

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

# Generate kubeconfig for kubelet
write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "" "${API_HOST}" "${API_PORT}" kubelet "" "http"
${CONTROLPLANE_SUDO} chown "$(whoami)" "${CERT_DIR}/kubelet.kubeconfig"
# Generate kubeconfig for kube-proxy
write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "" "${API_HOST}" "${API_PORT}" kube-proxy "" "http"
${CONTROLPLANE_SUDO} chown "$(whoami)" "${CERT_DIR}/kube-proxy.kubeconfig"

KUBELET_CLIENTCA=${KUBELET_CLIENTCA:-"${CERT_DIR}/client-ca.crt"}
${CONTROLPLANE_SUDO} chown "$(whoami)" "${KUBELET_CLIENTCA}"

HOSTNAME_OVERRIDE=${HOSTNAME_OVERRIDE:-"$(hostname)"}
CLUSTER_DNS=${CLUSTER_DNS:-"10.0.0.10"}
FEATURE_GATES="${FEATURE_GATES_COMMON_BASE}"
if [ -z ${DISABLE_NETWORK_SERVICE_SUPPORT} ]; then
  FEATURE_GATES="${FEATURE_GATES_COMMON_BASE},MandatoryArktosNetwork=true"
fi

if [[ ! -s "${KUBELET_CLIENTCA}" ]]; then
    echo "kubelet client ca cert not found at ${KUBELET_CLIENTCA}."
    echo "Please copy this file from Arktos master, e.g. in GCP run:"
    echo "gcloud compute scp <master-vm-node>:/var/run/kubernetes/client-ca.crt /tmp/arktos/"
    die "arktos worker node failed to start."
fi

make all WHAT="cmd/hyperkube cmd/kubelet cmd/kube-proxy"

# Check if the kubelet is still running
echo "DBG: Cleaning old processes for kubelet"
KUBELET_PID=`ps -ef |grep kubelet |grep -v grep |head -1 |awk '{print $2}'`
[[ -n "${KUBELET_PID-}" ]] && mapfile -t KUBELET_PIDS < <(pgrep -P "${KUBELET_PID}"; ps -o pid= -p "${KUBELET_PID}")
[[ -n "${KUBELET_PIDS-}" ]] && sudo kill "${KUBELET_PIDS[@]}" 2>/dev/null

# Check if the proxy is still running
echo "DBG: Cleaning old processes for kube-proxy"
PROXY_PID=`ps -ef |grep kube-proxy |grep -v grep |head -1 |awk '{print $2}'`
[[ -n "${PROXY_PID-}" ]] && mapfile -t PROXY_PIDS < <(pgrep -P "${PROXY_PID}"; ps -o pid= -p "${PROXY_PID}")
[[ -n "${PROXY_PIDS-}" ]] && sudo kill "${PROXY_PIDS[@]}" 2>/dev/null

# Check if the flannel is still running
echo "DBG: Cleaning old processes for flannel"
FLANNELD_PID=`ps -ef |grep flannel |grep -v grep |head -1 |awk '{print $2}'`
[[ -n "${FLANNELD_PID-}" ]] && mapfile -t FLANNELD_PIDS < <(pgrep -P "${FLANNELD_PID}"; ps -o pid= -p "${FLANNELD_PID}") 
[[ -n "${FLANNELD_PIDS-}" ]] && sudo kill "${FLANNELD_PIDS[@]}" 2>/dev/null


PROXY_TP_KUBECONFIGS=""

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
  --feature-gates=${FEATURE_GATES} \
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
  if  [[ -z "${API_TENANT_SERVER}" ]]; then
    echo ERROR: Please set API_TENANT_SERVER. For example: API_TENANT_SERVER=192.168.0.2 or API_TENANT_SERVER=192.168.0.2,192.168.0.5
    exit 1
  else
    KUBELET_TENANT_SERVER_KUBECONFIG_FLAG="--tenant-server-kubeconfig="
    kubeconfig_filename="tenant-server-kubelet"

    API_TENANT_SERVERS=(${API_TENANT_SERVER//,/ })
    serverCount=${#API_TENANT_SERVERS[@]}
    for (( pos=0; pos<${serverCount}; pos++ ));
    do
      # generate kubeconfig fpr tenant partitions
      write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "" "${API_TENANT_SERVERS[${pos}]}" "${API_PORT}" ${kubeconfig_filename} "" "http"
      ${CONTROLPLANE_SUDO} mv "${CERT_DIR}/${kubeconfig_filename}.kubeconfig" "${CERT_DIR}/${kubeconfig_filename}${pos}.kubeconfig"
      ${CONTROLPLANE_SUDO} chown "$(whoami)" "${CERT_DIR}/${kubeconfig_filename}${pos}.kubeconfig"

      KUBELET_TENANT_SERVER_KUBECONFIG_FLAG="${KUBELET_TENANT_SERVER_KUBECONFIG_FLAG}${CERT_DIR}/${kubeconfig_filename}${pos}.kubeconfig,"
      PROXY_TP_KUBECONFIGS="${PROXY_TP_KUBECONFIGS}${CERT_DIR}/${kubeconfig_filename}${pos}.kubeconfig,"
    done
    KUBELET_TENANT_SERVER_KUBECONFIG_FLAG=${KUBELET_TENANT_SERVER_KUBECONFIG_FLAG::-1}
    PROXY_TP_KUBECONFIGS=${PROXY_TP_KUBECONFIGS::-1}
  fi

  sudo ./_output/local/bin/linux/amd64/hyperkube kubelet \
  --v="${LOG_LEVEL}" \
  --container-runtime=remote \
  --hostname-override=${HOSTNAME_OVERRIDE} \
  --address='0.0.0.0' \
  --kubeconfig="${CERT_DIR}/kubelet.kubeconfig" \
  ${KUBELET_TENANT_SERVER_KUBECONFIG_FLAG} \
  --authorization-mode=Webhook \
  --authentication-token-webhook \
  --client-ca-file=${KUBELET_CLIENTCA} \
  --feature-gates=${FEATURE_GATES} \
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

if [ "${CNIPLUGIN}" == "flannel" ]; then
    echo "DBG: Installing Flannel cni plugin..."
    sleep 30  #need sometime for KCM to be fully functioning
    if [ "${IS_SCALE_OUT}" == "true" ]; then
       install_flannel "${RESOURCE_PARTITION_POD_CIDR}" "${API_HOST}"
    else
       install_flannel "${KUBE_CONTROLLER_MANAGER_CLUSTER_CIDR}" "${API_HOST}"
    fi
fi

# Start proxy
cat <<EOF > /tmp/kube-proxy.yaml
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
clientConnection:
  kubeconfig: ${CERT_DIR}/kube-proxy.kubeconfig
tenantPartitionKubeConfig: ${PROXY_TP_KUBECONFIGS}
hostnameOverride: ${HOSTNAME_OVERRIDE}
mode: ${KUBE_PROXY_MODE}
EOF

if [[ -n ${FEATURE_GATES} ]]; then
  echo "featureGates:"
  # Convert from foo=true,bar=false to
  #   foo: true
  #   bar: false
  for gate in $(echo "${FEATURE_GATES}" | tr ',' ' '); do
    echo "${gate}" | sed -e 's/\(.*\)=\(.*\)/  \1: \2/'
  done
fi >>/tmp/kube-proxy.yaml

sudo ./_output/local/bin/linux/amd64/hyperkube kube-proxy \
  --v="${LOG_LEVEL}" \
  --config=/tmp/kube-proxy.yaml \
  --master="http://${API_HOST}:${API_PORT}" > "${LOG_DIR}/kube-proxy.log" 2>&1 &

echo "kube proxy has been started. please check /tmp/kube-proxy.log for its running log."
