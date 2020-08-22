/*
Copyright 2016 The Kubernetes Authors.
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

package retry

import (
	"fmt"
	"k8s.io/apimachinery/pkg/watch"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestRetryOnConflict(t *testing.T) {
	opts := wait.Backoff{Factor: 1.0, Steps: 3}
	conflictErr := errors.NewConflict(schema.GroupResource{Resource: "test"}, "other", nil)

	// never returns
	err := RetryOnConflict(opts, func() error {
		return conflictErr
	})
	if err != conflictErr {
		t.Errorf("unexpected error: %v", err)
	}

	// returns immediately
	i := 0
	err = RetryOnConflict(opts, func() error {
		i++
		return nil
	})
	if err != nil || i != 1 {
		t.Errorf("unexpected error: %v", err)
	}

	// returns immediately on error
	testErr := fmt.Errorf("some other error")
	err = RetryOnConflict(opts, func() error {
		return testErr
	})
	if err != testErr {
		t.Errorf("unexpected error: %v", err)
	}

	// keeps retrying
	i = 0
	err = RetryOnConflict(opts, func() error {
		if i < 2 {
			i++
			return errors.NewConflict(schema.GroupResource{Resource: "test"}, "other", nil)
		}
		return nil
	})
	if err != nil || i != 2 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRetryOnTimeout_Timeout(t *testing.T) {
	opts := wait.Backoff{Factor: 1.0, Steps: 3}
	timeoutErr := errors.NewTimeoutError("timeout", 1)
	// never returns
	err := RetryOnTimeout(opts, func() error {
		return timeoutErr
	})
	if err != timeoutErr {
		t.Errorf("unexpected error: %v", err)
	}

	// returns immediately
	i := 0
	err = RetryOnTimeout(opts, func() error {
		i++
		return nil
	})
	if err != nil || i != 1 {
		t.Errorf("unexpected error: %v", err)
	}

	// returns immediately on error
	testErr := fmt.Errorf("some other error")
	err = RetryOnTimeout(opts, func() error {
		return testErr
	})
	if err != testErr {
		t.Errorf("unexpected error: %v", err)
	}

	// keeps retrying
	i = 0
	err = RetryOnTimeout(opts, func() error {
		if i < 2 {
			i++
			return errors.NewTimeoutError("timeout", 1)
		}
		return nil
	})
	if err != nil || i != 2 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRetryOnTimeout_ServerTimeout(t *testing.T) {
	opts := wait.Backoff{Factor: 1.0, Steps: 3}
	timeoutErr := errors.NewServerTimeout(schema.GroupResource{Resource: "test"}, "other", 1)
	// never returns
	err := RetryOnTimeout(opts, func() error {
		return timeoutErr
	})
	if err != timeoutErr {
		t.Errorf("unexpected error: %v", err)
	}

	// returns immediately
	i := 0
	err = RetryOnTimeout(opts, func() error {
		i++
		return nil
	})
	if err != nil || i != 1 {
		t.Errorf("unexpected error: %v", err)
	}

	// returns immediately on error
	testErr := fmt.Errorf("some other error")
	err = RetryOnTimeout(opts, func() error {
		return testErr
	})
	if err != testErr {
		t.Errorf("unexpected error: %v", err)
	}

	// keeps retrying
	i = 0
	err = RetryOnTimeout(opts, func() error {
		if i < 2 {
			i++
			return errors.NewServerTimeout(schema.GroupResource{Resource: "test"}, "other", 1)
		}
		return nil
	})
	if err != nil || i != 2 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRetryOnTimeout_ServiceUnavailable(t *testing.T) {
	opts := wait.Backoff{Factor: 1.0, Steps: 3}
	timeoutErr := errors.NewServiceUnavailable("service unavailable")
	// never returns
	err := RetryOnTimeout(opts, func() error {
		return timeoutErr
	})
	if err != timeoutErr {
		t.Errorf("unexpected error: %v", err)
	}

	// returns immediately
	i := 0
	err = RetryOnTimeout(opts, func() error {
		i++
		return nil
	})
	if err != nil || i != 1 {
		t.Errorf("unexpected error: %v", err)
	}

	// returns immediately on error
	testErr := fmt.Errorf("some other error")
	err = RetryOnTimeout(opts, func() error {
		return testErr
	})
	if err != testErr {
		t.Errorf("unexpected error: %v", err)
	}

	// keeps retrying
	i = 0
	err = RetryOnTimeout(opts, func() error {
		if i < 2 {
			i++
			return errors.NewServiceUnavailable("service unavailable")
		}
		return nil
	})
	if err != nil || i != 2 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRetryOnNoResponse_InternalError(t *testing.T) {
	opts := wait.Backoff{Factor: 1.0, Steps: 3}
	internalError := errors.NewInternalError(fmt.Errorf("internal error"))
	watcherMock := watch.NewAggregatedWatcher()
	// never returns
	watcher, err := RetryOnNoResponse(opts, func() (watch.Interface, error) {
		return nil, internalError
	})
	if err != internalError {
		t.Errorf("unexpected error: %v", err)
	}
	if watcher != nil {
		t.Errorf("Expected nil watcher at internal error but got not nil")
	}

	// returns immediately
	i := 0
	watcher, err = RetryOnNoResponse(opts, func() (watch.Interface, error) {
		i++
		return watcherMock, nil
	})
	if err != nil || i != 1 {
		t.Errorf("unexpected error: %v", err)
	}
	if watcher != watcherMock {
		t.Errorf("Expecting not nill watcher but got nil")
	}

	// returns immediately on error
	testErr := fmt.Errorf("some other error")
	watcher, err = RetryOnNoResponse(opts, func() (watch.Interface, error) {
		return nil, testErr
	})
	if err != testErr {
		t.Errorf("unexpected error: %v", err)
	}
	if watcher != nil {
		t.Errorf("Expected nil watcher at internal error but got not nil")
	}

	// keeps retrying
	i = 0
	watcher, err = RetryOnNoResponse(opts, func() (watch.Interface, error) {
		if i < 2 {
			i++
			return nil, errors.NewInternalError(fmt.Errorf("internal error"))
		}
		return watcher, nil
	})
	if err != nil || i != 2 {
		t.Errorf("unexpected error: %v", err)
	}
	if watcher != nil {
		t.Errorf("Expected nil watcher at internal error but got not nil")
	}
}

func TestRetryOnNoResponse__ServiceUnavailable(t *testing.T) {
	opts := wait.Backoff{Factor: 1.0, Steps: 3}
	internalError := errors.NewServiceUnavailable("service unavailable")
	watcherMock := watch.NewAggregatedWatcher()
	// never returns
	watcher, err := RetryOnNoResponse(opts, func() (watch.Interface, error) {
		return nil, internalError
	})
	if err != internalError {
		t.Errorf("unexpected error: %v", err)
	}
	if watcher != nil {
		t.Errorf("Expected nil watcher at internal error but got not nil")
	}

	// returns immediately
	i := 0
	watcher, err = RetryOnNoResponse(opts, func() (watch.Interface, error) {
		i++
		return watcherMock, nil
	})
	if err != nil || i != 1 {
		t.Errorf("unexpected error: %v", err)
	}
	if watcher != watcherMock {
		t.Errorf("Expecting not nill watcher but got nil")
	}

	// returns immediately on error
	testErr := fmt.Errorf("some other error")
	watcher, err = RetryOnNoResponse(opts, func() (watch.Interface, error) {
		return nil, testErr
	})
	if err != testErr {
		t.Errorf("unexpected error: %v", err)
	}
	if watcher != nil {
		t.Errorf("Expected nil watcher at internal error but got not nil")
	}

	// keeps retrying
	i = 0
	watcher, err = RetryOnNoResponse(opts, func() (watch.Interface, error) {
		if i < 2 {
			i++
			return nil, errors.NewServiceUnavailable("service unavailable")
		}
		return watcher, nil
	})
	if err != nil || i != 2 {
		t.Errorf("unexpected error: %v", err)
	}
	if watcher != nil {
		t.Errorf("Expected nil watcher at internal error but got not nil")
	}
}
