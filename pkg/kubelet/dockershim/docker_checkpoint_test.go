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

package dockershim

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPodSandboxCheckpoint(t *testing.T) {
	data := &CheckpointData{HostNetwork: true}
	checkpoint := NewPodSandboxCheckpoint("test-te", "ns1", "sandbox1", data)
	version, name, namespace, tenant, _, hostNetwork := checkpoint.GetData()
	assert.Equal(t, schemaVersion, version)
	assert.Equal(t, "test-te", tenant)
	assert.Equal(t, "ns1", namespace)
	assert.Equal(t, "sandbox1", name)
	assert.Equal(t, true, hostNetwork)
}
