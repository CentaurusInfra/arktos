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

package request

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/authentication/user"
)

// The key type is unexported to prevent collisions
type key int

const (
	// namespaceKey is the context key for the request namespace.
	namespaceKey key = iota

	// userKey is the context key for the request user.
	userKey

	// auditKey is the context key for the audit event.
	auditKey

	// audiencesKey is the context key for request audiences.
	audiencesKey

	// tenantKey is the context key for the request tenant.
	tenantKey
)

// NewContext instantiates a base context object for request flows.
func NewContext() context.Context {
	return context.TODO()
}

// NewDefaultContext instantiates a base context object for request flows in the default tenant/namespace
func NewDefaultContext() context.Context {
	ctx := WithNamespace(NewContext(), metav1.NamespaceDefault)
	ctx = WithTenant(ctx, metav1.TenantSystem)
	return ctx
}

// WithValue returns a copy of parent in which the value associated with key is val.
func WithValue(parent context.Context, key interface{}, val interface{}) context.Context {
	return context.WithValue(parent, key, val)
}

// WithTenant returns a copy of parent in which the tenant value is set
func WithTenant(parent context.Context, tenant string) context.Context {
	return WithValue(parent, tenantKey, tenant)
}

// TenantFrom returns the value of the tenant key on the ctx
func TenantFrom(ctx context.Context) (string, bool) {
	// for backward compatibility with code before multi-tenancy, we set the tenant value as "default" if tenantKey is not set
	if ctx.Value(tenantKey) == nil {
		return metav1.TenantSystem, true
	}
	tenant, ok := ctx.Value(tenantKey).(string)
	return tenant, ok
}

// TenantValue returns the value of the tenant key on the ctx, or the empty string if none
func TenantValue(ctx context.Context) string {
	tenant, _ := TenantFrom(ctx)
	return tenant
}

// WithNamespace returns a copy of parent in which the namespace value is set
func WithNamespace(parent context.Context, namespace string) context.Context {
	return WithValue(parent, namespaceKey, namespace)
}

// WithTenantAndNamespace returns a copy of parent in which the tenant and namespace value are set
func WithTenantAndNamespace(parent context.Context, tenant, namespace string) context.Context {
	return WithValue(
		WithValue(parent, namespaceKey, namespace),
		tenantKey,
		tenant)
}

// NamespaceFrom returns the value of the namespace key on the ctx
func NamespaceFrom(ctx context.Context) (string, bool) {
	namespace, ok := ctx.Value(namespaceKey).(string)
	return namespace, ok
}

// NamespaceValue returns the value of the namespace key on the ctx, or the empty string if none
func NamespaceValue(ctx context.Context) string {
	namespace, _ := NamespaceFrom(ctx)
	return namespace
}

// WithUser returns a copy of parent in which the user value is set
func WithUser(parent context.Context, user user.Info) context.Context {
	return WithValue(parent, userKey, user)
}

// UserFrom returns the value of the user key on the ctx
func UserFrom(ctx context.Context) (user.Info, bool) {
	user, ok := ctx.Value(userKey).(user.Info)
	return user, ok
}

// WithAuditEvent returns set audit event struct.
func WithAuditEvent(parent context.Context, ev *audit.Event) context.Context {
	return WithValue(parent, auditKey, ev)
}

// AuditEventFrom returns the audit event struct on the ctx
func AuditEventFrom(ctx context.Context) *audit.Event {
	ev, _ := ctx.Value(auditKey).(*audit.Event)
	return ev
}
