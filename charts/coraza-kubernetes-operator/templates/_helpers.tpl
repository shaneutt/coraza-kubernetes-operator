{{/*
Chart name, truncated to 63 chars.
*/}}
{{- define "coraza-operator.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Fully qualified app name, truncated to 63 chars.
*/}}
{{- define "coraza-operator.fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "coraza-operator.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "coraza-operator.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "coraza-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "coraza-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: coraza-controller-manager
{{- end }}

{{/*
Service account name.
*/}}
{{- define "coraza-operator.serviceAccountName" -}}
{{- include "coraza-operator.fullname" . }}
{{- end }}

{{/*
Manager service FQDN (used in Istio resources and envoy cluster name).
*/}}
{{- define "coraza-operator.serviceFQDN" -}}
{{- printf "%s.%s.svc.cluster.local" (include "coraza-operator.fullname" .) .Release.Namespace }}
{{- end }}
