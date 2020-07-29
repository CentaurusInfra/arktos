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

function verify_test_cluster_ready {
	if [ ! -f "${kubectl}" ]; then
		report_error "File ${kubectl} is missing."
		exit 1
	fi

	print_normal "Checking the Arktos cluster is ready ..."
	reset_test_parameter
	command="${kubectl} get nodes"
	expect_output="Ready"
	run_test

	if [ "${#errors[@]}" != "0" ] 
	then	
		printf "\n"
		report_error "Arktos cluster is not up for test. Or you have don't have cluster admin privilege."
		exit 1
	fi
}


source $(dirname $0)/helper/config-e2e-test.sh
source $(dirname $0)/helper/e2e-test-helper.sh

verify_test_cluster_ready

for test_case_file in $(echo "${test_case_files}" | tr "," "\n")
do
	test_case_file_path=$(readlink -f "${test_case_file_directory}/${test_case_file}")
	printf "${MAGENTA}***************************************************************${NC}\n"
	printf "${MAGENTA}starting tests in file ${test_case_file_path} ...${NC}\n"

	load_test_case_file_and_run "${test_case_file_path}"
done

print_summary


