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

echo "Done."
echo "Please run and add 'export PATH=\$PATH:/usr/local/go/bin' into your shell profile."
echo "You can proceed to run arktos-up.sh if you want to launch a single-node cluster."
