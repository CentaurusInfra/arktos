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
KUBE_MASTER_EIP=""
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

function create-node-pki {
  echo "Creating node pki files"

  local -r pki_dir="/etc/kubernetes/pki"
  mkdir -p "${pki_dir}"

  if [[ -z "${CA_CERT_BUNDLE:-}" ]]; then
    CA_CERT_BUNDLE="${CA_CERT}"
  fi

  CA_CERT_BUNDLE_PATH="${pki_dir}/ca-certificates.crt"
  echo "${CA_CERT_BUNDLE}" | base64 --decode > "${CA_CERT_BUNDLE_PATH}"

  if [[ ! -z "${KUBELET_CERT:-}" && ! -z "${KUBELET_KEY:-}" ]]; then
    KUBELET_CERT_PATH="${pki_dir}/kubelet.crt"
    echo "${KUBELET_CERT}" | base64 --decode > "${KUBELET_CERT_PATH}"

    KUBELET_KEY_PATH="${pki_dir}/kubelet.key"
    echo "${KUBELET_KEY}" | base64 --decode > "${KUBELET_KEY_PATH}"
  fi

  # TODO(mikedanese): remove this when we don't support downgrading to versions
  # < 1.6.
  ln -sf "${CA_CERT_BUNDLE_PATH}" /etc/kubernetes/ca.crt
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

# Restart any services that need restarting due to a library upgrade
# Uses needrestart
restart-updated-services() {
  # We default to restarting services, because this is only done as part of an update
  if [[ "${AUTO_RESTART_SERVICES:-true}" != "true" ]]; then
    echo "Auto restart of services prevented by AUTO_RESTART_SERVICES=${AUTO_RESTART_SERVICES}"
    return
  fi
  echo "Restarting services with updated libraries (needrestart -r a)"
  # The pipes make sure that needrestart doesn't think it is running with a TTY
  # Debian bug #803249; fixed but not necessarily in package repos yet
  echo "" | needrestart -r a 2>&1 | tee /dev/null
}

# Reboot the machine if /var/run/reboot-required exists
reboot-if-required() {
  if [[ ! -e "/var/run/reboot-required" ]]; then
    return
  fi

  echo "Reboot is required (/var/run/reboot-required detected)"
  if [[ -e "/var/run/reboot-required.pkgs" ]]; then
    echo "Packages that triggered reboot:"
    cat /var/run/reboot-required.pkgs
  fi

  # We default to rebooting the machine because this is only done as part of an update
  if [[ "${AUTO_REBOOT:-true}" != "true" ]]; then
    echo "Reboot prevented by AUTO_REBOOT=${AUTO_REBOOT}"
    return
  fi

  rm -f /var/run/reboot-required
  rm -f /var/run/reboot-required.pkgs
  echo "Triggering reboot"
  init 6
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

# The job of this function is simple, but the basic regular expression syntax makes
# this difficult to read. What we want to do is convert from [0-9]+B, KB, KiB, MB, etc
# into [0-9]+, Ki, Mi, Gi, etc.
# This is done in two steps:
#   1. Convert from [0-9]+X?i?B into [0-9]X? (X denotes the prefix, ? means the field
#      is optional.
#   2. Attach an 'i' to the end of the string if we find a letter.
# The two step process is needed to handle the edge case in which we want to convert
# a raw byte count, as the result should be a simple number (e.g. 5B -> 5).
function convert-bytes-gce-kube() {
  local -r storage_space=$1
  echo "${storage_space}" | sed -e 's/^\([0-9]\+\)\([A-Z]\)\?i\?B$/\1\2/g' -e 's/\([A-Z]\)$/\1i/'
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
  download-or-bust "${server_binary_tar_hash}" "${server_binary_tar_urls[@]}"

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

function node-docker-opts() {
  if [[ -n "${EXTRA_DOCKER_OPTS-}" ]]; then
    DOCKER_OPTS="${DOCKER_OPTS:-} ${EXTRA_DOCKER_OPTS}"
  fi

  # Decide whether to enable a docker registry mirror. This is taken from
  # the "kube-env" metadata value.
  if [[ -n "${DOCKER_REGISTRY_MIRROR_URL:-}" ]]; then
    echo "Enable docker registry mirror at: ${DOCKER_REGISTRY_MIRROR_URL}"
    DOCKER_OPTS="${DOCKER_OPTS:-} --registry-mirror=${DOCKER_REGISTRY_MIRROR_URL}"
  fi
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

  # Start AppArmor service before we have scripts to configure it properly
  if ! sudo systemctl is-active --quiet apparmor; then
    echo "Starting Apparmor service"
    sudo systemctl start apparmor
  fi
}

function setup-flannel-cni-conf() {
  mkdir -p /etc/cni/net.d
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
  rm -rf /tmp/k8s_install
  mkdir -p /tmp/k8s_install
  pushd /tmp/k8s_install
  cp -p /usr/share/google/kubernetes-server-linux-amd64.tar.gz .
  tar xf kubernetes-server-linux-amd64.tar.gz
  cp -p ./kubernetes/server/bin/kubelet /usr/bin/
  cp -p ./kubernetes/server/bin/kubectl /usr/bin/
  cp -p ./kubernetes/server/bin/kubeadm /usr/bin/
  KUBE_VER=`cat ./kubernetes/server/bin/kube-apiserver.docker_tag`
  if [[ "${KUBERNETES_MASTER}" == "true" ]]; then
    img_bins=(kube-apiserver kube-controller-manager kube-scheduler)
    for img in "${img_bins[@]}"; do
      echo "Loading docker image for $img"
      sudo docker load -i ./kubernetes/server/bin/$img.tar
    done
  fi
  echo "Loading docker image for kube-proxy"
  sudo docker load -i ./kubernetes/server/bin/kube-proxy.tar
  rm -rf /tmp/k8s_install/kubernetes
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

  KUBE_MASTER_EIP=`cat /etc/kubernetes/kube_env.yaml | grep MASTER_EIP | cut -d\' -f2`
  echo "Setting up kubernetes master: version $KUBE_VER EIP: $KUBE_MASTER_EIP."

  # Run kubeadm init - TODO: take pod-network-cidr and networking yaml as a params
  kubeadm init --pod-network-cidr=10.244.0.0/16 --kubernetes-version=$KUBE_VER --apiserver-cert-extra-sans=$KUBE_MASTER_EIP &> /etc/kubernetes/kubeadm-init-log
  if [ $? -eq 0 ]; then
    echo "kubeadm init successful."
    sudo mkdir -p /root/.kube
    sudo cp -i /etc/kubernetes/admin.conf /root/.kube/config
    sudo chown $(id -u):$(id -g) /root/.kube/config
    sudo mkdir -p /home/ubuntu/.kube
    sudo cp -i /etc/kubernetes/admin.conf /home/ubuntu/.kube/config
    sudo chown ubuntu:ubuntu /home/ubuntu/.kube/config

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

  KUBE_MASTER_IP=`cat /etc/kubernetes/kube_env.yaml | grep API_SERVERS | cut -d\' -f2`
  echo "Setting up kubernetes worker: version $KUBE_VER master IP: $KUBE_MASTER_IP."

  pushd /etc/kubernetes
  until wget http://$KUBE_MASTER_IP:8085/join_string &>/dev/null; do
    echo "Waiting for kubeadm join command .."
    sleep 5
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

# This script is re-used on AWS.  Some of the above functions will be replaced.
# The AWS kube-up script looks for this marker:
#+AWS_OVERRIDES_HERE

####################################################################################

if [[ -z "${is_push}" ]]; then
  echo "== kube-up node config starting =="
  set-broken-motd
  enable-root-ssh
  ensure-basic-networking
  ensure-install-dir
  ensure-packages
  set-kube-env
  setup-flannel-cni-conf
  if [[ "${KUBERNETES_MASTER}" == "true" ]]; then
    mount-master-pd
    ensure-apparmor-service
  fi
  create-node-pki
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
