#!/usr/bin/env bash

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

# sanity check for OpenStack provider
if [ "${CLOUD_PROVIDER}" == "openstack" ]; then
    if [ "${CLOUD_CONFIG}" == "" ]; then
        echo "Missing CLOUD_CONFIG env for OpenStack provider!"
        exit 1
    fi
    if [ ! -f "${CLOUD_CONFIG}" ]; then
        echo "Cloud config ${CLOUD_CONFIG} doesn't exist"
        exit 1
    fi
fi

# set feature gates if enable Pod priority and preemption
if [ "${ENABLE_POD_PRIORITY_PREEMPTION}" == true ]; then
    FEATURE_GATES="${FEATURE_GATES},PodPriority=true"
fi

# warn if users are running with swap allowed
if [ "${FAIL_SWAP_ON}" == "false" ]; then
    echo "WARNING : The kubelet is configured to not fail even if swap is enabled; production deployments should disable swap."
fi

if [ "$(id -u)" != "0" ]; then
    echo "WARNING : This script MAY be run as root for docker socket / iptables functionality; if failures occur, retry as root." 2>&1
fi

# Stop right away if the build fails
set -e

# Do dudiligence to ensure containerd service and socket in a working state
# Containerd service should be part of docker.io installation or apt-get install containerd for Ubuntu OS
if ! sudo systemctl is-active --quiet containerd; then
  echo "Containerd is required for Arktos"
  exit 1
fi

if [[ ! -e "${CONTAINERD_SOCK_PATH}" ]]; then
  echo "Containerd socket file check failed. Please check containerd socket file path"
  exit 1
fi

# local function to download runtime deployment file
copyRuntimeDeploymentFile() {
  if [[ $# != 2 ]]; then
    echo "Invalid args in copyRuntimeDeploymentFile"
    exit 1
  fi

  fileName=$1
  pathInSrc=$2
  if [[ (${OVERWRITE_DEPLOYMENT_FILES} == "true") || (! -f ${VIRTLET_DEPLOYMENT_FILES_DIR}/${fileName}) ]]; then 
    echo "Getting runtime deployment file " ${fileName}
    wget --no-check-certificate -O ${VIRTLET_DEPLOYMENT_FILES_DIR}/${fileName}  ${VIRTLET_DEPLOYMENT_FILES_SRC}/${pathInSrc}
  fi
}

# Get runtime deployment files
copyRuntimeDeploymentFile "libvirt-qemu" "apparmor/libvirt-qemu"
copyRuntimeDeploymentFile "libvirtd" "apparmor/libvirtd"
copyRuntimeDeploymentFile "virtlet" "apparmor/virtlet"
copyRuntimeDeploymentFile "vms" "apparmor/vms"
copyRuntimeDeploymentFile "vmruntime.yaml" "data/virtlet-ds.yaml"
copyRuntimeDeploymentFile "images.yaml" "images.yaml"

if [ "${APPARMOR_ENABLED}" == "true" ]; then
  echo "Config test env under apparmor enabled host"
  # Start AppArmor service before we have scripts to configure it properly
  if ! sudo systemctl is-active --quiet apparmor; then
    echo "Starting Apparmor service"
    sudo systemctl start apparmor
  fi

  # install runtime apparmor profiles and reload apparmor
  echo "Installing arktos runtime apparmor profiles"
  cp ${VIRTLET_DEPLOYMENT_FILES_DIR}/libvirt-qemu /etc/apparmor.d/abstractions/
  sudo install -m 0644 ${VIRTLET_DEPLOYMENT_FILES_DIR}/libvirtd ${VIRTLET_DEPLOYMENT_FILES_DIR}/virtlet ${VIRTLET_DEPLOYMENT_FILES_DIR}/vms -t /etc/apparmor.d/
  sudo apparmor_parser -r /etc/apparmor.d/libvirtd
  sudo apparmor_parser -r /etc/apparmor.d/virtlet
  sudo apparmor_parser -r /etc/apparmor.d/vms
  echo "Completed"
  echo "Setting annotations for the runtime daemonset"
  sed -i 's+apparmorlibvirtname+container.apparmor.security.beta.kubernetes.io/libvirt+g' ${VIRTLET_DEPLOYMENT_FILES_DIR}/vmruntime.yaml
  sed -i 's+apparmorlibvirtvalue+localhost/libvirtd+g' ${VIRTLET_DEPLOYMENT_FILES_DIR}/vmruntime.yaml
  sed -i 's+apparmorvmsname+container.apparmor.security.beta.kubernetes.io/vms+g' ${VIRTLET_DEPLOYMENT_FILES_DIR}/vmruntime.yaml
  sed -i 's+apparmorvmsvalue+localhost/vms+g' ${VIRTLET_DEPLOYMENT_FILES_DIR}/vmruntime.yaml
  sed -i 's+apparmorvirtletname+container.apparmor.security.beta.kubernetes.io/virtlet+g' ${VIRTLET_DEPLOYMENT_FILES_DIR}/vmruntime.yaml
  sed -i 's+apparmorvirtletvalue+localhost/virtlet+g' ${VIRTLET_DEPLOYMENT_FILES_DIR}/vmruntime.yaml
  echo "Completed"
else
  echo "Stopping Apparmor service"
  sudo systemctl stop apparmor
fi

ARKTOS_NETWORK_TEMPLATE=${ARKTOS_NETWORK_TEMPLATE:-}
DEFAULT_FLAT_NETWORK_TEMPLATE=${KUBE_ROOT}/hack/runtime/default_flat_network.json
DEFAULT_MIZAR_NETWORK_TEMPLATE=${KUBE_ROOT}/hack/runtime/default_mizar_network.json
if [ "${ARKTOS_NETWORK_TEMPLATE}" == "flat" ]; then 
  ARKTOS_NETWORK_TEMPLATE=${DEFAULT_FLAT_NETWORK_TEMPLATE}
fi
if [ "${ARKTOS_NETWORK_TEMPLATE}" == "mizar" ]; then 
  ARKTOS_NETWORK_TEMPLATE=${DEFAULT_MIZAR_NETWORK_TEMPLATE}
fi
if [ -n "${ARKTOS_NETWORK_TEMPLATE}" ] && [ ! -f "${ARKTOS_NETWORK_TEMPLATE}" ]; then 
  printf "\033[1;33m\nWarning: could not find newtork template file ${ARKTOS_NETWORK_TEMPLATE}. Setting ARKTOS_NETWORK_TEMPLATE to empty.\n\033[0m"
  ARKTOS_NETWORK_TEMPLATE=""
fi

source "${KUBE_ROOT}/hack/lib/util.sh"

function kube::common::detect_binary {
    # Detect the OS name/arch so that we can find our binary
    case "$(uname -s)" in
      Darwin)
        host_os=darwin
        ;;
      Linux)
        host_os=linux
        ;;
      *)
        echo "Unsupported host OS.  Must be Linux or Mac OS X." >&2
        exit 1
        ;;
    esac

    case "$(uname -m)" in
      x86_64*)
        host_arch=amd64
        ;;
      i?86_64*)
        host_arch=amd64
        ;;
      amd64*)
        host_arch=amd64
        ;;
      aarch64*)
        host_arch=arm64
        ;;
      arm64*)
        host_arch=arm64
        ;;
      arm*)
        host_arch=arm
        ;;
      i?86*)
        host_arch=x86
        ;;
      s390x*)
        host_arch=s390x
        ;;
      ppc64le*)
        host_arch=ppc64le
        ;;
      *)
        echo "Unsupported host arch. Must be x86_64, 386, arm, arm64, s390x or ppc64le." >&2
        exit 1
        ;;
    esac

   GO_OUT="${KUBE_ROOT}/_output/local/bin/${host_os}/${host_arch}"
}


# This function guesses where the existing cached binary build is for the `-O`
# flag
function kube::common::guess_built_binary_path {
  local hyperkube_path
  hyperkube_path=$(kube::util::find-binary "hyperkube")
  if [[ -z "${hyperkube_path}" ]]; then
    return
  fi
  echo -n "$(dirname "${hyperkube_path}")"
}

function kube::common::set_service_accounts {
    SERVICE_ACCOUNT_LOOKUP=${SERVICE_ACCOUNT_LOOKUP:-true}
    SERVICE_ACCOUNT_KEY=${SERVICE_ACCOUNT_KEY:-/tmp/kube-serviceaccount.key}
    # Generate ServiceAccount key if needed
    if [[ ! -f "${SERVICE_ACCOUNT_KEY}" ]]; then
      mkdir -p "$(dirname "${SERVICE_ACCOUNT_KEY}")"
      openssl genrsa -out "${SERVICE_ACCOUNT_KEY}" 2048 2>/dev/null
    fi
}

function kube::common::generate_certs {
    # Create CA signers
    # no need to regenerate CA in every run - optimize
    if [ "${REGENERATE_CA:-}" = true ] || ! [ -e "${CERT_DIR}/server-ca.key" ] || ! [ -e "${CERT_DIR}/server-ca.crt" ] ||
       ! [ -e "${CERT_DIR}/client-ca.key" ] || ! [ -e "${CERT_DIR}/client-ca.crt" ] ||
       ! [ -e "${CERT_DIR}/server-ca-config.json" ] || ! [ -e "${CERT_DIR}/client-ca-config.json" ]; then
      if [[ "${ENABLE_SINGLE_CA_SIGNER:-}" = true ]]; then
          kube::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" server '"client auth","server auth"'
          sudo cp "${CERT_DIR}/server-ca.key" "${CERT_DIR}/client-ca.key"
          sudo cp "${CERT_DIR}/server-ca.crt" "${CERT_DIR}/client-ca.crt"
          sudo cp "${CERT_DIR}/server-ca-config.json" "${CERT_DIR}/client-ca-config.json"
      else
          kube::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" server '"server auth"'
          kube::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" client '"client auth"'
      fi
    else
      echo "Skip generating CA as CA files existed and REGENERATE_CA != true. To regenerate CA files, export REGENERATE_CA=true"
    fi

    # Create auth proxy client ca
    kube::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" request-header '"client auth"'

    # serving cert for kube-apiserver
    kube::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "server-ca" kube-apiserver kubernetes.default kubernetes.default.svc "localhost" "${API_HOST_IP}" "${API_HOST}" "${FIRST_SERVICE_CLUSTER_IP}" "${API_HOST_IP_EXTERNAL}" "${APISERVERS_EXTRA:-}" "${PUBLIC_IP:-}"

    # Create client certs signed with client-ca, given id, given CN and a number of groups
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' controller system:kube-controller-manager
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' workload-controller system:workload-controller-manager
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' scheduler  system:kube-scheduler
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' admin system:admin system:masters
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' kube-apiserver kube-apiserver

    # Create matching certificates for kube-aggregator
    kube::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "server-ca" kube-aggregator api.kube-public.svc "${API_HOST}" "${API_HOST_IP}"
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" request-header-ca auth-proxy system:auth-proxy

    # TODO remove masters and add rolebinding
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' kube-aggregator system:kube-aggregator system:masters
    kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" kube-aggregator
}

function kube::common::start_apiserver()  {
    CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")
    
    #Increment ports to enable running muliple kube-apiserver simultaneously
    secureport="$(($1 + ${API_SECURE_PORT}))"
    insecureport="$(($1 + ${API_PORT}))"

    # Increment logs to enable each kube-apiserver have own log files
    apiserverlog="kube-apiserver$1.log"
    apiserverauditlog="kube-apiserver-audit$1.log"

    # Create apiservern.config for kube-apiserver partition
    configsuffix="$(($1 + 1))"
    configfilepath="${PARTITION_CONFIG_DIR}apiserver.config"
    ${CONTROLPLANE_SUDO} rm -f  $configfilepath
    ${CONTROLPLANE_SUDO} cp hack/apiserver.config $configfilepath
    echo "Copied the apiserver partition config file  $configfilepath..."

    security_admission=""
    if [[ -n "${DENY_SECURITY_CONTEXT_ADMISSION}" ]]; then
      security_admission=",SecurityContextDeny"
    fi
    if [[ -n "${PSP_ADMISSION}" ]]; then
      security_admission=",PodSecurityPolicy"
    fi
    if [[ -n "${NODE_ADMISSION}" ]]; then
      security_admission=",NodeRestriction"
    fi
    if [ "${ENABLE_POD_PRIORITY_PREEMPTION}" == true ]; then
      security_admission=",Priority"
      if [[ -n "${RUNTIME_CONFIG}" ]]; then
          RUNTIME_CONFIG+=","
      fi
      RUNTIME_CONFIG+="scheduling.k8s.io/v1alpha1=true"
    fi

    # Append security_admission plugin
    ENABLE_ADMISSION_PLUGINS="${ENABLE_ADMISSION_PLUGINS}${security_admission}"

    authorizer_arg=""
    if [[ -n "${AUTHORIZATION_MODE}" ]]; then
      authorizer_arg="--authorization-mode=${AUTHORIZATION_MODE}"
    fi
    priv_arg=""
    if [[ -n "${ALLOW_PRIVILEGED}" ]]; then
      priv_arg="--allow-privileged=${ALLOW_PRIVILEGED}"
    fi

    runtime_config=""
    if [[ -n "${RUNTIME_CONFIG}" ]]; then
      runtime_config="--runtime-config=${RUNTIME_CONFIG}"
    fi

    # Let the API server pick a default address when API_HOST_IP
    # is set to 127.0.0.1
    advertise_address=""
    if [[ "${API_HOST_IP}" != "127.0.0.1" ]]; then
        advertise_address="--advertise-address=${API_HOST_IP}"
    fi
    if [[ "${ADVERTISE_ADDRESS}" != "" ]] ; then
        advertise_address="--advertise-address=${ADVERTISE_ADDRESS}"
    fi
    node_port_range=""
    if [[ "${NODE_PORT_RANGE}" != "" ]] ; then
        node_port_range="--service-node-port-range=${NODE_PORT_RANGE}"
    fi

    service_group_id=""
    if [[ "${APISERVER_SERVICEGROUPID}" != "" ]]; then
      service_group_id="--service-group-id=${APISERVER_SERVICEGROUPID}"
    fi

    if [[ "${REUSE_CERTS}" != true ]]; then
      # Create Certs
      kube::common::generate_certs
    fi

    cloud_config_arg="--cloud-provider=${CLOUD_PROVIDER} --cloud-config=${CLOUD_CONFIG}"
    if [[ "${EXTERNAL_CLOUD_PROVIDER:-}" == "true" ]]; then
      cloud_config_arg="--cloud-provider=external"
    fi

    if [[ -n "${AUDIT_POLICY_FILE}" ]]; then
      cat <<EOF > /tmp/kube-audit-policy-file$i
# Log all requests at the Metadata level.
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
EOF
      AUDIT_POLICY_FILE="/tmp/kube-audit-policy-file$i"
    fi

    APISERVER_LOG=${LOG_DIR}/$apiserverlog
    ${CONTROLPLANE_SUDO} "${GO_OUT}/hyperkube" kube-apiserver "${authorizer_arg}" "${priv_arg}" ${runtime_config} \
      ${cloud_config_arg} \
      "${advertise_address}" \
      "${node_port_range}" \
      --v="${LOG_LEVEL}" \
      --vmodule="${LOG_SPEC}" \
      --audit-policy-file="${AUDIT_POLICY_FILE}" \
      --audit-log-path="${LOG_DIR}/$apiserverauditlog" \
      --cert-dir="${CERT_DIR}" \
      --client-ca-file="${CERT_DIR}/client-ca.crt" \
      --kubelet-client-certificate="${CERT_DIR}/client-kube-apiserver.crt" \
      --kubelet-client-key="${CERT_DIR}/client-kube-apiserver.key" \
      --service-account-key-file="${SERVICE_ACCOUNT_KEY}" \
      --service-account-lookup="${SERVICE_ACCOUNT_LOOKUP}" \
      --enable-admission-plugins="${ENABLE_ADMISSION_PLUGINS}" \
      --disable-admission-plugins="${DISABLE_ADMISSION_PLUGINS}" \
      --admission-control-config-file="${ADMISSION_CONTROL_CONFIG_FILE}" \
      --bind-address="${API_BIND_ADDR}" \
      --secure-port=$secureport \
      --tls-cert-file="${CERT_DIR}/serving-kube-apiserver.crt" \
      --tls-private-key-file="${CERT_DIR}/serving-kube-apiserver.key" \
      --insecure-bind-address="${API_HOST_IP}" \
      --insecure-port=$insecureport \
      --storage-backend="${STORAGE_BACKEND}" \
      --storage-media-type="${STORAGE_MEDIA_TYPE}" \
      --etcd-servers="http://${ETCD_HOST}:${ETCD_PORT}" \
      --service-cluster-ip-range="${SERVICE_CLUSTER_IP_RANGE}" \
      --feature-gates="${FEATURE_GATES}" \
      --external-hostname="${EXTERNAL_HOSTNAME}" \
      --requestheader-username-headers=X-Remote-User \
      --requestheader-group-headers=X-Remote-Group \
      --requestheader-extra-headers-prefix=X-Remote-Extra- \
      --requestheader-client-ca-file="${CERT_DIR}/request-header-ca.crt" \
      --requestheader-allowed-names=system:auth-proxy \
      --proxy-client-cert-file="${CERT_DIR}/client-auth-proxy.crt" \
      --proxy-client-key-file="${CERT_DIR}/client-auth-proxy.key" \
      ${service_group_id} \
      --partition-config="${configfilepath}" \
      --profiling=true \
      --contention-profiling=true \
      --cors-allowed-origins="${API_CORS_ALLOWED_ORIGINS}" >"${APISERVER_LOG}" 2>&1 &
    APISERVER_PID=$!
    APISERVER_PID_ARRAY+=($APISERVER_PID)
    # Wait for kube-apiserver to come up before launching the rest of the components.
    echo "Waiting for apiserver to come up"
    kube::util::wait_for_url "https://${API_HOST_IP}:$secureport/healthz" "apiserver: " 1 "${WAIT_FOR_URL_API_SERVER}" "${MAX_TIME_FOR_URL_API_SERVER}" \
        || { echo "check apiserver logs: ${APISERVER_LOG}" ; exit 1 ; }

    if [[ "${REUSE_CERTS}" != true ]]; then
        # Create kubeconfigs for all components, using client certs
        # TODO: Each api server has it own configuration files. However, since clients, such as controller, scheduler and etc do not support mutilple apiservers,admin.kubeconfig is kept for compability.
        ADMIN_CONFIG_API_HOST=${PUBLIC_IP:-${API_HOST}}
        kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${ADMIN_CONFIG_API_HOST}" "$secureport" admin
        ${CONTROLPLANE_SUDO} chown "${USER}" "${CERT_DIR}/client-admin.key" # make readable for kubectl
        kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${API_HOST}" "$secureport" controller
        kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${API_HOST}" "$secureport" scheduler
        kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${API_HOST}" "$secureport" workload-controller

        # Move the admin kubeconfig for each apiserver
        ${CONTROLPLANE_SUDO} cp "${CERT_DIR}/admin.kubeconfig" "${CERT_DIR}/admin$1.kubeconfig"
        ${CONTROLPLANE_SUDO} cp "${CERT_DIR}/workload-controller.kubeconfig" "${CERT_DIR}/workload-controller$1.kubeconfig"

        if [[ -z "${AUTH_ARGS}" ]]; then
            AUTH_ARGS="--client-key=${CERT_DIR}/client-admin.key --client-certificate=${CERT_DIR}/client-admin.crt"
        fi

        # Grant apiserver permission to speak to the kubelet
        # TODO kubelet can talk to mutilple apiservers. However, it needs to implement after code changes
        #${KUBECTL} --kubeconfig "${CERT_DIR}/admin$1.kubeconfig" create clusterrolebinding kube-apiserver-kubelet-admin --clusterrole=system:kubelet-api-admin --user=kube-apiserver
        bindings=$(${KUBECTL} --kubeconfig "${CERT_DIR}/admin.kubeconfig" get clusterrolebinding)
        if [[ $bindings == *"kube-apiserver-kubelet-admin"* ]] ; then
            echo "The cluster role binding kube-apiserver-kubelet-admin does exist"
        else
            ${KUBECTL} --kubeconfig "${CERT_DIR}/admin.kubeconfig" create clusterrolebinding kube-apiserver-kubelet-admin --clusterrole=system:kubelet-api-admin --user=kube-apiserver
        fi

        ${CONTROLPLANE_SUDO} cp "${CERT_DIR}/admin$1.kubeconfig" "${CERT_DIR}/admin-kube-aggregator$1.kubeconfig"
        ${CONTROLPLANE_SUDO} chown "$(whoami)" "${CERT_DIR}/admin-kube-aggregator$1.kubeconfig"
        ${KUBECTL} config set-cluster local-up-cluster --kubeconfig="${CERT_DIR}/admin-kube-aggregator$1.kubeconfig" --server="https://${API_HOST_IP}:31090"
        echo "use 'kubectl --kubeconfig=${CERT_DIR}/admin-kube-aggregator$1.kubeconfig' to use the aggregated API server"

        # Copy workload controller manager config to run path
        ${CONTROLPLANE_SUDO} cp "cmd/workload-controller-manager/config/controllerconfig.json" "${CERT_DIR}/controllerconfig.json"
        ${CONTROLPLANE_SUDO} chown "$(whoami)" "${CERT_DIR}/controllerconfig.json"
    fi
}

function kube::common::test_apiserver_off {
    # For the common local scenario, fail fast if server is already running.
    # this can happen if you run local-up-cluster.sh twice and kill etcd in between.
    if [[ "${API_PORT}" -gt "0" ]]; then
        if ! curl --silent -g "${API_HOST}:${API_PORT}" ; then
            echo "API SERVER insecure port is free, proceeding..."
        else
            echo "ERROR starting API SERVER, exiting. Some process on ${API_HOST} is serving already on ${API_PORT}"
            exit 1
        fi
    fi


    if ! curl --silent -k -g "${API_HOST}:${API_SECURE_PORT}" ; then
        echo "API SERVER secure port is free, proceeding..."
    else
        echo "ERROR starting API SERVER, exiting. Some process on ${API_HOST} is serving already on ${API_SECURE_PORT}"
        exit 1
    fi
}

function kube::common::start_workload_controller_manager {
    CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")
    controller_config_arg=("--controllerconfig=${WORKLOAD_CONTROLLER_CONFIG_PATH}")
    kubeconfigfilepaths="${CERT_DIR}/workload-controller.kubeconfig"
    if [[ $# -gt 1 ]] ; then
       kubeconfigfilepaths=$@
    fi
    echo "The kubeconfig has been set ${kubeconfigfilepaths}"
    WORKLOAD_CONTROLLER_LOG=${LOG_DIR}/workload-controller-manager.log
    ${CONTROLPLANE_SUDO} "${GO_OUT}/workload-controller-manager" \
      --v="${LOG_LEVEL}" \
      --kubeconfig "${kubeconfigfilepaths}" \
      "${controller_config_arg[@]}" >"${WORKLOAD_CONTROLLER_LOG}" 2>&1 &
    WORKLOAD_CTLRMGR_PID=$!
}

function kube::common::start_controller_manager {
    CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")
    kubeconfigfilepaths="${CERT_DIR}/controller.kubeconfig"
    if [[ $# -gt 1 ]] ; then
       kubeconfigfilepaths=$@
    fi

    node_cidr_args=()
    if [[ "${NET_PLUGIN}" == "kubenet" ]]; then
      node_cidr_args=("--allocate-node-cidrs=true" "--cluster-cidr=10.1.0.0/16")
    fi

    cloud_config_arg=("--cloud-provider=${CLOUD_PROVIDER}" "--cloud-config=${CLOUD_CONFIG}")
    if [[ "${EXTERNAL_CLOUD_PROVIDER:-}" == "true" ]]; then
      cloud_config_arg=("--cloud-provider=external")
      cloud_config_arg+=("--external-cloud-volume-plugin=${CLOUD_PROVIDER}")
      cloud_config_arg+=("--cloud-config=${CLOUD_CONFIG}")
    fi

    if [[ -z "${IS_RESOURCE_PARTITION}" ]]; then
       MASTER_ENDPOINT="https://${API_HOST}:${API_SECURE_PORT}"
    else
       MASTER_ENDPOINT=${SCALE_OUT_PROXY_ENDPOINT}
       if [ "${IS_RESOURCE_PARTITION}" == "true" ]; then
          KUBE_CONTROLLERS="nodelifecycle"
       else
          KUBE_CONTROLLERS="*,-nodelifecycle,-nodeipam"
       fi
    fi

    CTLRMGR_LOG=${LOG_DIR}/kube-controller-manager.log
    ${CONTROLPLANE_SUDO} "${GO_OUT}/hyperkube" kube-controller-manager \
      --v="${LOG_LEVEL}" \
      --allocate-node-cidrs="${KUBE_CONTROLLER_MANAGER_ALLOCATE_NODE_CIDR}" \
      --cluster-cidr="${KUBE_CONTROLLER_MANAGER_CLUSTER_CIDR}" \
      --vmodule="${LOG_SPEC}" \
      --service-account-private-key-file="${SERVICE_ACCOUNT_KEY}" \
      --root-ca-file="${ROOT_CA_FILE}" \
      --cluster-signing-cert-file="${CLUSTER_SIGNING_CERT_FILE}" \
      --cluster-signing-key-file="${CLUSTER_SIGNING_KEY_FILE}" \
      --enable-hostpath-provisioner="${ENABLE_HOSTPATH_PROVISIONER}" \
      ${node_cidr_args[@]+"${node_cidr_args[@]}"} \
      --pvclaimbinder-sync-period="${CLAIM_BINDER_SYNC_PERIOD}" \
      --feature-gates="${FEATURE_GATES}" \
      "${cloud_config_arg[@]}" \
      --kubeconfig "${kubeconfigfilepaths}" \
      --controllers=${KUBE_CONTROLLERS} \
      --leader-elect=false \
      --cert-dir="${CERT_DIR}" \
      --default-network-template-path="${ARKTOS_NETWORK_TEMPLATE}" \
      --master=${MASTER_ENDPOINT} ${CONTROLLER_MANAGER_EXTRA_FLAGS:-} >"${CTLRMGR_LOG}"  2>&1 &
      #--master="https://${API_HOST}:${API_SECURE_PORT}" >"${CTLRMGR_LOG}" 2>&1 &
    CTLRMGR_PID=$!
}

function kube::common::start_kubescheduler {
    CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")
    kubeconfigfilepaths="${CERT_DIR}/scheduler.kubeconfig"
    if [[ $# -gt 1 ]] ; then
       kubeconfigfilepaths=$@
    fi
  
    if [[ -z "${IS_RESOURCE_PARTITION}" ]]; then
       MASTER_ENDPOINT="https://${API_HOST}:${API_SECURE_PORT}"
    else
       MASTER_ENDPOINT=${SCALE_OUT_PROXY_ENDPOINT}
    fi

    SCHEDULER_LOG=${LOG_DIR}/kube-scheduler.log
    ${CONTROLPLANE_SUDO} "${GO_OUT}/hyperkube" kube-scheduler \
      --v="${LOG_LEVEL}" \
      --leader-elect=false \
      --kubeconfig "${kubeconfigfilepaths}" \
      --feature-gates="${FEATURE_GATES}" \
      --master=${MASTER_ENDPOINT} >"${SCHEDULER_LOG}" 2>&1 &
      #--master="https://${API_HOST}:${API_SECURE_PORT}" >"${SCHEDULER_LOG}" 2>&1 &
    SCHEDULER_PID=$!
}

function kube::common::start_kubelet {
    KUBELET_LOG=${LOG_DIR}/kubelet.log
    mkdir -p "${POD_MANIFEST_PATH}" &>/dev/null || sudo mkdir -p "${POD_MANIFEST_PATH}"

    cloud_config_arg=("--cloud-provider=${CLOUD_PROVIDER}" "--cloud-config=${CLOUD_CONFIG}")
    if [[ "${EXTERNAL_CLOUD_PROVIDER:-}" == "true" ]]; then
       cloud_config_arg=("--cloud-provider=external")
       cloud_config_arg+=("--provider-id=$(hostname)")
    fi

    mkdir -p "/var/lib/kubelet" &>/dev/null || sudo mkdir -p "/var/lib/kubelet"
    # Enable dns
    if [[ "${ENABLE_CLUSTER_DNS}" = true ]]; then
      if [[ "${ENABLE_NODELOCAL_DNS:-}" == "true" ]]; then
        dns_args=("--cluster-dns=${LOCAL_DNS_IP}" "--cluster-domain=${DNS_DOMAIN}")
      else
        dns_args=("--cluster-dns=${DNS_SERVER_IP}" "--cluster-domain=${DNS_DOMAIN}")
      fi
    else
      # To start a private DNS server set ENABLE_CLUSTER_DNS and
      # DNS_SERVER_IP/DOMAIN. This will at least provide a working
      # DNS server for real world hostnames.
      dns_args=("--cluster-dns=8.8.8.8")
    fi
    net_plugin_args=()
    if [[ -n "${NET_PLUGIN}" ]]; then
      net_plugin_args=("--network-plugin=${NET_PLUGIN}")
    fi

    auth_args=()
    if [[ "${KUBELET_AUTHORIZATION_WEBHOOK:-}" != "false" ]]; then
      auth_args+=("--authorization-mode=Webhook")
    fi
    if [[ "${KUBELET_AUTHENTICATION_WEBHOOK:-}" != "false" ]]; then
      auth_args+=("--authentication-token-webhook")
    fi
    if [[ -n "${CLIENT_CA_FILE:-}" ]]; then
      auth_args+=("--client-ca-file=${CLIENT_CA_FILE}")
    else
      auth_args+=("--client-ca-file=${CERT_DIR}/client-ca.crt")
    fi

    cni_conf_dir_args=()
    if [[ -n "${CNI_CONF_DIR}" ]]; then
      cni_conf_dir_args=("--cni-conf-dir=${CNI_CONF_DIR}")
    fi

    cni_bin_dir_args=()
    if [[ -n "${CNI_BIN_DIR}" ]]; then
      cni_bin_dir_args=("--cni-bin-dir=${CNI_BIN_DIR}")
    fi

    container_runtime_endpoint_args=()
    if [[ -n "${CONTAINER_RUNTIME_ENDPOINT}" ]]; then
      container_runtime_endpoint_args=("--container-runtime-endpoint=${CONTAINER_RUNTIME_ENDPOINT}")
    fi

    image_service_endpoint_args=()
    if [[ -n "${IMAGE_SERVICE_ENDPOINT}" ]]; then
      image_service_endpoint_args=("--image-service-endpoint=${IMAGE_SERVICE_ENDPOINT}")
    fi

    # shellcheck disable=SC2206
    all_kubelet_flags=(
      "--v=${LOG_LEVEL}"
      "--vmodule=${LOG_SPEC}"
      "--chaos-chance=${CHAOS_CHANCE}"
      "--container-runtime=${CONTAINER_RUNTIME}"
      "--hostname-override=${HOSTNAME_OVERRIDE}"
      "${cloud_config_arg[@]}"
      "--address=0.0.0.0"
      --kubeconfig "${CERT_DIR}"/kubelet.kubeconfig
      "--feature-gates=${FEATURE_GATES}"
      "--cpu-cfs-quota=${CPU_CFS_QUOTA}"
      "--enable-controller-attach-detach=${ENABLE_CONTROLLER_ATTACH_DETACH}"
      "--cgroups-per-qos=${CGROUPS_PER_QOS}"
      "--cgroup-driver=${CGROUP_DRIVER}"
      "--cgroup-root=${CGROUP_ROOT}"
      "--eviction-hard=${EVICTION_HARD}"
      "--eviction-soft=${EVICTION_SOFT}"
      "--eviction-pressure-transition-period=${EVICTION_PRESSURE_TRANSITION_PERIOD}"
      "--pod-manifest-path=${POD_MANIFEST_PATH}"
      "--fail-swap-on=${FAIL_SWAP_ON}"
      ${auth_args[@]+"${auth_args[@]}"}
      ${dns_args[@]+"${dns_args[@]}"}
      ${cni_conf_dir_args[@]+"${cni_conf_dir_args[@]}"}
      ${cni_bin_dir_args[@]+"${cni_bin_dir_args[@]}"}
      ${net_plugin_args[@]+"${net_plugin_args[@]}"}
      ${container_runtime_endpoint_args[@]+"${container_runtime_endpoint_args[@]}"}
      ${image_service_endpoint_args[@]+"${image_service_endpoint_args[@]}"}
      "--runtime-request-timeout=${RUNTIME_REQUEST_TIMEOUT}"
      "--port=${KUBELET_PORT}"
      ${KUBELET_FLAGS}
    )

    kube::common::generate_kubelet_certs

    # shellcheck disable=SC2024
    sudo -E "${GO_OUT}/hyperkube" kubelet "${all_kubelet_flags[@]}" >"${KUBELET_LOG}" 2>&1 &
    KUBELET_PID=$!

    # Quick check that kubelet is running.
    if [ -n "${KUBELET_PID}" ] && ps -p ${KUBELET_PID} > /dev/null; then
      echo "kubelet ( ${KUBELET_PID} ) is running."
    else
      cat "${KUBELET_LOG}" ; exit 1
    fi
}

function kube::common::start_kubeproxy {
    CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")
    kubeconfigfilepaths="${CERT_DIR}/kube-proxy.kubeconfig"
    if [[ $# -gt 1 ]] ; then
       kubeconfigfilepaths=$@
    fi

    PROXY_LOG=${LOG_DIR}/kube-proxy.log

    cat <<EOF > /tmp/kube-proxy.yaml
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
clientConnection:
  kubeconfig: ${kubeconfigfilepaths}
hostnameOverride: ${HOSTNAME_OVERRIDE}
mode: ${KUBE_PROXY_MODE}
EOF
    if [[ -n ${FEATURE_GATES} ]]; then
      echo "featureGates:"
      # Convert from foo=true,bar=false to
      #   foo: true
      #   bar: false
      for gate in $(echo "${FEATURE_GATES}" | tr ',' ' '); do
        echo "${gate}" | ${SED} -e 's/\(.*\)=\(.*\)/  \1: \2/'
      done
    fi >>/tmp/kube-proxy.yaml

    kube::common::generate_kubeproxy_certs
    
    if [[ -z "${IS_RESOURCE_PARTITION}" ]]; then
       MASTER_ENDPOINT="https://${API_HOST}:${API_SECURE_PORT}"
    else
       MASTER_ENDPOINT=${SCALE_OUT_PROXY_ENDPOINT}
    fi

    # shellcheck disable=SC2024
    sudo "${GO_OUT}/hyperkube" kube-proxy \
      --v="${LOG_LEVEL}" \
      --config=/tmp/kube-proxy.yaml \
      --master=${MASTER_ENDPOINT} >"${PROXY_LOG}" 2>&1 &
      #--master="https://${API_HOST}:${API_SECURE_PORT}" >"${PROXY_LOG}" 2>&1 &
    PROXY_PID=$!
}

function kube::common::generate_kubelet_certs {
  if [[ "${REUSE_CERTS}" != true ]]; then
        CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")
        kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' kubelet "system:node:${HOSTNAME_OVERRIDE}" system:nodes
        kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" kubelet
  fi
}

function kube::common::generate_kubeproxy_certs {
    if [[ "${REUSE_CERTS}" != true ]]; then
        CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")
        kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' kube-proxy system:kube-proxy system:nodes
        kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" kube-proxy
    fi
}


