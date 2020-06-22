/*
Copyright 2016 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kuberuntime

import (
	"k8s.io/kubernetes/pkg/kubelet/runtimeregistry"
	"net/http"
	"time"

	cadvisorapi "github.com/google/cadvisor/info/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	internalapi "k8s.io/cri-api/pkg/apis"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/kubelet/cm"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/lifecycle"
	proberesults "k8s.io/kubernetes/pkg/kubelet/prober/results"
	"k8s.io/kubernetes/pkg/kubelet/util/logreduction"
)

const (
	fakeSeccompProfileRoot = "/fakeSeccompProfileRoot"
)

type fakeHTTP struct {
	url string
	err error
}

func (f *fakeHTTP) Get(url string) (*http.Response, error) {
	f.url = url
	return nil, f.err
}

type fakePodStateProvider struct {
	existingPods map[types.UID]struct{}
	runningPods  map[types.UID]struct{}
}

func newFakePodStateProvider() *fakePodStateProvider {
	return &fakePodStateProvider{
		existingPods: make(map[types.UID]struct{}),
		runningPods:  make(map[types.UID]struct{}),
	}
}

func (f *fakePodStateProvider) IsPodDeleted(uid types.UID) bool {
	_, found := f.existingPods[uid]
	return !found
}

func (f *fakePodStateProvider) IsPodTerminated(uid types.UID) bool {
	_, found := f.runningPods[uid]
	return !found
}

func newFakeKubeRuntimeManager(rs internalapi.RuntimeService, is internalapi.ImageManagerService, machineInfo *cadvisorapi.MachineInfo, osInterface kubecontainer.OSInterface, runtimeHelper kubecontainer.RuntimeHelper, keyring credentialprovider.DockerKeyring) (*kubeGenericRuntimeManager, error) {
	recorder := &record.FakeRecorder{}

	// TODO: Add new UTs and dynamic set the filed for the runtimeService type
	//
	defaultRuntimeServiceName = reservedDefaultRuntimeServiceName
	imageServices := make(map[string]*runtimeregistry.ImageService)
	imageServices[defaultRuntimeServiceName] = &runtimeregistry.ImageService{}
	imageServices[defaultRuntimeServiceName].ServiceApi = is
	imageServices[defaultRuntimeServiceName].Name = reservedDefaultRuntimeServiceName
	imageServices[defaultRuntimeServiceName].WorkloadType = runtimeregistry.ContainerWorkloadType
	imageServices[defaultRuntimeServiceName].IsDefault = true

	runtimeServices := make(map[string]*runtimeregistry.RuntimeService)
	runtimeServices[defaultRuntimeServiceName] = &runtimeregistry.RuntimeService{}
	runtimeServices[defaultRuntimeServiceName].ServiceApi = rs
	runtimeServices[defaultRuntimeServiceName].Name = reservedDefaultRuntimeServiceName
	runtimeServices[defaultRuntimeServiceName].WorkloadType = runtimeregistry.ContainerWorkloadType
	runtimeServices[defaultRuntimeServiceName].IsDefault = true
	runtimeServices[defaultRuntimeServiceName].IsPrimary = true

	rr := runtimeregistry.KubeRuntimeRegistry{RuntimeServices: runtimeServices, ImageServices: imageServices}

	kubeRuntimeManager := &kubeGenericRuntimeManager{
		recorder:            recorder,
		cpuCFSQuota:         false,
		cpuCFSQuotaPeriod:   metav1.Duration{Duration: time.Microsecond * 100},
		livenessManager:     proberesults.NewManager(),
		containerRefManager: kubecontainer.NewRefManager(),
		machineInfo:         machineInfo,
		osInterface:         osInterface,
		runtimeHelper:       runtimeHelper,
		keyring:             keyring,
		seccompProfileRoot:  fakeSeccompProfileRoot,
		internalLifecycle:   cm.NewFakeInternalContainerLifecycle(),
		logReduction:        logreduction.NewLogReduction(identicalErrorDelay),
		runtimeRegistry:     &rr,
	}

	kubeRuntimeManager.podRuntimeServiceMap = make(map[string]internalapi.RuntimeService)

	typedVersion, err := rs.Version(kubeRuntimeAPIVersion)
	if err != nil {
		return nil, err
	}

	kubeRuntimeManager.containerGC = newContainerGC(rs, newFakePodStateProvider(), kubeRuntimeManager)
	kubeRuntimeManager.runtimeName = typedVersion.RuntimeName
	kubeRuntimeManager.runner = lifecycle.NewHandlerRunner(
		&fakeHTTP{},
		kubeRuntimeManager,
		kubeRuntimeManager)

	return kubeRuntimeManager, nil
}
