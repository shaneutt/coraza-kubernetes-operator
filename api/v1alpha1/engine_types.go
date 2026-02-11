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
// Engine - Schema Registration
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
// +kubebuilder:printcolumn:name="RuleSet",type=string,JSONPath=`.spec.ruleSet.name`
// +kubebuilder:printcolumn:name="Failure Policy",type=string,JSONPath=`.spec.failurePolicy`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type Engine struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	//
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Engine.
	//
	// +required
	Spec EngineSpec `json:"spec,omitzero"`

	// status defines the observed state of Engine.
	//
	// +optional
	// +kubebuilder:validation:MinProperties=1
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
// Engine - Spec
// -----------------------------------------------------------------------------

// EngineSpec defines the desired state of an Engine.
type EngineSpec struct {
	// ruleSet specifies the RuleSet resource that will be used to load rules
	// into the Engine.
	//
	// +required
	// +kubebuilder:validation:XValidation:rule="self.kind == 'RuleSet' && self.apiVersion == 'waf.k8s.coraza.io/v1alpha1'",message="only waf.k8s.coraza.io/v1alpha1 RuleSet kind is supported"
	// +kubebuilder:validation:XValidation:rule="!has(self.namespace) || self.namespace == ''",message="cross-namespace references are not currently supported"
	// +kubebuilder:validation:XValidation:rule="self.name != ''",message="ruleSet name must not be empty"
	RuleSet corev1.ObjectReference `json:"ruleSet,omitempty"`

	// driver specifies the driver configuration for the engine. This
	// determines how the WAF engine will be deployed and integrated with some
	// implementation. Currently only supports Istio ingress Gateways.
	//
	// +kubebuilder:validation:MinProperties=1
	// +required
	Driver DriverConfig `json:"driver"`

	// failurePolicy determines the behavior when the WAF is not ready or
	// encounters errors. Valid values are:
	//
	// - "Fail": Block traffic when the WAF is not ready or encounters errors
	// - "Allow": Allow traffic through when the WAF is not ready or encounters errors
	//
	// When omitted, this means the user has no opinion and the platform
	// will choose a reasonable default, which is subject to change over time.
	//
	// The current default is fail.
	//
	// +optional
	// +kubebuilder:default=fail
	FailurePolicy FailurePolicy `json:"failurePolicy,omitempty"`
}

// -----------------------------------------------------------------------------
// Engine - Status
// -----------------------------------------------------------------------------

// EngineStatus defines the observed state of Engine.
type EngineStatus struct {
	// conditions represent the current state of the Engine resource.
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
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=8
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ownedResources lists the resources created and managed by this Engine.
	//
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=8
	// +optional
	OwnedResources []corev1.ObjectReference `json:"ownedResources,omitempty"`
}

// -----------------------------------------------------------------------------
// Engine - Failure Policy
// -----------------------------------------------------------------------------

// FailurePolicy describes the failure policy for the Engine.
//
// +kubebuilder:validation:Enum=fail;allow
type FailurePolicy string

const (
	// FailurePolicyFail blocks traffic when the Engine is not ready or encounters
	// errors.
	FailurePolicyFail FailurePolicy = "fail"

	// FailurePolicyAllow allows traffic through when the Engine is not ready or
	// encounters errors.
	FailurePolicyAllow FailurePolicy = "allow"
)
