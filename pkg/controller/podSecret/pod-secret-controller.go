/*
Copyright 2021 Authors of Arktos.

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

package podSecret

import (
	"fmt"
	"k8s.io/api/core/v1"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"

	"k8s.io/klog"
)

// The secret controller watches scheduled pods and set the label of the referenced secrets with the hostname
//
type PodSecretController struct {
	kubeClient clientset.Interface

	// A store of pods, populated by the shared informer
	podLister corelisters.PodLister
	// podListerSynced returns true if the pod store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	podListerSynced cache.InformerSynced

	// To allow injection for testing.
	patchSecret func(refKey string) error

	// Nodes that need to be synced.
	queue workqueue.RateLimitingInterface
}

func NewPodSecretController(podInformer coreinformers.PodInformer, kubeClient clientset.Interface) *PodSecretController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().EventsWithMultiTenancy(metav1.NamespaceAll, metav1.TenantAll)})

	sc := &PodSecretController{
		kubeClient:      kubeClient,
		podLister:       podInformer.Lister(),
		podListerSynced: podInformer.Informer().HasSynced,
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sc.podAdded,
		UpdateFunc: sc.podUpdated,
		DeleteFunc: sc.podDeleted,
	})
	sc.podLister = podInformer.Lister()
	sc.podListerSynced = podInformer.Informer().HasSynced

	sc.patchSecret = sc.syncPodSecret

	return sc
}

// Run begins watching and syncing.
func (sc *PodSecretController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer sc.queue.ShutDown()

	klog.Infof("Starting Secret controller")
	defer klog.Infof("Shutting down Secret controller")

	if !controller.WaitForCacheSync("secret", stopCh, sc.podListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(sc.worker, time.Second, stopCh)
	}

	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (sc *PodSecretController) worker() {
	for sc.processNextWorkItem() {
	}
}

func (sc *PodSecretController) processNextWorkItem() bool {
	refKey, quit := sc.queue.Get()
	klog.V(2).Infof("refKey: %v", refKey)

	if quit {
		return false
	}
	defer sc.queue.Done(refKey)

	err := sc.patchSecret(refKey.(string))
	if err == nil {
		sc.queue.Forget(refKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("patch %q failed with %v", refKey, err))
	sc.queue.AddRateLimited(refKey)

	return true
}

func (sc *PodSecretController) podAdded(newPod interface{}) {
	pod := newPod.(*v1.Pod)
	klog.V(3).Infof("New Pod added: %s-%s-%s", pod.Tenant, pod.Namespace, pod.Name)

	if pod.Spec.NodeName != "" {
		referencedSecretKeys := getPodSecretKeys(pod)
		for _, refKey := range referencedSecretKeys {
			sc.queue.Add(refKey)
		}
	}

	return
}

func (sc *PodSecretController) podUpdated(old, cur interface{}) {
	new := cur.(*v1.Pod)
	prev := old.(*v1.Pod)

	klog.V(3).Infof("Pod Updated:%s-%s-%s", new.Tenant, new.Namespace, new.Name)
	if new.Spec.NodeName != "" && prev.Spec.NodeName != new.Spec.NodeName {
		referencedSecretKeys := getPodSecretKeys(new)
		for _, refKey := range referencedSecretKeys {
			sc.queue.Add(refKey)
		}
	}

	return
}

// removal of the hostname label from secret is tricky:
// only remove IIF no pod on the host references to the secret
// TODO: add impl logic
func (sc *PodSecretController) podDeleted(obj interface{}) {
	return
}

func (sc *PodSecretController) syncPodSecret(refKey string) error {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing pod secret %q (%v)", refKey, time.Since(startTime))
	}()

	tenant, namespace, secretName, nodeName, err := splitKey(refKey)
	// TODO: consider to delete this key under error case
	if err != nil {
		return err
	}

	// TODO: optimize it, by checking local cache first by adding list/watch for secrets
	secret, err := sc.kubeClient.CoreV1().SecretsWithMultiTenancy(namespace, tenant).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	labels := secret.GetLabels()

	// if the label is already on the secret, just return
	if _, found := labels[nodeName]; found {
		return nil
	}

	if labels == nil {
		labels = make(map[string]string)
	}
	labels[nodeName] = ""
	secret.SetLabels(labels)
	_, err = sc.kubeClient.CoreV1().SecretsWithMultiTenancy(namespace, tenant).Update(secret)

	if err != nil {
		klog.Infof("Update secret failed with error: %v", err)
	}

	return err
}

func getPodSecretKeys(pod *v1.Pod) []string {
	var referenceSecrets []string

	if pod.Spec.ImagePullSecrets != nil {
		for _, sec := range pod.Spec.ImagePullSecrets {
			referenceSecrets = append(referenceSecrets, key(pod.Tenant, pod.Namespace, sec.Name, pod.Spec.NodeName))
		}
	}
	for _, vol := range pod.Spec.Volumes {
		if vol.Secret != nil {
			referenceSecrets = append(referenceSecrets, key(pod.Tenant, pod.Namespace, vol.Secret.SecretName, pod.Spec.NodeName))
		}
	}

	return referenceSecrets
}

func key(tenant, namespace, name, nodeName string) string {
	result := name + "/" + nodeName
	if len(namespace) > 0 {
		result = namespace + "/" + result
	} else {
		result = metav1.NamespaceDefault + "/" + result
	}
	if len(tenant) > 0 {
		result = tenant + "/" + result
	} else {
		result = metav1.TenantSystem + "/" + result
	}
	return result
}

func splitKey(key string) (string, string, string, string, error) {
	s := strings.Split(key, "/")

	if len(s) != 4 {
		return "", "", "", "", fmt.Errorf("invalid key")
	}

	return s[0], s[1], s[2], s[3], nil
}
