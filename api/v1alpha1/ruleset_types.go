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
// RuleSet - Schema Registration
// -----------------------------------------------------------------------------

func init() {
	SchemeBuilder.Register(&RuleSet{}, &RuleSetList{})
}

// -----------------------------------------------------------------------------
// RuleSet
// -----------------------------------------------------------------------------

// RuleSet represents a set of Web Application Firewall (WAF) rules.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type RuleSet struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	//
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of RuleSet.
	//
	// +required
	Spec RuleSetSpec `json:"spec,omitzero"`

	// status defines the observed state of RuleSet.
	//
	// +optional
	Status RuleSetStatus `json:"status,omitempty,omitzero"`
}

// RuleSetList contains a list of RuleSet resources.
//
// +kubebuilder:object:root=true
type RuleSetList struct {
	metav1.TypeMeta `json:",inline"`

	// ListMeta is standard list metadata.
	//
	// +optional
	metav1.ListMeta `json:"metadata,omitzero"`

	// Items is the list of RuleSets.
	//
	// +required
	Items []RuleSet `json:"items"`
}

// -----------------------------------------------------------------------------
// RuleSet - Spec
// -----------------------------------------------------------------------------

// RuleSetSpec defines the desired state of RuleSet.
type RuleSetSpec struct {
	// rules is an ordered list of references to other objects that contain the
	// firewall rules to be compiled into a complete set.
	//
	// Currently, only core/v1 ConfigMap kind is supported for rule sources.
	//
	// +required
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=2048
	// +kubebuilder:validation:XValidation:rule="self.all(r, r.kind == 'ConfigMap' && r.apiVersion == 'v1')",message="only core/v1 ConfigMap kind is supported for rule sources"
	// +kubebuilder:validation:XValidation:rule="self.all(r, !has(r.namespace) || r.namespace == '')",message="cross-namespace references are not currently supported"
	Rules []corev1.ObjectReference `json:"rules,omitempty"`
}

// -----------------------------------------------------------------------------
// RuleSet - Status
// -----------------------------------------------------------------------------

// RuleSetStatus defines the observed state of RuleSet.
// +kubebuilder:validation:MinProperties=1
type RuleSetStatus struct {
	// conditions represent the current state of the RuleSet resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Ready": the RuleSet has been processed and and the rules have been cached
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	//
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MinItems=0
	// +kubebuilder:validation:MaxItems=8
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// -----------------------------------------------------------------------------
// RuleSet - Cache Server Types
// -----------------------------------------------------------------------------

// RuleSetCacheServerConfig defines the configuration for the RuleSet cache server.
// +kubebuilder:validation:MinProperties=0
type RuleSetCacheServerConfig struct {
	// pollIntervalSeconds specifies how often the WAF should check for
	// configuration updates. The value is specified in seconds.
	//
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	// +required
	PollIntervalSeconds int32 `json:"pollIntervalSeconds,omitempty"`
}
