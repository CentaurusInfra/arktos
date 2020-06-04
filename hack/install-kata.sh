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

set -e

testcmd () {
    command -v "$1" >/dev/null
}

kata_config_path=/etc/kata-containers
kata_config_file=configuration.toml
kata_config="${kata_config_path}/${kata_config_file}"
shim=qemu
kata_snap_path="/snap/kata-containers/current/usr"

# Install kata containers components in /snap/kata-containers 
function install_kata_components {
	if ! testcmd snap; then
		echo 'Missing snap. Snap is needed to install kata components. Skipped.'
		exit
	fi

	echo "* Install Kata components"
	sudo snap install kata-containers --classic

	# Check if system support Kata
	echo "* Checking Kata compatibility"
	local kata_check_msg=`$kata_snap_path/bin/kata-runtime kata-check 2>&1`
	echo $kata_check_msg
	if ! echo $kata_check_msg | grep -q "System is capable of running Kata Containers"; then
		echo "Aborted. Current system does not support Kata Containers."
		exit
	fi
}

# Setup kata config file
function setup_kata_config {
	local kata_config_backup="${kata_config_path}/${kata_config_file}.bk"
	local shim_binary="containerd-shim-kata-${shim}-v2"
	local shim_file="/usr/local/bin/${shim_binary}"
	local shim_backup="/usr/local/bin/${shim_binary}.bak"

	sudo mkdir -p kata_config_path 

	if [ -f "${kata_config}" ]; then
		echo "* Warning: ${kata_config} already exists. Making a backup in ${kata_config_backup}" >&2
		mv "${kata_config}" "${kata_config_backup}"
	fi

	echo "* Copy kata configuration.toml to ${kata_config_path}"
	mkdir -p /etc/kata-containers/
	sudo cp ${kata_snap_path}/share/defaults/kata-containers/configuration.toml /etc/kata-containers/

	# Setup shimv2
	# Currently containerd has an assumption on the location of the shimv2 implementation
	# Until support is added (see https://github.com/containerd/containerd/issues/3073),
	# create a link in /usr/local/bin/ to the v2-shim implementation in /opt/kata/bin.
	echo "* Setup ShimV2"

	if [ -f "${shim_file}" ]; then
		echo "* Warning: ${shim_binary} already exists" >&2
		if [ ! -f "${shim_backup}" ]; then
			mv "${shim_file}" "${shim_backup}"
		else
			rm "${shim_file}"
		fi
	fi
	cat << EOT | tee "$shim_file"
#!/bin/bash
KATA_CONF_FILE=${kata_config} ${kata_snap_path}/bin/containerd-shim-kata-v2 \$@
EOT
chmod +x "$shim_file"
}

# Setup containerd 
function setup_containerd {
	echo "* Setup Containerd for Kata"
	local containerd_config_path=/etc/containerd
	local containerd_config_file=config.toml
	local containerd_config="${containerd_config_path}/${containerd_config_file}"
	local containerd_config_backup="${containerd_config_path}/${containerd_config_file}.bk"

	sudo mkdir -p ${containerd_config_path}

	if [ -f "${containerd_config}" ]; then
		echo "* Warning: ${containerd_config} already exists, making a backup in ${containerd_config_backup}" >&2
		if [ ! -f "${containerd_config_backup}" ]; then
			echo "* Making a backup of ${containerd_config} to ${containerd_config_backup}"
			mv "${containerd_config}" "${containerd_config_backup}"
		else
			echo "* Removing containerd config ${containerd_config}"
			rm "${containerd_config}"
		fi
	fi

	cat << EOT | tee "$containerd_config"
[plugins.cri.containerd.runtimes.kata-${shim}]
  runtime_type = "io.containerd.kata-${shim}.v2"
  [plugins.cri.containerd.runtimes.kata-${shim}.options]
    ConfigPath = "/etc/kata-containers/configuration.toml"
EOT

echo "* Restart containerd"
sudo systemctl restart containerd
}

function setup_runtime_class {
	echo "For kubernetes 1.14: RUN kubectl apply -f https://raw.githubusercontent.com/kata-containers/packaging/master/kata-deploy/k8s-1.14/kata-qemu-runtimeClass.yaml"
	echo "For kubernetes 1.13: RUN kubectl apply -f https://raw.githubusercontent.com/kata-containers/packaging/master/kata-deploy/k8s-1.13/kata-qemu-runtimeClass.yaml"
	echo "create 1.14 runtimeClass by default"
	KUBECTL=${KUBECTL:-"kubectl"}
	${KUBECTL} apply -f https://raw.githubusercontent.com/kata-containers/packaging/master/kata-deploy/k8s-1.14/kata-qemu-runtimeClass.yaml
	echo "A Kata example: RUN kubectl apply -f https://raw.githubusercontent.com/kata-containers/packaging/master/kata-deploy/examples/test-deploy-kata-qemu.yaml" 
}

install_kata_components
setup_kata_config
setup_containerd
setup_runtime_class
