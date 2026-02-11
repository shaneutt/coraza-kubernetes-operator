---
applyTo: "internal/controller/**/*.go"
---

- Every reconciliation path must end with a status update. Check that all exit paths (error and success) set appropriate conditions (Ready, Progressing, Degraded).
- Requeue decisions matter: returning an error requeues with backoff; returning `ctrl.Result{RequeueAfter: ...}` requeues on a timer. Verify the right strategy is used.
- Watch predicates control what triggers reconciliation. Changes to predicates can cause missed events or infinite loops.
- RBAC markers (`kubebuilder:rbac`) must match the resources the controller actually touches. New resource types need new RBAC markers and a `make manifests` rerun.
- Owner references on created resources (e.g. WasmPlugin) are required for garbage collection. Any new resource the operator creates must have an owner reference back to the parent CRD.
- The Engine controller passes config fields to the WasmPlugin that the WASM plugin (separate repo) consumes. Changing field names or semantics is a cross-repo breaking change.
