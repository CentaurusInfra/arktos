/*
Copyright 2014 The Kubernetes Authors.
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

package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	internalqueue "k8s.io/kubernetes/pkg/scheduler/internal/queue"
	"net/http"
	"os"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	policyinformers "k8s.io/client-go/informers/policy/v1beta1"
	storageinformers "k8s.io/client-go/informers/storage/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/pkg/scheduler/api/latest"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/pkg/scheduler/core"
	"k8s.io/kubernetes/pkg/scheduler/factory"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	internalcache "k8s.io/kubernetes/pkg/scheduler/internal/cache"
	"k8s.io/kubernetes/pkg/scheduler/metrics"
)

const (
	// BindTimeoutSeconds defines the default bind timeout
	BindTimeoutSeconds = 100
	// SchedulerError is the reason recorded for events when an error occurs during scheduling a pod.
	SchedulerError = "SchedulerError"
	// ErrorRescheduleTimesLimits is the maximum global scheduler retry times
	ErrorRescheduleTimesLimits = 3
)

// Global token map storing token which avoid generating token every time
var tokenMap = make(map[string]string)

// Global queue storing unscheduled pod
var scheduleResultQueue = internalqueue.New(1)

// Global schedule retry times
var errorRescheduleTimes = make(map[string]int)

// Scheduler watches for new unscheduled pods. It attempts to find
// nodes that they fit on and writes bindings back to the api server.
type Scheduler struct {
	config *factory.Config
}

// Cache returns the cache in scheduler for test to check the data in scheduler.
func (sched *Scheduler) Cache() internalcache.Cache {
	return sched.config.SchedulerCache
}

type schedulerOptions struct {
	schedulerName                  string
	hardPodAffinitySymmetricWeight int32
	disablePreemption              bool
	percentageOfNodesToScore       int32
	bindTimeoutSeconds             int64
}

type server struct {
	Name           string              `json:"name"`
	ImageRef       string              `json:"imageRef"`
	FlavorRef      string              `json:"flavorRef"`
	Networks       []map[string]string `json:"networks"`
	SecurityGroups []map[string]string `json:"security_groups"`
}

type podWithDecision struct {
	schedulePod *v1.Pod
	result      core.ScheduleResult
}

// Option configures a Scheduler
type Option func(*schedulerOptions)

// WithName sets schedulerName for Scheduler, the default schedulerName is default-scheduler
func WithName(schedulerName string) Option {
	return func(o *schedulerOptions) {
		o.schedulerName = schedulerName
	}
}

// WithHardPodAffinitySymmetricWeight sets hardPodAffinitySymmetricWeight for Scheduler, the default value is 1
func WithHardPodAffinitySymmetricWeight(hardPodAffinitySymmetricWeight int32) Option {
	return func(o *schedulerOptions) {
		o.hardPodAffinitySymmetricWeight = hardPodAffinitySymmetricWeight
	}
}

// WithPreemptionDisabled sets disablePreemption for Scheduler, the default value is false
func WithPreemptionDisabled(disablePreemption bool) Option {
	return func(o *schedulerOptions) {
		o.disablePreemption = disablePreemption
	}
}

// WithPercentageOfNodesToScore sets percentageOfNodesToScore for Scheduler, the default value is 50
func WithPercentageOfNodesToScore(percentageOfNodesToScore int32) Option {
	return func(o *schedulerOptions) {
		o.percentageOfNodesToScore = percentageOfNodesToScore
	}
}

// WithBindTimeoutSeconds sets bindTimeoutSeconds for Scheduler, the default value is 100
func WithBindTimeoutSeconds(bindTimeoutSeconds int64) Option {
	return func(o *schedulerOptions) {
		o.bindTimeoutSeconds = bindTimeoutSeconds
	}
}

var defaultSchedulerOptions = schedulerOptions{
	schedulerName:                  v1.DefaultSchedulerName,
	hardPodAffinitySymmetricWeight: v1.DefaultHardPodAffinitySymmetricWeight,
	disablePreemption:              false,
	percentageOfNodesToScore:       schedulerapi.DefaultPercentageOfNodesToScore,
	bindTimeoutSeconds:             BindTimeoutSeconds,
}

// New returns a Scheduler
func New(client clientset.Interface,
	nodeInformer coreinformers.NodeInformer,
	podInformer coreinformers.PodInformer,
	pvInformer coreinformers.PersistentVolumeInformer,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	replicationControllerInformer coreinformers.ReplicationControllerInformer,
	replicaSetInformer appsinformers.ReplicaSetInformer,
	statefulSetInformer appsinformers.StatefulSetInformer,
	serviceInformer coreinformers.ServiceInformer,
	pdbInformer policyinformers.PodDisruptionBudgetInformer,
	storageClassInformer storageinformers.StorageClassInformer,
	recorder record.EventRecorder,
	schedulerAlgorithmSource kubeschedulerconfig.SchedulerAlgorithmSource,
	stopCh <-chan struct{},
	registry framework.Registry,
	plugins *kubeschedulerconfig.Plugins,
	pluginConfig []kubeschedulerconfig.PluginConfig,
	opts ...func(o *schedulerOptions)) (*Scheduler, error) {

	options := defaultSchedulerOptions
	for _, opt := range opts {
		opt(&options)
	}
	// Set up the configurator which can create schedulers from configs.
	configurator := factory.NewConfigFactory(&factory.ConfigFactoryArgs{
		SchedulerName:                  options.schedulerName,
		Client:                         client,
		NodeInformer:                   nodeInformer,
		PodInformer:                    podInformer,
		PvInformer:                     pvInformer,
		PvcInformer:                    pvcInformer,
		ReplicationControllerInformer:  replicationControllerInformer,
		ReplicaSetInformer:             replicaSetInformer,
		StatefulSetInformer:            statefulSetInformer,
		ServiceInformer:                serviceInformer,
		PdbInformer:                    pdbInformer,
		StorageClassInformer:           storageClassInformer,
		HardPodAffinitySymmetricWeight: options.hardPodAffinitySymmetricWeight,
		DisablePreemption:              options.disablePreemption,
		PercentageOfNodesToScore:       options.percentageOfNodesToScore,
		BindTimeoutSeconds:             options.bindTimeoutSeconds,
		Registry:                       registry,
		Plugins:                        plugins,
		PluginConfig:                   pluginConfig,
	})
	var config *factory.Config
	source := schedulerAlgorithmSource
	switch {
	case source.Provider != nil:
		// Create the config from a named algorithm provider.
		sc, err := configurator.CreateFromProvider(*source.Provider)
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler using provider %q: %v", *source.Provider, err)
		}
		config = sc
	case source.Policy != nil:
		// Create the config from a user specified policy source.
		policy := &schedulerapi.Policy{}
		switch {
		case source.Policy.File != nil:
			if err := initPolicyFromFile(source.Policy.File.Path, policy); err != nil {
				return nil, err
			}
		case source.Policy.ConfigMap != nil:
			if err := initPolicyFromConfigMap(client, source.Policy.ConfigMap, policy); err != nil {
				return nil, err
			}
		}
		sc, err := configurator.CreateFromConfig(*policy)
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler from policy: %v", err)
		}
		config = sc
	default:
		return nil, fmt.Errorf("unsupported algorithm source: %v", source)
	}
	// Additional tweaks to the config produced by the configurator.
	config.Recorder = recorder
	config.DisablePreemption = options.disablePreemption
	config.StopEverything = stopCh

	// Create the scheduler.
	sched := NewFromConfig(config)

	AddAllEventHandlers(sched, options.schedulerName, nodeInformer, podInformer, pvInformer, pvcInformer, serviceInformer, storageClassInformer)
	return sched, nil
}

// initPolicyFromFile initialize policy from file
func initPolicyFromFile(policyFile string, policy *schedulerapi.Policy) error {
	// Use a policy serialized in a file.
	_, err := os.Stat(policyFile)
	if err != nil {
		return fmt.Errorf("missing policy config file %s", policyFile)
	}
	data, err := ioutil.ReadFile(policyFile)
	if err != nil {
		return fmt.Errorf("couldn't read policy config: %v", err)
	}
	err = runtime.DecodeInto(latestschedulerapi.Codec, []byte(data), policy)
	if err != nil {
		return fmt.Errorf("invalid policy: %v", err)
	}
	return nil
}

// initPolicyFromConfigMap initialize policy from configMap
func initPolicyFromConfigMap(client clientset.Interface, policyRef *kubeschedulerconfig.SchedulerPolicyConfigMapSource, policy *schedulerapi.Policy) error {
	// Use a policy serialized in a config map value.
	policyConfigMap, err := client.CoreV1().ConfigMaps(policyRef.Namespace).Get(policyRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("couldn't get policy config map %s/%s: %v", policyRef.Namespace, policyRef.Name, err)
	}
	data, found := policyConfigMap.Data[kubeschedulerconfig.SchedulerPolicyConfigMapKey]
	if !found {
		return fmt.Errorf("missing policy config map value at key %q", kubeschedulerconfig.SchedulerPolicyConfigMapKey)
	}
	err = runtime.DecodeInto(latestschedulerapi.Codec, []byte(data), policy)
	if err != nil {
		return fmt.Errorf("invalid policy: %v", err)
	}
	return nil
}

// NewFromConfig returns a new scheduler using the provided Config.
func NewFromConfig(config *factory.Config) *Scheduler {
	metrics.Register()
	return &Scheduler{
		config: config,
	}
}

// Run begins watching and scheduling. It waits for cache to be synced, then starts a goroutine and returns immediately.
func (sched *Scheduler) Run() {
	if !sched.config.WaitForCacheSync() {
		return
	}

	go wait.Until(sched.scheduleOne, 0, sched.config.StopEverything)
	// go wait.Until(sched.globalScheduleOne, 0, sched.config.StopEverything)
}

// Config returns scheduler's config pointer. It is exposed for testing purposes.
func (sched *Scheduler) Config() *factory.Config {
	return sched.config
}

// recordFailedSchedulingEvent records an event for the pod that indicates the
// pod has failed to schedule.
// NOTE: This function modifies "pod". "pod" should be copied before being passed.
func (sched *Scheduler) recordSchedulingFailure(pod *v1.Pod, err error, reason string, message string) {
	sched.config.Error(pod, err)
	sched.config.Recorder.Event(pod, v1.EventTypeWarning, "FailedScheduling", message)
	sched.config.PodConditionUpdater.Update(pod, &v1.PodCondition{
		Type:    v1.PodScheduled,
		Status:  v1.ConditionFalse,
		Reason:  reason,
		Message: err.Error(),
	})
}

// schedule implements the scheduling algorithm and returns the suggested result(host,
// evaluated nodes number,feasible nodes number).
func (sched *Scheduler) schedule(pod *v1.Pod, pluginContext *framework.PluginContext) (core.ScheduleResult, error) {
	result, err := sched.config.Algorithm.Schedule(pod, sched.config.NodeLister, pluginContext)
	if err != nil {
		pod = pod.DeepCopy()
		sched.recordSchedulingFailure(pod, err, v1.PodReasonUnschedulable, err.Error())
		return core.ScheduleResult{}, err
	}
	return result, err
}

func (sched *Scheduler) globalSchedule(pod *v1.Pod) (core.ScheduleResult, error) {
	result, err := sched.config.Algorithm.GlobalSchedule(pod)
	return result, err
}

// preempt tries to create room for a pod that has failed to schedule, by preempting lower priority pods if possible.
// If it succeeds, it adds the name of the node where preemption has happened to the pod spec.
// It returns the node name and an error if any.
func (sched *Scheduler) preempt(preemptor *v1.Pod, scheduleErr error) (string, error) {
	preemptor, err := sched.config.PodPreemptor.GetUpdatedPod(preemptor)
	if err != nil {
		klog.Errorf("Error getting the updated preemptor pod object: %v", err)
		return "", err
	}

	node, victims, nominatedPodsToClear, err := sched.config.Algorithm.Preempt(preemptor, sched.config.NodeLister, scheduleErr)
	if err != nil {
		klog.Errorf("Error preempting victims to make room for %v/%v/%v.", preemptor.Tenant, preemptor.Namespace, preemptor.Name)
		return "", err
	}
	var nodeName = ""
	if node != nil {
		nodeName = node.Name
		// Update the scheduling queue with the nominated pod information. Without
		// this, there would be a race condition between the next scheduling cycle
		// and the time the scheduler receives a Pod Update for the nominated pod.
		sched.config.SchedulingQueue.UpdateNominatedPodForNode(preemptor, nodeName)

		// Make a call to update nominated node name of the pod on the API server.
		err = sched.config.PodPreemptor.SetNominatedNodeName(preemptor, nodeName)
		if err != nil {
			klog.Errorf("Error in preemption process. Cannot set 'NominatedPod' on pod %v/%v/%v: %v", preemptor.Tenant, preemptor.Namespace, preemptor.Name, err)
			sched.config.SchedulingQueue.DeleteNominatedPodIfExists(preemptor)
			return "", err
		}

		for _, victim := range victims {
			if err := sched.config.PodPreemptor.DeletePod(victim); err != nil {
				klog.Errorf("Error preempting pod %v/%v/%v: %v", victim.Tenant, victim.Namespace, victim.Name, err)
				return "", err
			}
			sched.config.Recorder.Eventf(victim, v1.EventTypeNormal, "Preempted", "by %v/%v/%v on node %v", preemptor.Tenant, preemptor.Namespace, preemptor.Name, nodeName)
		}
		metrics.PreemptionVictims.Set(float64(len(victims)))
	}
	// Clearing nominated pods should happen outside of "if node != nil". Node could
	// be nil when a pod with nominated node name is eligible to preempt again,
	// but preemption logic does not find any node for it. In that case Preempt()
	// function of generic_scheduler.go returns the pod itself for removal of
	// the 'NominatedPod' field.
	for _, p := range nominatedPodsToClear {
		rErr := sched.config.PodPreemptor.RemoveNominatedNodeName(p)
		if rErr != nil {
			klog.Errorf("Cannot remove 'NominatedPod' field of pod: %v", rErr)
			// We do not return as this error is not critical.
		}
	}
	return nodeName, err
}

// assumeVolumes will update the volume cache with the chosen bindings
//
// This function modifies assumed if volume binding is required.
func (sched *Scheduler) assumeVolumes(assumed *v1.Pod, host string) (allBound bool, err error) {
	allBound, err = sched.config.VolumeBinder.Binder.AssumePodVolumes(assumed, host)
	if err != nil {
		sched.recordSchedulingFailure(assumed, err, SchedulerError,
			fmt.Sprintf("AssumePodVolumes failed: %v", err))
	}
	return
}

// bindVolumes will make the API update with the assumed bindings and wait until
// the PV controller has completely finished the binding operation.
//
// If binding errors, times out or gets undone, then an error will be returned to
// retry scheduling.
func (sched *Scheduler) bindVolumes(assumed *v1.Pod) error {
	klog.V(5).Infof("Trying to bind volumes for pod \"%v/%v/%v\"", assumed.Tenant, assumed.Namespace, assumed.Name)
	err := sched.config.VolumeBinder.Binder.BindPodVolumes(assumed)
	if err != nil {
		klog.V(1).Infof("Failed to bind volumes for pod \"%v/%v/%v\": %v", assumed.Tenant, assumed.Namespace, assumed.Name, err)

		// Unassume the Pod and retry scheduling
		if forgetErr := sched.config.SchedulerCache.ForgetPod(assumed); forgetErr != nil {
			klog.Errorf("scheduler cache ForgetPod failed: %v", forgetErr)
		}

		sched.recordSchedulingFailure(assumed, err, "VolumeBindingFailed", err.Error())
		return err
	}

	klog.V(5).Infof("Success binding volumes for pod \"%v/%v/%v\"", assumed.Tenant, assumed.Namespace, assumed.Name)
	return nil
}

// assume signals to the cache that a pod is already in the cache, so that binding can be asynchronous.
// assume modifies `assumed`.
func (sched *Scheduler) assume(assumed *v1.Pod, host string) error {
	// Optimistically assume that the binding will succeed and send it to apiserver
	// in the background.
	// If the binding fails, scheduler will release resources allocated to assumed pod
	// immediately.
	assumed.Spec.NodeName = host

	if err := sched.config.SchedulerCache.AssumePod(assumed); err != nil {
		klog.Errorf("scheduler cache AssumePod failed: %v", err)

		// This is most probably result of a BUG in retrying logic.
		// We report an error here so that pod scheduling can be retried.
		// This relies on the fact that Error will check if the pod has been bound
		// to a node and if so will not add it back to the unscheduled pods queue
		// (otherwise this would cause an infinite loop).
		sched.recordSchedulingFailure(assumed, err, SchedulerError,
			fmt.Sprintf("AssumePod failed: %v", err))
		return err
	}
	// if "assumed" is a nominated pod, we should remove it from internal cache
	if sched.config.SchedulingQueue != nil {
		sched.config.SchedulingQueue.DeleteNominatedPodIfExists(assumed)
	}

	return nil
}

// bind binds a pod to a given node defined in a binding object.  We expect this to run asynchronously, so we
// handle binding metrics internally.
func (sched *Scheduler) bind(assumed *v1.Pod, b *v1.Binding) error {
	// bindingStart := time.Now()
	// If binding succeeded then PodScheduled condition will be updated in apiserver so that
	// it's atomic with setting host.
	err := sched.config.GetBinder(assumed).Bind(b)
	if finErr := sched.config.SchedulerCache.FinishBinding(assumed); finErr != nil {
		klog.Errorf("scheduler cache FinishBinding failed: %v", finErr)
	} else {
		klog.V(3).Infof("scheduler FinishBinding pass")
	}

	if err != nil {
		klog.V(1).Infof("Failed to bind pod: %v/%v/%v", assumed.Tenant, assumed.Namespace, assumed.Name)
		if err := sched.config.SchedulerCache.ForgetPod(assumed); err != nil {
			klog.Errorf("scheduler cache ForgetPod failed: %v", err)
		}
		sched.recordSchedulingFailure(assumed, err, SchedulerError,
			fmt.Sprintf("Binding rejected: %v", err))
		return err
	} else {
		klog.V(3).Infof("scheduler binding pass")
	}

	// metrics.BindingLatency.Observe(metrics.SinceInSeconds(bindingStart))
	// metrics.DeprecatedBindingLatency.Observe(metrics.SinceInMicroseconds(bindingStart))
	// metrics.SchedulingLatency.WithLabelValues(metrics.Binding).Observe(metrics.SinceInSeconds(bindingStart))
	// metrics.DeprecatedSchedulingLatency.WithLabelValues(metrics.Binding).Observe(metrics.SinceInSeconds(bindingStart))
	sched.config.Recorder.Eventf(assumed, v1.EventTypeNormal, "Scheduled", "Successfully assigned %v/%v/%v to %v", assumed.Tenant, assumed.Namespace, assumed.Name, b.Target.Name)
	return nil
}

func requestToken(host string) (string, error) {
	tokenRequestURL := "http://" + host + "/identity/v3/auth/tokens"

	// TODO: Please don't hard code json data
	tokenJsonData := `{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"admin","domain":{"id":"default"},"password":"secret"}}},"scope":{"project":{"name":"admin","domain":{"id":"default"}}}}}`

	// Make HTTP Request
	var tokenJsonDataBytes = []byte(tokenJsonData)
	req, _ := http.NewRequest("POST", tokenRequestURL, bytes.NewBuffer(tokenJsonDataBytes))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		klog.V(3).Infof("HTTP Post Token Request Failed: %v", err)
		return "", err
	}
	klog.V(3).Infof("HTTP Post Request Succeeded")
	defer resp.Body.Close()

	// http.StatusCreated = 201
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("Instance capacity has reached its limit")
	}

	// Token is stored in header
	respHeader := resp.Header
	klog.V(3).Infof("HTTP Post Token: %v", respHeader["X-Subject-Token"][0])

	return respHeader["X-Subject-Token"][0], nil
}

func serverCreate(host string, authToken string, manifest *v1.PodSpec) (string, error) {
	serverCreateRequestURL := "http://" + host + "/compute/v2.1/servers"
	serverStruct := server{
		Name:      manifest.VirtualMachine.Name,
		ImageRef:  manifest.VirtualMachine.Image,
		FlavorRef: manifest.VirtualMachine.FlavorRef,
		Networks: []map[string]string{
			{"uuid": manifest.Nics[0].Uuid},
		},
		SecurityGroups: []map[string]string{
			{"name": manifest.VirtualMachine.Scheduling.SecurityGroup[0].Name},
		},
	}
	serverJson := map[string]server{}
	serverJson["server"] = serverStruct
	finalData, _ := json.Marshal(serverJson)
	req, _ := http.NewRequest("POST", serverCreateRequestURL, bytes.NewBuffer(finalData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", authToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		klog.V(3).Infof("HTTP Post Instance Request Failed: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var instanceResponse map[string]interface{}
	if err := json.Unmarshal(body, &instanceResponse); err != nil {
		klog.V(3).Infof("Instance Create Response Unmarshal Failed")
		return "", err
	}

	// http.StatusForbidden = 403
	if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("Instance capacity has reached its limit")
	}

	if instanceResponse["server"] == nil {
		return "", fmt.Errorf("Bad request for server create")
	}
	serverResponse := instanceResponse["server"].(map[string]interface{})
	instanceID := serverResponse["id"].(string)

	return instanceID, nil
}

func checkInstanceStatus(host string, authToken string, instanceID string) (string, error) {
	instanceDetailsURL := "http://" + host + "/compute/v2.1/servers/" + instanceID
	req, _ := http.NewRequest("GET", instanceDetailsURL, nil)
	req.Header.Set("X-Auth-Token", authToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		klog.V(3).Infof("HTTP GET Instance Status Request Failed: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var instanceDetailsResponse map[string]interface{}
	if err := json.Unmarshal(body, &instanceDetailsResponse); err != nil {
		klog.V(3).Infof("Instance Detail Response Unmarshal Failed")
		return "", err
	}

	if instanceDetailsResponse["server"] == nil {
		return "", fmt.Errorf("Bad request for instance status check")
	}
	serverResponse := instanceDetailsResponse["server"].(map[string]interface{})
	instanceStatus := serverResponse["status"].(string)

	return instanceStatus, nil
}

func deleteInstance(host string, authToken string, instanceID string) error {
	instanceDetailsURL := "http://" + host + "/compute/v2.1/servers/" + instanceID
	req, _ := http.NewRequest("DELETE", instanceDetailsURL, nil)
	req.Header.Set("X-Auth-Token", authToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		klog.V(3).Infof("HTTP DELETE Instance Status Request Failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	// http.StatusNoContent = 204
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Bad request for HTTP DELETE instance request")
	} else {
		klog.V(3).Infof("HTTP DELETE Instance Status Request Success")
		return nil
	}
}

func tokenExpired(host string, authToken string) bool {
	checkTokenURL := "http://" + host + "/identity/v3/auth/tokens"
	req, _ := http.NewRequest("HEAD", checkTokenURL, nil)
	req.Header.Set("X-Auth-Token", authToken)
	req.Header.Set("X-Subject-Token", authToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		klog.V(3).Infof("HTTP Check Token Request Failed: %v", err)
	}
	defer resp.Body.Close()

	// http.StatusOK = 200
	if resp.StatusCode != http.StatusOK {
		klog.V(3).Infof("Token Expired")
		return true
	}
	klog.V(3).Infof("Token Not Expired")
	return false
}

func (sched *Scheduler) reschedule(pod *v1.Pod, rescheduleReason string) {
	errorRescheduleTimes[pod.Name]++
	// This action will enqueue the pod to scheduling queue,
	// and rerun scheduleOne() function
	sched.config.PodConditionUpdater.Update(pod, &v1.PodCondition{
		Type:    v1.PodScheduleFailed,
		Status:  v1.ConditionFalse,
		Reason:  rescheduleReason,
		Message: "Global Scheduler Retry Times: " + strconv.Itoa(errorRescheduleTimes[pod.Name]),
	})
}

func (sched *Scheduler) scheduleInstanceStatusCheck(host string, authToken string, instanceID string, pod *v1.Pod, manifest *v1.PodSpec) {
	instanceStatus, err := checkInstanceStatus(host, authToken, instanceID)
	if err != nil {
		return
	}

	// count indicate the duration time of BUILD status
	count := 0
	for {
		if instanceStatus == "BUILD" {
			klog.V(3).Infof("Instance Status: %v", instanceStatus)
			count++
			// Wait two minutes for creating instance if instance status is BUILD
			if count == 60 {
				klog.V(3).Infof("Create Instance Timeout!")
				if err := deleteInstance(host, authToken, instanceID); err != nil {
					klog.V(3).Infof("Instance Delete Failed!")
				}
				metrics.PodScheduleErrors.Inc()
				sched.reschedule(pod, "Create Instance Timeout")
				break
			}
			time.Sleep(2 * time.Second)
			instanceStatus, err = checkInstanceStatus(host, authToken, instanceID)
			if err != nil {
				return
			}
		} else if instanceStatus == "ERROR" {
			klog.V(3).Infof("Instance Status: %v", instanceStatus)
			// Send delete instance request
			if err := deleteInstance(host, authToken, instanceID); err != nil {
				klog.V(3).Infof("Instance Delete Failed!")
			}
			metrics.PodScheduleErrors.Inc()
			sched.reschedule(pod, "Global Schedule Failed")
			break
		} else if instanceStatus == "ACTIVE" {
			klog.V(3).Infof("Instance Status: %v", instanceStatus)
			metrics.PodScheduleSuccesses.Inc()
			sched.config.PodPhaseUpdater.Update(pod, v1.PodRunning)
			break
		}
	}
}

func (sched *Scheduler) scheduleResultDequeue(scheduleResultQueue *internalqueue.Queue, tokenMap map[string]string) {
	res, _ := scheduleResultQueue.Get(1)
	curPodWithDecision := res[0].(podWithDecision)
	host := curPodWithDecision.result.SuggestedHost
	manifest := &(curPodWithDecision.schedulePod.Spec)

	// Get a OpenStack token
	authToken, exist := tokenMap[host]
	if !exist || tokenExpired(host, authToken) {
		// Post Request a new token
		newToken, err := requestToken(host)
		if err != nil {
			klog.V(3).Infof("Token Request Failed.")
			sched.reschedule(curPodWithDecision.schedulePod, "Token Request Failed")
			return
		}
		authToken = newToken
		// Update tokenMap
		tokenMap[host] = authToken
	}

	// Create server in OpenStack
	instanceID, err := serverCreate(host, authToken, manifest)
	if err != nil {
		klog.V(3).Infof("Server create failed")
		sched.reschedule(curPodWithDecision.schedulePod, "Server create failed")
		return
	}
	klog.V(3).Infof("Instance ID: %v", instanceID)

	// Check OpenStack server status: BUILD / ERROR / ACTIVE
	sched.scheduleInstanceStatusCheck(host, authToken, instanceID, curPodWithDecision.schedulePod, manifest)
}

// scheduleOne does the entire scheduling workflow for a single pod.  It is serialized on the scheduling algorithm's host fitting.
func (sched *Scheduler) scheduleOne() {

	// Get the Pod to be scheduled from the queue
	pod := sched.config.NextPod()
	// pod could be nil when schedulerQueue is closed
	if pod == nil {
		return
	}
	if pod.DeletionTimestamp != nil {
		sched.config.Recorder.Eventf(pod, v1.EventTypeWarning, "FailedScheduling", "skip schedule deleting pod: %v/%v/%v", pod.Tenant, pod.Namespace, pod.Name)
		klog.V(3).Infof("Skip schedule deleting pod: %v/%v/%v", pod.Tenant, pod.Namespace, pod.Name)
		return
	}

	// if pod.Name doesn't exist in errorRescheduleTimes, errorRescheduleTimes[pod.Name] = 0
	if errorRescheduleTimes[pod.Name] == ErrorRescheduleTimesLimits {
		sched.config.PodPhaseUpdater.Update(pod, v1.PodFailed)
		return
	}

	klog.V(3).Infof("Attempting to schedule pod: %v/%v/%v", pod.Tenant, pod.Namespace, pod.Name)

	// Synchronously attempt to find a fit for the pod.
	start := time.Now()

	scheduleResult, err := sched.globalSchedule(pod)
	if err != nil {
		// TODO: err may have various types
		klog.Errorf("error selecting cluster for pod: %v", err)
		metrics.PodScheduleErrors.Inc()
		return
	}

	// Record the schedule result corresponding to the pod
	newPodWithDeciion := podWithDecision{
		schedulePod: pod,
		result:      scheduleResult,
	}

	metrics.SchedulingAlgorithmLatency.Observe(metrics.SinceInSeconds(start))
	metrics.DeprecatedSchedulingAlgorithmLatency.Observe(metrics.SinceInMicroseconds(start))

	// TODO: Update resources before Enqueue

	// Enqueue scheduleResultQueue with newPodWithDeciion struct
	scheduleResultQueue.Put(newPodWithDeciion)
	go sched.scheduleResultDequeue(scheduleResultQueue, tokenMap)
}
