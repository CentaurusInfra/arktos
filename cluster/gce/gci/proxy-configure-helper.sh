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

# Due to the GCE custom metadata size limit, we split the entire script into two
# files configure.sh and configure-helper.sh. The functionality of downloading
# kubernetes configuration, manifests, docker images, and binary files are
# put in configure.sh, which is uploaded via GCE custom metadata.

set -o errexit
set -o nounset
set -o pipefail


function setup-proxy() {
  sysctl -w net.ipv4.ip_local_port_range='12000 65000'
  sed -i '$afs.file-max = 1000000' /etc/sysctl.conf
  sysctl -p
  sed -i '$a*       hard    nofile          1000000' /etc/security/limits.conf
  sed -i '$a*       soft    nofile          1000000' /etc/security/limits.conf
  sed -i '$aroot       hard    nofile          1000000' /etc/security/limits.conf
  sed -i '$aroot       soft    nofile          1000000' /etc/security/limits.conf
  sed -i '$ahaproxy       hard    nofile          1000000' /etc/security/limits.conf
  sed -i '$ahaproxy       soft    nofile          1000000' /etc/security/limits.conf

  apt update -y
  apt install -y ${ARKTOS_SCALEOUT_PROXY_APP}

  patch-haproxy-prometheus

}

function config-proxy() {
  rm -f /etc/${ARKTOS_SCALEOUT_PROXY_APP}/${PROXY_CONFIG_FILE}
  mv /etc/${ARKTOS_SCALEOUT_PROXY_APP}/${PROXY_CONFIG_FILE}.tmp /etc/${ARKTOS_SCALEOUT_PROXY_APP}/${PROXY_CONFIG_FILE}
  echo "DBG ========================================"
  cat /etc/${ARKTOS_SCALEOUT_PROXY_APP}/${PROXY_CONFIG_FILE}
  echo "VDBG ========================================"

  echo "Restart proxy service"
  systemctl restart ${ARKTOS_SCALEOUT_PROXY_APP}
}

function patch-haproxy-prometheus {
  # based on https://www.haproxy.com/blog/haproxy-exposes-a-prometheus-metrics-endpoint
  echo 'Patching Haproxy to expose prometheus...'
  apt install -y git ca-certificates gcc libc6-dev liblua5.3-dev libpcre3-dev libssl-dev libsystemd-dev make wget zlib1g-dev
  git clone https://github.com/haproxy/haproxy.git /tmp/haproxy
  cd /tmp/haproxy;git checkout tags/v2.3.0
  cd /tmp/haproxy;make TARGET=linux-glibc USE_LUA=1 USE_OPENSSL=1 USE_PCRE=1 USE_ZLIB=1 USE_SYSTEMD=1 EXTRA_OBJS=contrib/prometheus-exporter/service-prometheus.o -j4
  cd /tmp/haproxy; make install-bin
  systemctl reset-failed haproxy.service
  systemctl stop haproxy
  cp /usr/local/sbin/haproxy /usr/sbin/haproxy
  systemctl start haproxy
  haproxy -vv|grep Prometheus
}

function start-proxy-prometheus {
  echo "Start prometheus on proxy"

  export RELEASE="2.2.1"
  cd /tmp/
  wget https://github.com/prometheus/prometheus/releases/download/v${RELEASE}/prometheus-${RELEASE}.linux-amd64.tar.gz
  tar xvf prometheus-${RELEASE}.linux-amd64.tar.gz 

  echo "configing prometheus-metrics"
  cat <<EOF >/tmp/prometheus-metrics.yaml
global:
  scrape_interval: 10s
scrape_configs:
  - job_name: prometheus-metrics
    static_configs:
    - targets: ['127.0.0.1:8404']
EOF

  cat <<EOF >/tmp/prometheus.service
[Unit]
Description=prometheus service
Requires=network-online.target
After=network-online.target

[Service]
Restart=always
RestartSec=10
ExecStart=/tmp/prometheus-${RELEASE}.linux-amd64/prometheus --config.file=/tmp/prometheus-metrics.yaml --web.listen-address=:9090 --web.enable-admin-api

[Install]
WantedBy=multi-user.target
EOF

  mv /tmp/prometheus.service /etc/systemd/system/
  systemctl daemon-reload 
  systemctl start prometheus.service

}

function write-pki-data {
  local data="${1}"
  local path="${2}"
  (umask 077; echo "${data}" | base64 --decode > "${path}")
}

function create-proxy-pki {
  echo "Creating proxy pki files"

  local -r pki_dir="/etc/${ARKTOS_SCALEOUT_PROXY_APP}/pki"
  mkdir -p "${pki_dir}/private"
  mkdir -p "${pki_dir}/issued"

  if [[ ! -z "${PROXY_CA_CERT:-}" ]]; then
    CA_CERT_PATH="${pki_dir}/ca.crt"
    write-pki-data "${PROXY_CA_CERT}" "${CA_CERT_PATH}"
  fi

  if [[ ! -z "${PROXY_CA_KEY:-}" ]]; then
    CA_KEY_PATH="${pki_dir}/private/ca.key"
    write-pki-data "${PROXY_CA_KEY}" "${CA_KEY_PATH}"
  fi

  if [[ ! -z "${PROXY_CERT:-}" ]]; then
    CA_KEY_PATH="${pki_dir}/issued/${SCALEOUT_PROXY_NAME}.crt"
    write-pki-data "${PROXY_CERT}" "${CA_KEY_PATH}"
  fi

  if [[ ! -z "${PROXY_KEY:-}" ]]; then
    CA_KEY_PATH="${pki_dir}/private/${SCALEOUT_PROXY_NAME}.key"
    write-pki-data "${PROXY_KEY}" "${CA_KEY_PATH}"
  fi

  if [[ ! -z "${PROXY_KUBECFG_CERT:-}" ]]; then
    CA_KEY_PATH="${pki_dir}/issued/kubecfg.crt"
    write-pki-data "${PROXY_KUBECFG_CERT}" "${CA_KEY_PATH}"
  fi

  if [[ ! -z "${SHARED_CA_CERT:-}" ]]; then
    CA_KEY_PATH="/etc/${ARKTOS_SCALEOUT_PROXY_APP}/ca.crt"
    write-pki-data "${SHARED_CA_CERT}" "${CA_KEY_PATH}"
  fi

  cat ${pki_dir}/issued/${SCALEOUT_PROXY_NAME}.crt > ${pki_dir}/kubemark-client-proxy.pem
  cat ${pki_dir}/private/${SCALEOUT_PROXY_NAME}.key >> ${pki_dir}/kubemark-client-proxy.pem
  
}

######### Main Function ##########
# redirect stdout/stderr to a file
exec >> /var/log/master-init.log 2>&1
echo "Setup proxy service"

KUBE_HOME="/home/kubernetes"
source "${KUBE_HOME}/kube-env"

if [[ -e "${KUBE_HOME}/kube-master-certs" ]]; then
  source "${KUBE_HOME}/kube-master-certs"
  create-proxy-pki
fi

setup-proxy
config-proxy

if [[ "${ENABLE_PROMETHEUS_DEBUG:-false}" == "true" ]]; then
  start-proxy-prometheus
fi

echo "Done for proxy-up"




