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


# Note that this script is also used by AWS; we include it and then override
# functions with AWS equivalents.  Note `#+AWS_OVERRIDES_HERE` below.
# TODO(justinsb): Refactor into common script & GCE specific script?

# If we have any arguments at all, this is a push and not just setup.
is_push=$@

function ensure-basic-networking() {
  # Deal with GCE networking bring-up race. (We rely on DNS for a lot,
  # and it's just not worth doing a whole lot of startup work if this
  # isn't ready yet.)
  until getent hosts time.google.com &>/dev/null; do
    echo 'Waiting for functional DNS (trying to resolve time.google.com)...'
    sleep 3
  done
#  until getent hosts $(hostname -f || echo _error_) &>/dev/null; do
#    echo 'Waiting for functional DNS (trying to resolve my own FQDN)...'
#    sleep 3
#  done
#  until getent hosts $(hostname -i || echo _error_) &>/dev/null; do
#    echo 'Waiting for functional DNS (trying to resolve my own IP)...'
#    sleep 3
#  done

  echo "Networking functional on $(hostname)"
}

# A hookpoint for installing any needed packages
ensure-packages() {
  apt-get-install curl
  # For reading kube_env.yaml
  apt-get-install python-yaml

  # TODO: Where to get safe_format_and_mount?
  mkdir -p /usr/share/google
  cd /usr/share/google
  download-or-bust "dc96f40fdc9a0815f099a51738587ef5a976f1da" https://raw.githubusercontent.com/GoogleCloudPlatform/compute-image-packages/82b75f314528b90485d5239ab5d5495cc22d775f/google-startup-scripts/usr/share/google/safe_format_and_mount
  chmod +x safe_format_and_mount
}

function ensure-install-dir() {
  INSTALL_DIR="/var/cache/kubernetes-install"
  mkdir -p ${INSTALL_DIR}
  cd ${INSTALL_DIR}
}

function set-broken-motd() {
  echo -e '\nBroken (or in progress) Kubernetes node setup! Suggested first step:\n  tail /var/log/startupscript.log\n' > /etc/motd
}

function reset-motd() {
  # kubelet is installed both on the master and nodes, and the version is easy to parse (unlike kubectl)
  local -r version="$(/usr/bin/kubelet --version=true | cut -f2 -d " ")"
  # This logic grabs either a release tag (v1.2.1 or v1.2.1-alpha.1),
  # or the git hash that's in the build info.
  local gitref="$(echo "${version}" | sed -r "s/(v[0-9]+\.[0-9]+\.[0-9]+)(-[a-z]+\.[0-9]+)?.*/\1\2/g")"
  local devel=""
  if [[ "${gitref}" != "${version}" ]]; then
    devel="
Note: This looks like a development version, which might not be present on GitHub.
If it isn't, the closest tag is at:
  https://github.com/kubernetes/kubernetes/tree/${gitref}
"
    gitref="${version//*+/}"
  fi
  cat > /etc/motd <<EOF

Welcome to Kubernetes ${version}!

You can find documentation for Kubernetes at:
  http://docs.kubernetes.io/

The source for this release can be found at:
  /usr/local/share/doc/kubernetes/kubernetes-src.tar.gz
Or you can download it at:
  https://storage.googleapis.com/kubernetes-release/release/${version}/kubernetes-src.tar.gz

It is based on the Kubernetes source at:
  https://github.com/kubernetes/kubernetes/tree/${gitref}
${devel}
For Kubernetes copyright and licensing information, see:
  /usr/local/share/doc/kubernetes/LICENSES

EOF
}

function curl-metadata() {
  curl --fail --retry 5 --silent -H 'Metadata-Flavor: Google' "http://metadata/computeMetadata/v1/instance/attributes/${1}"
}

set-kube-env() {
  local kube_env_yaml="/etc/kubernetes/kube_env.yaml"

  # kube-env has all the environment variables we care about, in a flat yaml format
  eval "$(python -c '
import pipes,sys,yaml

for k,v in yaml.load(sys.stdin).iteritems():
  print("""readonly {var}={value}""".format(var = k, value = pipes.quote(str(v))))
  print("""export {var}""".format(var = k))
  ' < """${kube_env_yaml}""")"
}

function remove-docker-artifacts() {
  echo "== Deleting docker0 =="
  apt-get-install bridge-utils

  # Remove docker artifacts on minion nodes, if present
  iptables -t nat -F || true
  ifconfig docker0 down || true
  brctl delbr docker0 || true
  echo "== Finished deleting docker0 =="
}

# Retry a download until we get it. Takes a hash and a set of URLs.
#
# $1 is the sha1 of the URL. Can be "" if the sha1 is unknown.
# $2+ are the URLs to download.
download-or-bust() {
  local -r hash="$1"
  shift 1

  urls=( $* )
  while true; do
    for url in "${urls[@]}"; do
      local file="${url##*/}"
      rm -f "${file}"
      if ! curl -f --ipv4 -Lo "${file}" --connect-timeout 20 --max-time 300 --retry 6 --retry-delay 10 "${url}"; then
        echo "== Failed to download ${url}. Retrying. =="
      elif [[ -n "${hash}" ]] && ! validate-hash "${file}" "${hash}"; then
        echo "== Hash validation of ${url} failed. Retrying. =="
      else
        if [[ -n "${hash}" ]]; then
          echo "== Downloaded ${url} (SHA1 = ${hash}) =="
        else
          echo "== Downloaded ${url} =="
        fi
        return
      fi
    done
  done
}

validate-hash() {
  local -r file="$1"
  local -r expected="$2"
  local actual

  actual=$(sha1sum ${file} | awk '{ print $1 }') || true
  if [[ "${actual}" != "${expected}" ]]; then
    echo "== ${file} corrupted, sha1 ${actual} doesn't match expected ${expected} =="
    return 1
  fi
}

apt-get-install() {
  local -r packages=( $@ )
  installed=true
  for package in "${packages[@]}"; do
    if ! dpkg -s "${package}" &>/dev/null; then
      installed=false
      break
    fi
  done
  if [[ "${installed}" == "true" ]]; then
    echo "== ${packages[@]} already installed, skipped apt-get install ${packages[@]} =="
    return
  fi

  apt-get-update

  # Forcibly install packages (options borrowed from Salt logs).
  until apt-get -q -y install $@; do
    echo "== install of packages $@ failed, retrying =="
    sleep 5
  done
}

apt-get-update() {
  echo "== Refreshing package database =="
  until apt-get update; do
    echo "== apt-get update failed, retrying =="
    sleep 5
  done
}

# Finds the master PD device
find-master-pd() {
  if ( grep "/mnt/master-pd" /proc/mounts ); then
    echo "Master PD already mounted; won't remount"
    MASTER_PD_DEVICE=""
    return
  fi
  echo "Waiting for master pd to be attached"
  attempt=0
  while true; do
    echo Attempt "$(($attempt+1))" to check for /dev/xvdb
    if [[ -e /dev/xvdb ]]; then
      echo "Found /dev/xvdb"
      MASTER_PD_DEVICE="/dev/xvdb"
      break
    fi
    attempt=$(($attempt+1))
    sleep 1
  done

  # Mount the master PD as early as possible
  echo "/dev/xvdb /mnt/master-pd ext4 noatime 0 0" >> /etc/fstab
}

# Mounts a persistent disk (formatting if needed) to store the persistent data
# on the master -- etcd's data, a few settings, and security certs/keys/tokens.
#
# This function can be reused to mount an existing PD because all of its
# operations modifying the disk are idempotent -- safe_format_and_mount only
# formats an unformatted disk, and mkdir -p will leave a directory be if it
# already exists.
mount-master-pd() {
  if [[ $PRESET_INSTANCES_ENABLED != "true" ]]; then
    find-master-pd
    if [[ -z "${MASTER_PD_DEVICE:-}" ]]; then
      return
    fi

    # Format and mount the disk, create directories on it for all of the master's
    # persistent data, and link them to where they're used.
    echo "Mounting master-pd"
    mkdir -p /mnt/master-pd
    /usr/share/google/safe_format_and_mount -m "mkfs.ext4 -F" "${MASTER_PD_DEVICE}" /mnt/master-pd &>/var/log/master-pd-mount.log || \
      { echo "!!! master-pd mount failed, review /var/log/master-pd-mount.log !!!"; return 1; }
  fi

  # Contains all the data stored in etcd
  mkdir -m 700 -p /mnt/master-pd/var/etcd
  # Contains the dynamically generated apiserver auth certs and keys
  mkdir -p /mnt/master-pd/srv/kubernetes
  # Directory for kube-apiserver to store SSH key (if necessary)
  mkdir -p /mnt/master-pd/srv/sshproxy

  ln -s -f /mnt/master-pd/var/etcd /var/etcd
  ln -s -f /mnt/master-pd/srv/kubernetes /srv/kubernetes
  ln -s -f /mnt/master-pd/srv/sshproxy /srv/sshproxy

  # This is a bit of a hack to get around the fact that salt has to run after the
  # PD and mounted directory are already set up. We can't give ownership of the
  # directory to etcd until the etcd user and group exist, but they don't exist
  # until salt runs if we don't create them here. We could alternatively make the
  # permissions on the directory more permissive, but this seems less bad.
  if ! id etcd &>/dev/null; then
    useradd -s /sbin/nologin -d /var/etcd etcd
  fi
  chown -R etcd /mnt/master-pd/var/etcd
  chgrp -R etcd /mnt/master-pd/var/etcd
}

function split-commas() {
  echo $1 | tr "," "\n"
}

function try-download-release() {
  # TODO(zmerlynn): Now we REALLy have no excuse not to do the reboot
  # optimization.

  local -r server_binary_tar_urls=( $(split-commas "${SERVER_BINARY_TAR_URL}") )
  local -r server_binary_tar="${server_binary_tar_urls[0]##*/}"
  if [[ -n "${SERVER_BINARY_TAR_HASH:-}" ]]; then
    local -r server_binary_tar_hash="${SERVER_BINARY_TAR_HASH}"
  else
    echo "Downloading binary release sha1 (not found in env)"
    download-or-bust "" "${server_binary_tar_urls[@]/.tar.gz/.tar.gz.sha1}"
    local -r server_binary_tar_hash=$(cat "${server_binary_tar}.sha1")
  fi

  echo "Downloading binary release tar (${server_binary_tar_urls[@]})"
  if [[ $PRESET_INSTANCES_ENABLED != "true" ]]; then    
    download-or-bust "${server_binary_tar_hash}" "${server_binary_tar_urls[@]}"
  else
    cp ${SERVER_BINARY_TAR_URL} ./
  fi

  echo "Unpacking and checking integrity of binary release tar"
  rm -rf kubernetes
  tar tzf "${server_binary_tar}" > /dev/null
}

function download-release() {
  # In case of failure checking integrity of release, retry.
  until try-download-release; do
    sleep 15
    echo "Couldn't download release. Retrying..."
  done

  echo "Running release install script"
}

function run-user-script() {
  if curl-metadata k8s-user-startup-script > "${INSTALL_DIR}/k8s-user-script.sh"; then
    user_script=$(cat "${INSTALL_DIR}/k8s-user-script.sh")
  fi
  if [[ ! -z ${user_script:-} ]]; then
    chmod u+x "${INSTALL_DIR}/k8s-user-script.sh"
    echo "== running user startup script =="
    "${INSTALL_DIR}/k8s-user-script.sh"
  fi
}

function ensure-apparmor-service() {
  local hostfqdn=$(hostname -f)
  local hostname=$(hostname)
  echo "127.0.1.1 $hostfqdn $hostname" >> /etc/hosts
  echo "$API_SERVERS $KUBERNETES_MASTER_NAME" >> /etc/hosts

  # Start AppArmor service before we have scripts to configure it properly
  if ! sudo systemctl is-active --quiet apparmor; then
    echo "Starting Apparmor service"
    sudo systemctl start apparmor
  fi
}

function setup-flannel-cni-conf() {
  cat > /etc/cni/net.d/10-flannel.conflist <<EOF
{
  "name": "cbr0",
  "plugins": [
    {
      "type": "flannel",
      "delegate": {
        "hairpinMode": true,
        "isDefaultGateway": true
      }
    },
    {
      "type": "portmap",
      "capabilities": {
        "portMappings": true
      }
    }
  ]
}
EOF
}

function setup-weave-cni-conf() {
  cat > /etc/cni/net.d/10-weave.conf <<EOF
{
  "name": "weave",
  "type": "weave-net"
}
EOF
}

function setup-bridge-cni-conf() {
  cat > /etc/cni/net.d/bridge.conf <<EOF
{
  "cniVersion": "0.3.1",
  "name": "containerd-net",
  "type": "bridge",
  "bridge": "cni0",
  "isGateway": true,
  "ipMasq": true,
  "ipam": {
    "type": "host-local",
    "subnet": "10.88.0.0/16",
    "routes": [
      { "dst": "0.0.0.0/0" }
    ]
  }
}
EOF
}

function setup-cni-network-conf() {
  mkdir -p /etc/cni/net.d
  case "${NETWORK_PROVIDER:-flannel}" in
    flannel)
    setup-flannel-cni-conf
    ;;
    weave)
    setup-weave-cni-conf
    ;;
    bridge)
    setup-bridge-cni-conf
    ;;
  esac
}

function enable-root-ssh() {
  sudo sed -i 's/.*sleep 10\" //' /root/.ssh/authorized_keys
}

function ensure-containerd() {
  echo "Installing containerd .."
  pushd /tmp
  sudo wget https://storage.googleapis.com/cri-containerd-release/cri-containerd-1.3.3.linux-amd64.tar.gz
  tar --no-overwrite-dir -C / -xzf cri-containerd-1.3.3.linux-amd64.tar.gz
  systemctl daemon-reload
  systemctl start containerd
  popd
}

function ensure-docker() {
  echo "Installing docker .."
  sudo apt-get install -y apt-transport-https ca-certificates curl
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
  sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
  sudo apt-get update -y
  sudo apt-get install -o DPkg::Options::="--force-confnew" -y docker-ce
  #TODO: Wait loop for docker up
  sleep 5
  systemctl status docker > /dev/null
  DOCKER_START_STATUS=$?
  if [ $DOCKER_START_STATUS -ne 0 ]; then
    echo "ERROR: Failed to start docker."
    exit $DOCKER_START_STATUS
  fi
}

function unpack-kubernetes() {
  pushd ${INSTALL_DIR}
  cp -p /usr/share/google/kubernetes-server-linux-amd64.tar.gz .
  tar xf kubernetes-server-linux-amd64.tar.gz
  cp -p ./kubernetes/server/bin/kubelet /usr/bin/
  cp -p ./kubernetes/server/bin/kubectl /usr/bin/
  cp -p ./kubernetes/server/bin/kubeadm /usr/bin/
  KUBE_VER=`cat ./kubernetes/server/bin/kube-apiserver.docker_tag`
  if [[ "${KUBERNETES_MASTER}" == "true" ]]; then
    img_bins=(kube-apiserver kube-controller-manager kube-scheduler workload-controller-manager)
    for img in "${img_bins[@]}"; do
      echo "Loading docker image for $img"
      sudo docker load -i ./kubernetes/server/bin/$img.tar
    done
  fi
  echo "Loading docker image for kube-proxy"
  sudo docker load -i ./kubernetes/server/bin/kube-proxy.tar
  popd

  sudo apt-get install -y iptables arptables ebtables
  sudo apt-get update -y
  sudo apt-get install -y apt-transport-https curl
  curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -
  cat <<EOF | sudo tee /etc/apt/sources.list.d/kubernetes.list
deb https://apt.kubernetes.io/ kubernetes-xenial main
EOF
  sudo apt-get update -y
  sudo apt-get install -y conntrack kubernetes-cni cri-tools
}

# Create the log file and set its properties.
#
# $1 is the file to create.
# $2: the log owner uid to set for the log file.
# $3: the log owner gid to set for the log file.
function prepare-log-file {
  touch $1
  chmod 644 $1
  chown "${2:-${LOG_OWNER_USER:-root}}":"${3:-${LOG_OWNER_GROUP:-root}}" $1
}

# secure_random generates a secure random string of bytes. This function accepts
# a number of secure bytes desired and returns a base64 encoded string with at
# least the requested entropy. Rather than directly reading from /dev/urandom,
# we use uuidgen which calls getrandom(2). getrandom(2) verifies that the
# entropy pool has been initialized sufficiently for the desired operation
# before reading from /dev/urandom.
#
# ARGS:
#   #1: number of secure bytes to generate. We round up to the nearest factor of 32.
function secure_random {
  local infobytes="${1}"
  if ((infobytes <= 0)); then
    echo "Invalid argument to secure_random: infobytes='${infobytes}'" 1>&2
    return 1
  fi

  local out=""
  for (( i = 0; i < "${infobytes}"; i += 32 )); do
    # uuids have 122 random bits, sha256 sums have 256 bits, so concatenate
    # three uuids and take their sum. The sum is encoded in ASCII hex, hence the
    # 64 character cut.
    out+="$(
     (
       uuidgen --random;
       uuidgen --random;
       uuidgen --random;
     ) | sha256sum \
       | head -c 64
    )";
  done
  # Finally, convert the ASCII hex to base64 to increase the density.
  echo -n "${out}" | xxd -r -p | base64 -w 0
}

# append_or_replace_prefixed_line ensures:
# 1. the specified file exists
# 2. existing lines with the specified ${prefix} are removed
# 3. a new line with the specified ${prefix}${suffix} is appended
function append_or_replace_prefixed_line {
  local -r file="${1:-}"
  local -r prefix="${2:-}"
  local -r suffix="${3:-}"
  local -r dirname="$(dirname ${file})"
  local -r tmpfile="$(mktemp -t filtered.XXXX --tmpdir=${dirname})"

  touch "${file}"
  awk "substr(\$0,0,length(\"${prefix}\")) != \"${prefix}\" { print }" "${file}" > "${tmpfile}"
  echo "${prefix}${suffix}" >> "${tmpfile}"
  mv "${tmpfile}" "${file}"
}

# After the first boot and on upgrade, these files exist on the master-pd
# and should never be touched again (except perhaps an additional service
# account, see NB below.) One exception is if METADATA_CLOBBERS_CONFIG is
# enabled. In that case the basic_auth.csv file will be rewritten to make
# sure it matches the metadata source of truth.
function create-master-auth {
  echo "Creating master auth files"
  local -r auth_dir="/etc/srv/kubernetes"
  mkdir -p ${auth_dir}
  local -r basic_auth_csv="${auth_dir}/basic_auth.csv"
  if [[ -n "${KUBE_PASSWORD:-}" && -n "${KUBE_USER:-}" ]]; then
    if [[ -e "${basic_auth_csv}" && "${METADATA_CLOBBERS_CONFIG:-false}" == "true" ]]; then
      # If METADATA_CLOBBERS_CONFIG is true, we want to rewrite the file
      # completely, because if we're changing KUBE_USER and KUBE_PASSWORD, we
      # have nothing to match on.  The file is replaced just below with
      # append_or_replace_prefixed_line.
      rm "${basic_auth_csv}"
    fi
    append_or_replace_prefixed_line "${basic_auth_csv}" "${KUBE_PASSWORD},${KUBE_USER},"      "admin,system:masters"
  fi

  local -r known_tokens_csv="${auth_dir}/known_tokens.csv"
  if [[ -e "${known_tokens_csv}" && "${METADATA_CLOBBERS_CONFIG:-false}" == "true" ]]; then
    rm "${known_tokens_csv}"
  fi
  if [[ -n "${KUBE_BEARER_TOKEN:-}" ]]; then
    append_or_replace_prefixed_line "${known_tokens_csv}" "${KUBE_BEARER_TOKEN},"             "admin,admin,system:masters"
  fi
  if [[ -n "${WORKLOAD_CONTROLLER_MANAGER_TOKEN:-}" ]]; then
    append_or_replace_prefixed_line "${known_tokens_csv}" "${WORKLOAD_CONTROLLER_MANAGER_TOKEN}," "system:workload-controller-manager,uid:system:workload-controller-manager"
  fi
  if [[ -n "${KUBE_CLUSTER_AUTOSCALER_TOKEN:-}" ]]; then
    append_or_replace_prefixed_line "${known_tokens_csv}" "${KUBE_CLUSTER_AUTOSCALER_TOKEN}," "cluster-autoscaler,uid:cluster-autoscaler"
  fi
  if [[ -n "${NODE_PROBLEM_DETECTOR_TOKEN:-}" ]]; then
    append_or_replace_prefixed_line "${known_tokens_csv}" "${NODE_PROBLEM_DETECTOR_TOKEN},"   "system:node-problem-detector,uid:node-problem-detector"
  fi
  if [[ -n "${ADDON_MANAGER_TOKEN:-}" ]]; then
    append_or_replace_prefixed_line "${known_tokens_csv}" "${ADDON_MANAGER_TOKEN},"   "system:addon-manager,uid:system:addon-manager,system:masters"
  fi
  if [[ -n "${EXTRA_STATIC_AUTH_COMPONENTS:-}" ]]; then
    # Create a static Bearer token and kubeconfig for extra, comma-separated components.
    IFS="," read -r -a extra_components <<< "${EXTRA_STATIC_AUTH_COMPONENTS:-}"
    for extra_component in "${extra_components[@]}"; do
      local token="$(secure_random 32)"
      append_or_replace_prefixed_line "${known_tokens_csv}" "${token}," "system:${extra_component},uid:system:${extra_component}"
      create-kubeconfig "${extra_component}" "${token}"
    done
  fi
}

function create-kubeconfig {
  local component=$1
  local token=$2
  echo "Creating kubeconfig file for component ${component}"
  mkdir -p /etc/srv/kubernetes/${component}
  cat <<EOF >/etc/srv/kubernetes/${component}/kubeconfig
apiVersion: v1
kind: Config
users:
- name: ${component}
  user:
    token: ${token}
clusters:
- name: local
  cluster:
    insecure-skip-tls-verify: true
    server: https://localhost:${API_BIND_PORT}
contexts:
- context:
    cluster: local
    user: ${component}
  name: ${component}
current-context: ${component}
EOF
}

# Starts workload controller manager.
# It prepares the log file, loads the docker image, calculates variables, sets them
# in the manifest file, and then copies the manifest file to /etc/kubernetes/manifests.
#
# Assumed vars (which are calculated in function compute-master-manifest-variables)
#   CLOUD_CONFIG_OPT
#   CLOUD_CONFIG_VOLUME
#   CLOUD_CONFIG_MOUNT
#   DOCKER_REGISTRY
function start-workload-controller-manager {
  CLOUD_CONFIG_MOUNT=""
  CLOUD_CONFIG_VOLUME=""
  PV_RECYCLER_MOUNT=""
  PV_RECYCLER_VOLUME=""
  FLEXVOLUME_HOSTPATH_MOUNT=""
  FLEXVOLUME_HOSTPATH_VOLUME=""
  DOCKER_REGISTRY="k8s.gcr.io"
  WORKLOAD_CONTROLLER_MANAGER_CPU_REQUEST="${WORKLOAD_CONTROLLER_MANAGER_CPU_REQUEST:-200m}"

  echo "Starting workload controller-manager .."
  mkdir -p /etc/srv/kubernetes/workload-controller-manager
  cp /var/cache/kubernetes-install/workload-controllerconfig.json /etc/srv/kubernetes/workload-controller-manager/
  create-kubeconfig "workload-controller-manager" ${WORKLOAD_CONTROLLER_MANAGER_TOKEN:-""}
  prepare-log-file /var/log/workload-controller-manager.log
  # Calculate variables and assemble the command line.
  local params="${WORKLOAD_CONTROLLER_MANAGER_TEST_LOG_LEVEL:-"--v=2"}"
  params+=" --controllerconfig=/etc/srv/kubernetes/workload-controller-manager/workload-controllerconfig.json"
  params+=" --kubeconfig=/etc/srv/kubernetes/workload-controller-manager/kubeconfig"

  # Disable using HPA metrics REST clients if metrics-server isn't enabled,
  # or if we want to explicitly disable it by setting HPA_USE_REST_CLIENT.

  local -r kube_rc_docker_tag=$(cat ${INSTALL_DIR}/kubernetes/server/bin/workload-controller-manager.docker_tag)
  local container_env=""
  if [[ -n "${ENABLE_CACHE_MUTATION_DETECTOR:-}" ]]; then
    container_env="\"env\":[{\"name\": \"KUBE_CACHE_MUTATION_DETECTOR\", \"value\": \"${ENABLE_CACHE_MUTATION_DETECTOR}\"}],"
  fi

  local -r src_file="/var/cache/kubernetes-install/workload-controller-manager.manifest"
  # Evaluate variables.
  sed -i -e "s@{{pillar\['kube_docker_registry'\]}}@${DOCKER_REGISTRY}@g" "${src_file}"
  sed -i -e "s@{{pillar\['workload-controller-manager_docker_tag'\]}}@${kube_rc_docker_tag}@g" "${src_file}"
  sed -i -e "s@{{params}}@${params}@g" "${src_file}"
  sed -i -e "s@{{container_env}}@${container_env}@g" ${src_file}
  sed -i -e "s@{{cloud_config_mount}}@${CLOUD_CONFIG_MOUNT}@g" "${src_file}"
  sed -i -e "s@{{cloud_config_volume}}@${CLOUD_CONFIG_VOLUME}@g" "${src_file}"
  sed -i -e "s@{{additional_cloud_config_mount}}@@g" "${src_file}"
  sed -i -e "s@{{additional_cloud_config_volume}}@@g" "${src_file}"
  sed -i -e "s@{{pv_recycler_mount}}@${PV_RECYCLER_MOUNT}@g" "${src_file}"
  sed -i -e "s@{{pv_recycler_volume}}@${PV_RECYCLER_VOLUME}@g" "${src_file}"
  sed -i -e "s@{{flexvolume_hostpath_mount}}@${FLEXVOLUME_HOSTPATH_MOUNT}@g" "${src_file}"
  sed -i -e "s@{{flexvolume_hostpath}}@${FLEXVOLUME_HOSTPATH_VOLUME}@g" "${src_file}"
  sed -i -e "s@{{cpurequest}}@${WORKLOAD_CONTROLLER_MANAGER_CPU_REQUEST}@g" "${src_file}"

  cp "${src_file}" /etc/kubernetes/manifests
}

function setup-kubelet() {
  cat > /lib/systemd/system/kubelet.service <<EOF
[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=https://kubernetes.io/docs/home/

[Service]
ExecStart=/usr/bin/kubelet
Restart=always
StartLimitInterval=0
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

  mkdir -p /etc/systemd/system/kubelet.service.d
  cat > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf <<EOF
# Note: This dropin only works with kubeadm and kubelet v1.11+
[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml"
# This is a file that "kubeadm init" and "kubeadm join" generates at runtime, populating the KUBELET_KUBEADM_ARGS variable dynamically
EnvironmentFile=-/var/lib/kubelet/kubeadm-flags.env
# This is a file that the user can use for overrides of the kubelet args as a last resort. Preferably, the user should use
# the .NodeRegistration.KubeletExtraArgs object in the configuration files instead. KUBELET_EXTRA_ARGS should be sourced from this file.
EnvironmentFile=-/etc/default/kubelet
ExecStart=
ExecStart=/usr/bin/kubelet \$KUBELET_KUBECONFIG_ARGS \$KUBELET_CONFIG_ARGS \$KUBELET_KUBEADM_ARGS \$KUBELET_EXTRA_ARGS
EOF

  ln -s -f /lib/systemd/system/kubelet.service /etc/systemd/system/multi-user.target.wants/kubelet.service
}

function setup-kubernetes-master() {
  setup-kubelet

  if [[ -z "$KUBERNETES_MASTER_NAME" ]]; then
    KUBE_NODE_NAME=`hostname`
  else
    KUBE_NODE_NAME=$KUBERNETES_MASTER_NAME
  fi

  echo "Setting up kubernetes master $KUBE_NODE_NAME: Version $KUBE_VER ExtIP: $MASTER_EXTERNAL_IP Port: $API_BIND_PORT."

  local init_phases="control-plane,etcd"
  local skip_phases="--skip-phases=${init_phases}"
  if [[ ${NETWORK_PROVIDER:-flannel} == "bridge" ]]; then
    skip_phases="--skip-phases=${init_phases},addon"
  fi
  local feature_gates=""
  if [[ ! -z ${FEATURE_GATES:-""} ]]; then
    feature_gates="feature-gates=${FEATURE_GATES}"
  fi
  local pod_net_cidr=""
  if [[ ! -z ${POD_NETWORK_CIDR} ]]; then
    pod_net_cidr="--pod-network-cidr=$POD_NETWORK_CIDR"
  fi

  kubeadm init phase control-plane apiserver --apiserver-bind-port=$API_BIND_PORT --kubernetes-version=$KUBE_VER --apiserver-extra-args=$feature_gates
  kubeadm init phase control-plane controller-manager --kubernetes-version=$KUBE_VER $pod_net_cidr --controller-manager-extra-args=$feature_gates
  kubeadm init phase control-plane scheduler --kubernetes-version=$KUBE_VER --scheduler-extra-args=$feature_gates
  kubeadm init phase etcd local

  sed -i "/listen-client-urls=/ s/$/,http:\/\/127.0.0.1:2382/" /etc/kubernetes/manifests/etcd.yaml
  sed -i "/- kube-apiserver/a \ \ \ \ - --token-auth-file=\/etc\/srv\/kubernetes\/known_tokens.csv" /etc/kubernetes/manifests/kube-apiserver.yaml
  sed -i "/- kube-apiserver/a \ \ \ \ - --basic-auth-file=\/etc\/srv\/kubernetes\/basic_auth.csv" /etc/kubernetes/manifests/kube-apiserver.yaml
  sed -i "/volumeMounts:/a \ \ \ \ - mountPath: \/etc\/srv\/kubernetes\n      name: etc-srv-kubernetes\n      readOnly: true" /etc/kubernetes/manifests/kube-apiserver.yaml
  sed -i "/volumes:/a \ \ - hostPath:\n      path: \/etc\/srv\/kubernetes\n      type: DirectoryOrCreate\n    name: etc-srv-kubernetes" /etc/kubernetes/manifests/kube-apiserver.yaml
  if [[ ! -z $feature_gates ]]; then
    sed -i "/KUBELET_CONFIG_ARGS=/a Environment=\"KUBELET_EXTRA_ARGS=--feature-gates=${FEATURE_GATES}\"" /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
    feature_gates="--$feature_gates"
  fi
  systemctl daemon-reload

  kubeadm init --node-name=$KUBE_NODE_NAME --apiserver-bind-port=$API_BIND_PORT --apiserver-cert-extra-sans=$MASTER_EXTERNAL_IP \
               --ignore-preflight-errors=all $skip_phases $pod_net_cidr &> /etc/kubernetes/kubeadm-init-log
  if [ $? -eq 0 ]; then
    echo "kubeadm init successful."
    sudo mkdir -p /root/.kube
    sudo cp -i /etc/kubernetes/admin.conf /root/.kube/config
    sudo chown $(id -u):$(id -g) /root/.kube/config
    sudo mkdir -p /home/ubuntu/.kube
    sudo cp -i /etc/kubernetes/admin.conf /home/ubuntu/.kube/config
    sudo chown ubuntu:ubuntu /home/ubuntu/.kube/config

    start-workload-controller-manager

    sudo mkdir -p /etc/kubernetes/kubeadm_join_cmd
    local line1=$(sudo cat /etc/kubernetes/kubeadm-init-log | grep "kubeadm join" | cut -d\\ -f1)
    local line2=$(sudo cat /etc/kubernetes/kubeadm-init-log | grep "discovery-token")
    sudo echo "$line1 $line2" > /etc/kubernetes/kubeadm_join_cmd/join_string
    pushd /etc/kubernetes/kubeadm_join_cmd
    python -m SimpleHTTPServer 8085 &
    popd
  else
    echo "kubeadm init failed. Error: $?"
    exit $?
  fi
}

function setup-kubernetes-worker() {
  setup-kubelet

  if [[ ! -z ${FEATURE_GATES:-""} ]]; then
    sed -i "/KUBELET_CONFIG_ARGS=/a Environment=\"KUBELET_EXTRA_ARGS=--feature-gates=${FEATURE_GATES}\"" /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
  fi
  systemctl daemon-reload

  KUBE_MASTER_IP=`cat /etc/kubernetes/kube_env.yaml | grep API_SERVERS | cut -d\' -f2`
  echo "Setting up kubernetes worker: version $KUBE_VER master IP: $KUBE_MASTER_IP."

  pushd /etc/kubernetes
  local start_time=$(date +%s)
  until wget http://$KUBE_MASTER_IP:8085/join_string &>/dev/null; do
    echo "Waiting for kubeadm join command .."
    sleep 5
    local elapsed=$(($(date +%s) - ${start_time}))
    if [[ ${elapsed} -gt 300 ]]; then
      echo
      echo "Waiting for kubeadm join command failed after 5 minutes elapsed"
      exit 1
    fi
  done
  local kubeadm_join_cmd=$(cat /etc/kubernetes/join_string)
  popd

  # Run kubeadm join
  echo "Executing join cmd: '$kubeadm_join_cmd'"
  $kubeadm_join_cmd
  if [ $? -eq 0 ]; then
    echo "kubeadm join successful."
  else
    echo "kubeadm join failed. Error: $?"
    exit $?
  fi

  mkdir -p /etc/kubernetes/manifests
}

####################################################################################

WORKLOAD_CONTROLLER_MANAGER_TOKEN="$(secure_random 32)"
KUBE_CLUSTER_AUTOSCALER_TOKEN="$(secure_random 32)"
ADDON_MANAGER_TOKEN="$(secure_random 32)"

if [[ -z "${is_push}" ]]; then
  echo "== kube-up node config starting =="
  set-broken-motd
  enable-root-ssh
  ensure-basic-networking
  ensure-install-dir
  ensure-packages
  set-kube-env
  setup-cni-network-conf
  if [[ "${KUBERNETES_MASTER}" == "true" ]]; then
    mount-master-pd
    ensure-apparmor-service
    create-master-auth
  fi
  ensure-docker
  ensure-containerd
  download-release
  unpack-kubernetes
  if [[ "${KUBERNETES_MASTER}" == "true" ]]; then
    if [[ "${KUBEMARK_MASTER}" == "false" ]]; then
      # Start master
      setup-kubernetes-master
    else
      echo "Skipping kubeadm master setup for kubemark master."
    fi
  else
    # Start worker
    setup-kubernetes-worker
  fi
  remove-docker-artifacts
  reset-motd

  run-user-script
  echo "== kube-up node config done =="
else
  echo "== kube-push node config starting =="
  ensure-basic-networking
  ensure-install-dir
  set-kube-env
  download-release
  reset-motd
  echo "== kube-push node config done =="
fi
