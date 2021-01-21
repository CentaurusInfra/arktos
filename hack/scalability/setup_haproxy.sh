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

print_help() {
        echo "This is a tool to set up haproxy for Arktos scale-out tests in the local host."
        echo 
        echo "To run it: "
        echo "    [ENABLE_HAPROXY_PROMETHEUS=true] TENANT_PARTITION_IP=\"##.##.##.##,##.##.##.##...\" RESOURCE_PARTITION_IP=##.##.##.## setup_haproxy.sh"
}

run_command_exit_if_failed() {
        command="$@"
        $command 
        if [[ $? != 0 ]] 
        then 
                printf "\033[0;31mFailed: ${command}\n"
                printf "Exit. \033[0m\n"
                exit 1
        fi
}

install_haproxy_if_needed() {
        haproxy -v > /dev/null 2>&1
        if [[ $? != 0 ]] 
        then 
                printf "\033[0;31mHaproxy does not exist, installing haproxy...\033[0m\n"
                run_command_exit_if_failed sudo apt-get update && sudo apt-get -y upgrade
                run_command_exit_if_failed sudo apt --assume-yes install haproxy
        fi
}

patch-haproxy-prometheus() {
  # based on https://www.haproxy.com/blog/haproxy-exposes-a-prometheus-metrics-endpoint
  echo 'Patching Haproxy to expose prometheus...'
  haproxy_patch_tmp=/tmp/haproxy
  run_command_exit_if_failed sudo apt install -y git ca-certificates gcc libc6-dev liblua5.3-dev libpcre3-dev libssl-dev libsystemd-dev make wget zlib1g-dev
  run_command_exit_if_failed git clone https://github.com/haproxy/haproxy.git ${haproxy_patch_tmp}
  run_command_exit_if_failed pushd ${haproxy_patch_tmp}
  run_command_exit_if_failed git checkout tags/v2.3.0
  run_command_exit_if_failed make TARGET=linux-glibc USE_LUA=1 USE_OPENSSL=1 USE_PCRE=1 USE_ZLIB=1 USE_SYSTEMD=1 EXTRA_OBJS=contrib/prometheus-exporter/service-prometheus.o -j4
  run_command_exit_if_failed sudo make install-bin
  run_command_exit_if_failed sudo systemctl reset-failed haproxy.service
  run_command_exit_if_failed sudo systemctl stop haproxy
  run_command_exit_if_failed sudo cp /usr/local/sbin/haproxy /usr/sbin/haproxy
  run_command_exit_if_failed sudo systemctl start haproxy
  run_command_exit_if_failed haproxy -vv|grep Prometheus
  run_command_exit_if_failed popd
  run_command_exit_if_failed rm -rf ${haproxy_patch_tmp}
}

function direct-haproxy-logging {
  haproxy_conf=`find /etc/rsyslog.d -name *haproxy.conf`

  if [ -z "$haproxy_conf" ]; then
    echo "haproxy conf file not found in /etc/rsyslog.d/"
    return
  fi

  if grep -q "UDPServerRun 514" "$haproxy_conf"; then
    echo "skipped updating haproxy.conf for directing logging"
    return
  fi

  sudo /bin/su -c "echo '
$ModLoad imudp
$UDPServerRun 514
local0.* -/var/log/haproxy.log
' >> $haproxy_conf"

  run_command_exit_if_failed sudo service rsyslog restart
  echo "haproxy logging directed to /var/log/haproxy.log only"
}

config_haproxy() {
        repo_root=$(git rev-parse --show-toplevel)
        local -r temp_haproxy_cfg="/tmp/haproxy.cfg"

        export GO111MODULE=on

        template_source=${repo_root}/cmd/haproxy-cfg-generator/data/haproxy.cfg.template

        pushd . > /dev/null

        cd ${repo_root}/cmd/haproxy-cfg-generator/ && go build -o /tmp/haproxy_cfg_generator "."

        popd > /dev/null

        TENANT_PARTITION_IP="${TENANT_PARTITION_IP:-}" RESOURCE_PARTITION_IP="${RESOURCE_PARTITION_IP:-}" /tmp/haproxy_cfg_generator -template=${template_source} -target="${temp_haproxy_cfg}" 
        
        if [[ $? != 0 ]] 
        then
                printf "\033[0;31mhaproxy_cfg_generator Failed\n"
                exit 1
        fi
 
        sed -i -e "/^KUBEMARK_ONLY:/d"  "${temp_haproxy_cfg}"
        sed -i -e "s/ONEBOX_ONLY://g" "${temp_haproxy_cfg}"

        run_command_exit_if_failed sudo cp "${temp_haproxy_cfg}" "/etc/haproxy/haproxy.cfg"
}

if [ "${1:-}" == "-h" ] || [ "${1:-}" == "--help" ]
then
        print_help
        exit
fi

install_haproxy_if_needed

if [[ "${ENABLE_HAPROXY_PROMETHEUS:-false}" == "true" ]]
then
        patch-haproxy-prometheus
fi

direct-haproxy-logging

config_haproxy

run_command_exit_if_failed sudo systemctl restart haproxy

printf "Haproxy is in $(systemctl is-active haproxy) status. Log file: /var/log/haproxy.log.\n"