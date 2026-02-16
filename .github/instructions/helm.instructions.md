---
applyTo: "charts/**"
---

- CRDs in `crds/` must be identical to `config/crd/bases/`. Run `make helm.sync-crds` after any API type change and verify the copied files match.
- RBAC rules in the ClusterRole template must stay in sync with `config/rbac/role.yaml`. Changes to controller RBAC markers (`kubebuilder:rbac`) require updating the chart.
- All configurable values must appear in `values.yaml` with sensible defaults. Helpers that reference `.Values.*` keys not present in `values.yaml` should be avoided.
- Istio-specific resources (ServiceEntry, DestinationRule) must be gated behind `{{ if .Values.istio.enabled }}`.
- PDB defaults must be safe for the default `replicaCount`. A PDB with `minAvailable >= replicaCount` blocks voluntary evictions.
