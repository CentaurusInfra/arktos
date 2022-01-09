/*
Copyright 2021 Authors of Arktos.

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

package systemdaemonset

import (
	"fmt"
	"io"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/core"
)

// PluginName indicates name of admission plugin.
const PluginName = "SystemDaemonSet"

// systemDaemonSet is an implementation of admission.Interface which permits daemonset in system tenant only.
type systemDaemonSet struct {
	*admission.Handler
}

func (s systemDaemonSet) Validate(a admission.Attributes, _ admission.ObjectInterfaces) (err error) {
	if _, ok := a.GetObject().(*apps.DaemonSet); !ok {
		return nil
	}
	tenant := a.GetTenant()
	if tenant != core.TenantSystem {
		return admission.NewForbidden(a, fmt.Errorf("only system tenant is allowed to have DaemonSet"))
	}

	return nil
}

var _ admission.ValidationInterface = systemDaemonSet{}

// New creates a system DaemonSet admission handler
func New() admission.Interface {
	return &systemDaemonSet{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New(), nil
	})
}
