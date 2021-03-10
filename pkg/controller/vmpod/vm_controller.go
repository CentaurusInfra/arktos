/*
Copyright 2020 Authors of Arktos.

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

package vmpod

import (
	"k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	statusutil "k8s.io/kubernetes/pkg/util/pod"

	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/util/metrics"

	"k8s.io/klog"
)

type VMPodController struct {
	kubeClient      clientset.Interface
	podLister       corelisters.PodLister
	podListerSynced cache.InformerSynced
}

func NewVMPod(kubeClient clientset.Interface, podInformer coreinformers.PodInformer) *VMPodController {
	if kubeClient != nil && kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		metrics.RegisterMetricAndTrackRateLimiterUsage("vm_controller", kubeClient.CoreV1().RESTClient().GetRateLimiter())
	}
	vmc := &VMPodController{
		kubeClient: kubeClient,
	}

	vmc.podLister = podInformer.Lister()
	vmc.podListerSynced = podInformer.Informer().HasSynced

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: vmc.updatePod,
	})

	return vmc
}

func (vmc *VMPodController) updatePod(old, cur interface{}) {
	newPod := cur.(*v1.Pod)
	oldPod := old.(*v1.Pod)

	klog.V(3).Infof("in vm controller, pod %v is updated", newPod.Name)

	if !(oldPod.Status.Phase == v1.PodNoSchedule &&
		newPod.Spec.VirtualMachine != nil && newPod.Spec.VirtualMachine.PowerSpec == v1.VmPowerSpecRunning) {
		return
	}

	oldStatus := newPod.Status.DeepCopy()
	newPod.Status.Phase = v1.PodPending
	_, patchBytes, _, err := statusutil.PatchPodStatus(vmc.kubeClient, newPod.Tenant, newPod.Namespace, newPod.Name, newPod.UID, *oldStatus, newPod.Status)
	klog.V(3).Infof("Patch status for pod %q with %q", newPod.Name, patchBytes)
	if err != nil {
		klog.Warningf("Failed to update status for pod %q: %v", newPod.Name, err)
		return
	}
}

func (vmc *VMPodController) Run(stop <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer klog.Infof("Shutting down VM controller")
	if !controller.WaitForCacheSync("VM", stop, vmc.podListerSynced) {
		return
	}
	<-stop
}
