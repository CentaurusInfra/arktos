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

# Convenience script to download and install etcd in third_party.
# Mostly just used by CI.

set -o errexit
set -o nounset
set -o pipefail

LOG_DIR=${LOG_DIR:-"/tmp"}

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${KUBE_ROOT}/hack/lib/init.sh"

# There are following commands to run the script
# hack/install-etcd.sh to install an etcd cluster
# hack/install-etcd.sh start to add etcd to path and start the etcd cluster
# hack/install-etcd.sh add member-name url-address, for example, hack/install-etcd.sh add etcd1 http://10.0.0.1:2379
if [ $# -lt 1 ] ; then
    kube::etcd::install
else
  echo "The current operation is $1"
  case $1 in
    add)
      if [ -n "$2" ]  && [ -n "$3" ]; then
        kube::etcd::add_member $2 $3
      fi
      ;;
    start)
      # install etcd if necessary
      if ! [[ $(which etcd) ]]; then
        if ! [ -f "${KUBE_ROOT}/third_party/etcd/etcd" ]; then
           echo "cannot find etcd locally. will install one."
           ${KUBE_ROOT}/hack/install-etcd.sh
        fi
        export PATH=$PATH:${KUBE_ROOT}/third_party/etcd
      fi
      echo "Starting etcd"
      export ETCD_LOGFILE=${LOG_DIR}/etcd.log
      kube::etcd::start
      ;;
    validate)
      kube::etcd::validate
      ;;
    version)
      # hack/install-etcd.sh version version1(3.4.3) version2(3.4.4) for comparing versions
      ver1=$(kube::etcd::version $2)
      echo "The version $2 has been converted to $ver1"
      ver2=$(kube::etcd::version $3)
      echo "The version $3 has been converted to $ver2"
      echo "The $ver1 is greater than $ver2 "
      [[ $ver1 -gt $ver2 ]] && echo true || echo false
      ;;
    *)
      echo "Unknown operation"
      ;;
  esac
fi
