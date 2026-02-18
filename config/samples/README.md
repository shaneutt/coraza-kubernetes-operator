# Sample: Coraza WAF with Istio Gateway

This sample deploys a Coraza WAF Engine in front of a simple echo service
using the Kubernetes Gateway API and Istio.

## What's included

| File | Description |
|------|-------------|
| `ruleset.yaml` | ConfigMaps with SecRule directives (base config, SQLi, XSS, custom) and a `RuleSet` CR that aggregates them |
| `engine.yaml` | `Engine` CR that references the RuleSet and configures the Istio WASM driver |
| `gateway.yaml` | Kubernetes Gateway API `Gateway` using the Istio gateway class |
| `httproute.yaml` | `HTTPRoute` that sends all traffic through the gateway to the echo service |
| `echo.yaml` | A simple echo Deployment and Service to act as the backend |

## Prerequisites

- A Kubernetes cluster with Istio installed
- The coraza-kubernetes-operator running in the cluster
- The Kubernetes Gateway API CRDs installed

## Deploy

```bash
kubectl apply -f config/samples/
```

## Test

Port-forward to the gateway:

```bash
kubectl port-forward svc/coraza-gateway-istio 8080:80
```

Normal request (should return "hello from echo"):

```bash
curl http://localhost:8080/
```

SQLi attempt (should be blocked by rule 1001):

```bash
curl "http://localhost:8080/?q=select+*+from+users"
```

XSS attempt (should be blocked by rule 2001):

```bash
curl "http://localhost:8080/?q=<script>alert(1)</script>"
```

Evil monkey (should be blocked by rule 3001):

```bash
curl "http://localhost:8080/?q=evilmonkey"
```

## Cleanup

```bash
kubectl delete -f config/samples/
```
