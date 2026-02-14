![CI](https://github.com/networking-incubator/coraza-kubernetes-operator/actions/workflows/ci.yml/badge.svg)
![RELEASE](https://img.shields.io/github/v/release/networking-incubator/coraza-kubernetes-operator?include_prereleases)
# Coraza Kubernetes Operator

[Web Application Firewall (WAF)] support for [Kubernetes] [Gateways].

[Web Application Firewall (WAF)]:https://www.cloudflare.com/learning/ddos/glossary/web-application-firewall-waf/
[Kubernetes]:https://github.com/kubernetes
[Gateways]:https://gateway-api.sigs.k8s.io/api-types/gateway/

## About

The Coraza Kubernetes Operator (CKO) enables declarative management of Web
Application Firewalls (WAF) on Kubernetes clusters. Users can deploy
firewall engines which are attached to gateways, and rules which those
engines enforce.

[CorazaWAF] is used as the firewall engine.

[CorazaWAF]:https://github.com/corazawaf/coraza

**Key Features:**

- `Engine` API - declaratively manage WAF instances
- `RuleSet` API - declaratively manage firewall rules
- [ModSecurity Seclang] compatibility

[ModSecurity Seclang]:https://github.com/owasp-modsecurity/ModSecurity/wiki/Reference-Manual-(v3.x)

### Supported Integrations

The operator integrates with other tools to attach WAF instances to
their gateways/proxies:

- `istio` - Istio integration ✅ **Currently Supported (ingress Gateway only)**
- `wasm` - WebAssembly deployment ✅ **Currently Supported**

> **Note**: Only Istio+Wasm is supported currently.

### Architecture

The CKO's `RuleSetController` responds to `RuleSet` resources by validating and
compiling the rules, which gets emitted to a cache. The keys for the cache are
the namespace/name of the `RuleSet`, allowing the compiled set of rules to be
polled from a cache server hosting the cache.

> **Note**: Currently, only [Seclang] rules are supported.

The `EngineController` responds to `Engine` resources by deploying a WAF engine
according to the type and mode provided, and attaching it to a `Gateway`. It
targets a `RuleSet` to poll the compiled ruleset from the cache server and apply
it to the `Engine`. Poll intervals are set so the rules can be dynamically
updated over time.

<img width="825" height="460" alt="cko-architecture-diagram" src="https://github.com/user-attachments/assets/e7b257e3-096f-4321-a40d-fe4e473480ac" />

[Seclang]:https://github.com/owasp-modsecurity/ModSecurity/wiki/Reference-Manual-(v3.x)

## Documentation

Documentation is available in the [wiki].

[wiki]:https://github.com/networking-incubator/coraza-kubernetes-operator/wiki

## Contributing

Contributions are welcome!

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache License 2.0 - see [LICENSE](LICENSE).
