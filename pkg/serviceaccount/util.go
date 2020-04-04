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

package serviceaccount

import (
	"k8s.io/api/core/v1"
	apiserverserviceaccount "k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	// PodNameKey is the key used in a user's "extra" to specify the pod name of
	// the authenticating request.
	PodNameKey = "authentication.kubernetes.io/pod-name"
	// PodUIDKey is the key used in a user's "extra" to specify the pod UID of
	// the authenticating request.
	PodUIDKey = "authentication.kubernetes.io/pod-uid"
)

// UserInfo returns a user.Info interface for the given namespace, service account name and UID
func UserInfo(namespace, name, uid, tenant string) user.Info {
	return (&ServiceAccountInfo{
		Name:      name,
		Namespace: namespace,
		UID:       uid,
		Tenant:    tenant,
	}).UserInfo()
}

type ServiceAccountInfo struct {
	Name, Namespace, UID, Tenant string
	PodName, PodUID              string
}

func (sa *ServiceAccountInfo) UserInfo() user.Info {
	info := &user.DefaultInfo{
		Name:   apiserverserviceaccount.MakeUsername(sa.Namespace, sa.Name),
		UID:    sa.UID,
		Groups: apiserverserviceaccount.MakeGroupNames(sa.Namespace),
		Tenant: sa.Tenant,
	}
	if sa.PodName != "" && sa.PodUID != "" {
		info.Extra = map[string][]string{
			PodNameKey: {sa.PodName},
			PodUIDKey:  {sa.PodUID},
		}
	}
	return info
}

// IsServiceAccountToken returns true if the secret is a valid api token for the service account
func IsServiceAccountToken(secret *v1.Secret, sa *v1.ServiceAccount) bool {
	if secret.Type != v1.SecretTypeServiceAccountToken {
		return false
	}

	name := secret.Annotations[v1.ServiceAccountNameKey]
	uid := secret.Annotations[v1.ServiceAccountUIDKey]
	if name != sa.Name {
		// Name must match
		return false
	}
	if secret.Tenant != sa.Tenant {
		// Tenant must match
		return false
	}
	if len(uid) > 0 && uid != string(sa.UID) {
		// If UID is specified, it must match
		return false
	}

	return true
}
