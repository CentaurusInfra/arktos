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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/kubernetes/pkg/apis/apps"
)

func TestSystemDaemonSetValidate(t *testing.T) {
	type args struct {
		obj    runtime.Object
		tenant string
		name   string
		gvk    schema.GroupVersionKind
		gvr    schema.GroupVersionResource
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "new daemon set in system tenant is allowed",
			args: args{
				obj:    &apps.DaemonSet{},
				tenant: "system",
				name:   "valid-ds",
				gvk:    apps.SchemeGroupVersion.WithKind("DaemonSet"),
				gvr:    apps.SchemeGroupVersion.WithResource("daemonsets"),
			},
			wantErr: false,
		},
		{
			name: "new daemon set in other tenant is not allowed",
			args: args{
				obj:    &apps.DaemonSet{},
				tenant: "other",
				name:   "invalid-ds",
				gvk:    apps.SchemeGroupVersion.WithKind("DaemonSet"),
				gvr:    apps.SchemeGroupVersion.WithResource("daemonsets"),
			},
			wantErr: true,
		},
		{
			name: "other resources(e.g. deployment), regardless of tenant, is fine",
			args: args{
				obj:    &apps.Deployment{},
				tenant: "other",
				name:   "valid-deploy",
				gvk:    apps.SchemeGroupVersion.WithKind("Deployment"),
				gvr:    apps.SchemeGroupVersion.WithResource("deployments"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, ok := New().(admission.ValidationInterface)
			if !ok {
				t.Fatalf("should have got a validator interface")
			}
			a := admission.NewAttributesRecord(
				tt.args.obj,
				nil,
				tt.args.gvk,
				tt.args.tenant,
				"ns-dummy",
				tt.name,
				tt.args.gvr,
				"",
				admission.Create,
				&metav1.CreateOptions{},
				false,
				nil)
			if err := s.Validate(a, nil); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
