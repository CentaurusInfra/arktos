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

if [[ ! -n "${KUBELET_IP}" ]]; then
    die "KUBELET_IP env var not set"
fi

echo "DBG: Flannel CNI plugin will be installed AFTER cluster is up"
[ "${CNIPLUGIN}" == "flannel" ] && ARKTOS_NO_CNI_PREINSTALLED="y"

# install cni plugin related packages
KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

IS_SCALE_OUT=${IS_SCALE_OUT:-"false"}
source "${KUBE_ROOT}/hack/lib/common-var-init.sh"

source ${KUBE_ROOT}/hack/arktos-cni.rc

mkdir -p /tmp/arktos

SECRET_FOLDER=${SECRET_FOLDER:-"/tmp/arktos"}
KUBELET_KUBECONFIG=${KUBELET_KUBECONFIG:-"${SECRET_FOLDER}/kubelet.kubeconfig"}
KUBELET_CLIENTCA=${KUBELET_CLIENTCA:-"${SECRET_FOLDER}/client-ca.crt"}
HOSTNAME_OVERRIDE=${HOSTNAME_OVERRIDE:-"$(hostname)"}
CLUSTER_DNS=${CLUSTER_DNS:-"10.0.0.10"}
FEATURE_GATES="${FEATURE_GATES_UP_OUT},MandatoryArktosNetwork=true"
KUBE_PROXY_KUBECONFIG=${KUBE_PROXY_KUBECONFIG:-"${SECRET_FOLDER}/kube-proxy.kubeconfig"}

if [[ ! -s "${KUBELET_KUBECONFIG}" ]]; then
    echo "kubelet kubeconfig file not found at ${KUBELET_KUBECONFIG}."
    echo "Please copy this file from Arktos master, e.g. in GCP run:"
    echo "gcloud compute scp <master-vm-name>:/var/run/kubernetes/kubelet.kubeconfig /tmp/arktos/"
    die "arktos worker node failed to start."
fi

if [[ ! -s "${KUBELET_CLIENTCA}" ]]; then
    echo "kubelet client ca cert not found at ${KUBELET_CLIENTCA}."
    echo "Please copy this file from Arktos master, e.g. in GCP run:"
    echo "gcloud compute scp <master-vm-node>:/var/run/kubernetes/client-ca.crt /tmp/arktos/"
    die "arktos worker node failed to start."
fi

if [[ ! -s "${KUBE_PROXY_KUBECONFIG}" ]]; then
    echo "kube-proxy client ca cert not found at ${KUBE_PROXY_KUBECONFIG}."
    echo "Please copy this file from Arktos master, e.g. in GCP run:"
    echo "gcloud compute scp <master-vm-node>:/var/run/kubernetes/kube-proxy.kubeconfig /tmp/arktos/"
    die "arktos worker node failed to start."
fi

make all WHAT=cmd/hyperkube

echo "Starting kubelet ....."
KUBELET_FLAGS="--tenant-server-kubeconfig="
if [[ "${IS_SCALE_OUT}" == "true" ]] && [ "${IS_RESOURCE_PARTITION}" == "true" ]; then
  kubeconfig_filename="tenant-server-kubelet"
  serverCount=`ls -al ${SECRET_FOLDER}/${kubeconfig_filename}*.kubeconfig |wc -l`
  if [ "$serverCount" -eq 0 ]; then
    echo "The kubelet kubeconfig file for scale-out not found under directory ${SECRET_FOLDER}."
    echo "Please copy this file from Arktos RP master, e.g. in GCP run:"
    echo "gcloud compute scp <master-rp-name>:/var/run/kubernetes/tenant-server-kubelet*.kubeconfig /tmp/arktos/"
    die "Arktos scale-out RP worker node failed to start."
  fi
 
  for (( pos=0; pos<${serverCount}; pos++ ));
  do
    KUBELET_FLAGS="${KUBELET_FLAGS}${SECRET_FOLDER}/${kubeconfig_filename}${pos}.kubeconfig,"
  done
  KUBELET_FLAGS=${KUBELET_FLAGS::-1}
fi

KUBELET_LOG="/tmp/kubelet.worker.log"
sudo ./_output/local/bin/linux/amd64/hyperkube kubelet \
--v=3 \
--container-runtime=remote \
--hostname-override=${HOSTNAME_OVERRIDE} \
--address=${KUBELET_IP} \
--kubeconfig=${KUBELET_KUBECONFIG} \
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
${KUBELET_FLAGS} \
> ${KUBELET_LOG} 2>&1 &

echo "kubelet has been started. please check ${KUBELET_LOG} for its running log."

if [ "${CNIPLUGIN}" == "flannel" ]; then
    echo "Installing Flannel cni plugin..."
    sleep 30  #need sometime for KCM to be fully functioning
    install_flannel
fi

echo "Starting kube-proxy ....."
## in scale-out env, we need to get hold of kubeconfig files for all TPs
## only applicable to node of a RP
TP_KUBEONFIGS=""
if [[ "${IS_SCALE_OUT}" == "true" ]] && [ "${IS_RESOURCE_PARTITION}" == "true" ]; then
  kubeconfig_filename="tenant-server-kube-proxy"
  serverCount=`ls -al ${SECRET_FOLDER}/${kubeconfig_filename}*.kubeconfig |wc -l`
  if [ "$serverCount" -eq 0 ]; then
    echo "The kube-proxy kubeconfig file for Arktos scale-out not found under directory ${SECRET_FOLDER}."
    echo "Please copy this file from Arktos RP master, e.g. in GCP run:"
    echo "gcloud compute scp <master-rp-name>:/var/run/kubernetes/tenant-server-kube-proxy*.kubeconfig /tmp/arktos/"
    die "Arktos scale-out RP worker node failed to start."
  fi
  for (( pos=0; pos<${serverCount}; pos++ ));
  do
    TP_KUBEONFIGS="${TP_KUBEONFIGS}${SECRET_FOLDER}/${kubeconfig_filename}${pos}.kubeconfig,"
    echo $TP_KUBEONFIGS
  done
  TP_KUBEONFIGS=${TP_KUBEONFIGS::-1}
fi

KUBE_PROXY_YAML="/tmp/kube-proxy.yaml"
#cat <<EOF > /tmp/kube-proxy.yaml
cat <<EOF > ${KUBE_PROXY_YAML}
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
clientConnection:
  kubeconfig: ${KUBE_PROXY_KUBECONFIG}
tenantPartitionKubeConfig: ${TP_KUBEONFIGS}
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
fi >> ${KUBE_PROXY_YAML}

KUBE_PROXY_LOG="/tmp/kube-proxy.worker.log"
sudo ./_output/local/bin/linux/amd64/hyperkube kube-proxy \
--v=3 \
--config=${KUBE_PROXY_YAML} \
--master="http://${API_HOST_IP_EXTERNAL}:8080" \
> ${KUBE_PROXY_LOG} 2>&1 &

echo "kube-proxy has been started. please check ${KUBE_PROXY_LOG} for its running log."

exit 0
