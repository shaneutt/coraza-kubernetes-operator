---
applyTo: "test/framework/**/*.go"
---

- Resource creation methods belong on `Scenario` and must register cleanup via `s.OnCleanup`. Cleanup functions must use `context.Background()` so they run even when the test context is cancelled.
- Assertion methods use `require.Eventually` with `DefaultTimeout` (60s) and `DefaultInterval` (2s). Do not introduce custom timeouts without justification.
- Builder functions (`Build*`) return `*unstructured.Unstructured`. The framework uses the dynamic client, not typed clients, to avoid importing the operator's Go types.
- Traffic assertion methods belong on `GatewayProxy`. New HTTP helpers should follow the polling pattern of `ExpectBlocked`/`ExpectAllowed`.
- GVR constants go in `resources.go`. New CRD types the framework interacts with need a GVR added there.
- Port allocation uses `Framework.AllocatePort()` (atomic counter from 29000). Do not hardcode ports.
