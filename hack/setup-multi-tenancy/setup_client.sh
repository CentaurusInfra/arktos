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

print_help() {
        echo "This is a tool to manage user context in multi-tenancy Arktos cluster. "
        echo 
        echo "To set up a context for a person under a given tenant:"
        echo "  setup_client.sh [tenant] [person] (optional [apiserver ip])"
        echo 
        echo "By default, the key and cert files will be saved under /tmp."
        echo "To save them in a different location:"
        echo "  OUTPUT_DIR=[desired_path] setup_client.sh [tenant] [person] (optional [apiserver ip])"
        echo 
        echo "To remove the context set up for a person under a given tenant:"
        echo "  REMOVE=TRUE setup_client.sh [tenant] [person]"
}

run_command() {
        command="$@"
        $command > /dev/null 2> ${tmpfile}
        if [[ $? != 0 ]] 
        then 
                printf "\033[0;31mFailed: ${command}\n"
                printf "$(cat ${tmpfile})\033[0m\n"
                let failures+=1
        fi
}

setup_client() {
        echo "setting up context ${context_name} for ${tenant}/${person} in group $group_name at apiserver $ip"
        echo "generating a client key ${tenant}.key..."
        run_command openssl genrsa -out ${output_dir}/${tenant_person}.key 2048

        echo "creating a sign request ${tenant}.csr..."
        run_command openssl req -new -key ${output_dir}/${tenant_person}.key -out ${output_dir}/${tenant}.csr -subj "/CN=${person}/O=tenant:${tenant}/OU=$group_name"

        echo "creating an tenant certificate ${tenant_person}.crt and get it signed by CA"
        run_command openssl x509 -req -in ${output_dir}/${tenant}.csr -CA /var/run/kubernetes/client-ca.crt -CAkey /var/run/kubernetes/client-ca.key -CAcreateserial -out ${output_dir}/${tenant_person}.crt -days 500

        echo "Setting up tenant context..."
        run_command kubectl config set-cluster ${cluster_name} --server=https://$ip:6443 --certificate-authority=/var/run/kubernetes/server-ca.crt 
        run_command kubectl config set-credentials ${tenant_person} --client-certificate=${output_dir}/${tenant_person}.crt --client-key=${output_dir}/${tenant_person}.key
        run_command kubectl config set-context ${context_name} --cluster=${cluster_name} --namespace=${namespace} --user=${tenant_person} --tenant=${tenant}
        
        echo "cleaned up ${tenant}.csr"
        rm ${output_dir}/${tenant}.csr

        echo "***************************"
        echo "Context has been setup for tenant ${tenant}/${person} in $CONFIG." 
        echo
        echo "Use 'kubectl config use-context ${context_name}' to set it as the default context"
        echo
        echo "Here's an example on how to access cluster:"
        echo
        set -x
        kubectl --context=${context_name} get pods
        { set +x; } 2>/dev/null
}

unset_client() {
        if [ "$(kubectl config current-context)" == "${context_name}" ]
        then
                run_command kubectl config unset current-context
        fi

        echo "removing client key and cert files of ${tenant}/${person} ..."
        files_to_rm=`kubectl config view -o json | jq -r --arg name "${tenant_person}" '.users[] | select(.name==$name).user | [ .["client-key"], .["client-certificate"]] | @tsv' `
        rm $files_to_rm > /dev/null 2>&1

        echo "Unsetting the context of ${tenant}/${person} in $CONFIG ..."
        run_command kubectl config unset contexts.${context_name} 
        run_command kubectl config unset users.${tenant_person}
        run_command kubectl config unset clusters.${cluster_name}
}

if [ "$#" -lt 2 ] || [ "$1" == "-h" ] || [ "$1" == "--help" ]
then
        print_help
        exit 1
fi

tenant=$1
person=$2
ip=${3:-localhost}
output_dir=${OUTPUT_DIR:-"/tmp/"}
namespace="default"
tenant_person="${tenant}-${person}"
group_name="${tenant}-group"
context_name="${tenant_person}-context"
cluster_name="${tenant_person}-cluster"

failures=0
tmpfile="/tmp/temp$(date +'%Y%m%d%H%M%s')"

CONFIG=/var/run/kubernetes/admin.kubeconfig
REMOVE=${REMOVE:-"FALSE"}

if [ "${REMOVE}" == "TRUE" ]
then
        unset_client
else
        setup_client
fi

rm ${tmpfile} > /dev/null 2>&1
exit $failures