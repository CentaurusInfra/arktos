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

# install cni plugin related packages
KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source ${KUBE_ROOT}/hack/arktos-cni.rc

mkdir -p /tmp/arktos

SECRET_FOLDER=${SECRET_FOLDER:-"/tmp/arktos"}
KUBELET_KUBECONFIG=${KUBELET_KUBECONFIG:-"${SECRET_FOLDER}/kubelet.kubeconfig"}
KUBELET_CLIENTCA=${KUBELET_CLIENTCA:-"${SECRET_FOLDER}/client-ca.crt"}
HOSTNAME_OVERRIDE=${HOSTNAME_OVERRIDE:-"$(hostname)"}
CLUSTER_DNS=${CLUSTER_DNS:-"10.0.0.10"}

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

make all WHAT=cmd/hyperkube

sudo ./_output/local/bin/linux/amd64/hyperkube kubelet \
--v=3 \
--container-runtime=remote \
--hostname-override=${HOSTNAME_OVERRIDE} \
--address=${KUBELET_IP} \
--kubeconfig=${KUBELET_KUBECONFIG} \
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
> /tmp/kubelet.worker.log 2>&1 &

echo "kubelet has been started. please check /tmp/kubelet.worker.log for its running log."

