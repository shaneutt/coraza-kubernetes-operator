---
applyTo: "internal/rulesets/cache/**/*.go"
---

- The cache is accessed concurrently by controllers (writers) and the HTTP server (readers). All access must be thread-safe via the existing mutex.
- The latest entry must never be pruned. Verify pruning logic preserves this invariant.
- The cache server API paths (`/rules/<key>` and `/rules/<key>/latest`) are consumed by the WASM plugin. Changing these paths or response formats is a cross-repo breaking change.
