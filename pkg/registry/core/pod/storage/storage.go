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

package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	storeerr "k8s.io/apiserver/pkg/storage/errors"
	"k8s.io/apiserver/pkg/util/dryrun"
	policyclient "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	"k8s.io/klog"
	podutil "k8s.io/kubernetes/pkg/api/pod"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/validation"
	"k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/printers"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
	"k8s.io/kubernetes/pkg/registry/core/pod"
	podrest "k8s.io/kubernetes/pkg/registry/core/pod/rest"
)

// PodStorage includes storage for pods and all sub resources
type PodStorage struct {
	Pod         *REST
	Binding     *BindingREST
	Action      *ActionREST
	Eviction    *EvictionREST
	Status      *StatusREST
	Log         *podrest.LogREST
	Proxy       *podrest.ProxyREST
	Exec        *podrest.ExecREST
	Attach      *podrest.AttachREST
	PortForward *podrest.PortForwardREST
}

// REST implements a RESTStorage for pods
type REST struct {
	*genericregistry.Store
	proxyTransport http.RoundTripper
}

// NewStorage returns a RESTStorage object that will work against pods.
func NewStorage(optsGetter generic.RESTOptionsGetter, k client.ConnectionInfoGetter, proxyTransport http.RoundTripper, podDisruptionBudgetClient policyclient.PodDisruptionBudgetsGetter, actionStore *genericregistry.Store) PodStorage {

	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &api.Pod{} },
		NewListFunc:              func() runtime.Object { return &api.PodList{} },
		PredicateFunc:            pod.MatchPod,
		DefaultQualifiedResource: api.Resource("pods"),

		CreateStrategy:      pod.Strategy,
		UpdateStrategy:      pod.Strategy,
		DeleteStrategy:      pod.Strategy,
		ReturnDeletedObject: true,

		TableConvertor: printerstorage.TableConvertor{TableGenerator: printers.NewTableGenerator().With(printersinternal.AddHandlers)},
	}
	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
		AttrFunc:    pod.GetAttrs,
		TriggerFunc: map[string]storage.IndexerFunc{"spec.nodeName": pod.NodeNameTriggerFunc},
	}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err) // TODO: Propagate error up
	}

	statusStore := *store
	statusStore.UpdateStrategy = pod.StatusStrategy

	return PodStorage{
		Pod:         &REST{store, proxyTransport},
		Binding:     &BindingREST{store: store},
		Action:      &ActionREST{store: store, ActionStore: actionStore},
		Eviction:    newEvictionStorage(store, podDisruptionBudgetClient),
		Status:      &StatusREST{store: &statusStore},
		Log:         &podrest.LogREST{Store: store, KubeletConn: k},
		Proxy:       &podrest.ProxyREST{Store: store, ProxyTransport: proxyTransport},
		Exec:        &podrest.ExecREST{Store: store, KubeletConn: k},
		Attach:      &podrest.AttachREST{Store: store, KubeletConn: k},
		PortForward: &podrest.PortForwardREST{Store: store, KubeletConn: k},
	}
}

// Implement Redirector.
var _ = rest.Redirector(&REST{})

// ResourceLocation returns a pods location from its HostIP
func (r *REST) ResourceLocation(ctx context.Context, name string) (*url.URL, http.RoundTripper, error) {
	return pod.ResourceLocation(r, r.proxyTransport, ctx, name)
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"po"}
}

// Implement CategoriesProvider
var _ rest.CategoriesProvider = &REST{}

// Categories implements the CategoriesProvider interface. Returns a list of categories a resource is part of.
func (r *REST) Categories() []string {
	return []string{"all"}
}

// BindingREST implements the REST endpoint for binding pods to nodes when etcd is in use.
type BindingREST struct {
	store *genericregistry.Store
}

// NamespaceScoped fulfill rest.Scoper
func (r *BindingREST) NamespaceScoped() bool {
	return r.store.NamespaceScoped()
}

// TenantScoped fulfill rest.Scoper
func (r *BindingREST) TenantScoped() bool {
	return r.store.TenantScoped()
}

// New creates a new binding resource
func (r *BindingREST) New() runtime.Object {
	return &api.Binding{}
}

var _ = rest.Creater(&BindingREST{})

// Create ensures a pod is bound to a specific host.
func (r *BindingREST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (out runtime.Object, err error) {
	binding := obj.(*api.Binding)

	// TODO: move me to a binding strategy
	if errs := validation.ValidatePodBinding(binding); len(errs) != 0 {
		return nil, errs.ToAggregate()
	}

	if createValidation != nil {
		if err := createValidation(binding.DeepCopyObject()); err != nil {
			return nil, err
		}
	}

	err = r.assignPod(ctx, binding.Name, binding.Target.Name, binding.Annotations, dryrun.IsDryRun(options.DryRun))
	out = &metav1.Status{Status: metav1.StatusSuccess}
	return
}

// setPodHostAndAnnotations sets the given pod's host to 'machine' if and only if it was
// previously 'oldMachine' and merges the provided annotations with those of the pod.
// Returns the current state of the pod, or an error.
func (r *BindingREST) setPodHostAndAnnotations(ctx context.Context, podID, oldMachine, machine string, annotations map[string]string, dryRun bool) (finalPod *api.Pod, err error) {
	podKey, err := r.store.KeyFunc(ctx, podID)
	if err != nil {
		return nil, err
	}
	err = r.store.Storage.GuaranteedUpdate(ctx, podKey, &api.Pod{}, false, nil, storage.SimpleUpdate(func(obj runtime.Object) (runtime.Object, error) {
		pod, ok := obj.(*api.Pod)
		if !ok {
			return nil, fmt.Errorf("unexpected object: %#v", obj)
		}
		if pod.DeletionTimestamp != nil {
			return nil, fmt.Errorf("pod %s is being deleted, cannot be assigned to a host", pod.Name)
		}
		if pod.Spec.VirtualMachine == nil && pod.Spec.NodeName != oldMachine {
			return nil, fmt.Errorf("pod %v is already assigned to node %q", pod.Name, pod.Spec.NodeName)
		}
		pod.Spec.NodeName = machine
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			pod.Annotations[k] = v
		}
		podutil.UpdatePodCondition(&pod.Status, &api.PodCondition{
			Type:   api.PodScheduled,
			Status: api.ConditionTrue,
		})
		finalPod = pod
		return pod, nil
	}), dryRun)
	return finalPod, err
}

// assignPod assigns the given pod to the given machine.
func (r *BindingREST) assignPod(ctx context.Context, podID string, machine string, annotations map[string]string, dryRun bool) (err error) {
	if _, err = r.setPodHostAndAnnotations(ctx, podID, "", machine, annotations, dryRun); err != nil {
		err = storeerr.InterpretGetError(err, api.Resource("pods"), podID)
		err = storeerr.InterpretUpdateError(err, api.Resource("pods"), podID)
		if _, ok := err.(*errors.StatusError); !ok {
			err = errors.NewConflict(api.Resource("pods/binding"), podID, err)
		}
	}
	return
}

// ActionREST implements the REST endpoint for invoking Action on a Pod.
type ActionREST struct {
	store       *genericregistry.Store
	ActionStore *genericregistry.Store
}

// NamespaceScoped fulfill rest.Scoper
func (r *ActionREST) NamespaceScoped() bool {
	return r.store.NamespaceScoped()
}

func (r *ActionREST) TenantScoped() bool {
	return r.store.TenantScoped()
}

// New creates a new CustomAction resource
func (r *ActionREST) New() runtime.Object {
	return &api.CustomAction{}
}

var _ = rest.Creater(&ActionREST{})

func getActionSpec(pod *api.Pod, actionRequest *api.CustomAction) (actionObj *api.Action, err error) {
	actionSpec := api.ActionSpec{
		NodeName: pod.Spec.NodeName,
		PodName:  pod.Name,
	}
	switch actionRequest.Operation {
	case string(api.RebootOp):
		actionSpec.RebootAction = &api.RebootAction{
			DelayInSeconds: actionRequest.RebootParams.DelayInSeconds,
		}
	case string(api.SnapshotOp):
		actionSpec.SnapshotAction = &api.SnapshotAction{
			SnapshotName: actionRequest.SnapshotParams.SnapshotName,
		}
	case string(api.RestoreOp):
		actionSpec.RestoreAction = &api.RestoreAction{
			SnapshotID: actionRequest.RestoreParams.SnapshotID,
		}
	default:
		return nil, errors.NewBadRequest(fmt.Sprintf("Unsupported operation '%s' for Pod '%s'", actionRequest.Operation, pod.Name))

	}

	actionObject := &api.Action{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Action",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%d", actionRequest.Operation, time.Now().Unix()),
		},
		Spec: actionSpec,
	}
	return actionObject, nil
}

func (r *ActionREST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (out runtime.Object, err error) {
	customAction := obj.(*api.CustomAction)

	klog.V(4).Infof("ActionREST Create customAction object:\n-------------\n%+v\n------------\n", customAction)

	path := request.PathValue(ctx)
	if path == "" {
		return nil, fmt.Errorf("invalid URL path: URL path is empty")
	}

	// path format to the action subresource: api/v1/tenants/{tenant}/namespaces/{namespace}/pods/{podName}/action
	eles := strings.Split(path, "/pods/")
	if len(eles) != 2 {
		return nil, fmt.Errorf("invalid URL path format")
	}
	podName := strings.Split(eles[1], "/")[0]
	if podName == "" {
		return nil, fmt.Errorf("invalid pod name for specified action")
	}

	podObj, getErr := r.store.Get(ctx, podName, &metav1.GetOptions{})
	if podObj == nil {
		return nil, getErr
	}
	pod := podObj.(*api.Pod)

	actionSpec, actionErr := getActionSpec(pod, customAction)
	if actionErr != nil {
		return nil, actionErr
	}

	runtimeObj, err := r.ActionStore.Create(ctx, actionSpec, nil, &metav1.CreateOptions{})
	if err == nil {
		actionObj := runtimeObj.(*api.Action)
		// TODO: post 130 release
		//       current design for supporting openstack is to limit the changes at the REST layer, i.e. the handlers, request and response
		//       if changing the storage layer is inevitable, consider to build the openstack action struct here directly.
		name := actionObj.Name
		if customAction.Operation == "snapshot" {
			name = customAction.SnapshotParams.SnapshotName
		} else if customAction.Operation == "restore" {
			name = customAction.RestoreParams.SnapshotID
		}

		out = &metav1.Status{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Action",
			},
			ListMeta: metav1.ListMeta{
				SelfLink:        actionObj.SelfLink,
				ResourceVersion: actionObj.ResourceVersion,
			},
			Status:  metav1.StatusSuccess,
			Message: fmt.Sprintf("Created action '%s' for Pod '%s'", customAction.Operation, pod.Name),
			Details: &metav1.StatusDetails{
				Name: name,
				UID:  actionObj.UID,
			},
		}
	} else {
		out = &metav1.Status{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Action",
			},
			Status:  metav1.StatusFailure,
			Message: fmt.Sprintf("Failed to create action '%s' for Pod '%s'", customAction.Operation, pod.Name),
			Reason:  metav1.StatusReason(err.Error()),
		}
	}
	return out, err
}

// StatusREST implements the REST endpoint for changing the status of a pod.
type StatusREST struct {
	store *genericregistry.Store
}

// New creates a new pod resource
func (r *StatusREST) New() runtime.Object {
	return &api.Pod{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	// We are explicitly setting forceAllowCreate to false in the call to the underlying storage because
	// subresources should never allow create on update.
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, false, options)
}
