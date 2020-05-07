#!/usr/bin/env bash

# Copyright 2015 The Kubernetes Authors.
# Copyright 2020 Authors of Arktos - file modified.
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

# Script that starts kubelet on kubemark-master as a supervisord process
# and then runs the master components as pods using kubelet.

set -o errexit
set -o nounset
set -o pipefail

# Define key path variables.
KUBEMARK_INSTALL="/var/cache/kubernetes-install/kubemark"
if [[ -z "${CLOUD_PROVIDER}" ]]; then
  CLOUD_PROVIDER="aws"
fi

function config-ip-firewall {
  echo "Configuring IP firewall rules"
  # The GCI image has host firewall which drop most inbound/forwarded packets.
  # We need to add rules to accept all TCP/UDP/ICMP packets.
  if iptables -L INPUT | grep "Chain INPUT (policy DROP)" > /dev/null; then
    echo "Add rules to accept all inbound TCP/UDP/ICMP packets"
    iptables -A INPUT -w -p TCP -j ACCEPT
    iptables -A INPUT -w -p UDP -j ACCEPT
    iptables -A INPUT -w -p ICMP -j ACCEPT
  fi
  if iptables -L FORWARD | grep "Chain FORWARD (policy DROP)" > /dev/null; then
    echo "Add rules to accept all forwarded TCP/UDP/ICMP packets"
    iptables -A FORWARD -w -p TCP -j ACCEPT
    iptables -A FORWARD -w -p UDP -j ACCEPT
    iptables -A FORWARD -w -p ICMP -j ACCEPT
  fi
}

function create-dirs {
  echo "Creating required directories"
  mkdir -p /etc/kubernetes/manifests
  mkdir -p /etc/kubernetes/addons
}

# Compute etcd related variables.
function compute-etcd-variables {
	ETCD_IMAGE="${ETCD_IMAGE:-}"
	ETCD_QUOTA_BYTES=""
	if [ "${ETCD_VERSION:0:2}" == "3." ]; then
		# TODO: Set larger quota to see if that helps with
		# 'mvcc: database space exceeded' errors. If so, pipe
		# though our setup scripts.
		ETCD_QUOTA_BYTES=" --quota-backend-bytes=4294967296 "
	fi
}

# Computes command line arguments to be passed to etcd-events.
function compute-etcd-events-params {
       local params="${ETCD_TEST_ARGS:-}"
       params+=" --name=etcd-$(hostname -s)"
       params+=" --listen-peer-urls=http://127.0.0.1:2381"
       params+=" --advertise-client-urls=http://127.0.0.1:4002"
       params+=" --listen-client-urls=http://0.0.0.0:4002"
       params+=" --data-dir=/var/etcd/data-events"
       params+=" ${ETCD_QUOTA_BYTES}"
       echo "${params}"
}

# Formats the given device ($1) if needed and mounts it at given mount point
# ($2).
function safe-format-and-mount() {
	device=$1
	mountpoint=$2

	# Format only if the disk is not already formatted.
	if ! tune2fs -l "${device}" ; then
		echo "Formatting '${device}'"
		mkfs.ext4 -F "${device}"
	fi

	echo "Mounting '${device}' at '${mountpoint}'"
	mount -o discard,defaults "${device}" "${mountpoint}"
}

# Finds a PD device with name '$1' attached to the master.
function find-attached-pd() {
	local -r pd_name=$1
	if [[ ! -e /dev/disk/by-id/${pd_name} ]]; then
		echo ""
	fi
	device_info=$(ls -l "/dev/disk/by-id/${pd_name}")
	relative_path=${device_info##* }
	echo "/dev/disk/by-id/${relative_path}"
}

# Mounts a persistent disk (formatting if needed) to store the persistent data
# on the master. safe-format-and-mount only formats an unformatted disk, and
# mkdir -p will leave a directory be if it already exists.
function mount-pd() {
	local -r pd_name=$1
	local -r mount_point=$2

	if [[ -z "${find-attached-pd ${pd_name}}" ]]; then
		echo "Can't find ${pd_name}. Skipping mount."
		return
	fi

	local -r pd_path="/dev/disk/by-id/${pd_name}"
	echo "Mounting PD '${pd_path}' at '${mount_point}'"
	# Format and mount the disk, create directories on it for all of the master's
	# persistent data, and link them to where they're used.
	mkdir -p "${mount_point}"
	safe-format-and-mount "${pd_path}" "${mount_point}"
	echo "Mounted PD '${pd_path}' at '${mount_point}'"

	# NOTE: These locations on the PD store persistent data, so to maintain
	# upgradeability, these locations should not change.  If they do, take care
	# to maintain a migration path from these locations to whatever new
	# locations.
}

function create-addonmanager-kubeconfig {
  echo "Creating addonmanager kubeconfig file"
  mkdir -p "${KUBEMARK_INSTALL}/addon-manager"
  cat <<EOF >"${KUBEMARK_INSTALL}/addon-manager/kubeconfig"
apiVersion: v1
kind: Config
users:
- name: addon-manager
  user:
    token: ${ADDON_MANAGER_TOKEN}
clusters:
- name: local
  cluster:
    insecure-skip-tls-verify: true
    server: https://localhost:443
contexts:
- context:
    cluster: local
    user: addon-manager
  name: addon-manager
current-context: addon-manager
EOF
}

# Create the log file and set its properties.
#
# $1 is the file to create.
function prepare-log-file {
	touch "$1"
	chmod 644 "$1"
	chown root:root "$1"
}

# A helper function for copying addon manifests and set dir/files
# permissions.
#
# $1: addon category under /etc/kubernetes
# $2: manifest source dir
function setup-addon-manifests {
  local -r src_dir="${KUBEMARK_INSTALL}/$2"
  local -r dst_dir="/etc/kubernetes/$1/$2"

  if [[ ! -d "${dst_dir}" ]]; then
    mkdir -p "${dst_dir}"
  fi

  local files
  files=$(find "${src_dir}" -maxdepth 1 -name "*.yaml")
  if [[ -n "${files}" ]]; then
    cp "${src_dir}/"*.yaml "${dst_dir}"
  fi
  chown -R root:root "${dst_dir}"
  chmod 755 "${dst_dir}"
  chmod 644 "${dst_dir}"/*
}

# Computes command line arguments to be passed to addon-manager.
function compute-kube-addon-manager-params {
  echo ""
}

# Start a kubernetes master component '$1' which can be any of the following:
# 1. etcd-events
# 2. kube-addon-manager
#
# It prepares the log file, loads the docker tag, calculates variables, sets them
# in the manifest file, and then copies the manifest file to /etc/kubernetes/manifests.
#
# Assumed vars:
#   DOCKER_REGISTRY
function start-kubemaster-component() {
	echo "Start master component $1"
	local -r component=$1
	prepare-log-file /var/log/"${component}".log
	local -r src_file="${KUBEMARK_INSTALL}/${component}.yaml"
	local -r params=$("compute-${component}-params")

	# Evaluate variables.
	sed -i -e "s@{{params}}@${params}@g" "${src_file}"
	sed -i -e "s@{{kube_docker_registry}}@${DOCKER_REGISTRY}@g" "${src_file}"
	sed -i -e "s@{{instance_prefix}}@${INSTANCE_PREFIX}@g" "${src_file}"
	if [ "${component:0:4}" == "etcd" ]; then
		sed -i -e "s@{{etcd_image}}@${ETCD_IMAGE}@g" "${src_file}"
	elif [ "${component}" == "kube-addon-manager" ]; then
		setup-addon-manifests "addons" "kubemark-rbac-bindings"
	else
		echo "Unsupported component $component"
		exit 1
	fi
	cp "${src_file}" /etc/kubernetes/manifests
}

############################### Main Function ########################################
echo "Start to configure master instance for kubemark"

# Extract files from the server tar and setup master env variables.
cd "${KUBEMARK_INSTALL}"
source "${KUBEMARK_INSTALL}/kubemark-master-env.sh"

# Setup IP firewall rules, required directory structure and etcd config.
config-ip-firewall
create-dirs
compute-etcd-variables

ADDON_MANAGER_TOKEN=$(cat /etc/srv/kubernetes/known_tokens.csv | grep addon-manager | awk -F , '{print $1}')
create-addonmanager-kubeconfig

# Mount master PD for event-etcd (if required) and create symbolic links to it.
{
	EVENT_STORE_IP="${EVENT_STORE_IP:-127.0.0.1}"
	EVENT_STORE_URL="${EVENT_STORE_URL:-http://${EVENT_STORE_IP}:4002}"
	if [ "${EVENT_PD:-}" == "true" ]; then
		event_etcd_mount_point="/mnt/disks/master-event-pd"
		mount-pd "google-master-event-pd" "${event_etcd_mount_point}"
		# Contains all the data stored in event etcd.
		mkdir -p "${event_etcd_mount_point}/var/etcd/events"
		chmod 700 "${event_etcd_mount_point}/var/etcd/events"
		ln -s -f "${event_etcd_mount_point}/var/etcd/events" /var/etcd/events
	fi
}

# Setup docker flags and load images of the master components.
DOCKER_REGISTRY="k8s.gcr.io"

if [[ -z "${ETCD_SERVERS:-}" ]]; then
  if [ "${EVENT_STORE_IP:-}" == "127.0.0.1" ]; then
    start-kubemaster-component "etcd-events"
  fi
fi
start-kubemaster-component "kube-addon-manager"
kubectl create -f /etc/kubernetes/addons/kubemark-rbac-bindings/

echo "Done configuration of kubermark master components"
