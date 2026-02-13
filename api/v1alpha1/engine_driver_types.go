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

type DriverType string

const (
	DriverTypeIstio DriverType = "Istio"
)

// -----------------------------------------------------------------------------
// Engine - Driver Config
// -----------------------------------------------------------------------------

// DriverConfig defines the driver configuration for the Engine.
//
// Exactly one driver must be specified.
//
// +kubebuilder:validation:XValidation:rule="[has(self.istio)].filter(x, x).size() == 1",message="exactly one driver must be specified"
type DriverConfig struct {
	// driver defines what is the driver type. Only the matching driver configuration
	// should be used
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +required
	Driver DriverType `json:"driver,omitempty,omitzero"`
	// istio configures the Engine to integrate with Istio service mesh.
	//
	// +kubebuilder:validation:MinProperties=0
	// +optional
	Istio *IstioDriverConfig `json:"istio,omitempty,omitzero"`
}
