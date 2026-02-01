{{/*
Expand the name of the chart.
*/}}
{{- define "fish-fry-orders.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "fish-fry-orders.fullname" -}}
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
Backend fullname
*/}}
{{- define "fish-fry-orders.backend.fullname" -}}
{{ include "fish-fry-orders.fullname" . }}-api
{{- end }}

{{/*
Frontend fullname
*/}}
{{- define "fish-fry-orders.frontend.fullname" -}}
{{ include "fish-fry-orders.fullname" . }}-frontend
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "fish-fry-orders.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "fish-fry-orders.labels" -}}
helm.sh/chart: {{ include "fish-fry-orders.chart" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Backend selector labels
*/}}
{{- define "fish-fry-orders.backend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "fish-fry-orders.name" . }}-api
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: api
{{- end }}

{{/*
Frontend selector labels
*/}}
{{- define "fish-fry-orders.frontend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "fish-fry-orders.name" . }}-frontend
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: frontend
{{- end }}
