/*
Copyright 2019 The Kubernetes Authors.

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

package controller

import (
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/metrics"
	"math"
	"sort"
	"time"
)

const (
	ControllerStateInit   string = "init"
	ControllerStateLocked string = "locked"
	ControllerStateWait   string = "wait"
	ControllerStateActive string = "active"
	ControllerStateError  string = "error"
)

type controllerInstance struct {
	instanceId    types.UID
	controllerKey int64
	lowerboundKey int64
	workloadNum   int
	isLocked      bool
}

type ControllerBase struct {
	controller_type        string
	controller_instance_id types.UID
	state                  string

	// use int64 as k8s base deal with int64 better
	controller_key int64

	worker_number int
	controllers   []controllerInstance
	curPos        int

	queue workqueue.RateLimitingInterface

	SyncHandler func(key string) error
	HandleErr   func(err error, key string)

	client clientset.Interface
}

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

func NewControllerBase(controller_type string, client clientset.Interface) (*ControllerBase, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

	if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := metrics.RegisterMetricAndTrackRateLimiterUsage(controller_type+"_controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	controller := &ControllerBase{
		client:                 client,
		controller_type:        controller_type,
		state:                  ControllerStateInit,
		controller_instance_id: uuid.NewUUID(),
		worker_number:          getDefaultNumberOfWorker(controller_type),
		controllers:            readControllers(controller_type),
		curPos:                 -1,

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	controller.controller_key = controller.generateKey()

	// First controller instance. No need to wait for others
	if len(controller.controllers) == 0 {
		controller.state = ControllerStateActive
	} else {
		controller.state = ControllerStateLocked
	}

	err := registController(controller)
	if err != nil {
		klog.Fatalf("Controller %s cannot be registed.", controller_type)
	}

	return controller, err
}

func (c *ControllerBase) GetControllerId() types.UID {
	return c.controller_instance_id
}

func (c *ControllerBase) Worker() {
	for c.ProcessNextWorkItem() {
		klog.Infof("processing next work item ...")
		fmt.Println("processing next work item ......")
	}
}

func (c *ControllerBase) ProcessNextWorkItem() bool {
	if !c.IsControllerActive() {
		fmt.Println("Controller is not active, worker idle ....")
		// TODO : compare key version and controller locked status version
		time.Sleep(1 * time.Second)
		return true
	}

	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	workloadKey := key.(string)

	err := c.SyncHandler(workloadKey)
	if err == nil {
		c.queue.Forget(key)
		return true
	}
	c.HandleErr(err, workloadKey)
	c.queue.AddRateLimited(key)

	return true
}

func (c *ControllerBase) GetQueue() workqueue.RateLimitingInterface {
	return c.queue
}

func (c *ControllerBase) SetQueue(queue workqueue.RateLimitingInterface) {
	c.queue = queue
}

func (c *ControllerBase) GetClient() clientset.Interface {
	return c.client
}

func (c *ControllerBase) Run(stopCh <-chan struct{}) {
	defer c.queue.ShutDown()

	klog.Infof("Starting %s controller", c.controller_type)
	defer klog.Infof("Shutting down %s controller", c.controller_type)

	for i := 0; i < c.worker_number; i++ {
		go wait.Until(c.Worker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *ControllerBase) GetControllerType() string {
	return c.controller_type
}

func (c *ControllerBase) IsControllerActive() bool {
	return c.state == ControllerStateActive
}

func (c *ControllerBase) IsInRange(key int64) bool {
	if key < 0 {
		return false
	}

	if key > c.controller_key || key <= c.controllers[c.curPos].lowerboundKey {
		return false
	}

	return true
}

func (c *ControllerBase) generateKey() int64 {
	if len(c.controllers) == 0 {
		return math.MaxInt64
	}

	min, max := c.getMaxInterval()
	return (max - min) / 2
}

func (c *ControllerBase) getMaxInterval() (int64, int64) {
	min := int64(0)
	max := int64(math.MaxInt64)

	maxWorkloadNum := -1

	for i := 0; i < len(c.controllers); i++ {
		item := c.controllers[i]

		if item.workloadNum > maxWorkloadNum {
			maxWorkloadNum = item.workloadNum
			max = item.controllerKey
			min = item.lowerboundKey
		}
	}

	return min, max
}

func (c *ControllerBase) getControllers() []controllerInstance {
	if c.controllers == nil {
		return readControllers(c.controller_type)
	}

	return c.controllers
}

func (c *ControllerBase) updateControllers(newControllerInstances []controllerInstance) {
	// Compare
	isUpdated, isSelfUpdated, newLowerBound, newUpperbound, newPos := c.tryConsolidateControllerInstances(newControllerInstances)
	if !isUpdated && !isSelfUpdated {
		return
	}

	if isSelfUpdated {
		c.state = ControllerStateWait
		c.curPos = newPos
	}
	if isUpdated {
		c.controllers = newControllerInstances
	}

	if c.state == ControllerStateWait {
		// TODO - wait for unlock or expire
	}

	if isSelfUpdated {
		// TODO - reset filter
		klog.Infof("New lowerbound = %v, new upperbound = %v", newLowerBound, newUpperbound)
	}

	c.state = ControllerStateActive
	return
}

// Assume both old & new controller instances are sorted by controller key
func (c *ControllerBase) tryConsolidateControllerInstances(newControllerInstances []controllerInstance) (isUpdated bool, isSelfUpdated bool, newLowerbound int64, newUpperbound int64, newPos int) {
	oldControllerInstances := c.controllers

	isUpdated = false
	if len(oldControllerInstances) != len(newControllerInstances) {
		isUpdated = true
	}

	// find position in new controller instances - assume current controller is in new controller instance list (deal with edge cases later)
	newPos = -1
	for i := 0; i < len(newControllerInstances); i++ {
		if newControllerInstances[i].instanceId == c.controller_instance_id {
			newPos = i
			break
		}
	}

	// current instance not in new controller instance map, this controller instance lost connection with registry, pause processing
	if newPos == -1 {
		c.state = ControllerStateError
		klog.Errorf("Current instance not in registry. Controller type %s, instance id %v, key %v", c.controller_type, c.controller_instance_id, c.controller_key)
		return true, false, 0, 0, 0
	} else if newPos == len(newControllerInstances)-1 && c.controller_key != math.MaxInt64 { // next to last become last
		c.controller_key = math.MaxInt64
		isSelfUpdated = true
		isUpdated = true
	}

	if c.curPos == -1 || c.controllers[c.curPos].lowerboundKey != newControllerInstances[newPos].lowerboundKey {
		return true, true, newControllerInstances[newPos].lowerboundKey, c.controller_key, newPos
	}

	if !isUpdated {
		for i := 0; i < len(newControllerInstances); i++ {
			if c.controllers[i].lowerboundKey != newControllerInstances[i].lowerboundKey ||
				c.controllers[i].workloadNum != newControllerInstances[i].workloadNum ||
				c.controllers[i].controllerKey != newControllerInstances[i].controllerKey ||
				c.controllers[i].instanceId != newControllerInstances[i].instanceId {
				isUpdated = true
			}
		}
	}

	return isUpdated, false, 0, 0, newPos
}

// get default # of workers from storage - TODO
func getDefaultNumberOfWorker(controllerType string) int {
	return 5
}

func readControllers(controllerType string) []controllerInstance {
	// fake controller instances - TODO: move to unit test, add informer watch here
	rawControllerInstances := fakeListControllerInstances(controllerType)
	newControllerInstances := sortControllerInstancesByKey(rawControllerInstances)

	return newControllerInstances
}

// TODO - registry current controller in registry
func registController(c *ControllerBase) error {
	currentControllerInstance := &controllerInstance{
		controllerKey: c.controller_key,
		isLocked:      c.state == ControllerStateLocked,
		instanceId:    c.controller_instance_id,
		workloadNum:   -1,
	}

	// TODO - regist in storage
	// mock
	newControllerInstances := fakeListControllerInstances(c.controller_type)
	newControllerInstances = append(newControllerInstances, *currentControllerInstance)
	newControllerInstances = sortControllerInstancesByKey(newControllerInstances)
	c.updateControllers(newControllerInstances)

	return nil
}

// Sort Controller Instances by controller key
func sortControllerInstancesByKey(rawControllerInstances []controllerInstance) []controllerInstance {
	// copy map
	var sortedControllerInstances []controllerInstance
	for _, instance := range rawControllerInstances {
		sortedControllerInstances = append(sortedControllerInstances, instance)
	}

	sort.Slice(sortedControllerInstances, func(i, j int) bool {
		return sortedControllerInstances[i].controllerKey < sortedControllerInstances[j].controllerKey
	})

	if len(sortedControllerInstances) > 0 {
		sortedControllerInstances[0].lowerboundKey = 0
	}

	for i := 1; i < len(sortedControllerInstances); i++ {
		sortedControllerInstances[i].lowerboundKey = sortedControllerInstances[i-1].controllerKey
	}

	return sortedControllerInstances
}

func fakeListControllerInstances(controllerType string) []controllerInstance {
	var controllerInstances []controllerInstance

	/*
		instance0 := controllerInstance{
			controllerKey: math.MaxInt32,
			isLocked:      false,
			instanceId:    uuid.NewUUID(),
			workloadNum:   10000,
		}
		controllerInstances = append(controllerInstances, instance0)

		instance1 := controllerInstance{
			controllerKey: math.MaxInt32 / 2,
			isLocked:      false,
			instanceId:    uuid.NewUUID(),
			workloadNum:   5000,
		}
		controllerInstances = append(controllerInstances, instance1)
	*/

	return controllerInstances
}
