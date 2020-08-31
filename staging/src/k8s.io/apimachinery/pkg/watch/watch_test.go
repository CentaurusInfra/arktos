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
	"github.com/stretchr/testify/assert"
	"reflect"
	"sync"
	"testing"
	"time"

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
			time.Sleep(10 * time.Millisecond)
			aggWatcher.removeWatcherAndError(w)
			wg.Done()
		}(aw, i)
	}

	wg.Wait()
	assert.True(t, aw.allowWatcherReset)
	assert.Nil(t, aw.GetErrors())
	assert.Equal(t, 0, aw.GetWatchersCount())
	assert.False(t, aw.stopped)
	aw.Stop()
	assert.True(t, aw.stopped)
}
