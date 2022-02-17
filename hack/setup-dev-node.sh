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

# Convenience script to setup a fresh Linux installation for Arktos developers.

set -o errexit
set -o nounset
set -o pipefail

echo "The script is to help install prerequisites of Arktos development environment"
echo "on a fresh Linux installation."
echo "It's been tested on Ubuntu 16.04 LTS and 18.04 LTS."

GOLANG_VERSION=${GOLANG_VERSION:-"1.13.9"}

echo "Update apt."
sudo apt -y update

echo "Install docker."
sudo apt -y install docker.io
sudo chmod o+rw /var/run/docker.sock; ls -al /var/run/docker.sock

echo "Install make & gcc."
sudo apt -y install make
sudo apt -y install gcc
sudo apt -y install jq

echo "Install golang."
wget https://dl.google.com/go/go${GOLANG_VERSION}.linux-amd64.tar.gz -P /tmp
sudo tar -C /usr/local -xzf /tmp/go${GOLANG_VERSION}.linux-amd64.tar.gz

kubectl_url="https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"

if [[ ! -f /usr/local/bin/kubectl ]]; then
    echo -n "Downloading kubectl binary..."
    sudo wget ${kubectl_url} -P /usr/local/bin
else
  # TODO: We should detect version of kubectl binary if it too old
  # download newer version.
  echo "Detected existing kubectl binary. Skipping download."
fi
sudo chmod a+x "/usr/local/bin/kubectl"

echo "GOROOT=/usr/local/go" >> $HOME/.profile
echo "GOPATH=$HOME/go" >> $HOME/.profile
echo "PATH=/usr/local/go/bin:$PATH" >> $HOME/.profile

echo "Done."
echo "Please run 'source $HOME/.profile' to enforce env PATH changing."
echo "You can proceed to run hack/arktos-up.sh if you want to launch a single-node cluster."
