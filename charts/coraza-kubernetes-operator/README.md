# coraza-kubernetes-operator Helm Chart

Deploys the [Coraza Kubernetes Operator](https://github.com/networking-incubator/coraza-kubernetes-operator) — declarative Web Application Firewall (WAF) support for Kubernetes Gateways.

## Installation

### Default (Kubernetes)

```bash
helm template coraza-kubernetes-operator \
  ./charts/coraza-kubernetes-operator \
  --namespace coraza-system \
  --include-crds
```

### OpenShift

```bash
helm template coraza-kubernetes-operator \
  ./charts/coraza-kubernetes-operator \
  --namespace coraza-system \
  --include-crds \
  -f charts/coraza-kubernetes-operator/examples/openshift-values.yaml
```

When `openshift.enabled=true`, `runAsUser`, `fsGroup`, and `fsGroupChangePolicy` are omitted from the pod security context so OpenShift can inject its own UID via SCCs.

### High Availability (Multi-Zone)

```bash
helm template coraza-kubernetes-operator \
  ./charts/coraza-kubernetes-operator \
  --namespace coraza-system \
  --include-crds \
  -f charts/coraza-kubernetes-operator/examples/ha-values.yaml
```

### High Availability on OpenShift (Multi-Zone)

```bash
helm template coraza-kubernetes-operator \
  ./charts/coraza-kubernetes-operator \
  --namespace coraza-system \
  --include-crds \
  -f charts/coraza-kubernetes-operator/examples/openshift-values.yaml \
  -f charts/coraza-kubernetes-operator/examples/ha-values.yaml
```

## Values

| Key                                                   | Type   | Default                                                   | Description                                                                                                 |
| ----------------------------------------------------- | ------ | --------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `replicas`                                            | int    | `1`                                                       | Number of operator replicas. A PodDisruptionBudget with `minAvailable: 1` is created automatically when > 1 |
| `image.repository`                                    | string | `ghcr.io/networking-incubator/coraza-kubernetes-operator` | Container image repository                                                                                  |
| `image.tag`                                           | string | `dev`                                                     | Container image tag                                                                                         |
| `image.pullPolicy`                                    | string | `IfNotPresent`                                            | Image pull policy                                                                                           |
| `imagePullSecrets`                                    | list   | `[]`                                                      | Image pull secrets                                                                                          |
| `resources.requests.cpu`                              | string | `10m`                                                     | CPU request                                                                                                 |
| `resources.requests.memory`                           | string | `64Mi`                                                    | Memory request                                                                                              |
| `resources.limits.cpu`                                | string | `500m`                                                    | CPU limit                                                                                                   |
| `resources.limits.memory`                             | string | `128Mi`                                                   | Memory limit                                                                                                |
| `metrics.enabled`                                     | bool   | `true`                                                    | Enable the controller-runtime metrics endpoint                                                              |
| `metrics.port`                                        | int    | `8443`                                                    | Metrics endpoint port inside the pod                                                                        |
| `metrics.servicePort`                                 | int    | `8443`                                                    | Service port for metrics                                                                                    |
| `metrics.secure`                                      | bool   | `true`                                                    | Serve metrics over HTTPS (self-signed by default)                                                           |
| `metrics.serviceMonitor.enabled`                      | bool   | `false`                                                   | Create a ServiceMonitor resource                                                                            |
| `metrics.serviceMonitor.scheme`                       | string | `""`                                                      | Override scrape scheme (`http` or `https`). Empty means auto based on `metrics.secure`                      |
| `metrics.serviceMonitor.interval`                     | string | `30s`                                                     | Scrape interval                                                                                             |
| `metrics.serviceMonitor.scrapeTimeout`                | string | `10s`                                                     | Scrape timeout                                                                                              |
| `metrics.serviceMonitor.honorLabels`                  | bool   | `false`                                                   | Honor Prometheus labels                                                                                     |
| `metrics.serviceMonitor.labels`                       | object | `{}`                                                      | Extra labels for the ServiceMonitor                                                                         |
| `metrics.serviceMonitor.tlsConfig.insecureSkipVerify` | bool   | `true`                                                    | Skip TLS verification for self-signed certs                                                                 |
| `resizePolicy`                                        | list   | `[{cpu: NotRequired}, {memory: NotRequired}]`             | In-place vertical scaling restart policy                                                                    |
| `istio.revision`                                      | string | `coraza`                                                  | Istio control plane revision label                                                                          |
| `openshift.enabled`                                   | bool   | `false`                                                   | Omit UID/fsGroup from pod security context for OpenShift SCC compatibility                                  |
| `nodeSelector`                                        | object | `{}`                                                      | Node selector constraints                                                                                   |
| `tolerations`                                         | list   | `[]`                                                      | Tolerations                                                                                                 |
| `affinity`                                            | object | `{}`                                                      | Affinity rules                                                                                              |
| `topologySpreadConstraints`                           | list   | `[]`                                                      | Topology spread constraints for pod scheduling                                                              |
