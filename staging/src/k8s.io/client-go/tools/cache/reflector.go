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

package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"k8s.io/client-go/apiserverupdate"
	"math/rand"
	"net"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/grafov/bcast"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/naming"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/pager"
	"k8s.io/klog"
	"k8s.io/utils/trace"
)

type filterBound struct {
	//ResetCh is the channel for incoming bounds changes
	resetCh *bcast.Member

	//sourceName is the app name that sends the reset message
	sourceName string

	//ownerKind is the owner's object kind
	ownerKind string

	//bounds is an array to save hashkey lower and upper bounds. The lower bound is
	// inclusive while the upper bound is exclusive
	bounds []int64
}

// Reflector watches a specified resource and causes all changes to be reflected in the given store.
type Reflector struct {
	// name identifies this reflector. By default it will be a file:line if possible.
	name string
	// metrics tracks basic metric information about the reflector
	metrics *reflectorMetrics

	// The type of object we expect to place in the store.
	expectedType reflect.Type
	// The destination to sync up with the watch source
	store Store
	// listerWatcher is used to perform lists and watches.
	listerWatcher ListerWatcher
	// period controls timing between one watch ending and
	// the beginning of the next one.
	period       time.Duration
	resyncPeriod time.Duration
	ShouldResync func() bool
	// clock allows tests to manipulate time
	clock clock.Clock
	// lastSyncResourceVersion is the resource version token last
	// observed when doing a sync with the underlying store
	// it is thread safe, but not synchronized with the underlying store
	lastSyncResourceVersion string
	// lastSyncResourceVersionMutex guards read/write access to lastSyncResourceVersion
	lastSyncResourceVersionMutex sync.RWMutex
	// WatchListPageSize is the requested chunk size of initial and resync watch lists.
	// Defaults to pager.PageSize.
	WatchListPageSize int64

	// filterBounds are a list of list/watch filtering bounds
	filterBounds []filterBound

	// aggChan is the aggregate channel of reset channels
	aggChan chan interface{}

	// clientSetUpdateChan is the channel to get client set updates
	clientSetUpdateChan *bcast.Member

	// listFromResourceVersion is the resource version that list should start from
	// Default is "0" so that it only reads from cache. After encounter "too old resource version",
	// it will be set to "" to get all data from storage; and then reset to "0" to get from cache in
	// later list and watch
	listFromResourceVersion string

	// There are some watch that can only happen to certain api servers
	allowPartialWatch bool
}

var (
	// We try to spread the load on apiserver by setting timeouts for
	// watch requests - it is random in [minWatchTimeout, 2*minWatchTimeout].
	minWatchTimeout = 5 * time.Minute
)

// NewNamespaceKeyedIndexerAndReflector creates an Indexer and a Reflector
// The indexer is configured to key on namespace
func NewNamespaceKeyedIndexerAndReflector(lw ListerWatcher, expectedType interface{}, resyncPeriod time.Duration) (indexer Indexer, reflector *Reflector) {
	indexer = NewIndexer(MetaNamespaceKeyFunc, Indexers{NamespaceIndex: MetaNamespaceIndexFunc})
	reflector = NewReflector(lw, expectedType, indexer, resyncPeriod)
	return indexer, reflector
}

// NewReflector creates a new Reflector object which will keep the given store up to
// date with the server's contents for the given resource. Reflector promises to
// only put things in the store that have the type of expectedType, unless expectedType
// is nil. If resyncPeriod is non-zero, then lists will be executed after every
// resyncPeriod, so that you can use reflectors to periodically process everything as
// well as incrementally processing the things that change.
func NewReflector(lw ListerWatcher, expectedType interface{}, store Store, resyncPeriod time.Duration) *Reflector {
	return NewNamedReflector(naming.GetNameFromCallsite(internalPackages...), lw, expectedType, store, resyncPeriod, false)
}

// NewReflectorWithReset creates a new Reflector object which will keep the given store up to
// date with the server's contents for the given resource. Reflector promises to
// only put things in the store that have the type of expectedType, unless expectedType
// is nil. If resyncPeriod is non-zero, then lists will be executed after every
// resyncPeriod, so that you can use reflectors to periodically process everything as
// well as incrementally processing the things that change. Also,  it introduces a  reset
// chan for any incoming bound changes
func NewReflectorWithReset(lw ListerWatcher, expectedType interface{}, store Store, resyncPeriod time.Duration, filterBounds []filterBound) *Reflector {
	r := NewNamedReflector(naming.GetNameFromCallsite(internalPackages...), lw, expectedType, store, resyncPeriod, false)
	r.filterBounds = filterBounds
	return r
}

// NewNamedReflector same as NewReflector, but with a specified name for logging
func NewNamedReflector(name string, lw ListerWatcher, expectedType interface{}, store Store, resyncPeriod time.Duration, allowPartialWatch bool) *Reflector {
	r := &Reflector{
		name:                    name,
		listerWatcher:           lw,
		store:                   store,
		expectedType:            reflect.TypeOf(expectedType),
		period:                  time.Second,
		resyncPeriod:            resyncPeriod,
		clock:                   &clock.RealClock{},
		filterBounds:            make([]filterBound, 0),
		aggChan:                 make(chan interface{}),
		clientSetUpdateChan:     apiserverupdate.WatchClientSetUpdate(),
		listFromResourceVersion: "0",
		allowPartialWatch:       allowPartialWatch,
	}
	return r
}

// internalPackages are packages that ignored when creating a default reflector name. These packages are in the common
// call chains to NewReflector, so they'd be low entropy names for reflectors
var internalPackages = []string{"client-go/tools/cache/"}

// Run starts a watch and handles watch events. Will restart the watch if it is closed.
// Run will exit when stopCh is closed.
func (r *Reflector) Run(stopCh <-chan struct{}) {
	klog.V(3).Infof("Starting reflector %v (%s) from %s", r.expectedType, r.resyncPeriod, r.name)
	wait.Until(func() {
		if len(r.filterBounds) == 0 {
			if err := r.ListAndWatch(stopCh); err != nil {
				utilruntime.HandleError(err)
			}
		} else {
			for i, fb := range r.filterBounds {
				go func(index int, fB filterBound) {
					for {
						select {
						case <-stopCh:
							return
						case signal, ok := <-fB.resetCh.Read:
							if !ok {
								klog.Errorf("reset channel closed. expectedType %v", r.expectedType)
								return
							} else {
								klog.V(4).Infof("Got reset channel message. expectedType %v, new bound [%+v]", r.expectedType, signal)
							}

							r.aggChan <- signal
						}
					}
				}(i, fb)
			}

			for {
				if err := r.ListAndWatch(stopCh); err != nil {
					if err == errorResetFilterBoundRequested || err == errorClientSetResetRequested {
						klog.V(4).Infof("Filter bounds reset message received, redo ListAndWatch with resetCh")
						continue
					}
					utilruntime.HandleError(err)
					break
				}
			}
		}
	}, r.period, stopCh)
}

var (
	// nothing will ever be sent down this channel
	neverExitWatch <-chan time.Time = make(chan time.Time)

	// Used to indicate that watching stopped so that a resync could happen.
	errorResyncRequested = errors.New("resync channel fired")

	// Used to indicate that watching stopped because of a signal from the stop
	// channel passed in from a client of the reflector.
	errorStopRequested = errors.New("Stop requested")

	// Used to indicate that watching stopped because filtering bounds are updated
	errorResetFilterBoundRequested = errors.New("Reset requested")

	// Used to indicate that watching stopped because api server clients are updated
	errorClientSetResetRequested = errors.New("Clientset reset requested")
)

// resyncChan returns a channel which will receive something when a resync is
// required, and a cleanup function.
func (r *Reflector) resyncChan() (<-chan time.Time, func() bool) {
	if r.resyncPeriod == 0 {
		return neverExitWatch, func() bool { return false }
	}
	// The cleanup function is required: imagine the scenario where watches
	// always fail so we end up listing frequently. Then, if we don't
	// manually stop the timer, we could end up with many timers active
	// concurrently.
	t := r.clock.NewTimer(r.resyncPeriod)
	return t.C(), t.Stop
}

// ListAndWatch first lists all items and get the resource version at the moment of call,
// and then use the resource version to watch.
// It returns error if ListAndWatch didn't even try to initialize watch.
func (r *Reflector) ListAndWatch(stopCh <-chan struct{}) error {
	klog.V(3).Infof("ListAndWatch %v. filter bounds %+v. name %s", r.expectedType, r.filterBounds, r.name)
	var resourceVersion string

	// Explicitly set "0" as resource version - it's fine for the List()
	// to be served from cache and potentially be delayed relative to
	// etcd contents. Reflector framework will catch up via Watch() eventually.
	// When ResourceVersion is empty, list will get from api server cache
	options := metav1.ListOptions{ResourceVersion: r.listFromResourceVersion}

	if len(r.filterBounds) > 0 {
		if r.hasInitBounds() {
			if !r.waitForBoundInit(stopCh) {
				return nil
			}
		} else {
			select {
			case <-stopCh:
				return nil
			default:
				klog.V(4).Infof("ListAndWatchWithReset default fall through. bounds %+v", r.filterBounds)
			}
		}

		// Pick up any bound changes
		options = appendFieldSelector(options, r.createHashkeyListOptions())
	}

	// LIST
	if err := func() error {
		initTrace := trace.New("Reflector " + r.name + " ListAndWatch")
		defer initTrace.LogIfLong(10 * time.Second)
		var list runtime.Object
		var err error
		listCh := make(chan struct{}, 1)
		panicCh := make(chan interface{}, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicCh <- r
				}
			}()
			// Attempt to gather list in chunks, if supported by listerWatcher, if not, the first
			// list request will return the full response.
			pager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
				return r.listerWatcher.List(opts)
			}))
			if r.WatchListPageSize != 0 {
				pager.PageSize = r.WatchListPageSize
			}
			// Pager falls back to full list if paginated list calls fail due to an "Expired" error.
			// Set resource version to "" as it cannot be limited to the cached ones due to the introduction of
			// 	api server data partition
			list, err = pager.List(context.Background(), options)
			close(listCh)
		}()

		select {
		case <-stopCh:
			return nil
		case r := <-panicCh:
			panic(r)
		case <-listCh:
		}
		if err != nil {
			return fmt.Errorf("%s: Failed to list %v: %v", r.name, r.expectedType, err)
		}
		initTrace.Step("Objects listed")
		listMetaInterface, err := meta.ListAccessor(list)
		if err != nil {
			return fmt.Errorf("%s: Unable to understand list result %#v: %v", r.name, list, err)
		}
		resourceVersion = listMetaInterface.GetResourceVersion()
		initTrace.Step("Resource version extracted")
		items, err := meta.ExtractList(list)
		if err != nil {
			return fmt.Errorf("%s: Unable to understand list result %#v (%v)", r.name, list, err)
		}
		initTrace.Step("Objects extracted")
		if err := r.syncWith(items, resourceVersion); err != nil {
			return fmt.Errorf("%s: Unable to sync list result: %v", r.name, err)
		}
		initTrace.Step("SyncWith done")
		r.setLastSyncResourceVersion(resourceVersion)
		initTrace.Step("Resource version updated")
		return nil
	}(); err != nil {
		return err
	}

	// RESYNC
	resyncerrc := make(chan error, 1)
	cancelCh := make(chan struct{})
	defer close(cancelCh)
	go func() {
		resyncCh, cleanup := r.resyncChan()
		defer func() {
			cleanup() // Call the last one written into cleanup
		}()
		for {
			select {
			case <-resyncCh:
			case <-stopCh:
				return
			case <-cancelCh:
				return
			}
			if r.ShouldResync == nil || r.ShouldResync() {
				klog.V(4).Infof("%s: forcing resync. type %v. resync period %v", r.name, r.expectedType, r.resyncPeriod)
				if err := r.store.Resync(); err != nil {
					resyncerrc <- err
					return
				}
			}
			cleanup()
			resyncCh, cleanup = r.resyncChan()
		}
	}()

	// WATCH
	for {
		// give the stopCh a chance to stop the loop, even in case of continue statements further down on errors
		select {
		case <-stopCh:
			return nil
		default:
		}

		timeoutSeconds := int64(minWatchTimeout.Seconds() * (rand.Float64() + 1.0))
		options = metav1.ListOptions{
			ResourceVersion: resourceVersion,
			// We want to avoid situations of hanging watchers. Stop any wachers that do not
			// receive any events within the timeout window.
			TimeoutSeconds: &timeoutSeconds,
			// To reduce load on kube-apiserver on watch restarts, you may enable watch bookmarks.
			// Reflector doesn't assume bookmarks are returned at all (if the server do not support
			// watch bookmarks, it will ignore this field).
			// Disabled in Alpha release of watch bookmarks feature.
			AllowWatchBookmarks: false,
			AllowPartialWatch:   r.allowPartialWatch,
		}

		if len(r.filterBounds) > 0 {
			options = appendFieldSelector(options, r.createHashkeyListOptions())
		}
		aggregatedWatcher := r.listerWatcher.Watch(options)
		err := aggregatedWatcher.GetErrors()
		if err != nil {
			switch err {
			case io.EOF:
				// watch closed normally
			case io.ErrUnexpectedEOF:
				klog.V(1).Infof("%s: Watch for %v closed with unexpected EOF: %v", r.name, r.expectedType, err)
			default:
				utilruntime.HandleError(fmt.Errorf("%s: Failed to watch %v: %v", r.name, r.expectedType, err))
			}
			// If this is "connection refused" error, it means that most likely apiserver is not responsive.
			// It doesn't make sense to re-list all objects because most likely we will be able to restart
			// watch where we ended.
			// If that's the case wait and resend watch request.
			if urlError, ok := err.(*url.Error); ok {
				if opError, ok := urlError.Err.(*net.OpError); ok {
					if errno, ok := opError.Err.(syscall.Errno); ok && errno == syscall.ECONNREFUSED {
						time.Sleep(time.Second)
						continue
					}
				}
			}
			return nil
		}

		if err := r.watchHandler(aggregatedWatcher, &resourceVersion, resyncerrc, stopCh); err != nil {
			if err == errorResetFilterBoundRequested || err == errorClientSetResetRequested {
				select {
				case cancelCh <- struct{}{}:
					klog.V(4).Infof("Sent message to Resync cancelCh.")
				default:
					klog.V(4).Infof("Resync cancelCh was closed.")
				}
				return err
			}
			if err != errorStopRequested {
				switch {
				case apierrs.IsResourceExpired(err):
					klog.V(4).Infof("%s: watch of %v ended with: %v", r.name, r.expectedType, err)
				default:
					if strings.Contains(err.Error(), "too old resource version") {
						// Watching stopped because it trying to get older resource
						// version that api server can provide, set listFromResourceVersion
						// to allow temporary list from storage directly and avoid too old
						// resource version in follow up watch
						r.listFromResourceVersion = ""
						err1 := fmt.Errorf("Error [%v] from watch type %v", err.Error(), r.expectedType)
						return err1
					}
					klog.Warningf("%s: watch of %v ended with: %v", r.name, r.expectedType, err)
				}
			}
			return nil
		}
	}
}

// syncWith replaces the store's items with the given list.
func (r *Reflector) syncWith(items []runtime.Object, resourceVersion string) error {
	found := make([]interface{}, 0, len(items))
	for _, item := range items {
		found = append(found, item)
	}
	return r.store.Replace(found, resourceVersion)
}

// watchHandler watches w and keeps *resourceVersion up to date.
func (r *Reflector) watchHandler(w watch.AggregatedWatchInterface, resourceVersion *string, errc chan error, stopCh <-chan struct{}) error {
	start := r.clock.Now()
	eventCount := 0

	// Stopping the watcher should be idempotent and if we return from this function there's no way
	// we're coming back in with the same watch interface.
	defer w.Stop()

loop:
	for {
		if len(r.filterBounds) > 0 {
			select {
			case <-r.clientSetUpdateChan.Read:
				klog.Infof("Got client set update message. Restarting ListAndWatch %v", r.expectedType)
				return errorClientSetResetRequested
			case <-stopCh:
				return errorStopRequested
			case signal, ok := <-r.aggChan:
				if !ok {
					klog.Error("resetCh channel closed")
					return errors.New("Reset channel closed")
				}
				r.setBounds(signal)

				return errorResetFilterBoundRequested
			case err := <-errc:
				return err
			case event, ok := <-w.ResultChan():
				if !ok {
					break loop
				}

				var err error
				err, eventCount = r.watchHandlerHelper(event, resourceVersion, eventCount)
				if err != nil {
					return err
				}
			}
		} else {
			select {
			case <-r.clientSetUpdateChan.Read:
				klog.Infof("Got client set update message. Restarting ListAndWatch %v", r.expectedType)
				return errorClientSetResetRequested
			case <-stopCh:
				return errorStopRequested
			case err := <-errc:
				return err
			case event, ok := <-w.ResultChan():
				if !ok {
					break loop
				}

				var err error
				err, eventCount = r.watchHandlerHelper(event, resourceVersion, eventCount)
				if err != nil {
					return err
				}
			}
		}
	}

	watchDuration := r.clock.Since(start)
	if watchDuration < 1*time.Second && eventCount == 0 {
		return fmt.Errorf("very short watch: %s: Unexpected watch close - watch lasted less than a second and no items received", r.name)
	}
	klog.V(4).Infof("%s: Watch close - %v total %v items received", r.name, r.expectedType, eventCount)
	return nil
}

func (r *Reflector) watchHandlerHelper(event watch.Event, resourceVersion *string, eventCount int) (error, int) {
	if event.Type == watch.Error {
		return apierrs.FromObject(event.Object), eventCount
	}
	if e, a := r.expectedType, reflect.TypeOf(event.Object); e != nil && e != a {
		utilruntime.HandleError(fmt.Errorf("%s: expected type %v, but watch event object had type %v", r.name, e, a))
		return nil, eventCount
	}
	meta, err := meta.Accessor(event.Object)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("%s: unable to understand watch event %#v", r.name, event))
		return nil, eventCount
	}
	newResourceVersion := meta.GetResourceVersion()
	switch event.Type {
	case watch.Added:
		err := r.store.Add(event.Object)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("%s: unable to add watch event object (%#v) to store: %v", r.name, event.Object, err))
		}
	case watch.Modified:
		err := r.store.Update(event.Object)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("%s: unable to update watch event object (%#v) to store: %v", r.name, event.Object, err))
		}
	case watch.Deleted:
		// TODO: Will any consumers need access to the "last known
		// state", which is passed in event.Object? If so, may need
		// to change this.
		err := r.store.Delete(event.Object)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("%s: unable to delete watch event object (%#v) from store: %v", r.name, event.Object, err))
		}
	case watch.Bookmark:
		// A `Bookmark` means watch has synced here, just update the resourceVersion
	default:
		utilruntime.HandleError(fmt.Errorf("%s: unable to understand watch event %#v", r.name, event))
	}
	*resourceVersion = newResourceVersion
	r.setLastSyncResourceVersion(newResourceVersion)
	if r.listFromResourceVersion == "" {
		r.listFromResourceVersion = "0"
	}
	eventCount++
	return nil, eventCount
}

// LastSyncResourceVersion is the resource version observed when last sync with the underlying store
// The value returned is not synchronized with access to the underlying store and is not thread-safe
func (r *Reflector) LastSyncResourceVersion() string {
	r.lastSyncResourceVersionMutex.RLock()
	defer r.lastSyncResourceVersionMutex.RUnlock()
	return r.lastSyncResourceVersion
}

func (r *Reflector) setLastSyncResourceVersion(v string) {
	r.lastSyncResourceVersionMutex.Lock()
	defer r.lastSyncResourceVersionMutex.Unlock()
	r.lastSyncResourceVersion = v
}

func (r *Reflector) ListerWatcher() ListerWatcher {
	return r.listerWatcher
}

func appendFieldSelector(target metav1.ListOptions, fieldSelector metav1.ListOptions) metav1.ListOptions {
	if len(target.FieldSelector) == 0 {
		target.FieldSelector = fieldSelector.FieldSelector
	} else {
		target.FieldSelector = target.FieldSelector + "," + fieldSelector.FieldSelector
	}
	return target
}

func (r *Reflector) setBounds(signal interface{}) {
	newBounds, _ := signal.(FilterBound)
	klog.V(4).Infof("Setbound Begin. Old bounds [%+v]. New bounds [%+v]. expectedType %v", r.filterBounds, newBounds, r.expectedType)
	isBoundReset := false
	for i, filterBound := range r.filterBounds {
		if filterBound.sourceName == newBounds.OwnerName {
			r.filterBounds[i].bounds = []int64{newBounds.LowerBound, newBounds.UpperBound}
			isBoundReset = true
		}
	}
	if !isBoundReset {
		klog.Errorf("Cannot found matching filtering bounds. Old bounds [%+v]. New bounds [%+v]", r.filterBounds, newBounds)
	}
	klog.V(4).Infof("Setbound DONE. New bounds [%+v]. expectedType %v", r.filterBounds, r.expectedType)
}

func (r *Reflector) hasInitBounds() bool {
	for _, filterBound := range r.filterBounds {
		if len(filterBound.bounds) == 0 || filterBound.bounds[0] == -1 && filterBound.bounds[1] == -1 {
			return true
		}
	}
	return false
}

// return false if stopCh send message
func (r *Reflector) waitForBoundInit(stopCh <-chan struct{}) bool {
	for {
		select {
		case msg, ok := <-r.aggChan:
			if !ok {
				klog.Errorf("Cannot read from closed channel. expectedType %v", r.expectedType)
				return false
			}
			r.setBounds(msg)
		case <-stopCh:
			return false
		}

		if !r.hasInitBounds() {
			klog.V(4).Infof("Init bound succeeded. expectedType %v, Bounds: %+v", r.expectedType, r.filterBounds)
			return true
		} else {
			klog.V(4).Infof("Waiting for more init bound. expectedType %v, Bounds: %+v", r.expectedType, r.filterBounds)
		}
	}
}

// currently do not support all range
func (r *Reflector) createHashkeyListOptions() metav1.ListOptions {
	klog.V(3).Infof("createHashkeyListOptions filterBounds [%+v], expectedType %v", r.filterBounds, r.expectedType)
	listOptions := make([]string, 0)
	for _, filterBound := range r.filterBounds {
		if filterBound.bounds[0] == -1 && filterBound.bounds[1] == -1 {
			continue
		}
		operator := "gt"
		if filterBound.bounds[0] == 0 {
			operator = "gte"
		}
		var option string
		if filterBound.ownerKind != "" {
			option = fmt.Sprintf("metadata.ownerReferences.hashkey.%s=%s:%v,metadata.ownerReferences.hashkey.%s=lte:%+v", filterBound.ownerKind, operator, filterBound.bounds[0], filterBound.ownerKind, filterBound.bounds[1])
		} else {
			option = fmt.Sprintf("metadata.hashkey=%s:%v,metadata.hashkey=lte:%+v", operator, filterBound.bounds[0], filterBound.bounds[1])
		}

		listOptions = append(listOptions, option)
	}

	return metav1.ListOptions{
		FieldSelector: strings.Join(listOptions, ";"),
	}
}
