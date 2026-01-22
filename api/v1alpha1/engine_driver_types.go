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

// -----------------------------------------------------------------------------
// Engine Driver Types
// -----------------------------------------------------------------------------

// DriverConfig defines the driver configuration for the Engine.
//
// +kubebuilder:validation:XValidation:rule="self.mode == 'wasm' ? has(self.wasm) : true",message="wasm configuration is required when mode is 'wasm'"
// +kubebuilder:validation:XValidation:rule="self.type == 'istio' ? has(self.istio) : true",message="istio configuration is required when type is 'istio'"
type DriverConfig struct {
	// Type specifies which integration driver to use.
	//
	// Currently, only the "istio" driver is supported.
	//
	// +required
	Type EngineDriverType `json:"type"`

	// Mode specifies the mode of operation for the driver, which determines how
	// the Engine is integrated with the target implementation.
	//
	// Currently, only "wasm" mode is supported, indicating that the Engine will
	// be deployed as a WebAssembly (WASM) module.
	//
	// +required
	Mode EngineDriverMode `json:"mode"`

	// Istio contains driver configuration specific to integration with Istio.
	//
	// This field is required when type is "istio".
	//
	// +optional
	Istio *IstioConfig `json:"istio,omitempty"`

	// Wasm contains WASM-specific configuration.
	//
	// This field is required when mode is "wasm".
	//
	// +optional
	Wasm *WasmConfig `json:"wasm,omitempty"`

	// RuleSetCacheServer contains configuration for the ruleset cache server.
	//
	// When omitted, no cache server will be used and no rulesets will be
	// dynamically loaded. This implies that your Engine will be deployed with
	// all rules statically embedded.
	//
	// +optional
	RuleSetCacheServer *RuleSetCacheServerConfig `json:"ruleSetCacheServer,omitempty"`
}

// EngineDriverType specifies which driver will be used to provision the Engine.
//
// Currently, only the "istio" driver is supported.
//
// +kubebuilder:validation:Enum=istio
type EngineDriverType string

const (
	// EngineDriverTypeIstio selects the Istio driver.
	EngineDriverTypeIstio EngineDriverType = "istio"
)

// EngineDriverMode specifies the mode of operation for the driver, which
// determines how the Engine is integrated with the target implementation.
//
// Currently, only "wasm" mode is supported, indicating that the Engine will be
// deployed as a WebAssembly (WASM) module.
//
// +kubebuilder:validation:Enum=wasm
type EngineDriverMode string

const (
	// EngineDriverModeWasm uses WebAssembly (WASM) to provision the Engine
	// with the target implementation type.
	EngineDriverModeWasm EngineDriverMode = "wasm"
)

// -----------------------------------------------------------------------------
// Driver Modes
// -----------------------------------------------------------------------------

// WasmConfig defines the configuration for Engine drivers deploying using
// WebAssembly (WASM).
type WasmConfig struct {
	// Image is the OCI image reference for the Coraza WASM plugin.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:Pattern=`^oci://`
	Image string `json:"image"`
}

// -----------------------------------------------------------------------------
// RuleSet Cache Server Types
// -----------------------------------------------------------------------------

// RuleSetCacheServerConfig defines the configuration for the RuleSet cache server.
type RuleSetCacheServerConfig struct {
	// PollIntervalSeconds specifies how often the WAF should check for
	// configuration updates. The value is specified in seconds.
	//
	// When omitted, this means the user has no opinion and the platform
	// will choose a reasonable default, which is subject to change over time.
	// The current default is 15 seconds.
	//
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	// +kubebuilder:default=15
	// +required
	PollIntervalSeconds int32 `json:"pollIntervalSeconds"`
}
