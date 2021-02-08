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

package garbagecollector

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
)

// Cluster-scoped or tenant-scoped resources don't have namespaces.
// Default to the item's namespace, but clear it for cluster-scoped or tenant-scoped resources
func resourceDefaultNamespace(namespaced bool, defaultNamespace string) string {
	if namespaced {
		return defaultNamespace
	}
	return ""
}

// cluster-scoped resources don't have tenants.  Default to the item's tenant, but clear it for cluster scoped resources
func resourceDefaultTenant(tenanted bool, defaultTenant string) string {
	if tenanted {
		return defaultTenant
	}
	return ""
}

// apiResource consults the REST mapper to translate an <apiVersion, kind,
// namespace> tuple to a unversioned.APIResource struct.
func (gc *GarbageCollector) apiResource(apiVersion, kind string) (schema.GroupVersionResource, bool, bool, error) {
	fqKind := schema.FromAPIVersionAndKind(apiVersion, kind)
	mapping, err := gc.restMapper.RESTMapping(fqKind.GroupKind(), fqKind.Version)
	if err != nil {
		return schema.GroupVersionResource{}, false, false, newRESTMappingError(kind, apiVersion)
	}
	namespaceScoped := (mapping.Scope == meta.RESTScopeNamespace)
	tenantScoped := (mapping.Scope == meta.RESTScopeNamespace || mapping.Scope == meta.RESTScopeTenant)
	return mapping.Resource, namespaceScoped, tenantScoped, nil
}

func (gc *GarbageCollector) deleteObject(item objectReference, policy *metav1.DeletionPropagation) error {
	resource, namespaced, tenanted, err := gc.apiResource(item.APIVersion, item.Kind)
	if err != nil {
		return err
	}
	uid := item.UID
	preconditions := metav1.Preconditions{UID: &uid}
	deleteOptions := metav1.DeleteOptions{Preconditions: &preconditions, PropagationPolicy: policy}

	namespace := resourceDefaultNamespace(namespaced, item.Namespace)
	tenant := resourceDefaultTenant(tenanted, item.Tenant)
	return gc.metadataClient.Resource(resource).NamespaceWithMultiTenancy(namespace, tenant).Delete(item.Name, &deleteOptions)
}

func (gc *GarbageCollector) getObject(item objectReference) (*metav1.PartialObjectMetadata, error) {
	resource, namespaced, tenanted, err := gc.apiResource(item.APIVersion, item.Kind)
	if err != nil {
		return nil, err
	}

	namespace := resourceDefaultNamespace(namespaced, item.Namespace)
	tenant := resourceDefaultTenant(tenanted, item.Tenant)
	return gc.metadataClient.Resource(resource).NamespaceWithMultiTenancy(namespace, tenant).Get(item.Name, metav1.GetOptions{})
}

func (gc *GarbageCollector) patchObject(item objectReference, patch []byte, pt types.PatchType) (*metav1.PartialObjectMetadata, error) {
	resource, namespaced, tenanted, err := gc.apiResource(item.APIVersion, item.Kind)
	if err != nil {
		return nil, err
	}

	namespace := resourceDefaultNamespace(namespaced, item.Namespace)
	tenant := resourceDefaultTenant(tenanted, item.Tenant)
	return gc.metadataClient.Resource(resource).NamespaceWithMultiTenancy(namespace, tenant).Patch(item.Name, pt, patch, metav1.PatchOptions{})
}

func (gc *GarbageCollector) removeFinalizer(owner *node, targetFinalizer string) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ownerObject, err := gc.getObject(owner.identity)
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("cannot finalize owner %s, because cannot get it: %v. The garbage collector will retry later", owner.identity, err)
		}
		accessor, err := meta.Accessor(ownerObject)
		if err != nil {
			return fmt.Errorf("cannot access the owner object %v: %v. The garbage collector will retry later", ownerObject, err)
		}
		finalizers := accessor.GetFinalizers()
		var newFinalizers []string
		found := false
		for _, f := range finalizers {
			if f == targetFinalizer {
				found = true
				continue
			}
			newFinalizers = append(newFinalizers, f)
		}
		if !found {
			klog.V(5).Infof("the %s finalizer is already removed from object %s", targetFinalizer, owner.identity)
			return nil
		}

		// remove the owner from dependent's OwnerReferences
		patch, err := json.Marshal(&objectForFinalizersPatch{
			ObjectMetaForFinalizersPatch: ObjectMetaForFinalizersPatch{
				ResourceVersion: accessor.GetResourceVersion(),
				Finalizers:      newFinalizers,
			},
		})
		if err != nil {
			return fmt.Errorf("unable to finalize %s due to an error serializing patch: %v", owner.identity, err)
		}
		_, err = gc.patchObject(owner.identity, patch, types.MergePatchType)
		return err
	})
	if errors.IsConflict(err) {
		return fmt.Errorf("updateMaxRetries(%d) has reached. The garbage collector will retry later for owner %v", retry.DefaultBackoff.Steps, owner.identity)
	}
	return err
}
