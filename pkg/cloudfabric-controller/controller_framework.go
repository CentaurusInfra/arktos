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
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/util/metrics"
	"math"
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
	workloadNum   int32
	isLocked      bool
}

type ControllerBase struct {
	controller_type        string
	controller_instance_id types.UID
	state                  string

	// use int64 as k8s base deal with int64 better
	controller_key int64

	worker_number          int
	sortedControllers      []controllerInstance
	controllerInstanceList []v1.ControllerInstance

	curPos int

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

	// Get existed controller instances from registry
	controllerInstances, err := readControllerInstances(client.CoreV1(), controller_type)

	if err != nil {
		//TODO
	}
	sortedControllerInstances := SortControllerInstancesByKey(controllerInstances)

	controller := &ControllerBase{
		client:                 client,
		controller_type:        controller_type,
		state:                  ControllerStateInit,
		controller_instance_id: uuid.NewUUID(),
		worker_number:          getDefaultNumberOfWorker(controller_type),
		controllerInstanceList: controllerInstances,
		sortedControllers:      sortedControllerInstances,
		curPos:                 -1,

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	controller.controller_key = controller.generateKey()

	// First controller instance. No need to wait for others
	if len(controller.sortedControllers) == 0 {
		controller.state = ControllerStateActive
	} else {
		controller.state = ControllerStateLocked
	}

	err = controller.registController()
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

	go wait.Until(c.reportHealth, time.Minute, stopCh)

	klog.Infof("All work started for controller %s instance %s", c.controller_type, c.controller_instance_id)

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

	if key > c.controller_key || key <= c.sortedControllers[c.curPos].lowerboundKey {
		return false
	}

	return true
}

func (c *ControllerBase) generateKey() int64 {
	if len(c.sortedControllers) == 0 {
		return math.MaxInt64
	}

	min, max := c.getMaxInterval()
	return (max - min) / 2
}

func (c *ControllerBase) getMaxInterval() (int64, int64) {
	min := int64(0)
	max := int64(math.MaxInt64)

	maxWorkloadNum := (int32)(-1)
	intervalFound := false

	for i := 0; i < len(c.sortedControllers); i++ {
		item := c.sortedControllers[i]

		if item.workloadNum > maxWorkloadNum {
			maxWorkloadNum = item.workloadNum
			max = item.controllerKey
			min = item.lowerboundKey
			intervalFound = true
		}
	}

	if !intervalFound && len(c.sortedControllers) > 0 {
		min = c.sortedControllers[0].lowerboundKey
		max = c.sortedControllers[0].controllerKey
	}

	return min, max
}

func (c *ControllerBase) updateControllers(newControllerInstances []controllerInstance) {
	// Compare
	isUpdated, isSelfUpdated, newLowerBound, newUpperbound, newPos := c.tryConsolidateControllerInstances(newControllerInstances)
	if !isUpdated && !isSelfUpdated {
		return
	}

	if isUpdated {
		c.sortedControllers = newControllerInstances
	}

	if isSelfUpdated {
		c.state = ControllerStateWait
		c.curPos = newPos
	}

	if c.state == ControllerStateWait {
		// TODO - wait for current processing workloads being done
	}

	if isSelfUpdated {
		// TODO - reset filter
		klog.Infof("New lowerbound = %v, new upperbound = %v", newLowerBound, newUpperbound)
	}

	if c.state == ControllerStateLocked {
		// TODO - wait for unlock
	}

	c.state = ControllerStateActive
	return
}

// Assume both old & new controller instances are sorted by controller key
func (c *ControllerBase) tryConsolidateControllerInstances(newControllerInstances []controllerInstance) (isUpdated bool, isSelfUpdated bool, newLowerbound int64, newUpperbound int64, newPos int) {
	oldControllerInstances := c.sortedControllers

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

	if c.curPos == -1 {
		c.curPos = newPos
		return true, false, newControllerInstances[newPos].lowerboundKey, c.controller_key, newPos
	}

	if c.sortedControllers[c.curPos].lowerboundKey != newControllerInstances[newPos].lowerboundKey {
		return true, true, newControllerInstances[newPos].lowerboundKey, c.controller_key, newPos
	}

	if !isUpdated {
		for i := 0; i < len(newControllerInstances); i++ {
			if c.sortedControllers[i].lowerboundKey != newControllerInstances[i].lowerboundKey ||
				c.sortedControllers[i].workloadNum != newControllerInstances[i].workloadNum ||
				c.sortedControllers[i].controllerKey != newControllerInstances[i].controllerKey ||
				c.sortedControllers[i].instanceId != newControllerInstances[i].instanceId {
				isUpdated = true
			}
		}
	}

	return isUpdated, false, 0, 0, newPos
}

// register current controller instance in registry
func (c *ControllerBase) registController() error {
	controllerInstanceInStoreage := v1.ControllerInstance{
		ControllerType: c.controller_type,
		UID:            c.controller_instance_id,
		HashKey:        c.controller_key,
		WorkloadNum:    0,
		IsLocked:       c.state == ControllerStateLocked,
		ObjectMeta: metav1.ObjectMeta{
			Name: string(c.controller_instance_id),
		},
	}

	isExist := isControllerInstanceExisted(c.controllerInstanceList, c.controller_instance_id)
	if isExist {
		// Error
		klog.Errorf("Trying to register new %s controller instance with id %v already existed in controller instance list", c.controller_type, c.controller_instance_id)
		return errors.NewAlreadyExists(apps.Resource("controllerinstances"), "UID")
	} else {
		/*
			if c.controllerInstanceList.Name == "" { // for unit test that mocked HTTP request
				c.controllerInstanceList.Name = c.controller_type
			}*/
		c.controllerInstanceList = append(c.controllerInstanceList, controllerInstanceInStoreage)

		// Write to registry
		_, err := c.client.CoreV1().ControllerInstances().Create(&controllerInstanceInStoreage)
		if err != nil {
			klog.Errorf("Error register controller %s instance %s, error %v", c.controller_type, c.controller_instance_id, err)
			// TODO
			return err
		}

		// Check controllers updates
		newSortedControllerInstances := SortControllerInstancesByKey(c.controllerInstanceList)
		c.updateControllers(newSortedControllerInstances)
	}

	return nil
}

// Periodically update controller instance in registry for two things:
//     1. Update workload # so that workload can be more evenly distributed
//     2. Renew TTL for current controller instance
func (c *ControllerBase) reportHealth() {
	klog.Infof("Controller %s instance %s report health", c.controller_type, c.controller_instance_id)
	controllerInstanceInStoreage := v1.ControllerInstance{
		ControllerType: c.controller_type,
		UID:            c.controller_instance_id,
		HashKey:        c.controller_key,
		WorkloadNum:    c.sortedControllers[c.curPos].workloadNum,
		ObjectMeta: metav1.ObjectMeta{
			Name: string(c.controller_instance_id),
		},
	}

	// Write to registry
	_, err := c.client.CoreV1().ControllerInstances().Update(&controllerInstanceInStoreage)
	if err != nil {
		klog.Errorf("Error update controller %s instance %s, error %v", c.controller_type, c.controller_instance_id, err)
		//TODO
	}
}

// get default # of workers from storage - TODO
func getDefaultNumberOfWorker(controllerType string) int {
	return 5
}

// Get controller instances by controller type from registry
//		Return sorted controller instance list & error if any
func readControllerInstances(c v1core.ControllerInstancesGetter, controllerType string) ([]v1.ControllerInstance, error) {
	var filterControllerInstances []v1.ControllerInstance

	controllerInstanceList, err := c.ControllerInstances().List(metav1.ListOptions{})
	if err == nil {
		for _, controllerInstance := range controllerInstanceList.Items {
			if controllerInstance.ControllerType == controllerType {
				filterControllerInstances = append(filterControllerInstances, controllerInstance)
			}
		}

		// the controller type already exists
		return filterControllerInstances, nil
	} else if !errors.IsNotFound(err) {
		klog.Errorf("Error getting controller instance map [%v]", err)
		// unexpected - error
		return nil, err
	}

	return filterControllerInstances, err
}
