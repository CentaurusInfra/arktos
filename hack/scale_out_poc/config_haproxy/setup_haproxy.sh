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
        echo "    TENANT_PARTITION_IP=\"##.##.##.##,##.##.##.##...\" RESOURCE_PARTITION_IP=##.##.##.## setup_haproxy.sh"
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
        local -r temp_haproxy_cfg="/tmp/haproxy.cfg"

        export GO111MODULE=on
        script_root=$(dirname "${BASH_SOURCE}")
        repo_root=$(cd $(dirname $0)/../../.. ; pwd)

        template_source=${repo_root}/hack/scale_out_poc/config_haproxy/haproxy.cfg.template

        pushd .

        cd ${script_root} && go build -o /tmp/haproxy_cfg_generator "./cfg_generator/"

        popd

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

config_haproxy

run_command_exit_if_failed sudo systemctl restart haproxy

printf "\033[0;32mHaproxy was configured and started. Please check out /var/log/haproxy.log for logs. \033[0m\n"

sudo systemctl status haproxy







returnCode=$?
exit ${returnCode}
