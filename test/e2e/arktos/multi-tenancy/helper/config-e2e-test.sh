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

# By default we use the kubectl binary built in the Arktos repository.
kubectl=${base_dir}/_output/bin/kubectl

#put the test suite file names below, one line one suite. The test suites will be run in the order defined.
test_suite_files="system_tenant_basic_tests \
                tenant_init_delete_tests \
                regular_tenant_kubectl_tests \
                multi_tenancy_controller_tests"
test_suite_file_directory=$(dirname $0)/test_suites/
test_data_file_directory=$(dirname $0)/testdata/
setup_client_script=${base_dir}/hack/setup-multi-tenancy/setup_client.sh

# The values of timeouts and retry intervals are in the unit of second
# timeout=0 means that there is not check on whether a command exits within a give time span
default_timeout=5
max_timeout=300

default_retry_count=0
max_retry_count=30

default_retry_interval=1
max_retry_interval=30
