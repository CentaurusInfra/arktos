# Running Arktos Multi-Tenancy E2E Tests

Thanks for your interests in using Arktos and running the multi-tenancy E2E tests. It may be extended to cover e2e tests for other features. If you are reading this doc, I expect you are a computer professional. So you might be able to get the tests running without this doc. However, I believe it helps if you read this first.

## Running the Tests
### Prerequisites

The tests requires an Arktos Cluster be running and the current user have the full access as an cluster admin. It is done by checking the command ```"kubectl get nodes"``` sees active nodes. The script terminates if the check fails.

### Running Test

Just run the following script under the same folder as this doc.
```
run-e2e-test.sh
``` 

Note: this script does NOT take any commandline options. 

### Configure the Test Run

Though the test script does not take commandline options, it does not mean you can not configure the test running. All the configurations are defined in the script ```config-e2e-test.sh```, which is under the 'helper' sub-folder. The author made this choice as she is pretty bad at typing and suffers the brain loss in remembering command options. 

The variable names defined in the config script are self-explanatory. For example, variable ```kubectl``` defines the kubectl binary to use in the test. By default, it is the kubectl binary built in the repo. You can point it to a different kubectl if you would like a different binary.

Plase keep in mind that the author thinks you are a computer pro and thus will make sensible changes when overriding the default config. Don't expect a bell ringing if you set the value of retry_interval to 1000 years or change the retry_count to -100. If you do this, well, enjoy the chaos! Just never try to check it in.

## Create Test Cases

The  test script will run the test cases in the files specified in the variable "test_case_files" in ```config-e2e-test.sh``` in sequence, one case by one case, one file by one file.

### Defining Test Cases

A test case file may contains multiple test cases. A test case is defined with lines like:
* "Command: ###" : this element is **required** and the only required element in a test case. This line should always be first line in a test case definition. 
When the test script sees a line of "Command: ###", it thinks the previous test case definition is complete.
* "ExpectOutput: ###": this element is optional. When defined, the test will verify the command output contains the expected content. Multiple checks can be defined in one line, which will be verified one by one. Each check content can be strings concatenated by commas, which means the test expects these strings appear in the same line.
For Instances, the following defintion means the test expects the output has a line containing both "AAA" and "BBB" and also has a line containing "CCC".
```
ExpectOutput: "AAA,BBB" "CCC"
```
* "ExpectFail: ###": this element is optional. If the value is set as "true", the test expects this command to have a non-zero exit code and report test failure if the command succeeds. By default or any other values, the test expects the command to succeed. Note that it means if you put the value as "yes", the test still expects the command to exit.
* "TimeOut: ###": this element is optional. It defines the limit of number of seconds you expect the command to complete. The test fails if the command did not finish within the timeout value. The default timeout is configurable in config-e2e-test.sh. If you don't want a test case undergo timeout check, set "TimeOut: 0"/
* "RetryCount: ###": this element is optional. It defines the maximal number of retries before a test case is declared as failed. The default retry count is defined to 0, which means no retrial.
* "RetryInterval: ###": this element is optional. It defines the number of seconds between two retries. The default retry count is set to 1 second.

A line starting with "#" is regarded as comment and thus ignored. An empty line is also ignored. Any other formats of lines in the test case files will also be ignored, yet the test script will pop a waring.

A test case definition can be as simple as just one line as below, which just checks the command's exit code is 0:
```
Command: ${kubectl} config view
```

Or, it can be as full-fledged as this one:
```
# the test is doomed to fail no matter how many times tried
Command: ${kubectl} delete tenant non-existing-tenant
ExpectFail: true
ExpectOutput: "non-existing-tenant,deleted"
Timeout: 10
RetryCount: 3
RetryInterval: 10
```

### Best Practice 

Define a test case with as few lines as possible. It also make sense to avoid a long test case file and group related test cases into one file.

Test is the best preventive medicine to code regression. I wish every relevant bug fixing is accompanied by a test case addition.

## What NOT to Expect

As the test script is built from scratch and based on Linux Bash, the author expects the test is quick & lightweight, and adding new test cases is simple. But please **DO NOT** expect it to have:
* Cleanly Isolated Test Cases

A test suite would be nice if the test cases are cleanly isolated, meaning each test case is independently testing one aspect of the target system, so each test case can be added or removed without affecting the others. Such a test environment usually needs to reset the target system after each test case. 

Due to the fact it is time-consuming to start a test cluster, the test environment is NOT cleaned after a test case or a test case file is done. Actually the test environment is not cleaned even after a complete test run. So a test case might impact the later test cases. Therefore please design the test cases with this constraint in mind. For example, the value of ${new_tenant} in the test is chosen to be a random string generated in each test run to make sure the test "kubectl create tenant ${new_tenant}" passes in each run without cleaning the environment.

If you want to clean the test environment, restart the test cluster by restarting the script hack/arktos-up.sh, and you will get a fresh and clean cluster.

* Smart Intelligence

Forget about anything like auto-logging, smart report and analysis. If you are really need a log, run the script with ">logfile" appended. I know you can do it as you are a real computer pro, since you reach the last line of a boring technical doc. :-)