---
applyTo: "api/**/*.go"
---

- Any change to types in this directory affects the CRD schema. Verify that `make manifests` and `make generate` have been run and the results committed.
- Check for CEL validation markers (kubebuilder comments). New fields should have appropriate validation.
- Enum fields must use `+kubebuilder:validation:Enum=` markers.
- Default values must use `+kubebuilder:default=` markers.
- Required fields must be documented as such in both the Go doc comment and the kubebuilder marker.
- The `zz_generated.deepcopy.go` file must be regenerated when types change. It should never be hand-edited.
