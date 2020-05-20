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

package fake

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	core "k8s.io/client-go/testing"
)

func (c *FakeEvents) CreateWithEventNamespace(event *v1.Event) (*v1.Event, error) {
	action := core.CreateActionImpl{}
	switch {
	case c.te == "" && c.ns == "":
		action = core.NewRootCreateAction(eventsResource, event)
	case c.te != "" && c.ns == "":
		action = core.NewTenantCreateAction(eventsResource, event, c.te)
	case c.te != "" && c.ns != "":
		action = core.NewCreateActionWithMultiTenancy(eventsResource, c.ns, event, c.te)
	default:
		return nil, fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	obj, err := c.Fake.Invokes(action, event)
	if obj == nil {
		return nil, err
	}

	return obj.(*v1.Event), err
}

// Update replaces an existing event. Returns the copy of the event the server returns, or an error.
func (c *FakeEvents) UpdateWithEventNamespace(event *v1.Event) (*v1.Event, error) {
	action := core.UpdateActionImpl{}
	switch {
	case c.te == "" && c.ns == "":
		action = core.NewRootUpdateAction(eventsResource, event)
	case c.te != "" && c.ns == "":
		action = core.NewTenantUpdateAction(eventsResource, event, c.te)
	case c.te != "" && c.ns != "":
		action = core.NewUpdateActionWithMultiTenancy(eventsResource, c.ns, event, c.te)
	default:
		return nil, fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	obj, err := c.Fake.Invokes(action, event)
	if obj == nil {
		return nil, err
	}

	return obj.(*v1.Event), err
}

// PatchWithEventNamespace patches an existing event. Returns the copy of the event the server returns, or an error.
// TODO: Should take a PatchType as an argument probably.
func (c *FakeEvents) PatchWithEventNamespace(event *v1.Event, data []byte) (*v1.Event, error) {
	action := core.PatchActionImpl{}
	// TODO: Should be configurable to support additional patch strategies.
	pt := types.StrategicMergePatchType
	switch {
	case c.te == "" && c.ns == "":
		action = core.NewRootPatchAction(eventsResource, event.Name, pt, data)
	case c.te != "" && c.ns == "":
		action = core.NewTenantPatchAction(eventsResource, event.Name, pt, data, c.te)
	case c.te != "" && c.ns != "":
		action = core.NewPatchActionWithMultiTenancy(eventsResource, c.ns, event.Name, pt, data, c.te)
	default:
		return nil, fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	obj, err := c.Fake.Invokes(action, event)
	if obj == nil {
		return nil, err
	}

	return obj.(*v1.Event), err
}

// Search returns a list of events matching the specified object.
func (c *FakeEvents) Search(scheme *runtime.Scheme, objOrRef runtime.Object) (*v1.EventList, error) {
	action := core.ListActionImpl{}
	switch {
	case c.te == "" && c.ns == "":
		action = core.NewRootListAction(eventsResource, eventsKind, metav1.ListOptions{})
	case c.te != "" && c.ns == "":
		action = core.NewTenantListAction(eventsResource, eventsKind, metav1.ListOptions{}, c.te)
	case c.te != "" && c.ns != "":
		action = core.NewListActionWithMultiTenancy(eventsResource, eventsKind, c.ns, metav1.ListOptions{}, c.te)
	default:
		return nil, fmt.Errorf("namespace is not-empty but tenant is empty")
	}

	if c.ns != "" {

	}
	obj, err := c.Fake.Invokes(action, &v1.EventList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*v1.EventList), err
}

func (c *FakeEvents) GetFieldSelector(involvedObjectName, involvedObjectNamespace, involvedObjectKind, involvedObjectUID *string) fields.Selector {
	return c.GetFieldSelectorWithMultiTenancy(involvedObjectName, involvedObjectNamespace, involvedObjectKind, involvedObjectUID, v1.TenantSystem)
}

func (c *FakeEvents) GetFieldSelectorWithMultiTenancy(involvedObjectName, involvedObjectNamespace, involvedObjectKind, involvedObjectUID *string, involvedObjectTenant string) fields.Selector {
	action := core.GenericActionImpl{}
	action.Verb = "get-field-selector"
	action.Resource = eventsResource

	c.Fake.Invokes(action, nil)
	return fields.Everything()
}
