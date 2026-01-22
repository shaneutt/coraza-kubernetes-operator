# Coraza Kubernetes Operator

Automated deployment and life-cycle management of [Coraza] on [Kubernetes].

[Coraza]:https://github.com/corazawaf
[Kubernetes]:https://github.com/kubernetes

## About

The Coraza Kubernetes Operator enables declarative management of Coraza Web
Application Firewall (WAF) policies in Kubernetes. It integrates the Coraza WAF
engine with gateway/proxy solutions to enforce rules for Kubernetes cluster
traffic.

**Key Features:**

- `Engine` API to declaratively deploy WAF instances
- `RuleSet` API to declaratively provide rules to WAF instances
- Dynamic `RuleSet` updates
- ModSec compatibility

### Supported Integrations

The operator integrates with other tools to attach WAF instances to
their gateways/proxies:

- `istio` - Istio integration ✅ **Currently Supported (ingress Gateway only)**
- `wasm` - WebAssembly deployment ✅ **Currently Supported**

> **Note**: Only Istio+Wasm is supported for now.

## Usage

Make sure your supported platform is deployed to the cluster, then choose one
of the installation methods.

> **Note**: For deploying Istio, we recommend the [Sail Operator].

[Sail Operator]:https://github.com/istio-ecosystem/sail-operator/

### Installation

#### Install with Kustomize

```bash
kubectl apply -k config/default
```

#### Install with Helm

TODO

### Firewall Deployment

Firstly deploy your `RuleSets` which organize all your rules.

> **Note**: Only `ConfigMaps` are supported for rules currently.

Once your `RuleSets` are deployed you can deploy an `Engine` to load and
enforce those rules on a `Gateway`.

> **Note**: Currently can only target an Istio `Gateway` resource.

You can find examples of `RuleSets` and `Engines` in `config/samples/`. The
documentation for these APIs is available in the [API Documentation](todo).

## Contributing

Contributions welcome!

See [Development](DEVELOPMENT.md).

For bugs and smaller issues, feel free to create an issue and/or PRs.

Please create a [GitHub Discussion] for any questions, or if you have a large
feature you want to propose so we can discuss it together first.

[GitHub Discussion]:https://github.com/shaneutt/coraza-kubernetes-operator/discussions

## License

Apache License 2.0 - see [LICENSE](LICENSE).
