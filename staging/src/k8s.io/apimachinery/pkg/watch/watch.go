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

package watch

import (
	"context"
	"fmt"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
)

// Interface can be implemented by anything that knows how to watch and report changes.
type Interface interface {
	// Stops watching. Will close the channel returned by ResultChan(). Releases
	// any resources used by the watch.
	Stop()

	// Returns a chan which will receive all the events. If an error occurs
	// or Stop() is called, this channel will be closed, in which case the
	// watch should be completely cleaned up.
	ResultChan() <-chan Event
}

type AggregatedWatchInterface interface {
	Stop()
	ResultChan() <-chan Event

	AddWatchInterface(Interface, error)
	GetErrors() error
	GetWatchersCount() int
}

type AggregatedWatcher struct {
	mapLock      sync.RWMutex
	watchers     map[int]Interface
	errs         map[int]error
	stopChs      map[int]chan int
	watcherIndex int

	aggChan chan Event
	wg      sync.WaitGroup

	stopped  bool
	stopLock sync.RWMutex

	ctx               context.Context
	cancel            context.CancelFunc
	allowWatcherReset bool
}

func NewAggregatedWatcher() *AggregatedWatcher {
	a := &AggregatedWatcher{
		watchers:          make(map[int]Interface),
		errs:              make(map[int]error),
		stopChs:           make(map[int]chan int),
		watcherIndex:      0,
		aggChan:           make(chan Event),
		stopped:           false,
		allowWatcherReset: false,
	}

	return a
}

func NewAggregatedWatcherWithReset(ctx context.Context) *AggregatedWatcher {
	a := NewAggregatedWatcher()
	a.allowWatcherReset = true
	a.ctx, a.cancel = context.WithCancel(ctx)
	go func(aw *AggregatedWatcher) {
		for {
			select {
			case <-aw.ctx.Done():
				aw.Stop()
				return
			}
		}
	}(a)

	return a
}

func NewAggregatedWatcherWithOneWatch(watcher Interface, err error) *AggregatedWatcher {
	a := NewAggregatedWatcher()
	a.AddWatchInterface(watcher, err)
	return a
}

func (a *AggregatedWatcher) AddWatchInterface(watcher Interface, err error) {
	stopCh := make(chan int)

	a.mapLock.Lock()
	a.watchers[a.watcherIndex] = watcher
	a.errs[a.watcherIndex] = err
	a.stopChs[a.watcherIndex] = stopCh
	a.watcherIndex++
	a.mapLock.Unlock()
	//klog.Infof("Added watch channel %v into aggregated chan %#v.", watcher, a.aggChan)

	if watcher != nil {
		a.wg.Add(1)

		go func(w Interface, a *AggregatedWatcher, stop chan int) {
			for {
				select {
				case <-stop:
					a.closeWatcher(w)
					return
				case signal, ok := <-w.ResultChan():
					if !ok {
						//klog.Infof("watch channel %v closed for aggregated chan %#v.", w, a.aggChan)
						a.closeWatcher(w)
						return
					} else {
						//klog.V(3).Infof("Get event (chan %#v) %s.", a.aggChan, PrintEvent(signal))
					}

					select {
					case <-stop:
						a.closeWatcher(w)
						return
					case a.aggChan <- signal:
						//klog.V(3).Infof("Sent event (chan %#v) %s.", a.aggChan, PrintEvent(signal))
					}
				}
			}
		}(watcher, a, stopCh)
	}
}

func (a *AggregatedWatcher) closeWatcher(watcher Interface) {
	a.mapLock.Lock()
	watcher.Stop()
	if watcher != nil {
		a.wg.Done()
	}

	for k, v := range a.watchers {
		if v == watcher {
			delete(a.watchers, k)
			delete(a.errs, k)
			delete(a.stopChs, k)
			break
		}
	}

	if !a.allowWatcherReset && len(a.watchers) == 0 {
		a.Stop()
	}
	a.mapLock.Unlock()
}

func PrintEvent(signal Event) string {
	message := ""
	message += fmt.Sprintf("Type: %v;", signal.Type)
	message += fmt.Sprintf("Object [%#v]", signal.Object)

	return message
}

func (a *AggregatedWatcher) Stop() {
	a.stopLock.Lock()
	if a.stopped {
		a.stopLock.Unlock()
		return
	}

	if !a.stopped {
		a.allowWatcherReset = false
		go func() {
			for i := len(a.watchers) - 1; i >= 0; i-- {
				a.stopChs[i] <- 1
			}
		}()
		a.wg.Wait()
		a.stopped = true
		close(a.aggChan)
	}
	a.stopLock.Unlock()
}

func (a *AggregatedWatcher) ResultChan() <-chan Event {
	return a.aggChan
}

func (a *AggregatedWatcher) GetErrors() error {
	a.mapLock.RLock()
	aggErr := make([]error, 0, len(a.errs))
	for _, err := range a.errs {
		if err != nil {
			aggErr = append(aggErr, err)
		}
	}

	a.mapLock.RUnlock()
	return utilerrors.NewAggregate(aggErr)
}

func (a *AggregatedWatcher) GetWatchersCount() int {
	a.mapLock.RLock()
	count := len(a.watchers)
	a.mapLock.RUnlock()
	return count
}

// EventType defines the possible types of events.
type EventType string

const (
	Added    EventType = "ADDED"
	Modified EventType = "MODIFIED"
	Deleted  EventType = "DELETED"
	Bookmark EventType = "BOOKMARK"
	Error    EventType = "ERROR"

	DefaultChanSize int32 = 100
)

// Event represents a single event to a watched resource.
// +k8s:deepcopy-gen=true
type Event struct {
	Type EventType

	// Object is:
	//  * If Type is Added or Modified: the new state of the object.
	//  * If Type is Deleted: the state of the object immediately before deletion.
	//  * If Type is Bookmark: the object (instance of a type being watched) where
	//    only ResourceVersion field is set. On successful restart of watch from a
	//    bookmark resourceVersion, client is guaranteed to not get repeat event
	//    nor miss any events.
	//  * If Type is Error: *api.Status is recommended; other types may make sense
	//    depending on context.
	Object runtime.Object
}

type emptyWatch chan Event

// NewEmptyWatch returns a watch interface that returns no results and is closed.
// May be used in certain error conditions where no information is available but
// an error is not warranted.
func NewEmptyWatch() Interface {
	ch := make(chan Event)
	close(ch)
	return emptyWatch(ch)
}

// Stop implements Interface
func (w emptyWatch) Stop() {
}

// ResultChan implements Interface
func (w emptyWatch) ResultChan() <-chan Event {
	return chan Event(w)
}

// FakeWatcher lets you test anything that consumes a watch.Interface; threadsafe.
type FakeWatcher struct {
	result  chan Event
	Stopped bool
	sync.Mutex
}

func NewFake() *FakeWatcher {
	return &FakeWatcher{
		result: make(chan Event),
	}
}

func NewFakeWithChanSize(size int, blocking bool) *FakeWatcher {
	return &FakeWatcher{
		result: make(chan Event, size),
	}
}

// Stop implements Interface.Stop().
func (f *FakeWatcher) Stop() {
	f.Lock()
	defer f.Unlock()
	if !f.Stopped {
		klog.V(4).Infof("Stopping fake watcher.")
		close(f.result)
		f.Stopped = true
	}
}

func (f *FakeWatcher) IsStopped() bool {
	f.Lock()
	defer f.Unlock()
	return f.Stopped
}

// Reset prepares the watcher to be reused.
func (f *FakeWatcher) Reset() {
	f.Lock()
	defer f.Unlock()
	f.Stopped = false
	f.result = make(chan Event)
}

func (f *FakeWatcher) ResultChan() <-chan Event {
	return f.result
}

// Add sends an add event.
func (f *FakeWatcher) Add(obj runtime.Object) {
	f.result <- Event{Added, obj}
}

// Modify sends a modify event.
func (f *FakeWatcher) Modify(obj runtime.Object) {
	f.result <- Event{Modified, obj}
}

// Delete sends a delete event.
func (f *FakeWatcher) Delete(lastValue runtime.Object) {
	f.result <- Event{Deleted, lastValue}
}

// Error sends an Error event.
func (f *FakeWatcher) Error(errValue runtime.Object) {
	f.result <- Event{Error, errValue}
}

// Action sends an event of the requested type, for table-based testing.
func (f *FakeWatcher) Action(action EventType, obj runtime.Object) {
	f.result <- Event{action, obj}
}

// RaceFreeFakeWatcher lets you test anything that consumes a watch.Interface; threadsafe.
type RaceFreeFakeWatcher struct {
	result  chan Event
	Stopped bool
	sync.Mutex
}

func NewRaceFreeFake() *RaceFreeFakeWatcher {
	return &RaceFreeFakeWatcher{
		result: make(chan Event, DefaultChanSize),
	}
}

// Stop implements Interface.Stop().
func (f *RaceFreeFakeWatcher) Stop() {
	f.Lock()
	defer f.Unlock()
	if !f.Stopped {
		klog.V(4).Infof("Stopping fake watcher.")
		close(f.result)
		f.Stopped = true
	}
}

func (f *RaceFreeFakeWatcher) IsStopped() bool {
	f.Lock()
	defer f.Unlock()
	return f.Stopped
}

// Reset prepares the watcher to be reused.
func (f *RaceFreeFakeWatcher) Reset() {
	f.Lock()
	defer f.Unlock()
	f.Stopped = false
	f.result = make(chan Event, DefaultChanSize)
}

func (f *RaceFreeFakeWatcher) ResultChan() <-chan Event {
	f.Lock()
	defer f.Unlock()
	return f.result
}

// Add sends an add event.
func (f *RaceFreeFakeWatcher) Add(obj runtime.Object) {
	f.Lock()
	defer f.Unlock()
	if !f.Stopped {
		select {
		case f.result <- Event{Added, obj}:
			return
		default:
			panic(fmt.Errorf("channel full"))
		}
	}
}

// Modify sends a modify event.
func (f *RaceFreeFakeWatcher) Modify(obj runtime.Object) {
	f.Lock()
	defer f.Unlock()
	if !f.Stopped {
		select {
		case f.result <- Event{Modified, obj}:
			return
		default:
			panic(fmt.Errorf("channel full"))
		}
	}
}

// Delete sends a delete event.
func (f *RaceFreeFakeWatcher) Delete(lastValue runtime.Object) {
	f.Lock()
	defer f.Unlock()
	if !f.Stopped {
		select {
		case f.result <- Event{Deleted, lastValue}:
			return
		default:
			panic(fmt.Errorf("channel full"))
		}
	}
}

// Error sends an Error event.
func (f *RaceFreeFakeWatcher) Error(errValue runtime.Object) {
	f.Lock()
	defer f.Unlock()
	if !f.Stopped {
		select {
		case f.result <- Event{Error, errValue}:
			return
		default:
			panic(fmt.Errorf("channel full"))
		}
	}
}

// Action sends an event of the requested type, for table-based testing.
func (f *RaceFreeFakeWatcher) Action(action EventType, obj runtime.Object) {
	f.Lock()
	defer f.Unlock()
	if !f.Stopped {
		select {
		case f.result <- Event{action, obj}:
			return
		default:
			panic(fmt.Errorf("channel full"))
		}
	}
}

// ProxyWatcher lets you wrap your channel in watch Interface. Threadsafe.
type ProxyWatcher struct {
	result chan Event
	stopCh chan struct{}

	mutex   sync.Mutex
	stopped bool
}

var _ Interface = &ProxyWatcher{}

// NewProxyWatcher creates new ProxyWatcher by wrapping a channel
func NewProxyWatcher(ch chan Event) *ProxyWatcher {
	return &ProxyWatcher{
		result:  ch,
		stopCh:  make(chan struct{}),
		stopped: false,
	}
}

// Stop implements Interface
func (pw *ProxyWatcher) Stop() {
	pw.mutex.Lock()
	defer pw.mutex.Unlock()
	if !pw.stopped {
		pw.stopped = true
		close(pw.stopCh)
	}
}

// Stopping returns true if Stop() has been called
func (pw *ProxyWatcher) Stopping() bool {
	pw.mutex.Lock()
	defer pw.mutex.Unlock()
	return pw.stopped
}

// ResultChan implements Interface
func (pw *ProxyWatcher) ResultChan() <-chan Event {
	return pw.result
}

// StopChan returns stop channel
func (pw *ProxyWatcher) StopChan() <-chan struct{} {
	return pw.stopCh
}
