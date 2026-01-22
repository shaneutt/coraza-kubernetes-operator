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
	SchemeBuilder.Register(&RuleSet{}, &RuleSetList{})
}

// -----------------------------------------------------------------------------
// RuleSet
// -----------------------------------------------------------------------------

// RuleSet represents a set of Web Application Firewall (WAF) rules.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Instance",type=string,JSONPath=`.spec.instance`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type RuleSet struct {
	metav1.TypeMeta `json:",inline"`

	// ObjectMeta is a standard object metadata.
	//
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// Spec defines the desired state of RuleSet.
	//
	// +required
	Spec RuleSetSpec `json:"spec"`

	// Status defines the observed state of RuleSet.
	//
	// +optional
	Status RuleSetStatus `json:"status,omitzero"`
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
// RuleSetSpec
// -----------------------------------------------------------------------------

// RuleSetSpec defines the desired state of RuleSet.
type RuleSetSpec struct {
	// Instance is the unique identifier for this ruleset in the cache. This is
	// used as the key when storing and retrieving rules from the cache server.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:default=default
	Instance string `json:"instance"`

	// Rules is an ordered list of references to other objects that contain the
	// firewall rules to be compiled into a complete set.
	//
	// Currently, only core/v1 ConfigMap kind is supported for rule sources.
	//
	// +required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=2048
	// +kubebuilder:validation:XValidation:rule="self.all(r, r.kind == 'ConfigMap' && r.apiVersion == 'v1')",message="only core/v1 ConfigMap kind is supported for rule sources"
	// +kubebuilder:validation:XValidation:rule="self.all(r, !has(r.namespace) || r.namespace == '')",message="cross-namespace references are not currently supported"
	Rules []corev1.ObjectReference `json:"rules"`
}

// RuleSetStatus defines the observed state of RuleSet.
type RuleSetStatus struct {
	// Conditions represent the current state of the RuleSet resource.
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
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}
