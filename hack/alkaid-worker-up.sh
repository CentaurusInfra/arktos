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

die() { echo "$*" 1>&2 ; exit 1; }

[ -n "${KUBELET_IP}" ] || die "KUBELET_IP env var not set"
[ -n "${KUBELET_KUBECONFIG}" ] || die "KUBELET_KUBECONFIG env var not set"

HOSTNAME_OVERRIDE=${HOSTNAME_OVERRIDE:-"$(hostname)"}
CLUSTER_DNS=${CLUSTER_DNS:-"10.0.0.10"}

sudo ./hyperkube kubelet --v=3 --container-runtime=remote --hostname-override=${HOSTNAME_OVERRIDE} --address=${KUBELET_IP} --kubeconfig=${KUBELET_KUBECONFIG} --feature-gates=AllAlpha=false --cpu-cfs-quota=true --enable-controller-attach-detach=true --cgroups-per-qos=true --cgroup-driver= --cgroup-root= --cluster-dns=${CLUSTER_DNS} --cluster-domain=cluster.local --container-runtime-endpoint="containerRuntime,container,/run/containerd/containerd.sock;vmRuntime,vm,/run/virtlet.sock" --runtime-request-timeout=2m --port=10250

