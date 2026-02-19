# Copilot Custom Instructions

## Project Overview

This is a Kubernetes operator (controller-runtime) that manages two CRDs: **Engine** and **RuleSet** (group `waf.k8s.coraza.io`, version `v1alpha1`). Engine creates Istio `WasmPlugin` resources. RuleSet aggregates SecLang WAF rules from ConfigMaps into an in-memory cache served over HTTP.

The operator is one half of a two-repo system. The other half is a WASM plugin (`coraza-proxy-wasm`) that runs inside Envoy/Istio and polls the operator's cache server for rules. Changes to WasmPlugin config fields (`cache_server_instance`, `cache_server_cluster`, `rule_reload_interval_seconds`) directly affect WASM plugin behavior. Flag any PR that changes these field names or the cache server HTTP API paths (`/rules/<key>`, `/rules/<key>/latest`) as a potential cross-repo breaking change.

## API Stability

- API is `v1alpha1`. New fields are typically backward-compatible only when optional (`omitempty`) and/or safely defaulted, and when validation does not reject previously-valid objects; removals or renames are breaking. Review any API type, defaulting, or validation change for backward compatibility.
- CRD changes must be accompanied by running `make manifests` — generated CRD YAML in `config/crd/bases/` must stay in sync with Go types.
- Deep copy must be regenerated: `make generate`. Check for uncommitted generated file diffs.

## Cross-Namespace References

Cross-namespace references between Engine, RuleSet, and ConfigMap are explicitly disallowed. Any PR that weakens this constraint needs justification.

## Testing Expectations

- Controller changes must have envtest unit tests (`internal/controller/*_test.go`).
- Cache changes must have unit tests (`internal/rulesets/cache/*_test.go`).
- Integration tests run against a Kind cluster with Istio (`test/integration/`).
- `make test` runs unit tests. `make test.integration` runs integration tests. Both must pass.

## Common Pitfalls

- **Status conditions:** Updates must set all three condition types (Ready, Progressing, Degraded). Missing one leaves stale status.
- **Owner references:** Any resource the operator creates must have an owner reference back to the parent CRD for garbage collection.

## Style and Conventions

- Go code must pass `make lint` (golangci-lint, config in `.golangci.yml`).
- Error wrapping: use `fmt.Errorf("context: %w", err)`, not `%v`.
- Logger: use structured logging via `logr`. No `fmt.Println` or `log.Printf`.
- Test assertions: use `testify` (`require` for fatal, `assert` for non-fatal).
