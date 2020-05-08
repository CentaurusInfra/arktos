# How to run e2e test in local dev environment

This doc is to describe how to run e2e test in your local dev environment. Please note, local dev environment is not fitting for many existing e2e test cases. So if those tests failed, it's not a surprise.

## Prerequisites

Arktos dev environment need to be ready. Please refer to [Set up developer environment](setup-dev-env.md).

E2e test is launched by kubetest. kubetest has limitations: it only runs under directory of kubernetes, and kubernetes must exist under folder $GOPATH/src/k8s.io/.

To run e2e test successfully, You have to move arktos directory to be under $GOPATH/src/k8s.io/ and rename it to kubernetes. 

## Install kubetest

 Please visit [kubetest](https://github.com/kubernetes/test-infra/blob/master/kubetest/README.md) to understand it. There is the latest info how to install kubetest if the script in this doc is not working.

```bash
cd $GOPATH/src/k8s.io
git clone https://github.com/kubernetes/test-infra.git
cd $GOPATH/src/k8s.io/test-infra/
GO111MODULE=on go install ./kubetest
```

kubetest will be installed in $GOPATH/bin/. You may want to add $GOPATH/bin/ to $PATH for easily invoking binaries under the path.

## Start local cluster

```bash
cd $GOPATH/src/k8s.io/kubernetes
make quick-release
./hack/arktos-up.sh
```

"make quick-release" is needed to build ginkgo for e2e test.

In the end of the output, you will see content like "cluster/kubectl.sh config set-cluster local --server=https://ip-172-30-0-88:6443"

The local-host-ip "ip-172-30-0-88" will be needed in the followed script. You need to change its value to what you see from output.


## Launch another terminal window and execute test

```bash
cd $GOPATH/src/k8s.io/kubernetes
export KUBECONFIG=/var/run/kubernetes/admin.kubeconfig
export KUBE_MASTER_URL=https://<local-host-ip>:6443
kubetest --test --test_args="--ginkgo.focus=RollingUpdateDeployment.?should.?delete.?old.?pods.?and.?create.?new.?ones --delete-namespace=false" --provider=local
```

Here needs some explanation for parameters of kubetest. For detailed and latest info, please check [kubetest](https://github.com/kubernetes/test-infra/blob/master/kubetest/README.md).

**--provider=local**:
Indicating it's running against local cluster.

**-delete-namespace=false**:
For now we have to put it to not deleting namespace after running e2e test. It's because a known issue: [namespace cannot be deleted and keep as "Terminating" status](https://github.com/futurewei-cloud/arktos/issues/187).

**--ginkgo.focus**:
It describes what's the test to run. It's using regular expression to match test name. In the example, it's to match a test case named "RollingUpdateDeployment should delete old pods and create new ones".  The test is located [here](https://github.com/futurewei-cloud/arktos/blob/master/test/e2e/apps/deployment.go#L82).

All the e2e tests are under https://github.com/futurewei-cloud/arktos/tree/master/test/e2e. Basically you can use regex to match any test case by name and run it. 
If you search "ginkgo.It(", you will find many test cases because test case name is after "ginkgo.It". 
Sometimes there are wrappers for "ginkgo.It". For example, you can search "framework.ConformanceIt(" which is Conformance test wrapper for "ginkgo.It".

