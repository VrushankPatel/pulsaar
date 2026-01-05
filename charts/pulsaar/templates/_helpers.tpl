{{/*
Expand the name of the chart.
*/}}
{{- define "pulsaar.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "pulsaar.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "pulsaar.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "pulsaar.labels" -}}
helm.sh/chart: {{ include "pulsaar.chart" . }}
{{ include "pulsaar.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "pulsaar.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pulsaar.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "pulsaar.serviceAccountName" -}}
{{- if .Values.webhook.serviceAccount.create }}
{{- default (include "pulsaar.fullname" .) .Values.webhook.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.webhook.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Webhook CA bundle
*/}}
{{- define "pulsaar.webhook.caBundle" -}}
{{- if .Values.webhook.tls.certManager.enabled }}
{{- $secret := lookup "v1" "Secret" .Release.Namespace .Values.webhook.tls.secretName }}
{{- if $secret }}
{{- $secret.data.ca\.crt | b64dec }}
{{- end }}
{{- else }}
{{- .Values.webhook.tls.caBundle | default "" }}
{{- end }}
{{- end }}