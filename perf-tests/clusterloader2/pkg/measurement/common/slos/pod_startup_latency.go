/*
Copyright 2018 The Kubernetes Authors.

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

package slos

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"k8s.io/kubernetes/perf-tests/clusterloader2/pkg/errors"
	"k8s.io/kubernetes/perf-tests/clusterloader2/pkg/measurement"
	measurementutil "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/measurement/util"
	"k8s.io/kubernetes/perf-tests/clusterloader2/pkg/measurement/util/informer"
	"k8s.io/kubernetes/perf-tests/clusterloader2/pkg/util"
)

const (
	defaultPodStartupLatencyThreshold = 5 * time.Second
	podStartupLatencyMeasurementName  = "PodStartupLatency"
	informerSyncTimeout               = time.Minute

	createPhase   = "create"
	schedulePhase = "schedule"
	runPhase      = "run"
	watchPhase    = "watch"
)

func init() {
	measurement.Register(podStartupLatencyMeasurementName, createPodStartupLatencyMeasurement)
}

func createPodStartupLatencyMeasurement() measurement.Measurement {
	return &podStartupLatencyMeasurement{
		selector:          measurementutil.NewObjectSelector(),
		podStartupEntries: measurementutil.NewObjectTransitionTimes(podStartupLatencyMeasurementName),
		eventQueue:        workqueue.New(),
	}
}

type eventData struct {
	obj      interface{}
	recvTime time.Time
}

type podStartupLatencyMeasurement struct {
	selector          *measurementutil.ObjectSelector
	isRunning         bool
	stopCh            chan struct{}
	podStartupEntries *measurementutil.ObjectTransitionTimes
	threshold         time.Duration
	// This queue can potentially grow indefinitely, beacause we put all changes here.
	// Usually it's not recommended pattern, but we need it for measuring PodStartupLatency.
	eventQueue *workqueue.Type
}

// Execute supports two actions:
// - start - Starts to observe pods and pods events.
// - gather - Gathers and prints current pod latency data.
// Does NOT support concurrency. Multiple calls to this measurement
// shouldn't be done within one step.
func (p *podStartupLatencyMeasurement) Execute(config *measurement.MeasurementConfig) ([]measurement.Summary, error) {
	action, err := util.GetString(config.Params, "action")
	if err != nil {
		return nil, err
	}

	switch action {
	case "start":
		if err := p.selector.Parse(config.Params); err != nil {
			return nil, err
		}
		p.threshold, err = util.GetDurationOrDefault(config.Params, "threshold", defaultPodStartupLatencyThreshold)
		if err != nil {
			return nil, err
		}
		return nil, p.start(config.ClusterFramework.GetClientSets().GetClient())
	case "gather":
		return p.gather(config.ClusterFramework.GetClientSets().GetClient(), config.Identifier)
	default:
		return nil, fmt.Errorf("unknown action %v", action)
	}

}

// Dispose cleans up after the measurement.
func (p *podStartupLatencyMeasurement) Dispose() {
	p.stop()
}

// String returns string representation of this measurement.
func (p *podStartupLatencyMeasurement) String() string {
	return podStartupLatencyMeasurementName + ": " + p.selector.String()
}

func (p *podStartupLatencyMeasurement) start(c clientset.Interface) error {
	if p.isRunning {
		klog.Infof("%s: pod startup latancy measurement already running", p)
		return nil
	}
	klog.Infof("%s: starting pod startup latency measurement...", p)
	p.isRunning = true
	p.stopCh = make(chan struct{})
	i := informer.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				p.selector.ApplySelectors(&options)
				return c.CoreV1().PodsWithMultiTenancy(p.selector.Namespace, util.GetTenant()).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				p.selector.ApplySelectors(&options)
				return c.CoreV1().PodsWithMultiTenancy(p.selector.Namespace, util.GetTenant()).Watch(options)
			},
		},
		p.addEvent,
	)
	go p.processEvents()
	return informer.StartAndSync(i, p.stopCh, informerSyncTimeout)
}

func (p *podStartupLatencyMeasurement) addEvent(_, obj interface{}) {
	event := &eventData{obj: obj, recvTime: time.Now()}
	p.eventQueue.Add(event)
}

func (p *podStartupLatencyMeasurement) processEvents() {
	for p.processNextWorkItem() {
	}
}

func (p *podStartupLatencyMeasurement) processNextWorkItem() bool {
	item, quit := p.eventQueue.Get()
	if quit {
		return false
	}
	defer p.eventQueue.Done(item)

	event, ok := item.(*eventData)
	if !ok {
		klog.Warningf("Couldn't convert work item to evetData: %v", item)
		return true
	}
	p.processEvent(event)
	return true
}

func (p *podStartupLatencyMeasurement) stop() {
	if p.isRunning {
		p.isRunning = false
		close(p.stopCh)
		p.eventQueue.ShutDown()
	}
}

func (p *podStartupLatencyMeasurement) gather(c clientset.Interface, identifier string) ([]measurement.Summary, error) {
	klog.Infof("%s: gathering pod startup latency measurement...", p)
	if !p.isRunning {
		return nil, fmt.Errorf("metric %s has not been started", podStartupLatencyMeasurementName)
	}

	p.stop()

	if err := p.gatherScheduleTimes(c); err != nil {
		return nil, err
	}

	podStartupLatency := p.podStartupEntries.CalculateTransitionsLatency(map[string]measurementutil.Transition{
		"create_to_schedule": {
			From: createPhase,
			To:   schedulePhase,
		},
		"schedule_to_run": {
			From: schedulePhase,
			To:   runPhase,
		},
		"run_to_watch": {
			From: runPhase,
			To:   watchPhase,
		},
		"schedule_to_watch": {
			From: schedulePhase,
			To:   watchPhase,
		},
		"pod_startup": {
			From:      createPhase,
			To:        watchPhase,
			Threshold: p.threshold,
		},
	})

	var err error
	if slosErr := podStartupLatency["pod_startup"].VerifyThreshold(p.threshold); slosErr != nil {
		err = errors.NewMetricViolationError("pod startup", slosErr.Error())
		klog.Errorf("%s: %v", p, err)
	}

	content, jsonErr := util.PrettyPrintJSON(measurementutil.LatencyMapToPerfData(podStartupLatency))
	if jsonErr != nil {
		return nil, jsonErr
	}
	summary := measurement.CreateSummary(fmt.Sprintf("%s_%s", podStartupLatencyMeasurementName, identifier), "json", content)
	return []measurement.Summary{summary}, err
}

func (p *podStartupLatencyMeasurement) gatherScheduleTimes(c clientset.Interface) error {
	selector := fields.Set{
		"involvedObject.kind": "Pod",
		"source":              corev1.DefaultSchedulerName,
	}.AsSelector().String()
	options := metav1.ListOptions{FieldSelector: selector}
	schedEvents, err := c.CoreV1().EventsWithMultiTenancy(p.selector.Namespace, util.GetTenant()).List(options)
	if err != nil {
		return err
	}
	for _, event := range schedEvents.Items {
		key := createMetaNamespaceKey(event.InvolvedObject.Namespace, event.InvolvedObject.Name)
		if _, exists := p.podStartupEntries.Get(key, createPhase); exists {
			if !event.EventTime.IsZero() {
				p.podStartupEntries.Set(key, schedulePhase, event.EventTime.Time)
			} else {
				p.podStartupEntries.Set(key, schedulePhase, event.FirstTimestamp.Time)
			}
		}
	}
	return nil
}

func (p *podStartupLatencyMeasurement) processEvent(event *eventData) {
	obj, recvTime := event.obj, event.recvTime
	if obj == nil {
		return
	}
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}
	if pod.Status.Phase == corev1.PodRunning {
		key := createMetaNamespaceKey(pod.Namespace, pod.Name)
		if _, found := p.podStartupEntries.Get(key, createPhase); !found {
			ct := recvTime
			p.podStartupEntries.Set(key, watchPhase, ct)
			p.podStartupEntries.Set(key, createPhase, pod.CreationTimestamp.Time)
			var startTime metav1.Time
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.State.Running != nil {
					if startTime.Before(&cs.State.Running.StartedAt) {
						startTime = cs.State.Running.StartedAt
					}
				}
			}
			if startTime != metav1.NewTime(time.Time{}) {
				p.podStartupEntries.Set(key, runPhase, startTime.Time)
			} else {
				klog.Errorf("%s: pod %v (%v) is reported to be running, but none of its containers is", p, pod.Name, pod.Namespace)
			}
		}
	}
}

func createMetaNamespaceKey(namespace, name string) string {
	return util.GetTenant() + "/" + namespace + "/" + name
}
