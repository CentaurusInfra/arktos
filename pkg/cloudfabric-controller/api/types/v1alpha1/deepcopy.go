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

package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// DeepCopyInto returns a generically typed copy of an object
func (in *ControllerManager) DeepCopyInto(out *ControllerManager) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is  copying the receiver, creating a new ControllerManager.
func (in *ControllerManager) DeepCopy() *ControllerManager {
	if in == nil {
		return nil
	}
	out := new(ControllerManager)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a generically typed copy of an object
func (in *ControllerManager) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is copying the receiver, writing into out. in must be non-nil.
func (in *ControllerManagerSpec) DeepCopyInto(out *ControllerManagerSpec) {
	*out = *in
	if in.Controllers != nil {
		in, out := &in.Controllers, &out.Controllers
		*out = make([]ControllerSpec, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *ControllerSpec) DeepCopyInto(out *ControllerSpec) {
	*out = *in
	return
}

// DeepCopyObject returns a generically typed copy of an object
func (in *ControllerSpec) DeepCopyObject() *ControllerSpec {
	if in == nil {
		return nil
	}
	out := new(ControllerSpec)
	in.DeepCopyInto(out)

	return out
}

// DeepCopyObject returns a generically typed copy of an object
func (in *ControllerManagerList) DeepCopyObject() runtime.Object {
	out := ControllerManagerList{}
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta

	if in.Items != nil {
		out.Items = make([]ControllerManager, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}

	return &out
}
