/*
Copyright 2017 The Kubernetes Authors.
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

package cloudfabriccontrollers

import (
	"fmt"
	"testing"
	"time"

	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
)

type UpdateReplicaSetFunc func(d *apps.ReplicaSet)

func UpdateReplicaSetWithRetriesWithMultiTenancy(c clientset.Interface, tenant, namespace, name string, applyUpdate UpdateReplicaSetFunc, logf LogfFn, pollInterval, pollTimeout time.Duration) (*apps.ReplicaSet, error) {
	var rs *apps.ReplicaSet
	var updateErr error
	pollErr := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		var err error
		if rs, err = c.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant).Get(name, metav1.GetOptions{}); err != nil {
			return false, err
		}
		// Apply the update, then attempt to push it to the apiserver.
		applyUpdate(rs)
		if rs, err = c.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant).Update(rs); err == nil {
			logf("Updating replica set %q", name)
			return true, nil
		}
		updateErr = err
		return false, nil
	})
	if pollErr == wait.ErrWaitTimeout {
		pollErr = fmt.Errorf("couldn't apply the provided updated to replicaset %q: %v", name, updateErr)
	}
	return rs, pollErr
}

// Verify .Status.Replicas is equal to .Spec.Replicas
func WaitRSStable(t *testing.T, clientSet clientset.Interface, rs *apps.ReplicaSet, pollInterval, pollTimeout time.Duration) error {
	desiredGeneration := rs.Generation
	if err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		newRS, err := clientSet.AppsV1().ReplicaSetsWithMultiTenancy(rs.Namespace, rs.Tenant).Get(rs.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return newRS.Status.ObservedGeneration >= desiredGeneration && newRS.Status.Replicas == *rs.Spec.Replicas, nil
	}); err != nil {
		return fmt.Errorf("failed to verify .Status.Replicas is equal to .Spec.Replicas for replicaset %q: %v", rs.Name, err)
	}
	return nil
}

func UpdateReplicaSetStatusWithRetriesWithMultiTenancy(c clientset.Interface, tenant, namespace, name string, applyUpdate UpdateReplicaSetFunc, logf LogfFn, pollInterval, pollTimeout time.Duration) (*apps.ReplicaSet, error) {
	var rs *apps.ReplicaSet
	var updateErr error
	pollErr := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		var err error
		if rs, err = c.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant).Get(name, metav1.GetOptions{}); err != nil {
			return false, err
		}
		// Apply the update, then attempt to push it to the apiserver.
		applyUpdate(rs)
		if rs, err = c.AppsV1().ReplicaSetsWithMultiTenancy(namespace, tenant).UpdateStatus(rs); err == nil {
			logf("Updating replica set %q", name)
			return true, nil
		}
		updateErr = err
		return false, nil
	})
	if pollErr == wait.ErrWaitTimeout {
		pollErr = fmt.Errorf("couldn't apply the provided update to replicaset %q: %v", name, updateErr)
	}
	return rs, pollErr
}
