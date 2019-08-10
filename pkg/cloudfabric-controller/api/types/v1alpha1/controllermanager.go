package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

//ControllerSpec defines the desired state of ControllerSpec
type ControllerSpec struct {
	Type    string `json:"type"`
	Workers int    `json:"workers"`
}

// ControllerManagerSpec defines the desired state of ControllerManager
type ControllerManagerSpec struct {
	ControllerManagerID   string           `json:"controller-manager-id,omitempty"`
	ControllerManagerType string           `json:"controller-manager-type,omitempty"`
	Controllers           []ControllerSpec `json:"controllers,omitempty"`
}

// ControllerManager controller manager
type ControllerManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ControllerManagerSpec `json:"spec,omitempty"`
}

// ControllerManagerList controller manager list
type ControllerManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ControllerManager `json:"items,omitempty"`
}
