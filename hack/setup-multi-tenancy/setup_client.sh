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

if [ "$#" -lt 2 ]
then
        echo "Usage: setup_client.sh [tenant] [person] (optional [host ip])"
        exit 1
fi

tenant=$1
person=$2
namespace="default"
tenant_person="${tenant}-${person}"
group_name="${tenant}-group"
context_name="${tenant_person}-context"
ip=${3:-localhost}
echo "setting up context $context_name for $tenant/$person in group $group_name at host $ip"

CONFIG=/var/run/kubernetes/admin.kubeconfig
#if [ -f "$CONFIG" ]; then
#    echo "making a backup of previous config at $CONFIG.bk"
#    mv $CONFIG $CONFIG.bk
#fi

echo "generating a client key $tenant.key..."
openssl genrsa -out $tenant_person.key 2048 > /dev/null 2>&1

echo "creating a sign request $tenant.csr..."
openssl req -new -key $tenant_person.key -out $tenant.csr -subj "/CN=$person/O=tenant:$tenant/OU=$group_name" > /dev/null 2>&1

echo "creating an tenant certificate ${tenant_person}.crt and get it signed by CA"
openssl x509 -req -in $tenant.csr -CA /var/run/kubernetes/client-ca.crt -CAkey /var/run/kubernetes/client-ca.key -CAcreateserial -out $tenant_person.crt -days 500 > /dev/null 2>&1

echo "Setting up tenant context..."
kubectl config set-cluster ${tenant_person}-cluster --server=https://$ip:6443 --certificate-authority=/var/run/kubernetes/server-ca.crt > /dev/null 2>&1
kubectl config set-credentials ${tenant_person} --client-certificate=$tenant_person.crt --client-key=$tenant_person.key > /dev/null 2>&1
kubectl config set-context $context_name --cluster=${tenant_person}-cluster --namespace=$namespace --user=$tenant_person --tenant=$tenant > /dev/null 2>&1
echo "cleaned up $tenant.csr"
rm $tenant.csr
echo "***************************"
echo "Context has been setup for tenant $tenant/$person in $CONFIG." 
echo
echo "Use 'kubectl use-context $context_name' to set it as the default context"
echo
echo "Here's an example on how to access cluster:"
echo
set -x
kubectl --context=${context_name} get pods
