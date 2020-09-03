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

export GO111MODULE=on
script_root=$(dirname "${BASH_SOURCE}")
repo_root=$(cd $(dirname $0)/../../../.. ; pwd)

#put the test suite file patterns below, one line one suite. The test suites will be run in the order defined.
test_suite_files="multi_tenancy_controller/test_*_controller.yaml \
				  tenant_init_delete_test.yaml \
				  misc/test_*.yaml"
test_suite_file_directory=${repo_root}/test/e2e/arktos/multi_tenancy/test_suites/

# The values of timeouts and retry intervals are in the unit of second
# timeout=0 means that there is not check on whether a command exits within a give time span
default_timeout=5
max_timeout=300

default_retry_count=0
max_retry_count=30

default_retry_interval=5
max_retry_interval=60

verbose=false

# The following is the common variables which would work across all the test suites.
# Remember to define here and update the command line flag of "-CommonVar" of "testrunner"
# By default we use the kubectl binary built in the Arktos repository.
kubectl=${repo_root}/_output/bin/kubectl
setup_client_script=${repo_root}/hack/setup-multi-tenancy/setup_client.sh
test_data_dir=${repo_root}/test/e2e/arktos/multi_tenancy/testdata/

cd ${script_root}/ && go build -o /tmp/testrunner './cmd/'

/tmp/testrunner -Verbose=${verbose} \
				-TestSuiteDir=${test_suite_file_directory} \
				-TestSuiteFiles="${test_suite_files}" \
				-DefaultTimeOut=${default_timeout} \
				-MaxTimeOut=${max_timeout} \
				-DefaultRetryCount=${default_retry_count} \
				-MaxRetryCount=${max_retry_count} \
				-DefaultRetryInterval=${default_retry_interval} \
				-MaxRetryInterval=${max_retry_interval} \
				-CommonVar="kubectl:${kubectl},setup_client_script:${setup_client_script},test_data_dir:${test_data_dir}"

returnCode=$?
exit ${returnCode}
