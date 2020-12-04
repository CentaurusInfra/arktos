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

print_help() {
        echo "This is a tool to set up haproxy for Arktos scale-out tests in the local host."
        echo 
        echo "To run it: "
        echo "    TENANT_PARTITION_ONE_IP=##.##.##.##  TENANT_PARTITION_TWO_IP=##.##.##.## RESOURCE_PARTITION_IP=##.##.##.## setup_haproxy.sh"
}

validate_ip_exit_if_invalid() {
        local var_name=$1
        local ip_addr=$2
        if ! [[ ${ip_addr} =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]
        then
                printf "\033[0;31m${var_name} (${ip_addr}) is not a valid ip address. Exit. \033[0m\n"
                exit 1
        fi
}

validate_params() {
        validate_ip_exit_if_invalid "tenant_partition_one_ip" "${tenant_partition_one_ip}"
        validate_ip_exit_if_invalid "tenant_partition_two_ip" "${tenant_partition_two_ip}"
        validate_ip_exit_if_invalid "resource_partition_ip" "${resource_partition_ip}"
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

config_haproxy() {
        script_root=$(dirname "${BASH_SOURCE}")
        local -r temp_file="/tmp/haproxy.cfg"
        run_command_exit_if_failed  cp "${script_root}/haproxy.cfg" "${temp_file}"
        
        sed -i -e "s@{{ *proxy_port *}}@${proxy_port}@g" "${temp_file}"

        sed -i -e "s@{{ *arktos_api_protocol *}}@${arktos_api_protocol}@g" "${temp_file}"  
        sed -i -e "s@{{ *arktos_api_port *}}@${arktos_api_port}@g" "${temp_file}"

        sed -i -e "s@{{ *resource_partition_ip *}}@${resource_partition_ip}@g" "${temp_file}"
        sed -i -e "s@{{ *tenant_partition_one_ip *}}@${tenant_partition_one_ip}@g" "${temp_file}"
        sed -i -e "s@{{ *tenant_partition_two_ip *}}@${tenant_partition_two_ip}@g" "${temp_file}"

        sed -i -e "s@{{ *connection_timeout *}}@${connection_timeout}@g" "${temp_file}"

        run_command_exit_if_failed sudo cp "${temp_file}" "/etc/haproxy/haproxy.cfg"
}

if [ "$1" == "-h" ] || [ "$1" == "--help" ]
then
        print_help
        exit
fi

proxy_port=${SCALE_OUT_PROXY_PORT:-8888}
connection_timeout=${CONN_TIMEOUT:-10m}

arktos_api_protocol=${ARKTOS_API_PROTOCOL:-http}
arktos_api_port=${ARKTOS_API_port:-8080}

resource_partition_ip=${RESOURCE_PARTITION_IP}
tenant_partition_one_ip=${TENANT_PARTITION_ONE_IP}
tenant_partition_two_ip=${TENANT_PARTITION_TWO_IP}

validate_params

install_haproxy_if_needed

config_haproxy

run_command_exit_if_failed sudo systemctl restart haproxy

printf "\033[0;32mHaproxy was configured and started. Please check out /var/log/haproxy.log for logs. \033[0m\n"

sudo systemctl status haproxy