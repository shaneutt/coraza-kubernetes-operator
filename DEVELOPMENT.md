# Development

Guides for developing the Coraza Kubernetes Operator (CKO).

> **Note**: See also: [CONTRIBUTING.md](CONTRIBUTING.md).

# Development Environment

A Kubernetes In Docker (KIND) cluster setup is provided. This will deploy
Istio (to provide a `Gateway`) and deploy the Coraza Kubernetes Operator.

> **Note**: Development and testing can be done on any Kubernetes cluster.

## Setup

Build your current changes:

```bash
make all
```

Create the cluster:

```bash
make cluster.kind
```

This will have built the operator with your current changes and loaded the
operator image into the cluster, and started the operator in the
`coraza-system` namespace.

When you make changes to the controllers and want to test them, you can just
run it again and it will rebuild, load and deploy:

```bash
make cluster.kind
```

When you're done, you can destroy the cluster with:

```bash
make clean.cluster.kind
```

# Testing

Run unit tests:

```bash
make test
```

Run unit tests (with coverage):

```bash
make test.coverage
```

Run the integration tests:

```bash
make test.integration
```

## Integration Test Framework

The `test/framework/` package provides structured integration test utilities.
See [`test/framework/README.md`](test/framework/README.md) for the full API
reference.

A test scenario manages its own namespaces, resources, and port-forwards with
automatic cleanup:

```go
func TestExample(t *testing.T) {
    s := fw.NewScenario(t)
    defer s.Cleanup()

    s.CreateNamespace("my-test")
    s.CreateConfigMap("my-test", "rules", `SecRuleEngine On
SecRule ARGS "@contains attack" "id:1,phase:2,deny,status:403"`)
    s.CreateRuleSet("my-test", "ruleset", []string{"rules"})
    s.CreateGateway("my-test", "gateway")
    s.ExpectGatewayProgrammed("my-test", "gateway")

    s.CreateEngine("my-test", "engine", framework.EngineOpts{
        RuleSetName: "ruleset",
        GatewayName: "gateway",
    })
    s.ExpectEngineReady("my-test", "engine")

    gw := s.ProxyToGateway("my-test", "gateway")
    gw.ExpectBlocked("/?q=attack")
    gw.ExpectAllowed("/?q=safe")
}
```

Example scenarios for the v0.2.0 validation issues live in `test/integration/`:

- `coreruleset_test.go` - CoreRuleSet compatibility (#12)
- `multiple_gateways_test.go` - Multiple Gateways (#13)
- `multi_engine_gateway_test.go` - Multiple Engines + Gateways (#52)
- `reconcile_test.go` - Reconciliation of live RuleSet/ConfigMap mutations

# Releasing

See [RELEASE.md](RELEASE.md).
