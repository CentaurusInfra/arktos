/*
Copyright 2018 The Kubernetes Authors.

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

package state

// State is a state of the cluster.
// It is composed of namespaces state and resources versions state.
type State struct {
	namespacesState       *namespacesState
	resourcesVersionState *resourcesVersionsState
}

// NewState creates new State instance.
func NewState() *State {
	return &State{
		namespacesState:       newNamespacesState(),
		resourcesVersionState: newResourcesVersionsState(),
	}
}

// GetNamespacesState returns namespaces state.
func (s *State) GetNamespacesState() *namespacesState {
	return s.namespacesState
}

// GetResourcesVersionState returns resources versions state.
func (s *State) GetResourcesVersionState() *resourcesVersionsState {
	return s.resourcesVersionState
}
