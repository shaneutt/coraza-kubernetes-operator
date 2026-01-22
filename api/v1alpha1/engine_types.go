/*
Copyright 2026 Shane Utt.

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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// -----------------------------------------------------------------------------
// Setup
// -----------------------------------------------------------------------------

func init() {
	SchemeBuilder.Register(&Engine{}, &EngineList{})
}

// -----------------------------------------------------------------------------
// Engine
// -----------------------------------------------------------------------------

// Engine represents an instance of a Web Application Firewall (WAF) engine.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Instance",type=string,JSONPath=`.spec.instance`
// +kubebuilder:printcolumn:name="Driver",type=string,JSONPath=`.spec.driver.type`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
type Engine struct {
	metav1.TypeMeta `json:",inline"`

	// ObjectMeta is a standard object metadata.
	//
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// Spec defines the desired state of Engine.
	//
	// +required
	Spec EngineSpec `json:"spec"`

	// Status defines the observed state of Engine.
	//
	// +optional
	Status EngineStatus `json:"status,omitzero"`
}

// EngineList contains a list of Engine resources.
//
// +kubebuilder:object:root=true
type EngineList struct {
	metav1.TypeMeta `json:",inline"`

	// ListMeta is standard list metadata.
	//
	// +optional
	metav1.ListMeta `json:"metadata,omitzero"`

	// Items is the list of Engines.
	//
	// +required
	Items []Engine `json:"items"`
}

// -----------------------------------------------------------------------------
// EngineSpec
// -----------------------------------------------------------------------------

// EngineSpec defines the desired state of an Engine.
type EngineSpec struct {
	// Instance is the name of the WAF instance that this engine serves.
	// This name is a unique identifier used to associate RuleSets with this
	// engine via labels.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Instance string `json:"instance"`

	// Driver specifies the driver configuration for the engine. This
	// determines how the WAF engine will be deployed and integrated with some
	// implementation. Currently only supports Istio ingress Gateways.
	//
	// +required
	Driver DriverConfig `json:"driver"`

	// FailurePolicy determines the behavior when the WAF is not ready or
	// encounters errors. Valid values are:
	//
	// - "Fail": Block traffic when the WAF is not ready or encounters errors
	// - "Allow": Allow traffic through when the WAF is not ready or encounters errors
	//
	// When omitted, this means the user has no opinion and the platform
	// will choose a reasonable default, which is subject to change over time.
	// The current default is fail.
	//
	// +kubebuilder:validation:Enum=fail;allow
	// +kubebuilder:default=fail
	// +required
	FailurePolicy FailurePolicy `json:"failurePolicy"`
}

// -----------------------------------------------------------------------------
// EngineStatus
// -----------------------------------------------------------------------------

// EngineStatus defines the observed state of Engine.
type EngineStatus struct {
	// Conditions represent the current state of the Engine resource.
	// Each condition has a unique type and reflects the status of a specific
	// aspect of the resource.
	//
	// Standard condition types include:
	// - "Ready": the engine has been successfully deployed and is operational
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	//
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// OwnedResources lists the resources created and managed by this Engine.
	//
	// +listType=atomic
	// +optional
	OwnedResources []corev1.ObjectReference `json:"ownedResources,omitempty"`
}

// -----------------------------------------------------------------------------
// Engine - Failure Policy
// -----------------------------------------------------------------------------

// FailurePolicy describes the failure policy for the Engine.
type FailurePolicy string

const (
	// FailurePolicyFail blocks traffic when the Engine is not ready or encounters
	// errors.
	FailurePolicyFail FailurePolicy = "fail"

	// FailurePolicyAllow allows traffic through when the Engine is not ready or
	// encounters errors.
	FailurePolicyAllow FailurePolicy = "allow"
)
