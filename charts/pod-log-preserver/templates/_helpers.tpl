{{/*
Chart name, optionally overridden.
*/}}
{{- define "pod-log-preserver.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Fully qualified app name. Truncated at 63 chars for the DNS name limit.
*/}}
{{- define "pod-log-preserver.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Chart name and version, for the helm.sh/chart label.
*/}}
{{- define "pod-log-preserver.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Selector labels — the stable identity used by the DaemonSet selector; never
add version or other churn-prone labels here.
*/}}
{{- define "pod-log-preserver.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pod-log-preserver.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Common labels applied to every resource.
*/}}
{{- define "pod-log-preserver.labels" -}}
helm.sh/chart: {{ include "pod-log-preserver.chart" . }}
{{ include "pod-log-preserver.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{/*
Fully qualified image reference. The tag defaults to the chart appVersion when
values.image.tag is empty.
*/}}
{{- define "pod-log-preserver.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}
