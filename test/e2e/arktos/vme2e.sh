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

function vm_pod_running {
	local pod=`cluster/kubectl.sh get pods | grep "vmdefault" | grep "Running"`
	if [ -z "${pod}" ]
	then
		return 0	
	else
		return 1	
	fi
}

function vanilla_pod_running {
	local pod=`cluster/kubectl.sh get pods | grep "k8s-vanilla-pod" | grep "Running"`
	if [ -z "${pod}" ]
	then
		return 0	
	else
		return 1	
	fi
}

# verify virtlet is running
virtlet=`cluster/kubectl.sh get pods -n kube-system | grep "virtlet-" | grep "Running"`
if [ -z "${virtlet}" ]
then
      echo "Virtlet is not in running states. Failed"
else
      echo "Virtlet is running"
fi

# verify VM pod is running 
# will retry for up to 30 seconds
cluster/kubectl.sh create -f test/yaml/vm_default.yaml  
retry=0
while vm_pod_running
do
	sleep 1 
	if [ $retry -gt 30 ]
	then
		echo "Failed to bring up VM pod in test/yaml/vm_default.yaml"
		echo "Test failed"
		exit
	fi	
	let retry=retry+1 
done
echo "VM pod test passed"
cluster/kubectl.sh delete -f test/yaml/vm_default.yaml & 

# verify regular pod is running 
# will retry for up to 30 seconds
cluster/kubectl.sh create -f test/yaml/vanilla.yaml
retry=0
while vanilla_pod_running
do
	sleep 1 
	if [ $retry -gt 30 ]
	then
		echo "Failed to bring up VM pod in test/yaml/vm_default.yaml"
		echo "Test failed"
		exit
	fi	
	retry=$((retry+1)) 
done
echo "Regular pod test passed"
cluster/kubectl.sh delete -f test/yaml/vanilla.yaml &
