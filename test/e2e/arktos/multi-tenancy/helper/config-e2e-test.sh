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

base_dir=$(cd $(dirname $0)/../../../.. ; pwd)
testdata_dir=${base_dir}/test/e2e/arktos/multi-tenancy/testdata/
setup_client_script=${base_dir}/hack/setup-multi-tenancy/setup_client.sh

# By default we use the kubectl binary built in the Arktos repository.
kubectl=${base_dir}/_output/bin/kubectl

#put the test case file names below, separated by comma. No space is accepted. The files will be tested in the order defined.
test_case_files=basic_tests,new_tenant_tests
test_case_file_directory=$(dirname $0)/testcase/

# The values of timeouts and retry intervals are in the unit of second
# timeout=0 means that there is not check on whether a command exits within a give time span
default_timeout=1
max_timeout=120

default_retry_count=0
max_retry_count=30

default_retry_interval=1
max_retry_interval=30

# create a test tenant name of 8 random characters
new_tenant="$(cat /dev/urandom | tr -dc 'a-z0-9' | fold -w 8 | head -n 1)"

printf "creating admin context for tenant ${new_tenant} ...."

${setup_client_script} ${new_tenant} admin

new_tenant_context=${new_tenant}-admin-context

new_namespace="$(cat /dev/urandom | tr -dc 'a-z0-9' | fold -w 5 | head -n 1)"


