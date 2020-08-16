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

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[1;34m'
MAGENTA='\033[0;95m'
NC='\033[0m' # No Color

TRUE=1
FALSE=0

errors=()

timeout_error_code=124

function report_error {
	error_cause=$1
	errors+=(" : ${error_cause}: \"${command}\" in test suite ${test_file}")
	printf "${RED}%s${NC}\n" "$1"
}

function report_warning {
	printf "\n${YELLOW}%s${NC}\n" "$1"
}

function report_success {
	printf "${GREEN}%s${NC}\n" "$1"
}

function print_normal {
	printf "\n${BLUE}%s${NC}" "$1"
}

function run_test {
	[[ "${retrying}" == "$FALSE" ]] && print_normal "Testing \"${command}\" ...."
        
	if [[ $timeout -gt 0 ]];  then
		# if the test case has non-default timeout setting, let the tester know
		[[ $timeout -ne $default_timeout ]] && print_normal "The test will fail if the command does not finish within $timeout seconds ..."
		cmd_output=$(timeout ${timeout} ${command} 2>&1)
	else 
		cmd_output=$(${command} 2>&1)
	fi

	cmd_exit_code=$?

	if [[ (${cmd_exit_code} == 0 && ${expect_fail} == $FALSE) || (${cmd_exit_code} != 0 && ${expect_fail} == $TRUE) ]]; then 
		cmd_exit_code_error=$FALSE
		check_output_contain "${cmd_output}" ${expect_output}
	else 
		cmd_exit_code_error=$TRUE
	fi

	if [[ ${cmd_exit_code_error} == $FALSE && $check_output_error == $FALSE ]]; then
		report_success "PASSED."
		return
	else
		printf "\nThe command output:\n${cmd_output}\n"
	fi

	if [[ $retry_count -gt 0 ]]; then
		retry_count=$(($retry_count-1))
		report_warning "Umm, the result does not look right. Another run may succeed. Retrying... "
		retrying=$TRUE
		sleep $retry_interval
		run_test
	else 
		if [[ ${cmd_exit_code_error} == $TRUE ]]; then
			[[ ( ${timeout} -gt 0 && ${cmd_exit_code} -eq ${timeout_error_code} ) ]] && report_error "Command not finished after ${timeout} seconds" && return
			[[ ${expect_fail} == $FALSE ]] && report_error "Failed" && return
			[[ ${expect_fail} == $TRUE ]] && report_error "Succeeded Unexpectedly" && return
		fi

		if [[ $check_output_error == $TRUE ]]; then 			
			report_warning "did not find ${expect_output} in the above output"
			report_error "Unexpected output"
		fi			
	fi
}

function check_output_contain {
	check_output_error=$FALSE
	output=$1

	while : 
	do 
		shift;
		local expected_output=$(echo $1)
		[[ -z "${expected_output}" ]] && break
			
		filtered_output=${output}
		for matching_string in $(echo "${expected_output}" | tr "," "\n")
		do 
			filtered_output=$(echo "${filtered_output}" | grep ${matching_string} )
			if [[ -z "${filtered_output}" ]];  then
				check_output_error=$TRUE
				report_warning "could not find \"${expected_output}\" in the command ouput"
				break
			fi
		done
	done
}

function print_summary {
	printf "${MAGENTA}\n================================ Test Result Summary ======================================${NC}"
	error_count=${#errors[@]}
	if [[ "${error_count}" == "0" ]];  then
		printf "\n"
		report_success "Yay, all tests passed!"
		exit 0
	else
		printf "\nTotally ${RED}${error_count}${NC} tests failed. They are:\n"
		printf "${MAGENTA}\t%s\n${NC}" "${errors[@]}"
		exit 1
	fi
}

function reset_test_parameter {
	command=""
	expect_output=""
	retry_count=$default_retry_count
	retry_interval=$default_retry_interval
	timeout=$default_timeout
	expect_fail=$FALSE
	retrying=$FALSE
}

function run_test_if_command_not_empty {
	if [[ "$command" != "" ]];  then
		run_test
		reset_test_parameter				
	fi
}

function load_test_suite_file_and_run {
	test_file=$1
	if [[ -z "${test_file}" ]];  then 
		report_error "No test file is specified!"
		exit 1
	fi

	if [[ ! -r "${test_file}" ]];  then
		report_error "Cannot open file $1!"
		exit
	fi

	reset_test_parameter
	while IFS= read -r line
	do
		# the following line trims the beginning/ending spaces and squeeze the in-between spaces and tabes into single spaces
		line=$(echo $line)
		line_lowercase=$(echo $line |  tr '[:upper:]' '[:lower:]')
		[[ $line =~ ^#.* || "$line" == "" ]] && continue

		if [[ $line_lowercase =~ ^configtest:.* ]]; then
			configtest=$(eval echo "${line:12}")
			run_test_if_command_not_empty
			print_normal "Running \"${configtest}\""
			echo "#!/bin/bash" > /tmp/e2e_test_temp.sh
			echo $configtest >> /tmp/e2e_test_temp.sh
			source /tmp/e2e_test_temp.sh			
			continue
		fi

		if [[ $line_lowercase =~ ^command:.* ]]; then
			run_test_if_command_not_empty
			command=$(eval echo "${line:9}")
			continue
		fi
		
		[[ $line_lowercase =~ ^expectoutput:.* ]] && expect_output=$(eval echo "${line:13}") && continue
		if [[ $line_lowercase =~ ^expectfail:.* ]]; then
			[[ "$(echo ${line_lowercase:11})" == "true" ]] && expect_fail=$TRUE
			continue
		fi

		if [[ $line_lowercase  =~ ^retrycount:.* ]]; then
			retry_count=${line:11}
			if [[ !($retry_count =~ ^[0-9]+*) ]]; then 
				retry_count=$default_retry_count
				report_warning "Retry count is not numeric in the line \"$line\". Set timeout to default value of $default_retry_count."
				continue
			fi

			if [[ $retry_count -gt $max_retry_count ]]; then 
				retry_count=$max_retry_count
				report_warning "The retry count in \"$line\" is too big. set the count to $max_retry_count."
				continue				
			fi
			continue
		fi

		if [[ $line_lowercase  =~ ^retryinterval:.* ]]; then
			retry_interval=${line:15}
			if [[ !($retry_interval =~ ^[0-9]+*) ]]; then 
				retry_interval=$default_retry_interval
				report_warning "Retry interval is not numeric in the line \"$line\". Set the interval to default value of $default_retry_interval."
				continue
			fi

			if [[ $retry_interval -gt $max_retry_interval ]]; then 
				retry_interval=$max_retry_interval
				report_warning "The retry interval in \"$line\" is too big. set the interval to $max_retry_interval."
				continue				
			fi
			continue
		fi

		if [[ $line_lowercase  =~ ^timeout:.* ]]; then
			timeout=$(echo ${line:8})
			if [[ !($timeout =~ ^[0-9]+*) ]]; then 
				timeout=$default_timeout
				report_warning "Timeout is not numeric in the line \"$line\". Set timeout to default value of $default_timeout."
				continue
			fi

			if [[ $timeout -gt $max_timeout ]]; then 
				timeout=$max_timeout
				report_warning "The timeout in \"$line\" is too big. set the interval to $max_timeout."
				continue				
			fi
			continue
		fi
			
		report_warning "Failed to parse line \"${line}\", ignore the line."	
	done < "${test_file}"

	run_test_if_command_not_empty
}
