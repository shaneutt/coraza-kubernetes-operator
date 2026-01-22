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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// -----------------------------------------------------------------------------
// Engine Driver - Istio Types
// -----------------------------------------------------------------------------

// IstioConfig defines Istio-specific configuration options for the Engine.
type IstioConfig struct {
	// WorkloadSelector specifies the selection criteria for attaching the WAF to
	// Istio resources.
	//
	// +required
	WorkloadSelector metav1.LabelSelector `json:"workloadSelector"`

	// Mode specifies what mechanism will be used to integrate the WAF with
	// Istio.
	//
	// Currently only supports "Gateway" mode, utilizing Gateway API resources.
	//
	// +required
	Mode IstioIntegrationMode `json:"mode"`
}

// IstioIntegrationMode specifies what mechanism will be used to integrate the WAF with
// Istio.
//
// Currently only supports "gateway" mode, utilizing Gateway API resources.
//
// +kubebuilder:validation:Enum=gateway
type IstioIntegrationMode string

const (
	// IstioIntegrationModeGateway applies the filter at the Gateway level.
	IstioIntegrationModeGateway IstioIntegrationMode = "gateway"
)
