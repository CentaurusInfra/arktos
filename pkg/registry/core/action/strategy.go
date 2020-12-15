/*
Copyright 2020 Authors of Arktos.

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

package action

import (
	"context"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/validation"
)

type actionStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that pplies when creating and updating
// Action objects via the REST API.
var Strategy = actionStrategy{legacyscheme.Scheme, names.SimpleNameGenerator}

func (actionStrategy) DefaultGarbageCollectionPolicy(ctx context.Context) rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

func (actionStrategy) NamespaceScoped() bool {
	return true
}

func (actionStrategy) TenantScoped() bool {
	return true
}

func (actionStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	// TODO: Initialize defaults
}

func (actionStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	// TODO: Do we allow this?
}

func (actionStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	action := obj.(*api.Action)
	return validation.ValidateAction(action)
}

// Canonicalize normalizes the object after validation.
func (actionStrategy) Canonicalize(obj runtime.Object) {
	// TODO: What do we do here?
}

func (actionStrategy) AllowCreateOnUpdate() bool {
	return true
}

func (actionStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	action := obj.(*api.Action)
	return validation.ValidateAction(action)
}

func (actionStrategy) AllowUnconditionalUpdate() bool {
	return true
}

type actionStatusStrategy struct {
	actionStrategy
}

var StatusStrategy = actionStatusStrategy{Strategy}

func (actionStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newAction := obj.(*api.Action)
	oldAction := old.(*api.Action)
	newAction.Spec = oldAction.Spec
}

func (actionStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateActionStatusUpdate(obj.(*api.Action), old.(*api.Action))
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	action, ok := obj.(*api.Action)
	if !ok {
		return nil, nil, fmt.Errorf("not an action")
	}
	return labels.Set(action.ObjectMeta.Labels), ActionToSelectableFields(action), nil
}

func MatchAction(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:       label,
		Field:       field,
		GetAttrs:    GetAttrs,
		IndexFields: []string{"spec.nodeName"},
	}
}

func NodeNameTriggerFunc(obj runtime.Object) string {
	return obj.(*api.Action).Spec.NodeName
}

// ActionToSelectableFields returns a field set that represents the object
func ActionToSelectableFields(action *api.Action) fields.Set {
	//TODO: What else do we allow filtering on? TBD
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&action.ObjectMeta, true)
	actionSpecificFieldsSet := fields.Set{
		"spec.nodeName":   action.Spec.NodeName,
		"status.complete": strconv.FormatBool(action.Status.Complete),
	}
	return generic.MergeFieldsSets(objectMetaFieldsSet, actionSpecificFieldsSet)
}
