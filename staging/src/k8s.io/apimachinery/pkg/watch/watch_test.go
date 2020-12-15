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
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type testType string

func (obj testType) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (obj testType) DeepCopyObject() runtime.Object   { return obj }

func TestFake(t *testing.T) {
	f := NewFake()

	table := []struct {
		t EventType
		s testType
	}{
		{Added, testType("foo")},
		{Modified, testType("qux")},
		{Modified, testType("bar")},
		{Deleted, testType("bar")},
		{Error, testType("error: blah")},
	}

	// Prove that f implements Interface by phrasing this as a function.
	consumer := func(w Interface) {
		for _, expect := range table {
			got, ok := <-w.ResultChan()
			if !ok {
				t.Fatalf("closed early")
			}
			if e, a := expect.t, got.Type; e != a {
				t.Fatalf("Expected %v, got %v", e, a)
			}
			if a, ok := got.Object.(testType); !ok || a != expect.s {
				t.Fatalf("Expected %v, got %v", expect.s, a)
			}
		}
		_, stillOpen := <-w.ResultChan()
		if stillOpen {
			t.Fatal("Never stopped")
		}
	}

	sender := func() {
		f.Add(testType("foo"))
		f.Action(Modified, testType("qux"))
		f.Modify(testType("bar"))
		f.Delete(testType("bar"))
		f.Error(testType("error: blah"))
		f.Stop()
	}

	go sender()
	consumer(f)
}

func TestRaceFreeFake(t *testing.T) {
	f := NewRaceFreeFake()

	table := []struct {
		t EventType
		s testType
	}{
		{Added, testType("foo")},
		{Modified, testType("qux")},
		{Modified, testType("bar")},
		{Deleted, testType("bar")},
		{Error, testType("error: blah")},
	}

	// Prove that f implements Interface by phrasing this as a function.
	consumer := func(w Interface) {
		for _, expect := range table {
			got, ok := <-w.ResultChan()
			if !ok {
				t.Fatalf("closed early")
			}
			if e, a := expect.t, got.Type; e != a {
				t.Fatalf("Expected %v, got %v", e, a)
			}
			if a, ok := got.Object.(testType); !ok || a != expect.s {
				t.Fatalf("Expected %v, got %v", expect.s, a)
			}
		}
		_, stillOpen := <-w.ResultChan()
		if stillOpen {
			t.Fatal("Never stopped")
		}
	}

	sender := func() {
		f.Add(testType("foo"))
		f.Action(Modified, testType("qux"))
		f.Modify(testType("bar"))
		f.Delete(testType("bar"))
		f.Error(testType("error: blah"))
		f.Stop()
	}

	go sender()
	consumer(f)
}

func TestEmpty(t *testing.T) {
	w := NewEmptyWatch()
	_, ok := <-w.ResultChan()
	if ok {
		t.Errorf("unexpected result channel result")
	}
	w.Stop()
	_, ok = <-w.ResultChan()
	if ok {
		t.Errorf("unexpected result channel result")
	}
}

func TestProxyWatcher(t *testing.T) {
	events := []Event{
		{Added, testType("foo")},
		{Modified, testType("qux")},
		{Modified, testType("bar")},
		{Deleted, testType("bar")},
		{Error, testType("error: blah")},
	}

	ch := make(chan Event, len(events))
	w := NewProxyWatcher(ch)

	for _, e := range events {
		ch <- e
	}

	for _, e := range events {
		g := <-w.ResultChan()
		if !reflect.DeepEqual(e, g) {
			t.Errorf("Expected %#v, got %#v", e, g)
			continue
		}
	}

	w.Stop()

	select {
	// Closed channel always reads immediately
	case <-w.StopChan():
	default:
		t.Error("Channel isn't closed")
	}

	// Test double close
	w.Stop()
}

func TestGetErrorsConcurrency(t *testing.T) {
	aw := NewAggregatedWatcher()

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(aggWatcher *AggregatedWatcher, i int) {
			err := errors.New("Test error")
			aggWatcher.AddWatchInterface(nil, err)
			errs := aggWatcher.GetErrors()
			aggWatcher.GetWatchersCount()
			assert.True(t, errs.Error() != "")
			wg.Done()
		}(aw, i)
	}

	wg.Wait()
	errs := aw.GetErrors()
	aw.Stop()
	assert.True(t, errs.Error() != "")
	assert.Equal(t, 1000, aw.GetWatchersCount())
}

func TestNewAggregatedWatcherWithReset(t *testing.T) {
	ctx := context.TODO()
	aw := NewAggregatedWatcherWithReset(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(aggWatcher *AggregatedWatcher, i int) {
			w := NewFake()
			aggWatcher.AddWatchInterface(w, nil)
			w.Stop()
			time.Sleep(3 * time.Millisecond)
			wg.Done()
		}(aw, i)
	}

	wg.Wait()
	assert.True(t, aw.allowWatcherReset)
	assert.Nil(t, aw.GetErrors())
	assert.False(t, aw.stopped)
	aw.Stop()
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 0, aw.GetWatchersCount())
	assert.True(t, aw.stopped)
}

func TestProxyWatcherInAggegatedWatch(t *testing.T) {
	events := []Event{
		{Added, testType("foo")},
		{Modified, testType("qux")},
		{Modified, testType("bar")},
		{Deleted, testType("bar")},
		{Error, testType("error: blah")},
	}

	ch := make(chan Event, len(events))
	w := NewProxyWatcher(ch)
	aw := NewAggregatedWatcher()
	aw.AddWatchInterface(w, nil)

	var wg sync.WaitGroup
	wg.Add(len(events))
	go func() {
		for _, e := range events {
			ch <- e
		}
	}()

	go func() {
		for _, e := range events {
			g := <-aw.ResultChan()
			if !reflect.DeepEqual(e, g) {
				t.Errorf("Expected %#v, got %#v", e, g)
				continue
			}
			wg.Done()
		}
	}()

	wg.Wait()
	aw.Stop()
	time.Sleep(time.Millisecond)
	assert.False(t, aw.allowWatcherReset)
	assert.Equal(t, 0, len(aw.watchers))
	assert.True(t, aw.stopped)
	assert.True(t, w.stopped)

	// Test double close
	aw.Stop()
}

// This is for the aggregated watch panic on timeout issue
// 3 watch channels - two send events consistently, one send error after some events
func TestAggregatedWatchOnError(t *testing.T) {
	failureTimes := 0
	for m := 0; m < 100; m++ {
		agg := NewAggregatedWatcher()

		ch1 := make(chan Event)
		ch2 := make(chan Event)
		ch3 := make(chan Event)
		w1 := NewProxyWatcher(ch1)
		w2 := NewProxyWatcher(ch2)
		w3 := NewProxyWatcher(ch3)

		agg.AddWatchInterface(w1, nil)
		agg.AddWatchInterface(w2, nil)
		agg.AddWatchInterface(w3, nil)

		var received bool
		received = false
		go func(rev *bool) {
			for range agg.ResultChan() {
				*rev = true
			}
		}(&received)

		var wg sync.WaitGroup
		var ch1Sent int
		var ch2Sent int
		var ch3Sent int
		ch1Sent = 0
		ch2Sent = 0
		ch3Sent = 0
		go func(sent *int) {
			for i := 0; i < 1000; i++ {
				ch1 <- Event{Added, testType("foo")}
				*sent = *sent + 1
			}
		}(&ch1Sent)

		go func(sent *int) {
			for i := 0; i < 1000; i++ {
				ch2 <- Event{Modified, testType("bar")}
				*sent = *sent + 1
			}
		}(&ch2Sent)

		wg.Add(1)
		go func(sent *int) {
			for i := 0; i < 10; i++ {
				ch3 <- Event{Deleted, testType("qux")}
				*sent = *sent + 1
			}
			wg.Done()
		}(&ch3Sent)

		wg.Wait()
		agg.Stop()
		time.Sleep(time.Millisecond)
		t.Logf("Channel 1 scheduled to sent 1000 but sent %d", ch1Sent)
		t.Logf("Channel 2 scheduled to sent 1000 but sent %d", ch2Sent)
		isFailed := false
		if !(1000 > ch1Sent && 1000 > ch2Sent && 10 == ch3Sent) {
			isFailed = true
		}

		if !(received && agg.stopped && !agg.allowWatcherReset && 0 == len(agg.watchers) &&
			w1.stopped && w2.stopped && w3.stopped) {
			isFailed = true
		}
		if isFailed {
			failureTimes++
		}
	}
	t.Logf("Failure times %d", failureTimes)
	assert.True(t, 10 >= failureTimes) // 90% pass
}

func TestAggregatedWatcher_Stop(t *testing.T) {
	sleepTime := 30 * time.Millisecond
	triedTimes := 1

	for {
		aggW := NewAggregatedWatcher()
		for m := 0; m < 5000; m++ {
			ch := make(chan Event)
			w := NewProxyWatcher(ch)
			aggW.AddWatchInterface(w, nil)
		}
		aggW.Stop()
		time.Sleep(sleepTime)
		if aggW.stopped && len(aggW.stopChans) == 0 && len(aggW.watchers) == 0 && len(aggW.errs) == 0 {
			aggW.mapLock.Lock()
			aggW.mapLock.Unlock()
			break
		}

		if triedTimes > 10 {
			t.Logf("Failed after %d trials", triedTimes)
			t.Fail()
		}
		triedTimes++
		sleepTime = sleepTime * 2
	}

	t.Logf("Succeed in %d trial", triedTimes)
}
