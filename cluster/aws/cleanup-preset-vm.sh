#!/bin/bash

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

set -o errexit
set -o nounset
set -o pipefail

KUBE_VER=""
KUBEMARK_MASTER=${KUBEMARK_MASTER:-false}

# this scipt is to clean up arktos in the preset machine

function set-kube-env() {
  local kube_env_yaml="/etc/kubernetes/kube_env.yaml"

  # kube-env has all the environment variables we care about, in a flat yaml format
  eval "$(python -c '
import pipes,sys,yaml

for k,v in yaml.load(sys.stdin).iteritems():
  print("""readonly {var}={value}""".format(var = k, value = pipes.quote(str(v))))
  print("""export {var}""".format(var = k))
  ' < """${kube_env_yaml}""")"
}

function stop-flannel-ds {
  pushd /tmp
  wget https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
  kubectl --kubeconfig=/home/ubuntu/.kube/config delete -f /tmp/kube-flannel.yml || true
  rm -rf /tmp/kube-flannel.yml
  popd
}

function stop-containers {
  kubeadm reset -f
  docker system prune -f
  docker stop $(docker ps -aq) || true
  docker rmi $(docker images -q)
}

function stop-services() {
  systemctl stop kubelet
}

function remove-packages() {
  apt-get autoremove -y -q kubernetes-cni cri-tools
}

function remove-files-dirs() {  
  rm -rf /etc/apt/sources.list.d/kubernetes.list
  rm -rf /etc/motd
  rm -rf /etc/systemd/system/multi-user.target.wants/kubelet.service
  rm -rf /etc/kubernetes
  rm -rf /etc/srv/kubernetes
  rm -rf /etc/systemd/system/kubelet.service.d
  rm -rf /home/ubuntu/.kube/
  rm -rf /lib/systemd/system/kubelet.service
  rm -rf /mnt/master-pd
  rm -rf /root/.kube
  rm -rf /srv/kubernetes
  rm -rf /tmp/bootstrap-script  
  rm -rf /tmp/kubernetes-server-linux-amd64.tar.gz
  rm -rf /tmp/master-user-data  
  rm -rf /usr/bin/kubeadm
  rm -rf /usr/bin/kubectl
  rm -rf /usr/bin/kubelet
  rm -rf /usr/libexec/kubernetes
  rm -rf /usr/libexec/kubernetes/kubelet-plugins
  rm -rf /usr/local/bin/kube-proxy
  rm -rf /usr/share/sosreport/sos/plugins/__pycache__/kubernetes.cpython-35.pyc
  rm -rf /usr/share/sosreport/sos/plugins/kubernetes.py  
  rm -rf /usr/bin/kubelet
  rm -rf /usr/share/google
  rm -rf /var/cache/apt/archives/kubernetes-cni_0.7.5-00_amd64.deb
  rm -rf /var/lib/cni/flannel
  rm -rf /var/log/containers/* 
  rm -rf /var/log/workload-controller-manager.log 
  rm -rf /var/cache/kubernetes-install
  rm -rf /var/log/pods  

  rm -rf /usr/share/sosreport/sos/plugins/__pycache__/etcd.cpython-35.pyc
  rm -rf /usr/share/sosreport/sos/plugins/etcd.py
  rm -rf /usr/share/mime/application/x-netcdf.xml
  rm -rf /var/lib/etcd
  rm -rf /var/etcd  
}

####################################################################################

echo "== cleanup starting =="

set-kube-env

stop-flannel-ds
stop-containers
stop-services
remove-packages
remove-files-dirs

echo "== cleanup done =="
